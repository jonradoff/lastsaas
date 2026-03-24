package handlers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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

// --- Auth Providers ---

func TestIntegration_GetProviders(t *testing.T) {
	env := setupTestServer(t)
	defer env.Cleanup()
	testutil.MarkSystemInitialized(t, env.DB)

	resp, err := env.Client.Get(env.Server.URL + "/api/auth/providers")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var providers map[string]bool
	json.NewDecoder(resp.Body).Decode(&providers)

	// Password should always be enabled
	if !providers["password"] {
		t.Error("expected password provider to be enabled")
	}
	// OAuth not configured in test env
	if providers["google"] {
		t.Error("expected google provider to be disabled in test env")
	}
}

// --- Login with inactive user ---

func TestIntegration_LoginInactiveUser(t *testing.T) {
	env := setupTestServer(t)
	defer env.Cleanup()
	testutil.MarkSystemInitialized(t, env.DB)

	user := testutil.CreateTestUser(t, env.DB, "inactive@test.com", "StrongP@ss1!", "Inactive User")
	// Deactivate user
	env.DB.Users().UpdateOne(context.Background(), bson.M{"_id": user.ID}, bson.M{
		"$set": bson.M{"isActive": false},
	})

	body := `{"email":"inactive@test.com","password":"StrongP@ss1!"}`
	resp, err := env.Client.Post(env.Server.URL+"/api/auth/login", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 for inactive user, got %d", resp.StatusCode)
	}
}

// --- Account lockout after failed attempts ---

func TestIntegration_LoginAccountLockout(t *testing.T) {
	env := setupTestServer(t)
	defer env.Cleanup()
	testutil.MarkSystemInitialized(t, env.DB)

	testutil.CreateTestUser(t, env.DB, "lockout@test.com", "StrongP@ss1!", "Lockout User")

	// Fail 5 times to trigger lockout
	for i := 0; i < 5; i++ {
		body := `{"email":"lockout@test.com","password":"WrongPassword1!"}`
		resp, _ := env.Client.Post(env.Server.URL+"/api/auth/login", "application/json", strings.NewReader(body))
		resp.Body.Close()
	}

	// 6th attempt should still fail — account locked
	body := `{"email":"lockout@test.com","password":"StrongP@ss1!"}`
	resp, err := env.Client.Post(env.Server.URL+"/api/auth/login", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	// Should be 429 (locked) or 401 (still rejected)
	if resp.StatusCode == http.StatusOK {
		t.Error("expected login to fail after lockout, but got 200")
	}
}

// --- MFA Setup Flow ---

func TestIntegration_MFASetup_NotAlreadyEnabled(t *testing.T) {
	env := setupTestServer(t)
	defer env.Cleanup()
	testutil.MarkSystemInitialized(t, env.DB)

	user := testutil.CreateTestUser(t, env.DB, "mfa@test.com", "StrongP@ss1!", "MFA User")
	req := env.authenticatedRequest(t, "POST", "/api/auth/mfa/setup", nil, user)
	resp, err := env.Client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, testutil.ReadResponseBody(t, resp))
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if result["secret"] == nil || result["secret"] == "" {
		t.Error("expected non-empty TOTP secret")
	}
}

func TestIntegration_MFASetup_AlreadyEnabled(t *testing.T) {
	env := setupTestServer(t)
	defer env.Cleanup()
	testutil.MarkSystemInitialized(t, env.DB)

	user := testutil.CreateTestUser(t, env.DB, "mfa2@test.com", "StrongP@ss1!", "MFA User 2")
	// Mark MFA as already enabled
	env.DB.Users().UpdateOne(context.Background(), bson.M{"_id": user.ID}, bson.M{
		"$set": bson.M{"totpEnabled": true},
	})

	req := env.authenticatedRequest(t, "POST", "/api/auth/mfa/setup", nil, user)
	resp, err := env.Client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusConflict {
		t.Errorf("expected 409 for already-enabled MFA, got %d", resp.StatusCode)
	}
}

func TestIntegration_MFAVerifySetup_NoSetupInitiated(t *testing.T) {
	env := setupTestServer(t)
	defer env.Cleanup()
	testutil.MarkSystemInitialized(t, env.DB)

	user := testutil.CreateTestUser(t, env.DB, "mfa3@test.com", "StrongP@ss1!", "MFA User 3")
	body := strings.NewReader(`{"code":"123456"}`)
	req := env.authenticatedRequest(t, "POST", "/api/auth/mfa/verify-setup", body, user)
	resp, err := env.Client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 when no setup initiated, got %d", resp.StatusCode)
	}
}

