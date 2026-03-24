package main

import (
	"context"
	"fmt"
	"log/slog"
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
	"lastsaas/internal/datadog"
	"lastsaas/internal/db"
	"lastsaas/internal/email"
	"lastsaas/internal/events"
	"lastsaas/internal/health"
	"lastsaas/internal/metrics"
	"lastsaas/internal/rescan"
	"lastsaas/internal/middleware"
	"lastsaas/internal/models"
	"lastsaas/internal/planstore"
	"lastsaas/internal/scanner"
	stripeservice "lastsaas/internal/stripe"
	"lastsaas/internal/syslog"
	"lastsaas/internal/telemetry"
	"lastsaas/internal/version"
	"lastsaas/internal/webhooks"

	"github.com/gorilla/mux"
	"github.com/rs/cors"
	"go.mongodb.org/mongo-driver/bson"
)

// spaHandler serves a single-page application from a static directory.
// For files that exist on disk, it serves them directly. For all other
// paths it serves index.html so the SPA router can handle them.
// When serving index.html, it replaces {{APP_NAME}} with the actual app name
// to prevent a title flicker while JavaScript loads branding data.
type spaHandler struct {
	staticPath string
	indexPath  string
	getAppName func() string
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
		// Serve index.html with no-store so browsers always fetch the latest version.
		// Replace the {{APP_NAME}} placeholder with the real app name so the
		// browser tab shows the correct title immediately, before JS loads.
		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
		indexFile := filepath.Join(h.staticPath, h.indexPath)
		data, readErr := os.ReadFile(indexFile)
		if readErr != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		appName := "App"
		if h.getAppName != nil {
			if name := h.getAppName(); name != "" {
				appName = name
			}
		}
		html := strings.Replace(string(data), "{{APP_NAME}}", appName, 1)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(html))
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
		slog.Error("Failed to load config", "error", err)
		os.Exit(1)
	}
	slog.Info("Starting MCPLens", "mode", cfg.Environment)

	// Connect to MongoDB
	database, err := db.NewMongoDB(cfg.Database.URI, cfg.Database.Name)
	if err != nil {
		slog.Error("Failed to connect to MongoDB", "error", err)
		os.Exit(1)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := database.Close(ctx); err != nil {
			slog.Error("Failed to close database connection", "error", err)
		}
	}()
	slog.Info("Connected to MongoDB")

	// Load and check version
	version.Load()
	version.CheckAndMigrate(database)

	// Seed and load configuration store
	if err := configstore.Seed(context.Background(), database); err != nil {
		slog.Error("Failed to seed config variables", "error", err)
		os.Exit(1)
	}
	cfgStore := configstore.New(database)
	if err := cfgStore.Load(context.Background()); err != nil {
		slog.Error("Failed to load config store", "error", err)
		os.Exit(1)
	}
	appCtx, appCancel := context.WithCancel(context.Background())
	defer appCancel()
	cfgStore.StartAutoReload(appCtx, 60*time.Second)
	slog.Info("Configuration store loaded", "reloadInterval", "60s")

	// Seed plans
	if err := planstore.Seed(context.Background(), database); err != nil {
		slog.Error("Failed to seed plans", "error", err)
		os.Exit(1)
	}

	// Initialize system logger
	sysLogger := syslog.New(database, cfgStore.Get)

	// Initialize DataDog integration (optional)
	var ddClient *datadog.Client
	if cfg.DataDog.APIKey != "" {
		site := cfg.DataDog.Site
		if site == "" {
			site = "us5.datadoghq.com"
		}
		ddClient = datadog.New(cfg.DataDog.APIKey, site, cfg.Environment, cfg.App.Name, cfg.DataDog.Hostname)
		defer ddClient.Stop()
		sysLogger.SetOnLog(ddClient.TrackSyslogEntry)

		// Verify end-to-end: validate key, send startup event + heartbeat metric
		if err := ddClient.Startup(context.Background(), version.Current); err != nil {
			slog.Warn("DataDog startup verification failed (integration will retry in background)", "error", err)
		}
	} else {
		slog.Warn("DataDog integration not configured", "reason", "missing API key")
	}

	sysLogger.Critical(context.Background(), fmt.Sprintf("System startup: MCPLens v%s", version.Current))

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
		slog.Info("Google OAuth configured")
	} else {
		slog.Warn("Google OAuth not configured", "reason", "missing credentials")
	}

	var githubOAuth *auth.GitHubOAuthService
	if cfg.OAuth.GitHubClientID != "" && cfg.OAuth.GitHubClientSecret != "" {
		githubOAuth = auth.NewGitHubOAuthService(
			cfg.OAuth.GitHubClientID,
			cfg.OAuth.GitHubClientSecret,
			cfg.OAuth.GitHubRedirectURL,
		)
		slog.Info("GitHub OAuth configured")
	} else {
		slog.Warn("GitHub OAuth not configured", "reason", "missing credentials")
	}

	var microsoftOAuth *auth.MicrosoftOAuthService
	if cfg.OAuth.MicrosoftClientID != "" && cfg.OAuth.MicrosoftClientSecret != "" {
		microsoftOAuth = auth.NewMicrosoftOAuthService(
			cfg.OAuth.MicrosoftClientID,
			cfg.OAuth.MicrosoftClientSecret,
			cfg.OAuth.MicrosoftRedirectURL,
		)
		slog.Info("Microsoft OAuth configured")
	} else {
		slog.Warn("Microsoft OAuth not configured", "reason", "missing credentials")
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
		slog.Info("Email service configured", "provider", "Resend")
	} else {
		slog.Warn("Email service not configured", "reason", "missing Resend API key")
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
		slog.Info("Stripe billing configured")
	} else {
		slog.Warn("Stripe billing not configured", "reason", "missing secret key")
	}

	webhookEncKey, err := webhooks.ParseEncryptionKey(cfg.Webhooks.EncryptionKey)
	if err != nil {
		slog.Error("Invalid webhook encryption key", "error", err)
		os.Exit(1)
	}
	if webhookEncKey != nil {
		slog.Info("Webhook secret encryption enabled")
	}
	webhookDispatcher := webhooks.NewDispatcher(database, webhookEncKey)
	defer webhookDispatcher.Stop()
	var emitter events.Emitter = webhookDispatcher

	// Initialize middleware
	authMiddleware := middleware.NewAuthMiddleware(jwtService, database)
	tenantMiddleware := middleware.NewTenantMiddleware(database)
	rateLimiter := middleware.NewDistributedRateLimiter(database.Database)
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
	if ddClient != nil {
		healthService.RegisterIntegration("datadog", health.NewDataDogChecker(ddClient))
	} else {
		healthService.RegisterIntegration("datadog", nil)
	}

	if ddClient != nil {
		healthService.SetOnHealthSnapshot(ddClient.TrackHealthSnapshot)
		healthService.SetOnIntegrationCheck(ddClient.TrackIntegrationChecks)
	}

	healthService.Start()
	defer healthService.Stop()

	// Initialize telemetry service
	telemetrySvc := telemetry.New(database)
	defer telemetrySvc.Stop()
	if ddClient != nil {
		telemetrySvc.SetOnTrack(ddClient.TrackTelemetryEvent)
	}

	// Initialize handlers
	bootstrapHandler := handlers.NewBootstrapHandler(database)
	authHandler := handlers.NewAuthHandler(database, jwtService, passwordService, googleOAuth, emailService, emitter, cfg.Frontend.URL, sysLogger)
	authHandler.SetGetConfig(cfgStore.Get)
	authHandler.SetRateLimiter(rateLimiter)
	if webhookEncKey != nil {
		authHandler.SetTOTPEncryptionKey(webhookEncKey)
	}
	authHandler.SetTelemetry(telemetrySvc)
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
	adminHandler.SetEmailService(emailService)
	messageHandler := handlers.NewMessageHandler(database)
	logHandler := handlers.NewLogHandler(database)
	configHandler := handlers.NewConfigHandler(database, cfgStore, sysLogger)
	plansHandler := handlers.NewPlansHandler(database, sysLogger, cfgStore, stripeSvc)
	bundlesHandler := handlers.NewBundlesHandler(database, sysLogger)
	healthHandler := handlers.NewHealthHandler(healthService)
	healthHandler.SetEmailService(emailService)
	billingHandler := handlers.NewBillingHandler(stripeSvc, database, emitter, sysLogger, cfgStore)
	billingHandler.SetTelemetry(telemetrySvc)
	promotionsHandler := handlers.NewPromotionsHandler(database, stripeSvc, cfgStore)
	webhookHandler := handlers.NewWebhookHandler(stripeSvc, database, emitter, sysLogger, cfgStore.Get)
	webhookHandler.SetTelemetry(telemetrySvc)
	pmHandler := handlers.NewPMHandler(database, telemetrySvc, sysLogger)
	eventDefsHandler := handlers.NewEventDefinitionsHandler(database, sysLogger)
	telemetryHandler := handlers.NewTelemetryHandler(telemetrySvc)
	apiKeysHandler := handlers.NewAPIKeysHandler(database, emitter, sysLogger)
	webhooksHandler := handlers.NewWebhooksHandler(database, sysLogger, webhookDispatcher)
	brandingHandler := handlers.NewBrandingHandler(database, cfgStore, sysLogger)
	announcementsHandler := handlers.NewAnnouncementsHandler(database, sysLogger)
	usageHandler := handlers.NewUsageHandler(database)
	scannerSvc := scanner.NewService(database.Database)
	webhookHandler.SetScanner(scannerSvc)
	scannerHandler := handlers.NewScannerHandler(scannerSvc)
	scannerHandler.SetDB(database)
	ogHandler := handlers.NewOGHandler(scannerSvc)
	apiV1Handler := handlers.NewAPIV1Handler(scannerSvc)
	agencyBrandingHandler := handlers.NewAgencyBrandingHandler(database, scannerSvc)
	leadsHandler := handlers.NewLeadsHandler(database)
	brandingHandler.SetAuthProviders(map[string]bool{
		"google":    googleOAuth != nil,
		"github":    githubOAuth != nil,
		"microsoft": microsoftOAuth != nil,
	})

	// Initialize daily metrics service
	metricsService := metrics.New(database)
	metricsService.Start()
	defer metricsService.Stop()

	// Initialize automated rescan service (scheduled rescans + score drop alerts)
	rescanService := rescan.New(database, scannerSvc, emailService, sysLogger)
	rescanService.Start()
	defer rescanService.Stop()

	// Setup router
	router := mux.NewRouter()

	// Health check — pings DB to verify actual readiness
	router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		w.Header().Set("Content-Type", "application/json")
		if err := database.Client.Ping(ctx, nil); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"status":"unhealthy","error":"database unreachable"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}).Methods("GET")

	api := router.PathPrefix("/api").Subrouter()
	api.Use(middleware.RequestID)
	api.Use(middleware.APIVersion)
	// Note: BodySizeLimit is applied at the outermost handler layer — not here to avoid double-wrapping

	// Version endpoint (public, no auth)
	api.HandleFunc("/version", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(fmt.Sprintf(`{"version":%q}`, version.Current)))
	}).Methods("GET")

	// --- Bootstrap status (always accessible, init is CLI-only) ---
	api.HandleFunc("/bootstrap/status", bootstrapHandler.Status).Methods("GET")

	// API documentation (public, no auth)
	api.HandleFunc("/docs", handlers.DocsHTML).Methods("GET")
	api.HandleFunc("/docs/markdown", handlers.DocsMarkdown).Methods("GET")
	api.HandleFunc("/docs/openapi.json", handlers.DocsOpenAPI).Methods("GET")

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

	guarded.HandleFunc("/auth/reset-password", rateLimiter.RateLimitHandler(
		middleware.ResetTokenVerifyLimit,
		func(r *http.Request) string { return middleware.GetClientIP(r) },
		authHandler.ResetPassword,
	)).Methods("POST")

	// OAuth code exchange (public — exchanges one-time code for tokens)
	guarded.HandleFunc("/auth/exchange-code", authHandler.ExchangeCode).Methods("POST")

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
	guarded.HandleFunc("/auth/magic-link/verify", rateLimiter.RateLimitHandler(
		middleware.MagicLinkVerifyLimit,
		func(r *http.Request) string { return middleware.GetClientIP(r) },
		authHandler.MagicLinkVerify,
	)).Methods("POST")

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
	protectedAuth.HandleFunc("/delete-account", authHandler.DeleteAccount).Methods("POST")
	protectedAuth.HandleFunc("/export-data", authHandler.ExportData).Methods("GET")

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

	// Public announcements route (require JWT)
	guarded.Handle("/announcements", authMiddleware.RequireAuth(http.HandlerFunc(announcementsHandler.ListPublic))).Methods("GET")

	// Usage metering routes (require JWT + tenant + active billing)
	usageAPI := guarded.PathPrefix("/usage").Subrouter()
	usageAPI.Use(authMiddleware.RequireAuth)
	usageAPI.Use(tenantMiddleware.RequireTenant)
	usageAPI.Use(middleware.RequireActiveBilling())
	usageAPI.HandleFunc("/record", rateLimiter.RateLimitHandler(
		middleware.UsageRecordLimit,
		func(r *http.Request) string { return "usage:" + middleware.GetClientIP(r) },
		usageHandler.RecordUsage,
	)).Methods("POST")
	usageAPI.HandleFunc("/summary", usageHandler.GetSummary).Methods("GET")

	// Anonymous telemetry route (rate-limited by IP, no auth)
	guarded.HandleFunc("/telemetry/track", rateLimiter.RateLimitHandler(
		middleware.TelemetryAnonymousLimit,
		func(r *http.Request) string { return "telemetry:" + middleware.GetClientIP(r) },
		telemetryHandler.TrackAnonymous,
	)).Methods("POST")

	// Authenticated telemetry routes (require JWT + tenant)
	telemetryAPI := guarded.PathPrefix("/telemetry").Subrouter()
	telemetryAPI.Use(authMiddleware.RequireAuth)
	telemetryAPI.Use(tenantMiddleware.RequireTenant)
	telemetryAPI.HandleFunc("/events", rateLimiter.RateLimitHandler(
		middleware.TelemetryAuthenticatedLimit,
		func(r *http.Request) string {
			user, _ := middleware.GetUserFromContext(r.Context())
			if user != nil {
				return "telemetry:" + user.ID.Hex()
			}
			return "telemetry:" + middleware.GetClientIP(r)
		},
		telemetryHandler.TrackAuthenticated,
	)).Methods("POST")
	telemetryAPI.HandleFunc("/events/batch", rateLimiter.RateLimitHandler(
		middleware.TelemetryAuthenticatedLimit,
		func(r *http.Request) string {
			user, _ := middleware.GetUserFromContext(r.Context())
			if user != nil {
				return "telemetry:" + user.ID.Hex()
			}
			return "telemetry:" + middleware.GetClientIP(r)
		},
		telemetryHandler.TrackBatch,
	)).Methods("POST")

	// --- Public API v1 routes (API key auth via Bearer lsk_...) ---
	v1API := guarded.PathPrefix("/v1").Subrouter()
	v1API.Use(authMiddleware.RequireAuth)
	v1API.HandleFunc("/scan", apiV1Handler.TriggerScan).Methods("POST")
	v1API.HandleFunc("/scan/{id}", apiV1Handler.GetScan).Methods("GET")
	v1API.HandleFunc("/stores/{domain}/latest", apiV1Handler.GetLatestStoreScan).Methods("GET")

	// --- Agency branding routes (auth + tenant required) ---
	agencyBrandingAPI := guarded.PathPrefix("/branding/agency").Subrouter()
	agencyBrandingAPI.Use(authMiddleware.RequireAuth)
	agencyBrandingAPI.Use(tenantMiddleware.RequireTenant)
	agencyBrandingAPI.HandleFunc("", agencyBrandingHandler.GetAgencyBranding).Methods("GET")
	agencyBrandingAPI.HandleFunc("", agencyBrandingHandler.UpdateAgencyBranding).Methods("PUT")

	// Scan report download (public; branding applied when ?branding=true + tenant auth)
	api.HandleFunc("/scan/{id}/report", agencyBrandingHandler.GetScanReport).Methods("GET")

	// --- Scanner routes ---
	// POST /api/scan — public, rate-limited (5/hour unauthenticated)
	// NOTE: scans can take up to 60 seconds; the server WriteTimeout should be raised
	// accordingly (see srv configuration below).
	api.HandleFunc("/scan", rateLimiter.RateLimitHandler(
		middleware.ScanPublicLimit,
		func(r *http.Request) string { return "scan:" + middleware.GetClientIP(r) },
		scannerHandler.TriggerScan,
	)).Methods("POST")

	// GET /api/scan/domain/{domain} — public, must be registered before /scan/{id}
	api.HandleFunc("/scan/domain/{domain}", scannerHandler.GetLatestDomainScan).Methods("GET")

	// GET /api/scan/{id} — public
	api.HandleFunc("/scan/{id}", scannerHandler.GetScan).Methods("GET")

	// GET /api/og/{domain} — public, returns SVG OG image
	api.HandleFunc("/og/{domain}", ogHandler.OGImage).Methods("GET")

	// GET /api/badge/{domain} — public, returns shields.io-compatible JSON
	api.HandleFunc("/badge/{domain}", ogHandler.Badge).Methods("GET")

	// --- Lead capture routes (public, rate-limited) ---
	api.HandleFunc("/leads", rateLimiter.RateLimitHandler(
		middleware.ScanPublicLimit,
		func(r *http.Request) string { return "leads:" + middleware.GetClientIP(r) },
		leadsHandler.CaptureLead,
	)).Methods("POST")
	api.HandleFunc("/leads/verify", leadsHandler.VerifyToken).Methods("GET")

	// GET /api/scans — requires auth + tenant (paid feature)
	scansAPI := guarded.PathPrefix("/scans").Subrouter()
	scansAPI.Use(authMiddleware.RequireAuth)
	scansAPI.Use(tenantMiddleware.RequireTenant)
	scansAPI.HandleFunc("", scannerHandler.ListScans).Methods("GET")

	// Tracked stores — requires auth + tenant (paid feature, entitlement-gated)
	trackedStoresAPI := guarded.PathPrefix("/tracked-stores").Subrouter()
	trackedStoresAPI.Use(authMiddleware.RequireAuth)
	trackedStoresAPI.Use(tenantMiddleware.RequireTenant)
	trackedStoresAPI.HandleFunc("", scannerHandler.ListTrackedStores).Methods("GET")
	trackedStoresAPI.HandleFunc("", scannerHandler.AddTrackedStore).Methods("POST")
	trackedStoresAPI.HandleFunc("/comparison", scannerHandler.GetTrackedStoresComparison).Methods("GET")
	trackedStoresAPI.HandleFunc("/{id}", scannerHandler.RemoveTrackedStore).Methods("DELETE")
	trackedStoresAPI.HandleFunc("/{id}/history", scannerHandler.GetTrackedStoreHistory).Methods("GET")
	trackedStoresAPI.HandleFunc("/{id}/label", scannerHandler.UpdateTrackedStoreLabel).Methods("PATCH")

	// Webhook route (no auth — uses Stripe signature verification)
	api.HandleFunc("/billing/webhook", webhookHandler.HandleWebhook).Methods("POST")

	// Billing routes (require JWT + tenant)
	billingAPI := guarded.PathPrefix("/billing").Subrouter()
	billingAPI.Use(authMiddleware.RequireAuth)
	billingAPI.Use(tenantMiddleware.RequireTenant)
	billingAPI.HandleFunc("/transactions", billingHandler.ListTransactions).Methods("GET")
	billingAPI.HandleFunc("/transactions/{id}/invoice", billingHandler.GetInvoice).Methods("GET")
	billingAPI.HandleFunc("/transactions/{id}/invoice/pdf", billingHandler.GetInvoicePDF).Methods("GET")
	billingAPI.HandleFunc("/config", billingHandler.GetConfig).Methods("GET")
	billingAPI.HandleFunc("/scan-purchase", billingHandler.PurchaseScan).Methods("POST")

	// Billing actions that modify the subscription (owner only)
	billingOwner := billingAPI.PathPrefix("").Subrouter()
	billingOwner.Use(middleware.RequireRole(models.RoleOwner))
	billingOwner.HandleFunc("/checkout", billingHandler.Checkout).Methods("POST")
	billingOwner.HandleFunc("/portal", billingHandler.Portal).Methods("POST")
	billingOwner.HandleFunc("/cancel", billingHandler.CancelSubscription).Methods("POST")

	// Admin routes — three tiers:
	//   adminAPI   = root tenant + user role  (read-only access to all admin data)
	//   adminWrite = root tenant + admin role (read-write: manage users, tenants, config, etc.)
	//   adminOwner = root tenant + owner role (destructive/sensitive: delete users, impersonate, branding, billing)
	adminAPI := guarded.PathPrefix("/admin").Subrouter()
	adminAPI.Use(authMiddleware.RequireAuth)
	adminAPI.Use(tenantMiddleware.RequireTenant)
	adminAPI.Use(middleware.RequireRootTenant())
	adminAPI.Use(middleware.RequireRole(models.RoleUser))

	// Read-only routes (user+ role)
	adminAPI.HandleFunc("/about", adminHandler.GetAbout).Methods("GET")
	adminAPI.HandleFunc("/dashboard", adminHandler.GetDashboard).Methods("GET")
	adminAPI.HandleFunc("/logs", logHandler.ListLogs).Methods("GET")
	adminAPI.HandleFunc("/logs/severity-counts", logHandler.SeverityCounts).Methods("GET")
	adminAPI.HandleFunc("/logs/export", rateLimiter.RateLimitHandler(
		middleware.CSVExportLimit,
		func(r *http.Request) string { return middleware.GetClientIP(r) },
		logHandler.ExportCSV,
	)).Methods("GET")
	adminAPI.HandleFunc("/config", configHandler.ListConfig).Methods("GET")
	adminAPI.HandleFunc("/config/{name}", configHandler.GetConfig).Methods("GET")
	adminAPI.HandleFunc("/tenants", adminHandler.ListTenants).Methods("GET")
	adminAPI.HandleFunc("/tenants/export", rateLimiter.RateLimitHandler(
		middleware.CSVExportLimit,
		func(r *http.Request) string { return middleware.GetClientIP(r) },
		adminHandler.ExportTenantsCSV,
	)).Methods("GET")
	adminAPI.HandleFunc("/tenants/{tenantId}", adminHandler.GetTenant).Methods("GET")
	adminAPI.HandleFunc("/plans", plansHandler.ListPlans).Methods("GET")
	adminAPI.HandleFunc("/plans/{planId}", plansHandler.GetPlan).Methods("GET")
	adminAPI.HandleFunc("/entitlement-keys", plansHandler.ListEntitlementKeys).Methods("GET")
	adminAPI.HandleFunc("/credit-bundles", bundlesHandler.ListBundles).Methods("GET")
	adminAPI.HandleFunc("/health/nodes", healthHandler.ListNodes).Methods("GET")
	adminAPI.HandleFunc("/health/metrics", healthHandler.GetMetrics).Methods("GET")
	adminAPI.HandleFunc("/health/current", healthHandler.GetCurrent).Methods("GET")
	adminAPI.HandleFunc("/health/integrations", healthHandler.GetIntegrations).Methods("GET")
	adminAPI.HandleFunc("/health/test-email", healthHandler.SendTestEmail).Methods("POST")
	adminAPI.HandleFunc("/promotions", promotionsHandler.ListPromotions).Methods("GET")
	adminAPI.HandleFunc("/promotions/eligible-products", promotionsHandler.ListEligibleProducts).Methods("GET")
	adminAPI.HandleFunc("/announcements", announcementsHandler.ListAll).Methods("GET")
	adminAPI.HandleFunc("/financial/transactions", billingHandler.AdminListTransactions).Methods("GET")
	adminAPI.HandleFunc("/financial/metrics", billingHandler.AdminGetMetrics).Methods("GET")
	adminAPI.HandleFunc("/api-keys", apiKeysHandler.ListAPIKeys).Methods("GET")
	adminAPI.HandleFunc("/members", adminHandler.ListRootMembers).Methods("GET")
	adminAPI.HandleFunc("/users", adminHandler.ListUsers).Methods("GET")
	adminAPI.HandleFunc("/users/export", rateLimiter.RateLimitHandler(
		middleware.CSVExportLimit,
		func(r *http.Request) string { return middleware.GetClientIP(r) },
		adminHandler.ExportUsersCSV,
	)).Methods("GET")
	adminAPI.HandleFunc("/users/{userId}", adminHandler.GetUser).Methods("GET")
	adminAPI.HandleFunc("/webhooks", webhooksHandler.ListWebhooks).Methods("GET")
	adminAPI.HandleFunc("/webhooks/event-types", webhooksHandler.ListEventTypes).Methods("GET")
	adminAPI.HandleFunc("/webhooks/{webhookId}", webhooksHandler.GetWebhook).Methods("GET")
	adminAPI.HandleFunc("/branding/media", brandingHandler.ListMedia).Methods("GET")
	adminAPI.HandleFunc("/branding/pages", brandingHandler.AdminListPages).Methods("GET")
	adminAPI.HandleFunc("/pm/funnel", pmHandler.GetFunnel).Methods("GET")
	adminAPI.HandleFunc("/pm/retention", pmHandler.GetRetention).Methods("GET")
	adminAPI.HandleFunc("/pm/engagement", pmHandler.GetEngagement).Methods("GET")
	adminAPI.HandleFunc("/pm/kpis", pmHandler.GetKPIs).Methods("GET")
	adminAPI.HandleFunc("/pm/events", pmHandler.GetCustomEvents).Methods("GET")
	adminAPI.HandleFunc("/pm/events/types", pmHandler.ListEventTypes).Methods("GET")
	adminAPI.HandleFunc("/pm/event-definitions", eventDefsHandler.ListEventDefinitions).Methods("GET")
	adminAPI.HandleFunc("/pm/event-definitions/sankey", eventDefsHandler.GetSankeyData).Methods("GET")

	// Admin-level write routes (admin+ role)
	adminWrite := adminAPI.PathPrefix("").Subrouter()
	adminWrite.Use(middleware.RequireRole(models.RoleAdmin))
	adminWrite.HandleFunc("/config", configHandler.CreateConfig).Methods("POST")
	adminWrite.HandleFunc("/config/{name}", configHandler.UpdateConfig).Methods("PUT")
	adminWrite.HandleFunc("/config/{name}", configHandler.DeleteConfig).Methods("DELETE")
	adminWrite.HandleFunc("/users/{userId}", adminHandler.UpdateUser).Methods("PUT")
	adminWrite.HandleFunc("/users/{userId}/status", adminHandler.UpdateUserStatus).Methods("PATCH")
	adminWrite.HandleFunc("/users/{userId}/role/{tenantId}", adminHandler.UpdateUserRole).Methods("PATCH")
	adminWrite.HandleFunc("/tenants/{tenantId}", adminHandler.UpdateTenant).Methods("PUT")
	adminWrite.HandleFunc("/tenants/{tenantId}/status", adminHandler.UpdateTenantStatus).Methods("PATCH")
	adminWrite.HandleFunc("/plans", plansHandler.CreatePlan).Methods("POST")
	adminWrite.HandleFunc("/plans/{planId}", plansHandler.UpdatePlan).Methods("PUT")
	adminWrite.HandleFunc("/plans/{planId}", plansHandler.DeletePlan).Methods("DELETE")
	adminWrite.HandleFunc("/plans/{planId}/archive", plansHandler.ArchivePlan).Methods("POST")
	adminWrite.HandleFunc("/plans/{planId}/unarchive", plansHandler.UnarchivePlan).Methods("POST")
	adminWrite.HandleFunc("/tenants/{tenantId}/plan", plansHandler.AssignPlan).Methods("PATCH")
	adminWrite.HandleFunc("/credit-bundles", bundlesHandler.CreateBundle).Methods("POST")
	adminWrite.HandleFunc("/credit-bundles/{bundleId}", bundlesHandler.UpdateBundle).Methods("PUT")
	adminWrite.HandleFunc("/credit-bundles/{bundleId}", bundlesHandler.DeleteBundle).Methods("DELETE")
	adminWrite.HandleFunc("/promotions", promotionsHandler.CreatePromotion).Methods("POST")
	adminWrite.HandleFunc("/promotions/update", promotionsHandler.UpdatePromotion).Methods("POST")
	adminWrite.HandleFunc("/promotions/deactivate", promotionsHandler.DeactivatePromotion).Methods("POST")
	adminWrite.HandleFunc("/announcements", announcementsHandler.Create).Methods("POST")
	adminWrite.HandleFunc("/announcements/{id}", announcementsHandler.Update).Methods("PUT")
	adminWrite.HandleFunc("/announcements/{id}", announcementsHandler.Delete).Methods("DELETE")
	adminWrite.HandleFunc("/api-keys", apiKeysHandler.CreateAPIKey).Methods("POST")
	adminWrite.HandleFunc("/api-keys/{keyId}", apiKeysHandler.DeleteAPIKey).Methods("DELETE")
	adminWrite.HandleFunc("/members/invite", adminHandler.InviteRootMember).Methods("POST")
	adminWrite.HandleFunc("/members/invitations/{invitationId}", adminHandler.CancelRootInvitation).Methods("DELETE")
	adminWrite.HandleFunc("/members/{userId}", adminHandler.RemoveRootMember).Methods("DELETE")
	adminWrite.HandleFunc("/webhooks", webhooksHandler.CreateWebhook).Methods("POST")
	adminWrite.HandleFunc("/webhooks/{webhookId}", webhooksHandler.UpdateWebhook).Methods("PUT")
	adminWrite.HandleFunc("/webhooks/{webhookId}", webhooksHandler.DeleteWebhook).Methods("DELETE")
	adminWrite.HandleFunc("/webhooks/{webhookId}/test", webhooksHandler.TestWebhook).Methods("POST")
	adminWrite.HandleFunc("/webhooks/{webhookId}/regenerate-secret", webhooksHandler.RegenerateSecret).Methods("POST")
	adminWrite.HandleFunc("/pm/event-definitions", eventDefsHandler.CreateEventDefinition).Methods("POST")
	adminWrite.HandleFunc("/pm/event-definitions/{defId}", eventDefsHandler.UpdateEventDefinition).Methods("PUT")
	adminWrite.HandleFunc("/pm/event-definitions/{defId}", eventDefsHandler.DeleteEventDefinition).Methods("DELETE")

	// Owner-only admin actions (impersonate, delete users, branding, billing management)
	adminOwner := adminAPI.PathPrefix("").Subrouter()
	adminOwner.Use(middleware.RequireRole(models.RoleOwner))
	adminOwner.HandleFunc("/members/{userId}/role", adminHandler.ChangeRootMemberRole).Methods("PATCH")
	adminOwner.HandleFunc("/users/{userId}/preflight-delete", adminHandler.PreflightDeleteUser).Methods("GET")
	adminOwner.HandleFunc("/users/{userId}/impersonate", adminHandler.ImpersonateUser).Methods("POST")
	adminOwner.HandleFunc("/users/{userId}", adminHandler.DeleteUser).Methods("DELETE")
	adminOwner.HandleFunc("/tenants/{tenantId}/cancel-subscription", billingHandler.AdminCancelSubscription).Methods("POST")
	adminOwner.HandleFunc("/tenants/{tenantId}/subscription", billingHandler.AdminUpdateSubscription).Methods("PATCH")
	adminOwner.HandleFunc("/branding", brandingHandler.UpdateBranding).Methods("PUT")
	adminOwner.HandleFunc("/branding/asset", brandingHandler.UploadAsset).Methods("POST")
	adminOwner.HandleFunc("/branding/asset/{key}", brandingHandler.DeleteAsset).Methods("DELETE")
	adminOwner.HandleFunc("/branding/media", brandingHandler.UploadMedia).Methods("POST")
	adminOwner.HandleFunc("/branding/media/{id}", brandingHandler.DeleteMedia).Methods("DELETE")
	adminOwner.HandleFunc("/branding/pages", brandingHandler.CreatePage).Methods("POST")
	adminOwner.HandleFunc("/branding/pages/{id}", brandingHandler.UpdatePage).Methods("PUT")
	adminOwner.HandleFunc("/branding/pages/{id}", brandingHandler.DeletePage).Methods("DELETE")

	// Serve frontend static files in production
	if cfg.Frontend.StaticDir != "" {
		slog.Info("Serving frontend", "staticDir", cfg.Frontend.StaticDir)
		spa := spaHandler{
			staticPath: cfg.Frontend.StaticDir,
			indexPath:  "index.html",
			getAppName: func() string {
				// Check branding config in DB first, fall back to configstore app.name.
				var bc models.BrandingConfig
				if err := database.BrandingConfig().FindOne(context.Background(), bson.M{}).Decode(&bc); err == nil && bc.AppName != "" {
					return bc.AppName
				}
				return cfgStore.Get("app.name")
			},
		}
		router.PathPrefix("/").Handler(spa)
	}

	// CORS
	c := cors.New(cors.Options{
		AllowedOrigins:   []string{cfg.Frontend.URL},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type", "X-Tenant-ID"},
		ExposedHeaders:   []string{"X-Request-ID", "X-API-Version"},
		AllowCredentials: true,
		MaxAge:           86400,
	})

	// Wrap with recovery + body size limit + security headers + CORS + metrics
	handler := middleware.Recovery(middleware.BodySizeLimit(middleware.SecurityHeaders(c.Handler(metricsCollector.Middleware(router)))))

	// Start server
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	srv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      90 * time.Second, // raised to accommodate scanner CLI (up to 60s)
		IdleTimeout:       60 * time.Second,
	}

	// Graceful shutdown
	go func() {
		slog.Info("Server listening", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Server failed", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("Shutting down server")
	// Cancel app-wide context to signal background services (config reload, etc.)
	appCancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("Server forced shutdown", "error", err)
		os.Exit(1)
	}
	slog.Info("Server stopped")
}
