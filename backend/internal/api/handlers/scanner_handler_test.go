package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"lastsaas/internal/testutil"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// --- Scan endpoints ---

func TestIntegration_TriggerScan_MissingDomain(t *testing.T) {
	env := setupTestServer(t)
	defer env.Cleanup()
	testutil.MarkSystemInitialized(t, env.DB)

	body := `{"domain":""}`
	resp, err := env.Client.Post(env.Server.URL+"/api/scan", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for empty domain, got %d", resp.StatusCode)
	}
}

func TestIntegration_TriggerScan_InvalidBody(t *testing.T) {
	env := setupTestServer(t)
	defer env.Cleanup()
	testutil.MarkSystemInitialized(t, env.DB)

	resp, err := env.Client.Post(env.Server.URL+"/api/scan", "application/json", strings.NewReader("not json"))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid body, got %d", resp.StatusCode)
	}
}

func TestIntegration_GetScan_NotFound(t *testing.T) {
	env := setupTestServer(t)
	defer env.Cleanup()
	testutil.MarkSystemInitialized(t, env.DB)

	fakeID := primitive.NewObjectID().Hex()
	resp, err := env.Client.Get(env.Server.URL + "/api/scan/" + fakeID)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestIntegration_GetLatestDomainScan_NotFound(t *testing.T) {
	env := setupTestServer(t)
	defer env.Cleanup()
	testutil.MarkSystemInitialized(t, env.DB)

	resp, err := env.Client.Get(env.Server.URL + "/api/scan/domain/nonexistent-domain.com")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

// --- Tracked stores ---

func TestIntegration_TrackedStores_NoAuth(t *testing.T) {
	env := setupTestServer(t)
	defer env.Cleanup()
	testutil.MarkSystemInitialized(t, env.DB)

	resp, err := env.Client.Get(env.Server.URL + "/api/tracked-stores")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestIntegration_TrackedStores_NoPlan(t *testing.T) {
	env := setupTestServer(t)
	defer env.Cleanup()
	testutil.MarkSystemInitialized(t, env.DB)

	owner := testutil.CreateTestUser(t, env.DB, "noplan@test.com", "Test1234!@#$", "No Plan User")
	tenant := testutil.CreateTestTenant(t, env.DB, "NoPlan Tenant", owner.ID, false)

	// Try to add tracked store without a plan
	body := strings.NewReader(`{"domain":"allbirds.com"}`)
	req := env.tenantRequest(t, "POST", "/api/tracked-stores", body, owner, tenant.ID.Hex())
	resp, err := env.Client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusPaymentRequired {
		t.Errorf("expected 402 (no plan), got %d", resp.StatusCode)
	}
}

func TestIntegration_TrackedStores_EmptyDomain(t *testing.T) {
	env := setupTestServer(t)
	defer env.Cleanup()
	testutil.MarkSystemInitialized(t, env.DB)

	owner := testutil.CreateTestUser(t, env.DB, "empty@test.com", "Test1234!@#$", "Empty Domain User")
	tenant := testutil.CreateTestTenant(t, env.DB, "Empty Tenant", owner.ID, false)

	body := strings.NewReader(`{"domain":""}`)
	req := env.tenantRequest(t, "POST", "/api/tracked-stores", body, owner, tenant.ID.Hex())
	resp, err := env.Client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestIntegration_ListTrackedStores_Empty(t *testing.T) {
	env := setupTestServer(t)
	defer env.Cleanup()
	testutil.MarkSystemInitialized(t, env.DB)

	owner := testutil.CreateTestUser(t, env.DB, "listempty@test.com", "Test1234!@#$", "List Empty User")
	tenant := testutil.CreateTestTenant(t, env.DB, "ListEmpty Tenant", owner.ID, false)

	req := env.tenantRequest(t, "GET", "/api/tracked-stores", nil, owner, tenant.ID.Hex())
	resp, err := env.Client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result struct {
		Stores    []interface{} `json:"stores"`
		Total     int           `json:"total"`
		MaxStores int64         `json:"maxStores"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	if result.Total != 0 {
		t.Errorf("expected 0 tracked stores, got %d", result.Total)
	}
}

func TestIntegration_ListScans_Empty(t *testing.T) {
	env := setupTestServer(t)
	defer env.Cleanup()
	testutil.MarkSystemInitialized(t, env.DB)

	owner := testutil.CreateTestUser(t, env.DB, "scanlist@test.com", "Test1234!@#$", "Scan List User")
	tenant := testutil.CreateTestTenant(t, env.DB, "ScanList Tenant", owner.ID, false)

	req := env.tenantRequest(t, "GET", "/api/scans", nil, owner, tenant.ID.Hex())
	resp, err := env.Client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result struct {
		Scans []interface{} `json:"scans"`
		Total int64         `json:"total"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	if result.Total != 0 {
		t.Errorf("expected 0 scans, got %d", result.Total)
	}
}

func TestIntegration_RemoveTrackedStore_NotFound(t *testing.T) {
	env := setupTestServer(t)
	defer env.Cleanup()
	testutil.MarkSystemInitialized(t, env.DB)

	owner := testutil.CreateTestUser(t, env.DB, "removefail@test.com", "Test1234!@#$", "Remove Fail User")
	tenant := testutil.CreateTestTenant(t, env.DB, "RemoveFail Tenant", owner.ID, false)

	fakeID := primitive.NewObjectID().Hex()
	req := env.tenantRequest(t, "DELETE", "/api/tracked-stores/"+fakeID, nil, owner, tenant.ID.Hex())
	resp, err := env.Client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}