func TestIntegration_MFADisable_NotEnabled(t *testing.T) {
	env := setupTestServer(t)
	defer env.Cleanup()
	testutil.MarkSystemInitialized(t, env.DB)

	user := testutil.CreateTestUser(t, env.DB, "mfa4@test.com", "StrongP@ss1!", "MFA User 4")
	body := strings.NewReader(`{"code":"123456"}`)
	req := env.authenticatedRequest(t, "POST", "/api/auth/mfa/disable", body, user)
	resp, err := env.Client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 when MFA not enabled, got %d", resp.StatusCode)
	}
}

func TestIntegration_MFAChallenge_MissingFields(t *testing.T) {
	env := setupTestServer(t)
	defer env.Cleanup()
	testutil.MarkSystemInitialized(t, env.DB)

	body := `{"mfaToken":"","code":""}`
	resp, err := env.Client.Post(env.Server.URL+"/api/auth/mfa/challenge", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for missing fields, got %d", resp.StatusCode)
	}
}

func TestIntegration_MFAChallenge_InvalidToken(t *testing.T) {
	env := setupTestServer(t)
	defer env.Cleanup()
	testutil.MarkSystemInitialized(t, env.DB)

	body := `{"mfaToken":"invalid-jwt-token","code":"123456"}`
	resp, err := env.Client.Post(env.Server.URL+"/api/auth/mfa/challenge", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 for invalid MFA token, got %d", resp.StatusCode)
	}
}

// --- Email Verification ---

func TestIntegration_VerifyEmail_InvalidToken(t *testing.T) {
	env := setupTestServer(t)
	defer env.Cleanup()
	testutil.MarkSystemInitialized(t, env.DB)

	body := `{"token":"nonexistent-token"}`
	resp, err := env.Client.Post(env.Server.URL+"/api/auth/verify-email", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid token, got %d", resp.StatusCode)
	}
}

func TestIntegration_VerifyEmail_EmptyToken(t *testing.T) {
	env := setupTestServer(t)
	defer env.Cleanup()
	testutil.MarkSystemInitialized(t, env.DB)

	body := `{"token":""}`
	resp, err := env.Client.Post(env.Server.URL+"/api/auth/verify-email", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for empty token, got %d", resp.StatusCode)
	}
}

func TestIntegration_VerifyEmail_ValidToken(t *testing.T) {
	env := setupTestServer(t)
	defer env.Cleanup()
	testutil.MarkSystemInitialized(t, env.DB)

	user := testutil.CreateTestUser(t, env.DB, "verify@test.com", "StrongP@ss1!", "Verify User")
	// Mark user as unverified
	env.DB.Users().UpdateOne(context.Background(), bson.M{"_id": user.ID}, bson.M{
		"$set": bson.M{"emailVerified": false},
	})

	// Create a verification token
	rawToken := "test-verify-token-123"
	h := sha256.Sum256([]byte(rawToken))
	hashedToken := hex.EncodeToString(h[:])
	env.DB.VerificationTokens().InsertOne(context.Background(), models.VerificationToken{
		ID:        primitive.NewObjectID(),
		UserID:    user.ID,
		Token:     hashedToken,
		Type:      models.TokenTypeEmailVerification,
		ExpiresAt: time.Now().Add(24 * time.Hour),
		CreatedAt: time.Now(),
	})

	body := `{"token":"` + rawToken + `"}`
	resp, err := env.Client.Post(env.Server.URL+"/api/auth/verify-email", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", resp.StatusCode, testutil.ReadResponseBody(t, resp))
	}

	// Verify user is now marked as verified
	var updated models.User
	env.DB.Users().FindOne(context.Background(), bson.M{"_id": user.ID}).Decode(&updated)
	if !updated.EmailVerified {
		t.Error("expected user to be email verified after token usage")
	}
}

