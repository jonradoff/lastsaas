package testutil

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"lastsaas/internal/auth"
	"lastsaas/internal/config"
	"lastsaas/internal/db"
	"lastsaas/internal/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// loadEnvTest loads the .env.test file into the process environment.
func loadEnvTest() {
	dir, _ := os.Getwd()
	for i := 0; i < 6; i++ {
		envPath := filepath.Join(dir, ".env.test")
		data, err := os.ReadFile(envPath)
		if err == nil {
			for _, line := range strings.Split(string(data), "\n") {
				line = strings.TrimSpace(line)
				if line == "" || strings.HasPrefix(line, "#") {
					continue
				}
				if idx := strings.IndexByte(line, '='); idx > 0 {
					key := strings.TrimSpace(line[:idx])
					val := strings.TrimSpace(line[idx+1:])
					os.Setenv(key, val)
				}
			}
			return
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
}

// MustConnectTestDB connects to the test database. It fails the test if the database name
// does not contain "test" to prevent accidental use of production databases.
// Returns the database handle and a cleanup function that drops the database.
func MustConnectTestDB(t *testing.T) (*db.MongoDB, func()) {
	t.Helper()

	loadEnvTest()
	os.Setenv("LASTSAAS_ENV", "test")
	SetConfigDir(t)

	// Skip gracefully when no MongoDB URI is configured (e.g. in CI without .env.test)
	uri := os.Getenv("MONGODB_URI")
	if uri == "" {
		t.Skip("skipping: MONGODB_URI not set (no .env.test file)")
	}

	cfg, err := config.Load("test")
	if err != nil {
		t.Fatalf("testutil: failed to load test config: %v", err)
	}

	if !strings.Contains(strings.ToLower(cfg.Database.Name), "test") {
		t.Fatalf("testutil: REFUSING to run tests — database name %q does not contain 'test'. "+
			"This safety guard prevents accidental use of production databases.", cfg.Database.Name)
	}

	database, err := db.NewMongoDB(cfg.Database.URI, cfg.Database.Name)
	if err != nil {
		t.Fatalf("testutil: failed to connect to test database: %v", err)
	}

	cleanup := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		// Drop individual collections instead of the whole database to avoid
		// "database is in the process of being dropped" errors when multiple
		// test packages run in parallel against the same database.
		colls, _ := database.Database.ListCollectionNames(ctx, bson.M{})
		for _, name := range colls {
			database.Database.Collection(name).Drop(ctx)
		}
		database.Close(ctx)
	}

	return database, cleanup
}

// SetConfigDir finds and sets the LASTSAAS_CONFIG_DIR env var.
// It looks for a directory containing YAML config files to avoid matching Go source packages.
func SetConfigDir(t *testing.T) {
	t.Helper()
	if os.Getenv("LASTSAAS_CONFIG_DIR") != "" {
		return
	}
	dir, _ := os.Getwd()
	for i := 0; i < 6; i++ {
		for _, candidate := range []string{
			filepath.Join(dir, "config"),
			filepath.Join(dir, "backend", "config"),
		} {
			if hasYAMLConfigs(candidate) {
				os.Setenv("LASTSAAS_CONFIG_DIR", candidate)
				return
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatalf("testutil: could not find config directory")
}

func hasYAMLConfigs(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() && (filepath.Ext(e.Name()) == ".yaml" || filepath.Ext(e.Name()) == ".yml") {
			return true
		}
	}
	return false
}

// CleanupCollections drops all test collections for isolation between tests.
func CleanupCollections(t *testing.T, database *db.MongoDB) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	collections := []string{
		"users", "tenants", "tenant_memberships", "refresh_tokens",
		"verification_tokens", "oauth_states", "revoked_tokens",
		"system_config", "invitations", "audit_log", "messages",
		"system_logs", "config_vars", "plans", "credit_bundles",
		"system_nodes", "system_metrics", "financial_transactions",
		"stripe_mappings", "counters", "daily_metrics", "webhook_events",
		"leader_locks", "api_keys", "webhooks", "webhook_deliveries",
		"branding_config", "branding_assets", "custom_pages",
		"webauthn_credentials", "webauthn_sessions", "sso_connections",
		"announcements", "usage_events", "rate_limits",
	}
	for _, name := range collections {
		database.Database.Collection(name).Drop(ctx)
	}
}

// TestConfig returns a loaded test config.
func TestConfig(t *testing.T) *config.Config {
	t.Helper()
	loadEnvTest()
	os.Setenv("LASTSAAS_ENV", "test")
	SetConfigDir(t)

	cfg, err := config.Load("test")
	if err != nil {
		t.Fatalf("testutil: failed to load test config: %v", err)
	}
	return cfg
}

// CreateTestUser creates a user in the test database and returns it.
func CreateTestUser(t *testing.T, database *db.MongoDB, email, password, displayName string) *models.User {
	t.Helper()
	ctx := context.Background()

	pwService := auth.NewTestPasswordService()
	hash, err := pwService.HashPassword(password)
	if err != nil {
		t.Fatalf("testutil: failed to hash password: %v", err)
	}

	user := models.User{
		ID:            primitive.NewObjectID(),
		Email:         email,
		DisplayName:   displayName,
		PasswordHash:  hash,
		AuthMethods:   []models.AuthMethod{models.AuthMethodPassword},
		EmailVerified: true,
		IsActive:      true,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	_, err = database.Users().InsertOne(ctx, user)
	if err != nil {
		t.Fatalf("testutil: failed to create test user: %v", err)
	}
	return &user
}

// CreateTestTenant creates a tenant and its owner membership.
func CreateTestTenant(t *testing.T, database *db.MongoDB, name string, ownerID primitive.ObjectID, isRoot bool) *models.Tenant {
	t.Helper()
	ctx := context.Background()

	tenant := models.Tenant{
		ID:        primitive.NewObjectID(),
		Name:      name,
		Slug:      strings.ToLower(strings.ReplaceAll(name, " ", "-")),
		IsRoot:    isRoot,
		IsActive:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	_, err := database.Tenants().InsertOne(ctx, tenant)
	if err != nil {
		t.Fatalf("testutil: failed to create test tenant: %v", err)
	}

	membership := models.TenantMembership{
		ID:        primitive.NewObjectID(),
		UserID:    ownerID,
		TenantID:  tenant.ID,
		Role:      models.RoleOwner,
		JoinedAt:  time.Now(),
		UpdatedAt: time.Now(),
	}
	_, err = database.TenantMemberships().InsertOne(ctx, membership)
	if err != nil {
		t.Fatalf("testutil: failed to create test membership: %v", err)
	}

	return &tenant
}

// MarkSystemInitialized marks the system as initialized in the test database.
func MarkSystemInitialized(t *testing.T, database *db.MongoDB) {
	t.Helper()
	ctx := context.Background()
	now := time.Now()
	_, err := database.SystemConfig().InsertOne(ctx, models.SystemConfig{
		ID:            primitive.NewObjectID(),
		Initialized:   true,
		InitializedAt: &now,
		Version:       "test",
	})
	if err != nil {
		t.Fatalf("testutil: failed to mark system initialized: %v", err)
	}
}

// InsertTestLogs inserts sample log entries for testing.
func InsertTestLogs(t *testing.T, database *db.MongoDB, count int, severity models.LogSeverity, category models.LogCategory) {
	t.Helper()
	ctx := context.Background()
	for i := 0; i < count; i++ {
		entry := models.SystemLog{
			ID:        primitive.NewObjectID(),
			Severity:  severity,
			Category:  category,
			Message:   fmt.Sprintf("Test log entry %d", i+1),
			CreatedAt: time.Now().Add(-time.Duration(i) * time.Minute),
		}
		_, err := database.SystemLogs().InsertOne(ctx, entry)
		if err != nil {
			t.Fatalf("testutil: failed to insert test log: %v", err)
		}
	}
}

// CountDocuments counts documents in a collection matching the given filter.
func CountDocuments(t *testing.T, database *db.MongoDB, collection string, filter bson.M) int64 {
	t.Helper()
	ctx := context.Background()
	count, err := database.Database.Collection(collection).CountDocuments(ctx, filter)
	if err != nil {
		t.Fatalf("testutil: failed to count documents: %v", err)
	}
	return count
}

// ParseJSON decodes a response body into the target struct.
func ParseJSON(t *testing.T, resp *http.Response, target interface{}) {
	t.Helper()
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		t.Fatalf("testutil: failed to parse JSON response: %v", err)
	}
}

// ReadResponseBody reads and returns the full response body as a string.
func ReadResponseBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	defer resp.Body.Close()
	scanner := bufio.NewScanner(resp.Body)
	var sb strings.Builder
	for scanner.Scan() {
		sb.WriteString(scanner.Text())
	}
	return sb.String()
}

// CreateTestMembership creates a tenant membership for a user.
func CreateTestMembership(t *testing.T, database *db.MongoDB, userID, tenantID primitive.ObjectID, role models.MemberRole) *models.TenantMembership {
	t.Helper()
	ctx := context.Background()
	membership := models.TenantMembership{
		ID:        primitive.NewObjectID(),
		UserID:    userID,
		TenantID:  tenantID,
		Role:      role,
		JoinedAt:  time.Now(),
		UpdatedAt: time.Now(),
	}
	_, err := database.TenantMemberships().InsertOne(ctx, membership)
	if err != nil {
		t.Fatalf("testutil: failed to create test membership: %v", err)
	}
	return &membership
}

// CreateTestPlan creates a plan in the test database.
func CreateTestPlan(t *testing.T, database *db.MongoDB, name string, monthlyPriceCents int64, isSystem bool) *models.Plan {
	t.Helper()
	ctx := context.Background()
	now := time.Now()
	plan := models.Plan{
		ID:                primitive.NewObjectID(),
		Name:              name,
		Description:       "Test plan: " + name,
		PricingModel:      models.PricingModelFlat,
		MonthlyPriceCents: monthlyPriceCents,
		CreditResetPolicy: models.CreditResetPolicyReset,
		Entitlements:      map[string]models.EntitlementValue{},
		IsSystem:          isSystem,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	_, err := database.Plans().InsertOne(ctx, plan)
	if err != nil {
		t.Fatalf("testutil: failed to create test plan: %v", err)
	}
	return &plan
}

// CreateTestAPIKey creates an API key in the test database.
func CreateTestAPIKey(t *testing.T, database *db.MongoDB, name, keyHash string, authority models.APIKeyAuthority, createdBy primitive.ObjectID) *models.APIKey {
	t.Helper()
	ctx := context.Background()
	now := time.Now()
	apiKey := models.APIKey{
		ID:         primitive.NewObjectID(),
		Name:       name,
		KeyHash:    keyHash,
		KeyPreview: "test1234",
		Authority:  authority,
		CreatedBy:  createdBy,
		CreatedAt:  now,
		IsActive:   true,
	}
	_, err := database.APIKeys().InsertOne(ctx, apiKey)
	if err != nil {
		t.Fatalf("testutil: failed to create test API key: %v", err)
	}
	return &apiKey
}

// CreateTestWebhook creates a webhook in the test database.
func CreateTestWebhook(t *testing.T, database *db.MongoDB, name, url, secret string, events []models.WebhookEventType, createdBy primitive.ObjectID) *models.Webhook {
	t.Helper()
	ctx := context.Background()
	now := time.Now()
	webhook := models.Webhook{
		ID:            primitive.NewObjectID(),
		Name:          name,
		URL:           url,
		Secret:        secret,
		SecretPreview: "sec12345",
		Events:        events,
		IsActive:      true,
		CreatedBy:     createdBy,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	_, err := database.Webhooks().InsertOne(ctx, webhook)
	if err != nil {
		t.Fatalf("testutil: failed to create test webhook: %v", err)
	}
	return &webhook
}

// CreateTestInvitation creates an invitation in the test database.
func CreateTestInvitation(t *testing.T, database *db.MongoDB, email string, tenantID, invitedBy primitive.ObjectID, role models.MemberRole) *models.Invitation {
	t.Helper()
	ctx := context.Background()
	now := time.Now()
	invitation := models.Invitation{
		ID:        primitive.NewObjectID(),
		TenantID:  tenantID,
		Email:     email,
		Role:      role,
		Token:     "test-token-" + primitive.NewObjectID().Hex(),
		Status:    models.InvitationPending,
		InvitedBy: invitedBy,
		ExpiresAt: now.Add(7 * 24 * time.Hour),
		CreatedAt: now,
	}
	_, err := database.Invitations().InsertOne(ctx, invitation)
	if err != nil {
		t.Fatalf("testutil: failed to create test invitation: %v", err)
	}
	return &invitation
}
