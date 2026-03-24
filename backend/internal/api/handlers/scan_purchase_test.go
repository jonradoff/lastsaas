package handlers

import (
	"net/http"
	"strings"
	"testing"

	"lastsaas/internal/testutil"
)

func TestIntegration_PurchaseScan_NotAuthenticated(t *testing.T) {
	env := setupTestServer(t)
	defer env.Cleanup()
	testutil.MarkSystemInitialized(t, env.DB)

	body := `{"domain":"allbirds.com","feature":"assess"}`
	req, _ := http.NewRequest("POST", env.Server.URL+"/api/billing/scan-purchase", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := env.Client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestIntegration_PurchaseScan_MissingDomain(t *testing.T) {
	env := setupTestServer(t)
	defer env.Cleanup()
	testutil.MarkSystemInitialized(t, env.DB)

	owner := testutil.CreateTestUser(t, env.DB, "purchase@test.com", "Test1234!@#$", "Owner")
	tenant := testutil.CreateTestTenant(t, env.DB, "Purchase Tenant", owner.ID, false)

	body := strings.NewReader(`{"domain":"","feature":"assess"}`)
	req := env.tenantRequest(t, "POST", "/api/billing/scan-purchase", body, owner, tenant.ID.Hex())
	resp, err := env.Client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for empty domain, got %d", resp.StatusCode)
	}
}

func TestIntegration_PurchaseScan_InvalidFeature(t *testing.T) {
	env := setupTestServer(t)
	defer env.Cleanup()
	testutil.MarkSystemInitialized(t, env.DB)

	owner := testutil.CreateTestUser(t, env.DB, "badfeature@test.com", "Test1234!@#$", "Owner")
	tenant := testutil.CreateTestTenant(t, env.DB, "BadFeature Tenant", owner.ID, false)

	body := strings.NewReader(`{"domain":"allbirds.com","feature":"invalid"}`)
	req := env.tenantRequest(t, "POST", "/api/billing/scan-purchase", body, owner, tenant.ID.Hex())
	resp, err := env.Client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid feature, got %d", resp.StatusCode)
	}
}

func TestIntegration_PurchaseScan_NilStripe(t *testing.T) {
	env := setupTestServer(t)
	defer env.Cleanup()
	testutil.MarkSystemInitialized(t, env.DB)

	owner := testutil.CreateTestUser(t, env.DB, "nostripe@test.com", "Test1234!@#$", "Owner")
	tenant := testutil.CreateTestTenant(t, env.DB, "NoStripe Tenant", owner.ID, false)

	body := strings.NewReader(`{"domain":"allbirds.com","feature":"assess"}`)
	req := env.tenantRequest(t, "POST", "/api/billing/scan-purchase", body, owner, tenant.ID.Hex())
	resp, err := env.Client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	// With nil Stripe, should return 503
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("expected 503 with nil Stripe, got %d", resp.StatusCode)
	}
}

func TestIntegration_PurchaseScan_ValidAssess(t *testing.T) {
	env := setupTestServer(t)
	defer env.Cleanup()
	testutil.MarkSystemInitialized(t, env.DB)

	owner := testutil.CreateTestUser(t, env.DB, "assess@test.com", "Test1234!@#$", "Owner")
	tenant := testutil.CreateTestTenant(t, env.DB, "Assess Tenant", owner.ID, false)

	body := strings.NewReader(`{"domain":"allbirds.com","feature":"assess"}`)
	req := env.tenantRequest(t, "POST", "/api/billing/scan-purchase", body, owner, tenant.ID.Hex())
	resp, err := env.Client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	// With nil Stripe in test env, should fail at the Stripe step (not validation)
	if resp.StatusCode == http.StatusBadRequest {
		t.Error("should not fail on validation for valid assess request")
	}
}

func TestIntegration_PurchaseScan_ValidSimulate(t *testing.T) {
	env := setupTestServer(t)
	defer env.Cleanup()
	testutil.MarkSystemInitialized(t, env.DB)

	owner := testutil.CreateTestUser(t, env.DB, "simulate@test.com", "Test1234!@#$", "Owner")
	tenant := testutil.CreateTestTenant(t, env.DB, "Simulate Tenant", owner.ID, false)

	body := strings.NewReader(`{"domain":"gymshark.com","feature":"simulate"}`)
	req := env.tenantRequest(t, "POST", "/api/billing/scan-purchase", body, owner, tenant.ID.Hex())
	resp, err := env.Client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	// With nil Stripe, should fail at Stripe step, not validation
	if resp.StatusCode == http.StatusBadRequest {
		t.Error("should not fail on validation for valid simulate request")
	}
}
