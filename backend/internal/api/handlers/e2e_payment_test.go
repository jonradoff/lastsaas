package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"lastsaas/internal/models"
	"lastsaas/internal/testutil"

	"go.mongodb.org/mongo-driver/bson"
)

// TestE2E_FreePlanCheckoutAndEntitlements exercises:
// Register → Create free plan → Checkout → Verify plan assigned → Verify tracked stores blocked (no entitlement)
func TestE2E_FreePlanCheckoutAndEntitlements(t *testing.T) {
	env := setupTestServer(t)
	defer env.Cleanup()
	testutil.MarkSystemInitialized(t, env.DB)

	// Register user (gets auto personal tenant)
	regBody := `{"email":"plantest@test.com","password":"StrongP@ss1!","displayName":"Plan Test"}`
	regResp, _ := env.Client.Post(env.Server.URL+"/api/auth/register", "application/json", strings.NewReader(regBody))
	var authResp AuthResponse
	json.NewDecoder(regResp.Body).Decode(&authResp)
	regResp.Body.Close()

	tenantID := authResp.Memberships[0].TenantID
	accessToken := authResp.AccessToken

	// Create a free plan with 0 tracked stores
	plan := testutil.CreateTestPlan(t, env.DB, "Free", 0, true)

	// Checkout free plan
	checkoutBody := `{"planId":"` + plan.ID.Hex() + `","billingInterval":"month"}`
	checkoutReq, _ := http.NewRequest("POST", env.Server.URL+"/api/billing/checkout", strings.NewReader(checkoutBody))
	checkoutReq.Header.Set("Authorization", "Bearer "+accessToken)
	checkoutReq.Header.Set("X-Tenant-ID", tenantID)
	checkoutReq.Header.Set("Content-Type", "application/json")
	checkoutResp, err := env.Client.Do(checkoutReq)
	if err != nil {
		t.Fatal(err)
	}
	defer checkoutResp.Body.Close()

	if checkoutResp.StatusCode != http.StatusOK {
		t.Fatalf("free checkout: expected 200, got %d: %s", checkoutResp.StatusCode, testutil.ReadResponseBody(t, checkoutResp))
	}

	// Verify tenant has plan assigned and billing active
	var tenant models.Tenant
	env.DB.Tenants().FindOne(context.Background(), bson.M{"slug": authResp.Memberships[0].TenantSlug}).Decode(&tenant)
	if tenant.PlanID == nil || *tenant.PlanID != plan.ID {
		t.Error("expected tenant to have the free plan assigned")
	}
	if tenant.BillingStatus != models.BillingStatusActive {
		t.Errorf("expected billing status 'active', got '%s'", tenant.BillingStatus)
	}

	// Try to add tracked store — should be blocked (free plan has 0 entitlement)
	addStoreBody := `{"domain":"allbirds.com"}`
	addStoreReq, _ := http.NewRequest("POST", env.Server.URL+"/api/tracked-stores", strings.NewReader(addStoreBody))
	addStoreReq.Header.Set("Authorization", "Bearer "+accessToken)
	addStoreReq.Header.Set("X-Tenant-ID", tenantID)
	addStoreReq.Header.Set("Content-Type", "application/json")
	addStoreResp, _ := env.Client.Do(addStoreReq)
	defer addStoreResp.Body.Close()

	if addStoreResp.StatusCode != http.StatusPaymentRequired {
		t.Errorf("expected 402 (no entitlement), got %d", addStoreResp.StatusCode)
	}
}

