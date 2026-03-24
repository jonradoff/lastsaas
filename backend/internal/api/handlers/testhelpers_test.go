package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"lastsaas/internal/auth"
	"lastsaas/internal/config"
	"lastsaas/internal/configstore"
	"lastsaas/internal/db"
	"lastsaas/internal/events"
	"lastsaas/internal/middleware"
	"lastsaas/internal/models"
	"lastsaas/internal/scanner"
	"lastsaas/internal/syslog"
	"lastsaas/internal/testutil"

	"github.com/gorilla/mux"
)

// sharedDB is a single DB connection shared across all tests in this package.
var sharedDB *db.MongoDB
var sharedDBCleanup func()

func TestMain(m *testing.M) {
	sharedDB, sharedDBCleanup = testutil.ConnectTestDB()
	code := m.Run()
	if sharedDBCleanup != nil {
		sharedDBCleanup()
	}
	os.Exit(code)
}

// testEnv holds all services needed for handler integration tests.
type testEnv struct {
	DB              *db.MongoDB
	Config          *config.Config
	JWTService      *auth.JWTService
	PasswordService *auth.PasswordService
	Emitter         events.Emitter
	SysLogger       *syslog.Logger
	ConfigStore     *configstore.Store
	Server          *httptest.Server
	Client          *http.Client
	Cleanup         func()
}

