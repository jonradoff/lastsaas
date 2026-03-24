package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"lastsaas/internal/testutil"
)

// TestE2E_FullRegistrationToSessionFlow exercises the complete auth lifecycle:
// Register → Login → Get Me → List Sessions → Change Password → Login with new → Logout
func TestE2E_FullRegistrationToSessionFlow(t *testing.T) {
	env := setupTestServer(t)
	defer env.Cleanup()
	testutil.MarkSystemInitialized(t, env.DB)

	// Step 1: Register
	regBody := `{"email":"e2e@test.com","password":"StrongP@ss1!","displayName":"E2E User"}`
	regResp, err := env.Client.Post(env.Server.URL+"/api/auth/register", "application/json", strings.NewReader(regBody))
	if err != nil {
		t.Fatal(err)
	}
	var authResp AuthResponse
	json.NewDecoder(regResp.Body).Decode(&authResp)
	regResp.Body.Close()

	if regResp.StatusCode != http.StatusCreated {
		t.Fatalf("register: expected 201, got %d", regResp.StatusCode)
	}
	if authResp.AccessToken == "" || authResp.RefreshToken == "" {
		t.Fatal("register: expected tokens in response")
	}
	if authResp.User == nil || authResp.User.Email != "e2e@test.com" {
		t.Fatal("register: expected user in response")
	}
	if len(authResp.Memberships) == 0 {
		t.Fatal("register: expected at least one membership (personal tenant)")
	}

	accessToken := authResp.AccessToken
	refreshToken := authResp.RefreshToken

	// Step 2: Get Me with access token
	meReq, _ := http.NewRequest("GET", env.Server.URL+"/api/auth/me", nil)
	meReq.Header.Set("Authorization", "Bearer "+accessToken)
	meResp, err := env.Client.Do(meReq)
	if err != nil {
		t.Fatal(err)
	}
	defer meResp.Body.Close()
	if meResp.StatusCode != http.StatusOK {
		t.Fatalf("get me: expected 200, got %d", meResp.StatusCode)
	}

	var meResult struct {
		User struct {
			Email       string `json:"email"`
			DisplayName string `json:"displayName"`
		} `json:"user"`
	}
	json.NewDecoder(meResp.Body).Decode(&meResult)
	if meResult.User.Email != "e2e@test.com" {
		t.Errorf("get me: expected email 'e2e@test.com', got '%s'", meResult.User.Email)
	}

	// Step 3: List sessions
	sessReq, _ := http.NewRequest("GET", env.Server.URL+"/api/auth/sessions", nil)
	sessReq.Header.Set("Authorization", "Bearer "+accessToken)
	sessResp, _ := env.Client.Do(sessReq)
	defer sessResp.Body.Close()
	if sessResp.StatusCode != http.StatusOK {
		t.Fatalf("list sessions: expected 200, got %d", sessResp.StatusCode)
	}

	// Step 4: Refresh token
	refreshBody := `{"refreshToken":"` + refreshToken + `"}`
	refResp, _ := env.Client.Post(env.Server.URL+"/api/auth/refresh", "application/json", strings.NewReader(refreshBody))
	var newAuth AuthResponse
	json.NewDecoder(refResp.Body).Decode(&newAuth)
	refResp.Body.Close()
	if refResp.StatusCode != http.StatusOK {
		t.Fatalf("refresh: expected 200, got %d", refResp.StatusCode)
	}
	if newAuth.AccessToken == "" {
		t.Fatal("refresh: expected new access token")
	}
	newAccessToken := newAuth.AccessToken

	// Step 5: Change password using new token
	cpBody := strings.NewReader(`{"currentPassword":"StrongP@ss1!","newPassword":"NewStr0ng!P@ss"}`)
	cpReq, _ := http.NewRequest("POST", env.Server.URL+"/api/auth/change-password", cpBody)
	cpReq.Header.Set("Authorization", "Bearer "+newAccessToken)
	cpReq.Header.Set("Content-Type", "application/json")
	cpResp, _ := env.Client.Do(cpReq)
	defer cpResp.Body.Close()
	if cpResp.StatusCode != http.StatusOK {
		t.Fatalf("change password: expected 200, got %d", cpResp.StatusCode)
	}

	// Step 6: Login with NEW password
	loginBody := `{"email":"e2e@test.com","password":"NewStr0ng!P@ss"}`
	loginResp, _ := env.Client.Post(env.Server.URL+"/api/auth/login", "application/json", strings.NewReader(loginBody))
	defer loginResp.Body.Close()
	if loginResp.StatusCode != http.StatusOK {
		t.Fatalf("login with new password: expected 200, got %d", loginResp.StatusCode)
	}

	var loginAuth AuthResponse
	json.NewDecoder(loginResp.Body).Decode(&loginAuth)

	// Step 7: Login with OLD password should fail
	oldLoginBody := `{"email":"e2e@test.com","password":"StrongP@ss1!"}`
	oldLoginResp, _ := env.Client.Post(env.Server.URL+"/api/auth/login", "application/json", strings.NewReader(oldLoginBody))
	defer oldLoginResp.Body.Close()
	if oldLoginResp.StatusCode == http.StatusOK {
		t.Error("login with old password should fail after change")
	}

	// Step 8: Logout
	logoutReq, _ := http.NewRequest("POST", env.Server.URL+"/api/auth/logout", nil)
	logoutReq.Header.Set("Authorization", "Bearer "+loginAuth.AccessToken)
	logoutResp, _ := env.Client.Do(logoutReq)
	defer logoutResp.Body.Close()
	if logoutResp.StatusCode != http.StatusOK {
		t.Fatalf("logout: expected 200, got %d", logoutResp.StatusCode)
	}
}

// TestE2E_RegistrationCreatesPersonalTenant verifies that registering creates
// a personal tenant and membership automatically.
func TestE2E_RegistrationCreatesPersonalTenant(t *testing.T) {
	env := setupTestServer(t)
	defer env.Cleanup()
	testutil.MarkSystemInitialized(t, env.DB)

	regBody := `{"email":"tenant@test.com","password":"StrongP@ss1!","displayName":"Tenant User"}`
	regResp, _ := env.Client.Post(env.Server.URL+"/api/auth/register", "application/json", strings.NewReader(regBody))
	var authResp AuthResponse
	json.NewDecoder(regResp.Body).Decode(&authResp)
	regResp.Body.Close()

	if len(authResp.Memberships) == 0 {
		t.Fatal("expected auto-created tenant membership")
	}

	membership := authResp.Memberships[0]
	if membership.Role != "owner" {
		t.Errorf("expected role 'owner', got '%s'", membership.Role)
	}
	if membership.TenantName == "" {
		t.Error("expected non-empty tenant name")
	}
}