// TestE2E_PaidPlanWithTrackedStoresEntitlement exercises:
// Setup paid plan with max_tracked_stores=3 → Checkout → Add stores → Hit limit
func TestE2E_PaidPlanWithTrackedStoresEntitlement(t *testing.T) {
	env := setupTestServer(t)
	defer env.Cleanup()
	testutil.MarkSystemInitialized(t, env.DB)

	// Register
	regBody := `{"email":"paid@test.com","password":"StrongP@ss1!","displayName":"Paid User"}`
	regResp, _ := env.Client.Post(env.Server.URL+"/api/auth/register", "application/json", strings.NewReader(regBody))
	var authResp AuthResponse
	json.NewDecoder(regResp.Body).Decode(&authResp)
	regResp.Body.Close()

	tenantID := authResp.Memberships[0].TenantID
	accessToken := authResp.AccessToken

	// Create paid plan with max_tracked_stores = 2
	plan := testutil.CreateTestPlan(t, env.DB, "Pro", 5000, false)
	// Add entitlement
	env.DB.Plans().UpdateOne(context.Background(), bson.M{"_id": plan.ID}, bson.M{
		"$set": bson.M{
			"entitlements": bson.M{
				"max_tracked_stores": bson.M{
					"type":         "numeric",
					"numericValue": 2,
					"description":  "Max tracked stores",
				},
			},
		},
	})

	// Assign plan to tenant (waived billing for test)
	env.DB.Tenants().UpdateOne(context.Background(), bson.M{"slug": authResp.Memberships[0].TenantSlug}, bson.M{
		"$set": bson.M{
			"planId":        plan.ID,
			"billingStatus": models.BillingStatusActive,
			"billingWaived": true,
		},
	})

	makeStoreReq := func(domain string) *http.Response {
		body := `{"domain":"` + domain + `"}`
		req, _ := http.NewRequest("POST", env.Server.URL+"/api/tracked-stores", strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+accessToken)
		req.Header.Set("X-Tenant-ID", tenantID)
		req.Header.Set("Content-Type", "application/json")
		resp, _ := env.Client.Do(req)
		return resp
	}

	// Add first store — should succeed
	resp1 := makeStoreReq("allbirds.com")
	defer resp1.Body.Close()
	if resp1.StatusCode != http.StatusCreated {
		t.Fatalf("add store 1: expected 201, got %d", resp1.StatusCode)
	}

	// Add second store — should succeed
	resp2 := makeStoreReq("gymshark.com")
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusCreated {
		t.Fatalf("add store 2: expected 201, got %d", resp2.StatusCode)
	}

	// Add third store — should be blocked (limit is 2)
	resp3 := makeStoreReq("colourpop.com")
	defer resp3.Body.Close()
	if resp3.StatusCode != http.StatusPaymentRequired {
		t.Errorf("add store 3: expected 402 (limit reached), got %d", resp3.StatusCode)
	}

	// List tracked stores — verify count and maxStores
	listReq, _ := http.NewRequest("GET", env.Server.URL+"/api/tracked-stores", nil)
	listReq.Header.Set("Authorization", "Bearer "+accessToken)
	listReq.Header.Set("X-Tenant-ID", tenantID)
	listResp, _ := env.Client.Do(listReq)
	defer listResp.Body.Close()

	var listResult struct {
		Stores    []interface{} `json:"stores"`
		Total     int           `json:"total"`
		MaxStores int64         `json:"maxStores"`
	}
	json.NewDecoder(listResp.Body).Decode(&listResult)
	if listResult.Total != 2 {
		t.Errorf("expected 2 tracked stores, got %d", listResult.Total)
	}
	if listResult.MaxStores != 2 {
		t.Errorf("expected maxStores=2, got %d", listResult.MaxStores)
	}
}

