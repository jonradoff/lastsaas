package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"lastsaas/internal/models"
	"lastsaas/internal/testutil"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// --- Checkout with free plan (no Stripe needed) ---

func TestIntegration_Checkout_FreePlan_NoStripe(t *testing.T) {
	env := setupTestServer(t)
	defer env.Cleanup()
	testutil.MarkSystemInitialized(t, env.DB)

	owner := testutil.CreateTestUser(t, env.DB, "free@test.com", "Test1234!@#$", "Free Plan Owner")
	tenant := testutil.CreateTestTenant(t, env.DB, "Free Tenant", owner.ID, false)
	plan := testutil.CreateTestPlan(t, env.DB, "Free", 0, true)

	body := strings.NewReader(`{"planId":"` + plan.ID.Hex() + `","billingInterval":"month"}`)
	req := env.tenantRequest(t, "POST", "/api/billing/checkout", body, owner, tenant.ID.Hex())
	resp, err := env.Client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	// Free plan should assign directly without Stripe
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 for free plan checkout, got %d: %s", resp.StatusCode, testutil.ReadResponseBody(t, resp))
	}
}

func TestIntegration_Checkout_FreePlan_AssignsToTenant(t *testing.T) {
	env := setupTestServer(t)
	defer env.Cleanup()
	testutil.MarkSystemInitialized(t, env.DB)

	owner := testutil.CreateTestUser(t, env.DB, "freeassign@test.com", "Test1234!@#$", "Owner")
	tenant := testutil.CreateTestTenant(t, env.DB, "Assign Tenant", owner.ID, false)
	plan := testutil.CreateTestPlan(t, env.DB, "Starter", 0, true)

	body := strings.NewReader(`{"planId":"` + plan.ID.Hex() + `","billingInterval":"year"}`)
	req := env.tenantRequest(t, "POST", "/api/billing/checkout", body, owner, tenant.ID.Hex())
	resp, _ := env.Client.Do(req)
	resp.Body.Close()

	// Verify tenant got the plan assigned
	var updated models.Tenant
	env.DB.Tenants().FindOne(context.Background(), bson.M{"_id": tenant.ID}).Decode(&updated)
	if updated.PlanID == nil || *updated.PlanID != plan.ID {
		t.Error("expected tenant to have the free plan assigned")
	}
	if updated.BillingStatus != models.BillingStatusActive {
		t.Errorf("expected billing status 'active', got '%s'", updated.BillingStatus)
	}
}

// --- Checkout validation ---

func TestIntegration_Checkout_InvalidPlanID(t *testing.T) {
	env := setupTestServer(t)
	defer env.Cleanup()
	testutil.MarkSystemInitialized(t, env.DB)

	owner := testutil.CreateTestUser(t, env.DB, "badplan@test.com", "Test1234!@#$", "Owner")
	tenant := testutil.CreateTestTenant(t, env.DB, "BadPlan Tenant", owner.ID, false)

	body := strings.NewReader(`{"planId":"not-a-valid-hex","billingInterval":"month"}`)
	req := env.tenantRequest(t, "POST", "/api/billing/checkout", body, owner, tenant.ID.Hex())
	resp, err := env.Client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid plan ID, got %d", resp.StatusCode)
	}
}

func TestIntegration_Checkout_NonexistentPlan(t *testing.T) {
	env := setupTestServer(t)
	defer env.Cleanup()
	testutil.MarkSystemInitialized(t, env.DB)

	owner := testutil.CreateTestUser(t, env.DB, "noplan@test.com", "Test1234!@#$", "Owner")
	tenant := testutil.CreateTestTenant(t, env.DB, "NoPlan Tenant", owner.ID, false)

	fakePlanID := primitive.NewObjectID().Hex()
	body := strings.NewReader(`{"planId":"` + fakePlanID + `","billingInterval":"month"}`)
	req := env.tenantRequest(t, "POST", "/api/billing/checkout", body, owner, tenant.ID.Hex())
	resp, err := env.Client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 for nonexistent plan, got %d", resp.StatusCode)
	}
}

func TestIntegration_Checkout_InvalidBillingInterval(t *testing.T) {
	env := setupTestServer(t)
	defer env.Cleanup()
	testutil.MarkSystemInitialized(t, env.DB)

	owner := testutil.CreateTestUser(t, env.DB, "badinterval@test.com", "Test1234!@#$", "Owner")
	tenant := testutil.CreateTestTenant(t, env.DB, "BadInterval Tenant", owner.ID, false)
	plan := testutil.CreateTestPlan(t, env.DB, "Pro", 1999, false)

	body := strings.NewReader(`{"planId":"` + plan.ID.Hex() + `","billingInterval":"weekly"}`)
	req := env.tenantRequest(t, "POST", "/api/billing/checkout", body, owner, tenant.ID.Hex())
	resp, err := env.Client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid billing interval, got %d", resp.StatusCode)
	}
}

func TestIntegration_Checkout_EmptyBody(t *testing.T) {
	env := setupTestServer(t)
	defer env.Cleanup()
	testutil.MarkSystemInitialized(t, env.DB)

	owner := testutil.CreateTestUser(t, env.DB, "empty@test.com", "Test1234!@#$", "Owner")
	tenant := testutil.CreateTestTenant(t, env.DB, "Empty Tenant", owner.ID, false)

	body := strings.NewReader(`{}`)
	req := env.tenantRequest(t, "POST", "/api/billing/checkout", body, owner, tenant.ID.Hex())
	resp, err := env.Client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	// Empty body means no planId or bundleId — should fail
	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
		t.Errorf("expected error for empty checkout body, got %d", resp.StatusCode)
	}
}

