package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"lastsaas/internal/api/handlers"
	"lastsaas/internal/auth"
	"lastsaas/internal/config"
	"lastsaas/internal/configstore"
	"lastsaas/internal/db"
	"lastsaas/internal/email"
	"lastsaas/internal/events"
	"lastsaas/internal/health"
	"lastsaas/internal/metrics"
	"lastsaas/internal/middleware"
	"lastsaas/internal/models"
	"lastsaas/internal/planstore"
	stripeservice "lastsaas/internal/stripe"
	"lastsaas/internal/syslog"
	"lastsaas/internal/version"
	"lastsaas/internal/webhooks"

	"github.com/gorilla/mux"
	"github.com/rs/cors"
)

// spaHandler serves a single-page application from a static directory.
// For files that exist on disk, it serves them directly. For all other
// paths it serves index.html so the SPA router can handle them.
type spaHandler struct {
	staticPath string
	indexPath  string
}

func (h spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Never serve the SPA for /api/ paths — those should be handled by API routes
	if strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/api" {
		http.NotFound(w, r)
		return
	}

	path := filepath.Join(h.staticPath, filepath.Clean(r.URL.Path))

	fi, err := os.Stat(path)
	if os.IsNotExist(err) || (err == nil && fi.IsDir()) {
		// Serve index.html with no-store so browsers always fetch the latest version
		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
		http.ServeFile(w, r, filepath.Join(h.staticPath, h.indexPath))
		return
	}
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	http.FileServer(http.Dir(h.staticPath)).ServeHTTP(w, r)
}