// setupTestServer creates a full test HTTP server with all routes wired up.
func setupTestServer(t *testing.T) *testEnv {
	t.Helper()

	if sharedDB == nil {
		t.Skip("skipping: no test database connection")
	}

	testutil.CleanupCollections(t, sharedDB)

	cfg := testutil.TestConfig(t)

	jwtService := auth.NewJWTService(
		cfg.JWT.AccessSecret,
		cfg.JWT.RefreshSecret,
		cfg.JWT.AccessTTLMin,
		cfg.JWT.RefreshTTLDay,
	)
	passwordService := auth.NewTestPasswordService()
	emitter := events.NewNoopEmitter()
	sysLogger := syslog.New(sharedDB, nil)
	cfgStore := configstore.New(sharedDB)
	cfgStore.Load(context.Background())

	// Handlers
	bootstrapHandler := NewBootstrapHandler(sharedDB)
	authHandler := NewAuthHandler(sharedDB, jwtService, passwordService, nil, nil, emitter, cfg.Frontend.URL, sysLogger)
	adminHandler := NewAdminHandler(sharedDB, emitter, sysLogger)
	adminHandler.SetJWTService(jwtService)
	logHandler := NewLogHandler(sharedDB)
	tenantHandler := NewTenantHandler(sharedDB, nil, emitter, sysLogger)
	plansHandler := NewPlansHandler(sharedDB, sysLogger, cfgStore, nil)
	billingHandler := NewBillingHandler(nil, sharedDB, emitter, sysLogger, cfgStore)
	apiKeysHandler := NewAPIKeysHandler(sharedDB, emitter, sysLogger)
	webhooksHandler := NewWebhooksHandler(sharedDB, sysLogger, nil)

	// Scanner handler (uses DB directly for tracked stores, no CLI needed)
	scannerSvc := scanner.NewService(sharedDB.Database)
	scannerHandler := NewScannerHandler(scannerSvc)
	scannerHandler.SetDB(sharedDB)

	// Middleware
	authMiddleware := middleware.NewAuthMiddleware(jwtService, sharedDB)
	tenantMiddleware := middleware.NewTenantMiddleware(sharedDB)

	// Router
	router := mux.NewRouter()
	api := router.PathPrefix("/api").Subrouter()

	api.HandleFunc("/bootstrap/status", bootstrapHandler.Status).Methods("GET")

	guarded := api.PathPrefix("").Subrouter()
	guarded.Use(bootstrapHandler.BootstrapGuard)

	// Public auth routes (no rate limiting in tests)
	guarded.HandleFunc("/auth/register", authHandler.Register).Methods("POST")
	guarded.HandleFunc("/auth/login", authHandler.Login).Methods("POST")
	guarded.HandleFunc("/auth/refresh", authHandler.Refresh).Methods("POST")
	guarded.HandleFunc("/auth/verify-email", authHandler.VerifyEmail).Methods("POST")
	guarded.HandleFunc("/auth/resend-verification", authHandler.ResendVerification).Methods("POST")
	guarded.HandleFunc("/auth/forgot-password", authHandler.ForgotPassword).Methods("POST")
	guarded.HandleFunc("/auth/reset-password", authHandler.ResetPassword).Methods("POST")
	guarded.HandleFunc("/auth/providers", authHandler.GetProviders).Methods("GET")
	guarded.HandleFunc("/auth/mfa/challenge", authHandler.MFAChallenge).Methods("POST")

	// Protected auth routes
	protectedAuth := guarded.PathPrefix("/auth").Subrouter()
	protectedAuth.Use(authMiddleware.RequireAuth)
	protectedAuth.HandleFunc("/me", authHandler.GetMe).Methods("GET")
	protectedAuth.HandleFunc("/logout", authHandler.Logout).Methods("POST")
	protectedAuth.HandleFunc("/change-password", authHandler.ChangePassword).Methods("POST")
	protectedAuth.HandleFunc("/mfa/setup", authHandler.MFASetup).Methods("POST")
	protectedAuth.HandleFunc("/mfa/verify-setup", authHandler.MFAVerifySetup).Methods("POST")
	protectedAuth.HandleFunc("/mfa/disable", authHandler.MFADisable).Methods("POST")
	protectedAuth.HandleFunc("/mfa/regenerate-codes", authHandler.MFARegenerateRecoveryCodes).Methods("POST")
	protectedAuth.HandleFunc("/sessions", authHandler.ListSessions).Methods("GET")
	protectedAuth.HandleFunc("/sessions", authHandler.RevokeAllSessions).Methods("DELETE")
	protectedAuth.HandleFunc("/sessions/{sessionId}", authHandler.RevokeSession).Methods("DELETE")
	protectedAuth.HandleFunc("/preferences", authHandler.UpdatePreferences).Methods("PATCH")
	protectedAuth.HandleFunc("/complete-onboarding", authHandler.CompleteOnboarding).Methods("POST")

	// Tenant routes
	tenantAPI := guarded.PathPrefix("/tenant").Subrouter()
	tenantAPI.Use(authMiddleware.RequireAuth)
	tenantAPI.Use(tenantMiddleware.RequireTenant)

	tenantAPI.HandleFunc("/members", tenantHandler.ListMembers).Methods("GET")
	tenantAPI.HandleFunc("/activity", tenantHandler.GetActivity).Methods("GET")

	tenantSettingsRouter := tenantAPI.PathPrefix("/settings").Subrouter()
	tenantSettingsRouter.Use(middleware.RequireRole(models.RoleAdmin))
	tenantSettingsRouter.HandleFunc("", tenantHandler.UpdateTenantSettings).Methods("PATCH")

	inviteRouter := tenantAPI.PathPrefix("/members/invite").Subrouter()
	inviteRouter.Use(middleware.RequireRole(models.RoleAdmin))
	inviteRouter.HandleFunc("", tenantHandler.InviteMember).Methods("POST")

	removeRouter := tenantAPI.PathPrefix("/members/{userId}").Subrouter()
	removeRouter.Use(middleware.RequireRole(models.RoleAdmin))
	removeRouter.HandleFunc("", tenantHandler.RemoveMember).Methods("DELETE")

	ownerRouter := tenantAPI.PathPrefix("/members/{userId}").Subrouter()
	ownerRouter.Use(middleware.RequireRole(models.RoleOwner))
	ownerRouter.HandleFunc("/role", tenantHandler.ChangeRole).Methods("PATCH")
	ownerRouter.HandleFunc("/transfer-ownership", tenantHandler.TransferOwnership).Methods("POST")

	// Scanner routes (public)
	guarded.HandleFunc("/scan", scannerHandler.TriggerScan).Methods("POST")
	guarded.HandleFunc("/scan/{id}", scannerHandler.GetScan).Methods("GET")
	guarded.HandleFunc("/scan/domain/{domain}", scannerHandler.GetLatestDomainScan).Methods("GET")

	// Tracked stores routes (authenticated + tenant)
	trackedAPI := guarded.PathPrefix("/tracked-stores").Subrouter()
	trackedAPI.Use(authMiddleware.RequireAuth)
	trackedAPI.Use(tenantMiddleware.RequireTenant)
	trackedAPI.HandleFunc("", scannerHandler.ListTrackedStores).Methods("GET")
	trackedAPI.HandleFunc("", scannerHandler.AddTrackedStore).Methods("POST")
	trackedAPI.HandleFunc("/{id}", scannerHandler.RemoveTrackedStore).Methods("DELETE")
	trackedAPI.HandleFunc("/{id}/history", scannerHandler.GetTrackedStoreHistory).Methods("GET")

	// Scan list (authenticated + tenant)
	scanListAPI := guarded.PathPrefix("/scans").Subrouter()
	scanListAPI.Use(authMiddleware.RequireAuth)
	scanListAPI.Use(tenantMiddleware.RequireTenant)
	scanListAPI.HandleFunc("", scannerHandler.ListScans).Methods("GET")

	// Billing routes
	billingAPI := guarded.PathPrefix("/billing").Subrouter()
	billingAPI.Use(authMiddleware.RequireAuth)
	billingAPI.Use(tenantMiddleware.RequireTenant)
	billingAPI.HandleFunc("/checkout", billingHandler.Checkout).Methods("POST")
	billingAPI.HandleFunc("/portal", billingHandler.Portal).Methods("POST")
	billingAPI.HandleFunc("/transactions", billingHandler.ListTransactions).Methods("GET")
	billingAPI.HandleFunc("/cancel", billingHandler.CancelSubscription).Methods("POST")
	billingAPI.HandleFunc("/config", billingHandler.GetConfig).Methods("GET")
	billingAPI.HandleFunc("/scan-purchase", billingHandler.PurchaseScan).Methods("POST")

	// Admin routes
	adminAPI := guarded.PathPrefix("/admin").Subrouter()
	adminAPI.Use(authMiddleware.RequireAuth)
	adminAPI.Use(tenantMiddleware.RequireTenant)
	adminAPI.Use(middleware.RequireRootTenant())
	adminAPI.Use(middleware.RequireRole(models.RoleAdmin))

	adminAPI.HandleFunc("/dashboard", adminHandler.GetDashboard).Methods("GET")
	adminAPI.HandleFunc("/logs", logHandler.ListLogs).Methods("GET")
	adminAPI.HandleFunc("/logs/severity-counts", logHandler.SeverityCounts).Methods("GET")
	adminAPI.HandleFunc("/plans", plansHandler.ListPlans).Methods("GET")
	adminAPI.HandleFunc("/plans/{planId}", plansHandler.GetPlan).Methods("GET")
	adminAPI.HandleFunc("/entitlement-keys", plansHandler.ListEntitlementKeys).Methods("GET")
	adminAPI.HandleFunc("/api-keys", apiKeysHandler.ListAPIKeys).Methods("GET")
	adminAPI.HandleFunc("/api-keys", apiKeysHandler.CreateAPIKey).Methods("POST")
	adminAPI.HandleFunc("/api-keys/{keyId}", apiKeysHandler.DeleteAPIKey).Methods("DELETE")
	adminAPI.HandleFunc("/webhooks", webhooksHandler.ListWebhooks).Methods("GET")
	adminAPI.HandleFunc("/webhooks/event-types", webhooksHandler.ListEventTypes).Methods("GET")
	adminAPI.HandleFunc("/webhooks", webhooksHandler.CreateWebhook).Methods("POST")
	adminAPI.HandleFunc("/webhooks/{webhookId}", webhooksHandler.GetWebhook).Methods("GET")
	adminAPI.HandleFunc("/webhooks/{webhookId}", webhooksHandler.UpdateWebhook).Methods("PUT")
	adminAPI.HandleFunc("/webhooks/{webhookId}", webhooksHandler.DeleteWebhook).Methods("DELETE")
	adminAPI.HandleFunc("/webhooks/{webhookId}/regenerate-secret", webhooksHandler.RegenerateSecret).Methods("POST")
	adminAPI.HandleFunc("/financial/transactions", billingHandler.AdminListTransactions).Methods("GET")
	adminAPI.HandleFunc("/financial/metrics", billingHandler.AdminGetMetrics).Methods("GET")
	adminAPI.HandleFunc("/members", adminHandler.ListRootMembers).Methods("GET")
	adminAPI.HandleFunc("/members/invite", adminHandler.InviteRootMember).Methods("POST")
	adminAPI.HandleFunc("/members/invitations/{invitationId}", adminHandler.CancelRootInvitation).Methods("DELETE")
	adminAPI.HandleFunc("/members/{userId}", adminHandler.RemoveRootMember).Methods("DELETE")

	// Owner-only admin actions
	adminOwner := adminAPI.PathPrefix("").Subrouter()
	adminOwner.Use(middleware.RequireRole(models.RoleOwner))
	adminOwner.HandleFunc("/tenants", adminHandler.ListTenants).Methods("GET")
	adminOwner.HandleFunc("/tenants/{tenantId}", adminHandler.GetTenant).Methods("GET")
	adminOwner.HandleFunc("/tenants/{tenantId}/status", adminHandler.UpdateTenantStatus).Methods("PATCH")
	adminOwner.HandleFunc("/users", adminHandler.ListUsers).Methods("GET")
	adminOwner.HandleFunc("/users/{userId}", adminHandler.GetUser).Methods("GET")
	adminOwner.HandleFunc("/users/{userId}/status", adminHandler.UpdateUserStatus).Methods("PATCH")
	adminOwner.HandleFunc("/plans", plansHandler.CreatePlan).Methods("POST")
	adminOwner.HandleFunc("/plans/{planId}", plansHandler.UpdatePlan).Methods("PUT")
	adminOwner.HandleFunc("/plans/{planId}", plansHandler.DeletePlan).Methods("DELETE")
	adminOwner.HandleFunc("/plans/{planId}/archive", plansHandler.ArchivePlan).Methods("POST")
	adminOwner.HandleFunc("/plans/{planId}/unarchive", plansHandler.UnarchivePlan).Methods("POST")
	adminOwner.HandleFunc("/tenants/{tenantId}/plan", plansHandler.AssignPlan).Methods("PATCH")
	adminOwner.HandleFunc("/members/{userId}/role", adminHandler.ChangeRootMemberRole).Methods("PATCH")

	server := httptest.NewServer(router)

	return &testEnv{
		DB:              sharedDB,
		Config:          cfg,
		JWTService:      jwtService,
		PasswordService: passwordService,
		Emitter:         emitter,
		SysLogger:       sysLogger,
		ConfigStore:     cfgStore,
		Server:          server,
		Client:          server.Client(),
		Cleanup: func() {
			server.Close()
		},
	}
}