// --- Billing waiver ---

func TestIntegration_Checkout_WaivedBilling_PaidPlan(t *testing.T) {
	env := setupTestServer(t)
	defer env.Cleanup()
	testutil.MarkSystemInitialized(t, env.DB)

	owner := testutil.CreateTestUser(t, env.DB, "waived@test.com", "Test1234!@#$", "Owner")
	tenant := testutil.CreateTestTenant(t, env.DB, "Waived Tenant", owner.ID, false)
	// Mark as billing waived
	env.DB.Tenants().UpdateOne(context.Background(), bson.M{"_id": tenant.ID}, bson.M{
		"$set": bson.M{"billingWaived": true},
	})
	plan := testutil.CreateTestPlan(t, env.DB, "Pro", 5000, false)

	body := strings.NewReader(`{"planId":"` + plan.ID.Hex() + `","billingInterval":"month"}`)
	req := env.tenantRequest(t, "POST", "/api/billing/checkout", body, owner, tenant.ID.Hex())
	resp, err := env.Client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	// Waived tenant should get plan assigned directly, no Stripe
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 for waived billing checkout, got %d: %s", resp.StatusCode, testutil.ReadResponseBody(t, resp))
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if result["waived"] != true {
		t.Error("expected waived=true in response")
	}
}

// --- Cancel subscription (no active subscription) ---

func TestIntegration_CancelSubscription_NotAuthenticated(t *testing.T) {
	env := setupTestServer(t)
	defer env.Cleanup()
	testutil.MarkSystemInitialized(t, env.DB)

	req, _ := http.NewRequest("POST", env.Server.URL+"/api/billing/cancel", nil)
	resp, err := env.Client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

// --- Portal (no Stripe customer) ---

func TestIntegration_Portal_NoStripeCustomer(t *testing.T) {
	env := setupTestServer(t)
	defer env.Cleanup()
	testutil.MarkSystemInitialized(t, env.DB)

	owner := testutil.CreateTestUser(t, env.DB, "noportal@test.com", "Test1234!@#$", "Owner")
	tenant := testutil.CreateTestTenant(t, env.DB, "NoPortal Tenant", owner.ID, false)

	req := env.tenantRequest(t, "POST", "/api/billing/portal", nil, owner, tenant.ID.Hex())
	resp, err := env.Client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	// Without Stripe customer, portal should fail
	if resp.StatusCode == http.StatusOK {
		t.Error("expected error when no Stripe customer exists")
	}
}

// --- ListTransactions pagination ---

func TestIntegration_ListTransactions_WithData(t *testing.T) {
	env := setupTestServer(t)
	defer env.Cleanup()
	testutil.MarkSystemInitialized(t, env.DB)

	owner := testutil.CreateTestUser(t, env.DB, "txlist@test.com", "Test1234!@#$", "Owner")
	tenant := testutil.CreateTestTenant(t, env.DB, "TxList Tenant", owner.ID, false)

	// Insert some test transactions
	now := time.Now()
	for i := 0; i < 3; i++ {
		env.DB.FinancialTransactions().InsertOne(context.Background(), models.FinancialTransaction{
			ID:          primitive.NewObjectID(),
			TenantID:    tenant.ID,
			UserID:      owner.ID,
			Type:        models.TransactionSubscription,
			AmountCents: 1999,
			Description: "Test subscription",
			CreatedAt:   now,
		})
	}

	req := env.tenantRequest(t, "GET", "/api/billing/transactions?page=1&perPage=2", nil, owner, tenant.ID.Hex())
	resp, err := env.Client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result struct {
		Total        int64                          `json:"total"`
		Transactions []models.FinancialTransaction `json:"transactions"`
	}
	json.NewDecoder(resp.Body).Decode(&result)

	if result.Total != 3 {
		t.Errorf("expected total=3, got %d", result.Total)
	}
	if len(result.Transactions) != 2 {
		t.Errorf("expected 2 transactions (page size), got %d", len(result.Transactions))
	}
}

// --- Billing default interval ---

func TestIntegration_Checkout_DefaultBillingInterval(t *testing.T) {
	env := setupTestServer(t)
	defer env.Cleanup()
	testutil.MarkSystemInitialized(t, env.DB)

	owner := testutil.CreateTestUser(t, env.DB, "default@test.com", "Test1234!@#$", "Owner")
	tenant := testutil.CreateTestTenant(t, env.DB, "Default Tenant", owner.ID, false)
	// Mark as billing waived so we don't need Stripe
	env.DB.Tenants().UpdateOne(context.Background(), bson.M{"_id": tenant.ID}, bson.M{
		"$set": bson.M{"billingWaived": true},
	})
	plan := testutil.CreateTestPlan(t, env.DB, "ProDefault", 5000, false)

	// Don't specify billingInterval — should default to "year"
	body := strings.NewReader(`{"planId":"` + plan.ID.Hex() + `"}`)
	req := env.tenantRequest(t, "POST", "/api/billing/checkout", body, owner, tenant.ID.Hex())
	resp, err := env.Client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 with default interval, got %d: %s", resp.StatusCode, testutil.ReadResponseBody(t, resp))
	}

	// Verify tenant billing interval defaulted to year
	var updated models.Tenant
	env.DB.Tenants().FindOne(context.Background(), bson.M{"_id": tenant.ID}).Decode(&updated)
	if updated.BillingInterval != "year" {
		t.Errorf("expected billing interval 'year', got '%s'", updated.BillingInterval)
	}
}