// TestE2E_PlanUpgradeUnlocksEntitlements exercises:
// Start with free plan (0 stores) → Upgrade to paid → Now can add stores
func TestE2E_PlanUpgradeUnlocksEntitlements(t *testing.T) {
	env := setupTestServer(t)
	defer env.Cleanup()
	testutil.MarkSystemInitialized(t, env.DB)

	regBody := `{"email":"upgrade@test.com","password":"StrongP@ss1!","displayName":"Upgrade User"}`
	regResp, _ := env.Client.Post(env.Server.URL+"/api/auth/register", "application/json", strings.NewReader(regBody))
	var authResp AuthResponse
	json.NewDecoder(regResp.Body).Decode(&authResp)
	regResp.Body.Close()

	tenantID := authResp.Memberships[0].TenantID
	accessToken := authResp.AccessToken
	tenantSlug := authResp.Memberships[0].TenantSlug

	// Create free plan (no entitlements)
	freePlan := testutil.CreateTestPlan(t, env.DB, "Free", 0, true)

	// Assign free plan
	env.DB.Tenants().UpdateOne(context.Background(), bson.M{"slug": tenantSlug}, bson.M{
		"$set": bson.M{
			"planId":        freePlan.ID,
			"billingStatus": models.BillingStatusActive,
		},
	})

	// Try to add store — blocked
	addBody := `{"domain":"allbirds.com"}`
	addReq, _ := http.NewRequest("POST", env.Server.URL+"/api/tracked-stores", strings.NewReader(addBody))
	addReq.Header.Set("Authorization", "Bearer "+accessToken)
	addReq.Header.Set("X-Tenant-ID", tenantID)
	addReq.Header.Set("Content-Type", "application/json")
	addResp1, _ := env.Client.Do(addReq)
	defer addResp1.Body.Close()
	if addResp1.StatusCode != http.StatusPaymentRequired {
		t.Fatalf("before upgrade: expected 402, got %d", addResp1.StatusCode)
	}

	// Create paid plan with tracked stores
	paidPlan := testutil.CreateTestPlan(t, env.DB, "Pro", 5000, false)
	env.DB.Plans().UpdateOne(context.Background(), bson.M{"_id": paidPlan.ID}, bson.M{
		"$set": bson.M{
			"entitlements": bson.M{
				"max_tracked_stores": bson.M{
					"type":         "numeric",
					"numericValue": 10,
					"description":  "Max tracked stores",
				},
			},
		},
	})

	// "Upgrade" — assign paid plan (simulating successful checkout webhook)
	env.DB.Tenants().UpdateOne(context.Background(), bson.M{"slug": tenantSlug}, bson.M{
		"$set": bson.M{
			"planId":        paidPlan.ID,
			"billingStatus": models.BillingStatusActive,
		},
	})

	// Try to add store again — should succeed now
	addReq2, _ := http.NewRequest("POST", env.Server.URL+"/api/tracked-stores", strings.NewReader(addBody))
	addReq2.Header.Set("Authorization", "Bearer "+accessToken)
	addReq2.Header.Set("X-Tenant-ID", tenantID)
	addReq2.Header.Set("Content-Type", "application/json")
	addResp2, _ := env.Client.Do(addReq2)
	defer addResp2.Body.Close()
	if addResp2.StatusCode != http.StatusCreated {
		t.Errorf("after upgrade: expected 201, got %d", addResp2.StatusCode)
	}
}

// TestE2E_UnauthenticatedCannotAccessPaidFeatures verifies auth gate.
func TestE2E_UnauthenticatedCannotAccessPaidFeatures(t *testing.T) {
	env := setupTestServer(t)
	defer env.Cleanup()
	testutil.MarkSystemInitialized(t, env.DB)

	endpoints := []struct {
		method string
		path   string
	}{
		{"GET", "/api/tracked-stores"},
		{"POST", "/api/tracked-stores"},
		{"GET", "/api/scans"},
		{"GET", "/api/billing/config"},
		{"POST", "/api/billing/checkout"},
	}

	for _, ep := range endpoints {
		req, _ := http.NewRequest(ep.method, env.Server.URL+ep.path, nil)
		resp, err := env.Client.Do(req)
		if err != nil {
			t.Fatalf("%s %s: %v", ep.method, ep.path, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("%s %s: expected 401, got %d", ep.method, ep.path, resp.StatusCode)
		}
	}
}