// authenticatedRequest creates a request with a Bearer token.
func (env *testEnv) authenticatedRequest(t *testing.T, method, path string, body *strings.Reader, user *models.User) *http.Request {
	t.Helper()
	var bodyReader *strings.Reader
	if body != nil {
		bodyReader = body
	} else {
		bodyReader = strings.NewReader("")
	}

	req, err := http.NewRequest(method, env.Server.URL+path, bodyReader)
	if err != nil {
		t.Fatalf("testhelper: failed to create request: %v", err)
	}

	token, err := env.JWTService.GenerateAccessToken(user.ID.Hex(), user.Email, user.DisplayName)
	if err != nil {
		t.Fatalf("testhelper: failed to generate token: %v", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	return req
}

// adminRequest creates an authenticated request with admin tenant context.
func (env *testEnv) adminRequest(t *testing.T, method, path string, body *strings.Reader, user *models.User, tenantID string) *http.Request {
	t.Helper()
	req := env.authenticatedRequest(t, method, path, body, user)
	req.Header.Set("X-Tenant-ID", tenantID)
	return req
}

// tenantRequest creates an authenticated request with tenant context (alias for adminRequest).
func (env *testEnv) tenantRequest(t *testing.T, method, path string, body *strings.Reader, user *models.User, tenantID string) *http.Request {
	t.Helper()
	return env.adminRequest(t, method, path, body, user, tenantID)
}

// createAdminEnv creates a fully set up admin user with root tenant.
func createAdminEnv(t *testing.T, env *testEnv) (*models.User, *models.Tenant) {
	t.Helper()
	testutil.MarkSystemInitialized(t, env.DB)
	user := testutil.CreateTestUser(t, env.DB, "admin@test.com", "Test1234!@#$", "Test Admin")
	tenant := testutil.CreateTestTenant(t, env.DB, "Root Tenant", user.ID, true)
	return user, tenant
}