func TestIntegration_VerifyEmail_ExpiredToken(t *testing.T) {
	env := setupTestServer(t)
	defer env.Cleanup()
	testutil.MarkSystemInitialized(t, env.DB)

	user := testutil.CreateTestUser(t, env.DB, "expired@test.com", "StrongP@ss1!", "Expired Token User")

	rawToken := "expired-token-123"
	h := sha256.Sum256([]byte(rawToken))
	hashedToken := hex.EncodeToString(h[:])
	env.DB.VerificationTokens().InsertOne(context.Background(), models.VerificationToken{
		ID:        primitive.NewObjectID(),
		UserID:    user.ID,
		Token:     hashedToken,
		Type:      models.TokenTypeEmailVerification,
		ExpiresAt: time.Now().Add(-1 * time.Hour), // already expired
		CreatedAt: time.Now().Add(-2 * time.Hour),
	})

	body := `{"token":"` + rawToken + `"}`
	resp, err := env.Client.Post(env.Server.URL+"/api/auth/verify-email", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for expired token, got %d", resp.StatusCode)
	}
}

// --- Resend Verification ---

func TestIntegration_ResendVerification_NonexistentEmail(t *testing.T) {
	env := setupTestServer(t)
	defer env.Cleanup()
	testutil.MarkSystemInitialized(t, env.DB)

	// Should return 200 regardless (no user enumeration)
	body := `{"email":"nobody@test.com"}`
	resp, err := env.Client.Post(env.Server.URL+"/api/auth/resend-verification", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 (no user enumeration), got %d", resp.StatusCode)
	}
}