func main() {
	config.LoadEnvFile()

	// Load config
	env := config.GetEnv()
	cfg, err := config.Load(env)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	log.Printf("Starting LastSaaS in %s mode", cfg.Environment)

	// Connect to MongoDB
	database, err := db.NewMongoDB(cfg.Database.URI, cfg.Database.Name)
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		database.Close(ctx)
	}()
	log.Println("Connected to MongoDB")

	// Load and check version
	version.Load()
	version.CheckAndMigrate(database)

	// Seed and load configuration store
	if err := configstore.Seed(context.Background(), database); err != nil {
		log.Fatalf("Failed to seed config variables: %v", err)
	}
	cfgStore := configstore.New(database)
	if err := cfgStore.Load(context.Background()); err != nil {
		log.Fatalf("Failed to load config store: %v", err)
	}
	log.Println("Configuration store loaded")

	// Seed plans
	if err := planstore.Seed(context.Background(), database); err != nil {
		log.Fatalf("Failed to seed plans: %v", err)
	}

	// Initialize system logger
	sysLogger := syslog.New(database, cfgStore.Get)
	sysLogger.Critical(context.Background(), fmt.Sprintf("System startup: LastSaaS v%s", version.Current))

	// Initialize services
	jwtService := auth.NewJWTService(
		cfg.JWT.AccessSecret,
		cfg.JWT.RefreshSecret,
		cfg.JWT.AccessTTLMin,
		cfg.JWT.RefreshTTLDay,
	)
	passwordService := auth.NewPasswordService()

	var googleOAuth *auth.GoogleOAuthService
	if cfg.OAuth.GoogleClientID != "" && cfg.OAuth.GoogleClientSecret != "" {
		googleOAuth = auth.NewGoogleOAuthService(
			cfg.OAuth.GoogleClientID,
			cfg.OAuth.GoogleClientSecret,
			cfg.OAuth.GoogleRedirectURL,
		)
		log.Println("Google OAuth configured")
	} else {
		log.Println("Google OAuth not configured (missing credentials)")
	}

	var githubOAuth *auth.GitHubOAuthService
	if cfg.OAuth.GitHubClientID != "" && cfg.OAuth.GitHubClientSecret != "" {
		githubOAuth = auth.NewGitHubOAuthService(
			cfg.OAuth.GitHubClientID,
			cfg.OAuth.GitHubClientSecret,
			cfg.OAuth.GitHubRedirectURL,
		)
		log.Println("GitHub OAuth configured")
	} else {
		log.Println("GitHub OAuth not configured (missing credentials)")
	}

	var microsoftOAuth *auth.MicrosoftOAuthService
	if cfg.OAuth.MicrosoftClientID != "" && cfg.OAuth.MicrosoftClientSecret != "" {
		microsoftOAuth = auth.NewMicrosoftOAuthService(
			cfg.OAuth.MicrosoftClientID,
			cfg.OAuth.MicrosoftClientSecret,
			cfg.OAuth.MicrosoftRedirectURL,
		)
		log.Println("Microsoft OAuth configured")
	} else {
		log.Println("Microsoft OAuth not configured (missing credentials)")
	}

	var emailService *email.ResendService
	if cfg.Email.ResendAPIKey != "" {
		emailService = email.NewResendService(
			cfg.Email.ResendAPIKey,
			cfg.Email.FromEmail,
			cfg.Email.FromName,
			cfg.App.Name,
			cfg.Frontend.URL,
			cfgStore.Get,
		)
		log.Println("Email service configured (Resend)")
	} else {
		log.Println("Email service not configured (missing Resend API key)")
	}

	// Initialize Stripe service (optional — billing works without it)
	var stripeSvc *stripeservice.Service
	if cfg.Stripe.SecretKey != "" {
		stripeSvc = stripeservice.New(
			cfg.Stripe.SecretKey,
			cfg.Stripe.PublishableKey,
			cfg.Stripe.WebhookSecret,
			database,
			cfg.Frontend.URL,
		)
		log.Println("Stripe billing configured")
	} else {
		log.Println("Stripe billing not configured (missing secret key)")
	}

	webhookDispatcher := webhooks.NewDispatcher(database)
	var emitter events.Emitter = webhookDispatcher

	// Initialize middleware
	authMiddleware := middleware.NewAuthMiddleware(jwtService, database)
	tenantMiddleware := middleware.NewTenantMiddleware(database)
	rateLimiter := middleware.NewRateLimiter()
	defer rateLimiter.Stop()
	metricsCollector := middleware.NewMetricsCollector()

	// Initialize health monitoring
	healthService := health.New(database, metricsCollector, cfgStore.Get)

	// Register integration health checks
	healthService.RegisterIntegration("mongodb", health.NewMongoChecker(database.Client))
	if cfg.Stripe.SecretKey != "" {
		healthService.RegisterIntegration("stripe", health.NewStripeChecker())
	} else {
		healthService.RegisterIntegration("stripe", nil)
	}
	if cfg.Email.ResendAPIKey != "" {
		healthService.RegisterIntegration("resend", health.NewResendChecker(cfg.Email.ResendAPIKey))
	} else {
		healthService.RegisterIntegration("resend", nil)
	}
	if cfg.OAuth.GoogleClientID != "" && cfg.OAuth.GoogleClientSecret != "" {
		healthService.RegisterIntegration("google_oauth", health.NewGoogleOAuthChecker())
	} else {
		healthService.RegisterIntegration("google_oauth", nil)
	}
	if cfg.OAuth.GitHubClientID != "" && cfg.OAuth.GitHubClientSecret != "" {
		healthService.RegisterIntegration("github_oauth", health.NewGitHubOAuthChecker())
	} else {
		healthService.RegisterIntegration("github_oauth", nil)
	}
	if cfg.OAuth.MicrosoftClientID != "" && cfg.OAuth.MicrosoftClientSecret != "" {
		healthService.RegisterIntegration("microsoft_oauth", health.NewMicrosoftOAuthChecker())
	} else {
		healthService.RegisterIntegration("microsoft_oauth", nil)
	}
	if cfgStore.Get("auth.passkeys.enabled") == "true" {
		healthService.RegisterIntegration("webauthn", health.NewWebAuthnChecker())
	} else {
		healthService.RegisterIntegration("webauthn", nil)
	}
	if cfgStore.Get("auth.sso.enabled") == "true" {
		healthService.RegisterIntegration("saml_sso", health.NewSAMLChecker())
	} else {
		healthService.RegisterIntegration("saml_sso", nil)
	}

	healthService.Start()
	defer healthService.Stop()

	// Initialize handlers
	bootstrapHandler := handlers.NewBootstrapHandler(database)
	authHandler := handlers.NewAuthHandler(database, jwtService, passwordService, googleOAuth, emailService, emitter, cfg.Frontend.URL, sysLogger)
	authHandler.SetGetConfig(cfgStore.Get)
	if githubOAuth != nil {
		authHandler.SetGitHubOAuth(githubOAuth)
	}
	if microsoftOAuth != nil {
		authHandler.SetMicrosoftOAuth(microsoftOAuth)
	}
	tenantHandler := handlers.NewTenantHandler(database, emailService, emitter, sysLogger)
	if stripeSvc != nil {
		tenantHandler.SetStripe(stripeSvc)
	}
	adminHandler := handlers.NewAdminHandler(database, emitter, sysLogger)
	adminHandler.SetHealthService(healthService, cfgStore.Get)
	adminHandler.SetJWTService(jwtService)
	messageHandler := handlers.NewMessageHandler(database)
	logHandler := handlers.NewLogHandler(database)
	configHandler := handlers.NewConfigHandler(database, cfgStore, sysLogger)
	plansHandler := handlers.NewPlansHandler(database, sysLogger, cfgStore, stripeSvc)
	bundlesHandler := handlers.NewBundlesHandler(database, sysLogger)
	healthHandler := handlers.NewHealthHandler(healthService)
	billingHandler := handlers.NewBillingHandler(stripeSvc, database, emitter, sysLogger)
	webhookHandler := handlers.NewWebhookHandler(stripeSvc, database, emitter, sysLogger, cfgStore.Get)
	apiKeysHandler := handlers.NewAPIKeysHandler(database, emitter, sysLogger)
	webhooksHandler := handlers.NewWebhooksHandler(database, sysLogger, webhookDispatcher)
	brandingHandler := handlers.NewBrandingHandler(database, cfgStore, sysLogger)
	brandingHandler.SetAuthProviders(map[string]bool{
		"google":    googleOAuth != nil,
		"github":    githubOAuth != nil,
		"microsoft": microsoftOAuth != nil,
	})

	// Initialize daily metrics service
	metricsService := metrics.New(database)
	metricsService.Start()
	defer metricsService.Stop()

	// Setup router
	router := mux.NewRouter()

	// Health check
	router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}).Methods("GET")

	api := router.PathPrefix("/api").Subrouter()

	// --- Bootstrap status (always accessible, init is CLI-only) ---
	api.HandleFunc("/bootstrap/status", bootstrapHandler.Status).Methods("GET")

	// API documentation (public, no auth)
	api.HandleFunc("/docs", handlers.DocsHTML).Methods("GET")
	api.HandleFunc("/docs/markdown", handlers.DocsMarkdown).Methods("GET")

	// --- Public branding routes (no auth, no bootstrap guard) ---
	api.HandleFunc("/branding", brandingHandler.GetBranding).Methods("GET")
	api.HandleFunc("/branding/asset/{key}", brandingHandler.ServeAsset).Methods("GET")
	api.HandleFunc("/branding/media/{id}", brandingHandler.ServeMedia).Methods("GET")
	api.HandleFunc("/branding/page/{slug}", brandingHandler.GetPublicPage).Methods("GET")
	api.HandleFunc("/branding/pages", brandingHandler.ListPublicPages).Methods("GET")

	// --- Guarded routes (require system to be initialized) ---
	guarded := api.PathPrefix("").Subrouter()
	guarded.Use(bootstrapHandler.BootstrapGuard)

	// Public auth routes
	guarded.HandleFunc("/auth/register", rateLimiter.RateLimitHandler(
		middleware.AccountCreationLimit,
		func(r *http.Request) string { return middleware.GetClientIP(r) },
		authHandler.Register,
	)).Methods("POST")

	guarded.HandleFunc("/auth/login", rateLimiter.RateLimitHandler(
		middleware.LoginAttemptLimit,
		func(r *http.Request) string { return middleware.GetClientIP(r) },
		authHandler.Login,
	)).Methods("POST")

	guarded.HandleFunc("/auth/refresh", rateLimiter.RateLimitHandler(
		middleware.TokenRefreshLimit,
		func(r *http.Request) string { return middleware.GetClientIP(r) },
		authHandler.Refresh,
	)).Methods("POST")

	guarded.HandleFunc("/auth/verify-email", rateLimiter.RateLimitHandler(
		middleware.EmailVerificationLimit,
		func(r *http.Request) string { return middleware.GetClientIP(r) },
		authHandler.VerifyEmail,
	)).Methods("POST")

	guarded.HandleFunc("/auth/resend-verification", rateLimiter.RateLimitHandler(
		middleware.ResendVerificationLimit,
		func(r *http.Request) string { return middleware.GetClientIP(r) },
		authHandler.ResendVerification,
	)).Methods("POST")

	guarded.HandleFunc("/auth/forgot-password", rateLimiter.RateLimitHandler(
		middleware.PasswordResetLimit,
		func(r *http.Request) string { return middleware.GetClientIP(r) },
		authHandler.ForgotPassword,
	)).Methods("POST")

	guarded.HandleFunc("/auth/reset-password", authHandler.ResetPassword).Methods("POST")

	// Auth providers discovery (public)
	guarded.HandleFunc("/auth/providers", authHandler.GetProviders).Methods("GET")

	// MFA challenge (public — uses special mfa token)
	guarded.HandleFunc("/auth/mfa/challenge", rateLimiter.RateLimitHandler(
		middleware.MFAChallengeLimit,
		func(r *http.Request) string { return middleware.GetClientIP(r) },
		authHandler.MFAChallenge,
	)).Methods("POST")

	// Magic link (public)
	guarded.HandleFunc("/auth/magic-link", rateLimiter.RateLimitHandler(
		middleware.MagicLinkLimit,
		func(r *http.Request) string { return middleware.GetClientIP(r) },
		authHandler.MagicLinkRequest,
	)).Methods("POST")
	guarded.HandleFunc("/auth/magic-link/verify", authHandler.MagicLinkVerify).Methods("POST")

	// Google OAuth routes
	if googleOAuth != nil {
		guarded.HandleFunc("/auth/google", rateLimiter.RateLimitHandler(
			middleware.OAuthInitLimit,
			func(r *http.Request) string { return middleware.GetClientIP(r) },
			authHandler.GoogleOAuth,
		)).Methods("GET")
		guarded.HandleFunc("/auth/google/callback", authHandler.GoogleOAuthCallback).Methods("GET")
	}

	// GitHub OAuth routes
	if githubOAuth != nil {
		guarded.HandleFunc("/auth/github", rateLimiter.RateLimitHandler(
			middleware.OAuthInitLimit,
			func(r *http.Request) string { return middleware.GetClientIP(r) },
			authHandler.GitHubOAuth,
		)).Methods("GET")
		guarded.HandleFunc("/auth/github/callback", authHandler.GitHubOAuthCallback).Methods("GET")
	}

	// Microsoft OAuth routes
	if microsoftOAuth != nil {
		guarded.HandleFunc("/auth/microsoft", rateLimiter.RateLimitHandler(
			middleware.OAuthInitLimit,
			func(r *http.Request) string { return middleware.GetClientIP(r) },
			authHandler.MicrosoftOAuth,
		)).Methods("GET")
		guarded.HandleFunc("/auth/microsoft/callback", authHandler.MicrosoftOAuthCallback).Methods("GET")
	}

	// Protected auth routes (require JWT)
	protectedAuth := guarded.PathPrefix("/auth").Subrouter()
	protectedAuth.Use(authMiddleware.RequireAuth)
	protectedAuth.HandleFunc("/me", authHandler.GetMe).Methods("GET")
	protectedAuth.HandleFunc("/logout", authHandler.Logout).Methods("POST")
	protectedAuth.HandleFunc("/change-password", authHandler.ChangePassword).Methods("POST")
	protectedAuth.HandleFunc("/accept-invitation", authHandler.AcceptInvitation).Methods("POST")
	protectedAuth.HandleFunc("/mfa/setup", authHandler.MFASetup).Methods("POST")
	protectedAuth.HandleFunc("/mfa/verify-setup", authHandler.MFAVerifySetup).Methods("POST")
	protectedAuth.HandleFunc("/mfa/disable", authHandler.MFADisable).Methods("POST")
	protectedAuth.HandleFunc("/mfa/regenerate-codes", authHandler.MFARegenerateRecoveryCodes).Methods("POST")
	protectedAuth.HandleFunc("/sessions", authHandler.ListSessions).Methods("GET")
	protectedAuth.HandleFunc("/sessions/{id}", authHandler.RevokeSession).Methods("DELETE")
	protectedAuth.HandleFunc("/sessions", authHandler.RevokeAllSessions).Methods("DELETE")
	protectedAuth.HandleFunc("/preferences", authHandler.UpdatePreferences).Methods("PATCH")
	protectedAuth.HandleFunc("/complete-onboarding", authHandler.CompleteOnboarding).Methods("POST")

	// Tenant-scoped routes (require JWT + tenant context)
	tenantAPI := guarded.PathPrefix("/tenant").Subrouter()
	tenantAPI.Use(authMiddleware.RequireAuth)
	tenantAPI.Use(tenantMiddleware.RequireTenant)

	tenantAPI.HandleFunc("/members", tenantHandler.ListMembers).Methods("GET")
	tenantAPI.HandleFunc("/activity", tenantHandler.GetActivity).Methods("GET")

	// Tenant settings (owner only)
	tenantSettingsRouter := tenantAPI.PathPrefix("/settings").Subrouter()
	tenantSettingsRouter.Use(middleware.RequireRole(models.RoleOwner))
	tenantSettingsRouter.HandleFunc("", tenantHandler.UpdateTenantSettings).Methods("PATCH")

	// Invite requires admin+
	inviteRouter := tenantAPI.PathPrefix("/members/invite").Subrouter()
	inviteRouter.Use(middleware.RequireRole(models.RoleAdmin))
	inviteRouter.HandleFunc("", rateLimiter.RateLimitHandler(
		middleware.InvitationLimit,
		func(r *http.Request) string { return middleware.GetClientIP(r) },
		tenantHandler.InviteMember,
	)).Methods("POST")

	// Remove requires admin+
	removeRouter := tenantAPI.PathPrefix("/members/{userId}").Subrouter()
	removeRouter.Use(middleware.RequireRole(models.RoleAdmin))
	removeRouter.HandleFunc("", tenantHandler.RemoveMember).Methods("DELETE")

	// Role change + ownership transfer require owner
	ownerRouter := tenantAPI.PathPrefix("/members/{userId}").Subrouter()
	ownerRouter.Use(middleware.RequireRole(models.RoleOwner))
	ownerRouter.HandleFunc("/role", tenantHandler.ChangeRole).Methods("PATCH")
	ownerRouter.HandleFunc("/transfer-ownership", tenantHandler.TransferOwnership).Methods("POST")

	// Message routes (require JWT, user-scoped)
	messageAPI := guarded.PathPrefix("/messages").Subrouter()
	messageAPI.Use(authMiddleware.RequireAuth)
	messageAPI.HandleFunc("", messageHandler.ListMessages).Methods("GET")
	messageAPI.HandleFunc("/unread-count", messageHandler.UnreadCount).Methods("GET")
	messageAPI.HandleFunc("/{messageId}/read", messageHandler.MarkRead).Methods("PATCH")

	// Public plans route (require JWT, not admin)
	guarded.Handle("/plans", authMiddleware.RequireAuth(http.HandlerFunc(plansHandler.ListPlansPublic))).Methods("GET")

	// Public credit bundles route (require JWT, not admin)
	guarded.Handle("/credit-bundles", authMiddleware.RequireAuth(http.HandlerFunc(bundlesHandler.ListBundlesPublic))).Methods("GET")

	// Webhook route (no auth — uses Stripe signature verification)
	api.HandleFunc("/billing/webhook", webhookHandler.HandleWebhook).Methods("POST")

	// Billing routes (require JWT + tenant)
	billingAPI := guarded.PathPrefix("/billing").Subrouter()
	billingAPI.Use(authMiddleware.RequireAuth)
	billingAPI.Use(tenantMiddleware.RequireTenant)
	billingAPI.HandleFunc("/checkout", billingHandler.Checkout).Methods("POST")
	billingAPI.HandleFunc("/portal", billingHandler.Portal).Methods("POST")
	billingAPI.HandleFunc("/transactions", billingHandler.ListTransactions).Methods("GET")
	billingAPI.HandleFunc("/transactions/{id}/invoice", billingHandler.GetInvoice).Methods("GET")
	billingAPI.HandleFunc("/transactions/{id}/invoice/pdf", billingHandler.GetInvoicePDF).Methods("GET")
	billingAPI.HandleFunc("/cancel", billingHandler.CancelSubscription).Methods("POST")
	billingAPI.HandleFunc("/config", billingHandler.GetConfig).Methods("GET")

	// Admin routes (require JWT + root tenant + admin+ role)
	adminAPI := guarded.PathPrefix("/admin").Subrouter()
	adminAPI.Use(authMiddleware.RequireAuth)
	adminAPI.Use(tenantMiddleware.RequireTenant)
	adminAPI.Use(middleware.RequireRootTenant())
	adminAPI.Use(middleware.RequireRole(models.RoleAdmin))

	adminAPI.HandleFunc("/about", adminHandler.GetAbout).Methods("GET")
	adminAPI.HandleFunc("/dashboard", adminHandler.GetDashboard).Methods("GET")
	adminAPI.HandleFunc("/logs", logHandler.ListLogs).Methods("GET")
	adminAPI.HandleFunc("/config", configHandler.ListConfig).Methods("GET")
	adminAPI.HandleFunc("/config", configHandler.CreateConfig).Methods("POST")
	adminAPI.HandleFunc("/config/{name}", configHandler.GetConfig).Methods("GET")
	adminAPI.HandleFunc("/config/{name}", configHandler.UpdateConfig).Methods("PUT")
	adminAPI.HandleFunc("/config/{name}", configHandler.DeleteConfig).Methods("DELETE")
	adminAPI.HandleFunc("/tenants", adminHandler.ListTenants).Methods("GET")
	adminAPI.HandleFunc("/tenants/{tenantId}", adminHandler.GetTenant).Methods("GET")
	adminAPI.HandleFunc("/plans", plansHandler.ListPlans).Methods("GET")
	adminAPI.HandleFunc("/plans/{planId}", plansHandler.GetPlan).Methods("GET")
	adminAPI.HandleFunc("/entitlement-keys", plansHandler.ListEntitlementKeys).Methods("GET")
	adminAPI.HandleFunc("/credit-bundles", bundlesHandler.ListBundles).Methods("GET")
	adminAPI.HandleFunc("/health/nodes", healthHandler.ListNodes).Methods("GET")
	adminAPI.HandleFunc("/health/metrics", healthHandler.GetMetrics).Methods("GET")
	adminAPI.HandleFunc("/health/current", healthHandler.GetCurrent).Methods("GET")
	adminAPI.HandleFunc("/health/integrations", healthHandler.GetIntegrations).Methods("GET")
	adminAPI.HandleFunc("/financial/transactions", billingHandler.AdminListTransactions).Methods("GET")
	adminAPI.HandleFunc("/financial/metrics", billingHandler.AdminGetMetrics).Methods("GET")
	adminAPI.HandleFunc("/api-keys", apiKeysHandler.ListAPIKeys).Methods("GET")
	adminAPI.HandleFunc("/api-keys", apiKeysHandler.CreateAPIKey).Methods("POST")
	adminAPI.HandleFunc("/api-keys/{keyId}", apiKeysHandler.DeleteAPIKey).Methods("DELETE")
	adminAPI.HandleFunc("/webhooks", webhooksHandler.ListWebhooks).Methods("GET")
	adminAPI.HandleFunc("/webhooks/event-types", webhooksHandler.ListEventTypes).Methods("GET")
	adminAPI.HandleFunc("/webhooks", webhooksHandler.CreateWebhook).Methods("POST")
	adminAPI.HandleFunc("/webhooks/{webhookId}", webhooksHandler.GetWebhook).Methods("GET")
	adminAPI.HandleFunc("/webhooks/{webhookId}", webhooksHandler.UpdateWebhook).Methods("PUT")
	adminAPI.HandleFunc("/webhooks/{webhookId}", webhooksHandler.DeleteWebhook).Methods("DELETE")
	adminAPI.HandleFunc("/webhooks/{webhookId}/test", webhooksHandler.TestWebhook).Methods("POST")
	adminAPI.HandleFunc("/webhooks/{webhookId}/regenerate-secret", webhooksHandler.RegenerateSecret).Methods("POST")

	// Owner-only admin actions
	adminOwner := adminAPI.PathPrefix("").Subrouter()
	adminOwner.Use(middleware.RequireRole(models.RoleOwner))
	adminOwner.HandleFunc("/tenants/{tenantId}", adminHandler.UpdateTenant).Methods("PUT")
	adminOwner.HandleFunc("/tenants/{tenantId}/status", adminHandler.UpdateTenantStatus).Methods("PATCH")
	adminOwner.HandleFunc("/users", adminHandler.ListUsers).Methods("GET")
	adminOwner.HandleFunc("/users/{userId}", adminHandler.GetUser).Methods("GET")
	adminOwner.HandleFunc("/users/{userId}", adminHandler.UpdateUser).Methods("PUT")
	adminOwner.HandleFunc("/users/{userId}/status", adminHandler.UpdateUserStatus).Methods("PATCH")
	adminOwner.HandleFunc("/users/{userId}/role/{tenantId}", adminHandler.UpdateUserRole).Methods("PATCH")
	adminOwner.HandleFunc("/users/{userId}/preflight-delete", adminHandler.PreflightDeleteUser).Methods("GET")
	adminOwner.HandleFunc("/users/{userId}/impersonate", adminHandler.ImpersonateUser).Methods("POST")
	adminOwner.HandleFunc("/users/{userId}", adminHandler.DeleteUser).Methods("DELETE")
	adminOwner.HandleFunc("/plans", plansHandler.CreatePlan).Methods("POST")
	adminOwner.HandleFunc("/plans/{planId}", plansHandler.UpdatePlan).Methods("PUT")
	adminOwner.HandleFunc("/plans/{planId}", plansHandler.DeletePlan).Methods("DELETE")
	adminOwner.HandleFunc("/plans/{planId}/archive", plansHandler.ArchivePlan).Methods("POST")
	adminOwner.HandleFunc("/plans/{planId}/unarchive", plansHandler.UnarchivePlan).Methods("POST")
	adminOwner.HandleFunc("/tenants/{tenantId}/plan", plansHandler.AssignPlan).Methods("PATCH")
	adminOwner.HandleFunc("/credit-bundles", bundlesHandler.CreateBundle).Methods("POST")
	adminOwner.HandleFunc("/credit-bundles/{bundleId}", bundlesHandler.UpdateBundle).Methods("PUT")
	adminOwner.HandleFunc("/credit-bundles/{bundleId}", bundlesHandler.DeleteBundle).Methods("DELETE")
	adminOwner.HandleFunc("/tenants/{tenantId}/cancel-subscription", billingHandler.AdminCancelSubscription).Methods("POST")
	adminOwner.HandleFunc("/tenants/{tenantId}/subscription", billingHandler.AdminUpdateSubscription).Methods("PATCH")
	adminOwner.HandleFunc("/branding", brandingHandler.UpdateBranding).Methods("PUT")
	adminOwner.HandleFunc("/branding/asset", brandingHandler.UploadAsset).Methods("POST")
	adminOwner.HandleFunc("/branding/asset/{key}", brandingHandler.DeleteAsset).Methods("DELETE")
	adminOwner.HandleFunc("/branding/media", brandingHandler.ListMedia).Methods("GET")
	adminOwner.HandleFunc("/branding/media", brandingHandler.UploadMedia).Methods("POST")
	adminOwner.HandleFunc("/branding/media/{id}", brandingHandler.DeleteMedia).Methods("DELETE")
	adminOwner.HandleFunc("/branding/pages", brandingHandler.AdminListPages).Methods("GET")
	adminOwner.HandleFunc("/branding/pages", brandingHandler.CreatePage).Methods("POST")
	adminOwner.HandleFunc("/branding/pages/{id}", brandingHandler.UpdatePage).Methods("PUT")
	adminOwner.HandleFunc("/branding/pages/{id}", brandingHandler.DeletePage).Methods("DELETE")

	// Serve frontend static files in production
	if cfg.Frontend.StaticDir != "" {
		log.Printf("Serving frontend from %s", cfg.Frontend.StaticDir)
		spa := spaHandler{staticPath: cfg.Frontend.StaticDir, indexPath: "index.html"}
		router.PathPrefix("/").Handler(spa)
	}

	// CORS
	c := cors.New(cors.Options{
		AllowedOrigins:   []string{cfg.Frontend.URL},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type", "X-Tenant-ID"},
		AllowCredentials: true,
		MaxAge:           86400,
	})

	// Wrap with security headers + CORS + metrics
	handler := middleware.SecurityHeaders(c.Handler(metricsCollector.Middleware(router)))

	// Start server
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	go func() {
		log.Printf("Server listening on %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced shutdown: %v", err)
	}
	log.Println("Server stopped")
}