func TestIntegration_ResendVerification_AlreadyVerified(t *testing.T) {
	env := setupTestServer(t)
	defer env.Cleanup()
	testutil.MarkSystemInitialized(t, env.DB)

	testutil.CreateTestUser(t, env.DB, "verified@test.com", "StrongP@ss1!", "Verified User")

	body := `{"email":"verified@test.com"}`
	resp, err := env.Client.Post(env.Server.URL+"/api/auth/resend-verification", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	// Should still return 200 (no enumeration)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestIntegration_ResendVerification_EmptyEmail(t *testing.T) {
	env := setupTestServer(t)
	defer env.Cleanup()
	testutil.MarkSystemInitialized(t, env.DB)

	body := `{"email":""}`
	resp, err := env.Client.Post(env.Server.URL+"/api/auth/resend-verification", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for empty email, got %d", resp.StatusCode)
	}
}

// --- Forgot Password ---

func TestIntegration_ForgotPassword_ReturnsOK(t *testing.T) {
	env := setupTestServer(t)
	defer env.Cleanup()
	testutil.MarkSystemInitialized(t, env.DB)

	testutil.CreateTestUser(t, env.DB, "forgot@test.com", "StrongP@ss1!", "Forgot User")

	body := `{"email":"forgot@test.com"}`
	resp, err := env.Client.Post(env.Server.URL+"/api/auth/forgot-password", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestIntegration_ForgotPassword_NonexistentEmail(t *testing.T) {
	env := setupTestServer(t)
	defer env.Cleanup()
	testutil.MarkSystemInitialized(t, env.DB)

	// Should return 200 regardless (no user enumeration)
	body := `{"email":"nobody@test.com"}`
	resp, err := env.Client.Post(env.Server.URL+"/api/auth/forgot-password", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 (no enumeration), got %d", resp.StatusCode)
	}
}

func TestIntegration_ForgotPassword_EmptyEmail(t *testing.T) {
	env := setupTestServer(t)
	defer env.Cleanup()
	testutil.MarkSystemInitialized(t, env.DB)

	body := `{"email":""}`
	resp, err := env.Client.Post(env.Server.URL+"/api/auth/forgot-password", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for empty email, got %d", resp.StatusCode)
	}
}

// --- Reset Password ---

func TestIntegration_ResetPassword_InvalidToken(t *testing.T) {
	env := setupTestServer(t)
	defer env.Cleanup()
	testutil.MarkSystemInitialized(t, env.DB)

	body := `{"token":"invalid-reset-token","newPassword":"NewStr0ng!Pass"}`
	resp, err := env.Client.Post(env.Server.URL+"/api/auth/reset-password", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid reset token, got %d", resp.StatusCode)
	}
}

func TestIntegration_ResetPassword_WeakNewPassword(t *testing.T) {
	env := setupTestServer(t)
	defer env.Cleanup()
	testutil.MarkSystemInitialized(t, env.DB)

	body := `{"token":"some-token","newPassword":"weak"}`
	resp, err := env.Client.Post(env.Server.URL+"/api/auth/reset-password", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for weak password, got %d", resp.StatusCode)
	}
}

func TestIntegration_ResetPassword_MissingFields(t *testing.T) {
	env := setupTestServer(t)
	defer env.Cleanup()
	testutil.MarkSystemInitialized(t, env.DB)

	body := `{"token":"","newPassword":""}`
	resp, err := env.Client.Post(env.Server.URL+"/api/auth/reset-password", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for missing fields, got %d", resp.StatusCode)
	}
}

func TestIntegration_ResetPassword_ValidToken(t *testing.T) {
	env := setupTestServer(t)
	defer env.Cleanup()
	testutil.MarkSystemInitialized(t, env.DB)

	user := testutil.CreateTestUser(t, env.DB, "resetpw@test.com", "StrongP@ss1!", "Reset User")

	rawToken := "test-reset-token-456"
	h := sha256.Sum256([]byte(rawToken))
	hashedToken := hex.EncodeToString(h[:])
	env.DB.VerificationTokens().InsertOne(context.Background(), models.VerificationToken{
		ID:        primitive.NewObjectID(),
		UserID:    user.ID,
		Token:     hashedToken,
		Type:      models.TokenTypePasswordReset,
		ExpiresAt: time.Now().Add(30 * time.Minute),
		CreatedAt: time.Now(),
	})

	body := `{"token":"` + rawToken + `","newPassword":"BrandNewP@ss1!"}`
	resp, err := env.Client.Post(env.Server.URL+"/api/auth/reset-password", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", resp.StatusCode, testutil.ReadResponseBody(t, resp))
	}

	// Verify old password no longer works
	loginBody := `{"email":"resetpw@test.com","password":"StrongP@ss1!"}`
	loginResp, _ := env.Client.Post(env.Server.URL+"/api/auth/login", "application/json", strings.NewReader(loginBody))
	defer loginResp.Body.Close()
	if loginResp.StatusCode == http.StatusOK {
		t.Error("old password should no longer work after reset")
	}

	// Verify new password works
	loginBody2 := `{"email":"resetpw@test.com","password":"BrandNewP@ss1!"}`
	loginResp2, _ := env.Client.Post(env.Server.URL+"/api/auth/login", "application/json", strings.NewReader(loginBody2))
	defer loginResp2.Body.Close()
	if loginResp2.StatusCode != http.StatusOK {
		t.Errorf("new password should work after reset, got %d", loginResp2.StatusCode)
	}
}

// --- Sessions ---

func TestIntegration_ListSessions(t *testing.T) {
	env := setupTestServer(t)
	defer env.Cleanup()
	testutil.MarkSystemInitialized(t, env.DB)

	// Register to create a session
	regBody := `{"email":"sessions@test.com","password":"StrongP@ss1!","displayName":"Sessions User"}`
	regResp, _ := env.Client.Post(env.Server.URL+"/api/auth/register", "application/json", strings.NewReader(regBody))
	var authResp AuthResponse
	json.NewDecoder(regResp.Body).Decode(&authResp)
	regResp.Body.Close()

	req, _ := http.NewRequest("GET", env.Server.URL+"/api/auth/sessions", nil)
	req.Header.Set("Authorization", "Bearer "+authResp.AccessToken)
	resp, err := env.Client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	sessions, ok := result["sessions"].([]interface{})
	if !ok {
		t.Fatal("expected sessions array in response")
	}
	if len(sessions) == 0 {
		t.Error("expected at least one session after registration")
	}
}

func TestIntegration_RevokeAllSessions(t *testing.T) {
	env := setupTestServer(t)
	defer env.Cleanup()
	testutil.MarkSystemInitialized(t, env.DB)

	regBody := `{"email":"revoke@test.com","password":"StrongP@ss1!","displayName":"Revoke User"}`
	regResp, _ := env.Client.Post(env.Server.URL+"/api/auth/register", "application/json", strings.NewReader(regBody))
	var authResp AuthResponse
	json.NewDecoder(regResp.Body).Decode(&authResp)
	regResp.Body.Close()

	req, _ := http.NewRequest("DELETE", env.Server.URL+"/api/auth/sessions", nil)
	req.Header.Set("Authorization", "Bearer "+authResp.AccessToken)
	resp, err := env.Client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

// --- Preferences ---

func TestIntegration_UpdatePreferences(t *testing.T) {
	env := setupTestServer(t)
	defer env.Cleanup()
	testutil.MarkSystemInitialized(t, env.DB)

	user := testutil.CreateTestUser(t, env.DB, "prefs@test.com", "StrongP@ss1!", "Prefs User")
	body := strings.NewReader(`{"themePreference":"dark"}`)
	req := env.authenticatedRequest(t, "PATCH", "/api/auth/preferences", body, user)
	resp, err := env.Client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", resp.StatusCode, testutil.ReadResponseBody(t, resp))
	}
}

// --- Onboarding ---

func TestIntegration_CompleteOnboarding(t *testing.T) {
	env := setupTestServer(t)
	defer env.Cleanup()
	testutil.MarkSystemInitialized(t, env.DB)

	user := testutil.CreateTestUser(t, env.DB, "onboard@test.com", "StrongP@ss1!", "Onboard User")
	req := env.authenticatedRequest(t, "POST", "/api/auth/complete-onboarding", nil, user)
	resp, err := env.Client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

// --- Login returns MFA challenge when TOTP enabled ---

func TestIntegration_LoginWithMFAEnabled(t *testing.T) {
	env := setupTestServer(t)
	defer env.Cleanup()
	testutil.MarkSystemInitialized(t, env.DB)

	user := testutil.CreateTestUser(t, env.DB, "mfalogin@test.com", "StrongP@ss1!", "MFA Login User")
	// Enable TOTP on this user
	env.DB.Users().UpdateOne(context.Background(), bson.M{"_id": user.ID}, bson.M{
		"$set": bson.M{"totpEnabled": true, "totpSecret": "JBSWY3DPEHPK3PXP"},
	})

	body := `{"email":"mfalogin@test.com","password":"StrongP@ss1!"}`
	resp, err := env.Client.Post(env.Server.URL+"/api/auth/login", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if result["mfaRequired"] != true {
		t.Error("expected mfaRequired=true when TOTP is enabled")
	}
	if result["mfaToken"] == nil || result["mfaToken"] == "" {
		t.Error("expected non-empty mfaToken")
	}
	// Should NOT return access/refresh tokens
	if result["accessToken"] != nil {
		t.Error("should not return accessToken when MFA is required")
	}
}

// --- Register with invitation token (invalid) ---

func TestIntegration_RegisterWithInvalidInvitation(t *testing.T) {
	env := setupTestServer(t)
	defer env.Cleanup()
	testutil.MarkSystemInitialized(t, env.DB)

	body := `{"email":"invited@test.com","password":"StrongP@ss1!","displayName":"Invited User","invitationToken":"nonexistent"}`
	resp, err := env.Client.Post(env.Server.URL+"/api/auth/register", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	// Registration should still succeed even if invitation fails — user gets a personal tenant
	if resp.StatusCode != http.StatusCreated {
		t.Errorf("expected 201 (register succeeds despite bad invitation), got %d", resp.StatusCode)
	}
}

// --- Token refresh revokes old token ---

func TestIntegration_RefreshTokenRevokesOld(t *testing.T) {
	env := setupTestServer(t)
	defer env.Cleanup()
	testutil.MarkSystemInitialized(t, env.DB)

	regBody := `{"email":"tokenrotate@test.com","password":"StrongP@ss1!","displayName":"Rotate User"}`
	regResp, _ := env.Client.Post(env.Server.URL+"/api/auth/register", "application/json", strings.NewReader(regBody))
	var authResp AuthResponse
	json.NewDecoder(regResp.Body).Decode(&authResp)
	regResp.Body.Close()

	// First refresh should succeed
	refreshBody := `{"refreshToken":"` + authResp.RefreshToken + `"}`
	resp1, _ := env.Client.Post(env.Server.URL+"/api/auth/refresh", "application/json", strings.NewReader(refreshBody))
	defer resp1.Body.Close()
	if resp1.StatusCode != http.StatusOK {
		t.Fatalf("first refresh should succeed, got %d", resp1.StatusCode)
	}

	// Using the same refresh token again should fail (it was revoked)
	resp2, _ := env.Client.Post(env.Server.URL+"/api/auth/refresh", "application/json", strings.NewReader(refreshBody))
	defer resp2.Body.Close()
	if resp2.StatusCode == http.StatusOK {
		t.Error("second use of same refresh token should fail (token rotation)")
	}
}

// --- Login trims and lowercases email ---

func TestIntegration_LoginEmailNormalization(t *testing.T) {
	env := setupTestServer(t)
	defer env.Cleanup()
	testutil.MarkSystemInitialized(t, env.DB)

	testutil.CreateTestUser(t, env.DB, "normalize@test.com", "StrongP@ss1!", "Normalize User")

	body := `{"email":"  Normalize@Test.COM  ","password":"StrongP@ss1!"}`
	resp, err := env.Client.Post(env.Server.URL+"/api/auth/login", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 with normalized email, got %d", resp.StatusCode)
	}
}
