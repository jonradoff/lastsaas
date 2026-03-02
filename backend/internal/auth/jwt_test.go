package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	testAccessSecret  = "test-access-secret-minimum16chars"
	testRefreshSecret = "test-refresh-secret-minimum16chars"
)

func newTestJWTService() *JWTService {
	return NewJWTService(testAccessSecret, testRefreshSecret, 15, 7)
}

func TestGenerateAccessToken(t *testing.T) {
	svc := newTestJWTService()
	token, err := svc.GenerateAccessToken("user123", "user@test.com", "Test User")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}
}

func TestGenerateRefreshToken(t *testing.T) {
	svc := newTestJWTService()
	token, err := svc.GenerateRefreshToken("user123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}
}

func TestValidateAccessToken(t *testing.T) {
	svc := newTestJWTService()
	token, _ := svc.GenerateAccessToken("user123", "user@test.com", "Test User")

	claims, err := svc.ValidateAccessToken(token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if claims.UserID != "user123" {
		t.Errorf("expected UserID 'user123', got %q", claims.UserID)
	}
	if claims.Email != "user@test.com" {
		t.Errorf("expected Email 'user@test.com', got %q", claims.Email)
	}
	if claims.DisplayName != "Test User" {
		t.Errorf("expected DisplayName 'Test User', got %q", claims.DisplayName)
	}
}

func TestValidateRefreshToken(t *testing.T) {
	svc := newTestJWTService()
	token, _ := svc.GenerateRefreshToken("user123")

	claims, err := svc.ValidateRefreshToken(token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if claims.UserID != "user123" {
		t.Errorf("expected UserID 'user123', got %q", claims.UserID)
	}
}

func TestExpiredAccessToken(t *testing.T) {
	svc := &JWTService{
		accessSecret:  []byte(testAccessSecret),
		refreshSecret: []byte(testRefreshSecret),
		accessTTL:     -1 * time.Hour, // already expired
		refreshTTL:    7 * 24 * time.Hour,
	}
	token, _ := svc.GenerateAccessToken("user123", "user@test.com", "Test User")

	_, err := svc.ValidateAccessToken(token)
	if err != ErrExpiredToken {
		t.Errorf("expected ErrExpiredToken, got %v", err)
	}
}

func TestExpiredRefreshToken(t *testing.T) {
	svc := &JWTService{
		accessSecret:  []byte(testAccessSecret),
		refreshSecret: []byte(testRefreshSecret),
		accessTTL:     15 * time.Minute,
		refreshTTL:    -1 * time.Hour, // already expired
	}
	token, _ := svc.GenerateRefreshToken("user123")

	_, err := svc.ValidateRefreshToken(token)
	if err != ErrExpiredToken {
		t.Errorf("expected ErrExpiredToken, got %v", err)
	}
}

func TestInvalidAccessTokenSignature(t *testing.T) {
	svc := newTestJWTService()
	token, _ := svc.GenerateAccessToken("user123", "user@test.com", "Test User")

	// Create another service with a different secret
	otherSvc := NewJWTService("different-secret-minimum16char", testRefreshSecret, 15, 7)
	_, err := otherSvc.ValidateAccessToken(token)
	if err != ErrInvalidToken {
		t.Errorf("expected ErrInvalidToken, got %v", err)
	}
}

func TestInvalidRefreshTokenSignature(t *testing.T) {
	svc := newTestJWTService()
	token, _ := svc.GenerateRefreshToken("user123")

	otherSvc := NewJWTService(testAccessSecret, "different-refresh-secret-min16", 15, 7)
	_, err := otherSvc.ValidateRefreshToken(token)
	if err != ErrInvalidToken {
		t.Errorf("expected ErrInvalidToken, got %v", err)
	}
}

func TestMalformedToken(t *testing.T) {
	svc := newTestJWTService()

	_, err := svc.ValidateAccessToken("not-a-valid-jwt")
	if err != ErrInvalidToken {
		t.Errorf("expected ErrInvalidToken, got %v", err)
	}
}

func TestEmptyToken(t *testing.T) {
	svc := newTestJWTService()

	_, err := svc.ValidateAccessToken("")
	if err != ErrInvalidToken {
		t.Errorf("expected ErrInvalidToken, got %v", err)
	}
}

func TestAccessTokenCantValidateAsRefresh(t *testing.T) {
	svc := newTestJWTService()
	accessToken, _ := svc.GenerateAccessToken("user123", "user@test.com", "Test User")

	// Access and refresh tokens use different secrets, so cross-validation should fail
	_, err := svc.ValidateRefreshToken(accessToken)
	if err == nil {
		t.Error("expected error when validating access token as refresh token")
	}
}

func TestMFAToken(t *testing.T) {
	svc := newTestJWTService()
	token, err := svc.GenerateMFAToken("user123", "user@test.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	claims, err := svc.ValidateAccessToken(token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !claims.MFAPending {
		t.Error("expected MFAPending to be true")
	}
	if claims.UserID != "user123" {
		t.Errorf("expected UserID 'user123', got %q", claims.UserID)
	}
}

func TestImpersonationToken(t *testing.T) {
	svc := newTestJWTService()
	token, err := svc.GenerateImpersonationToken("target123", "target@test.com", "Target User", "admin456")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	claims, err := svc.ValidateAccessToken(token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if claims.ImpersonatedBy != "admin456" {
		t.Errorf("expected ImpersonatedBy 'admin456', got %q", claims.ImpersonatedBy)
	}
}

func TestGetAccessTTL(t *testing.T) {
	svc := NewJWTService(testAccessSecret, testRefreshSecret, 30, 14)
	if svc.GetAccessTTL() != 30*time.Minute {
		t.Errorf("expected 30m, got %v", svc.GetAccessTTL())
	}
}

func TestGetRefreshTTL(t *testing.T) {
	svc := NewJWTService(testAccessSecret, testRefreshSecret, 30, 14)
	if svc.GetRefreshTTL() != 14*24*time.Hour {
		t.Errorf("expected 336h, got %v", svc.GetRefreshTTL())
	}
}

func TestDefaultTTLValues(t *testing.T) {
	svc := NewJWTService(testAccessSecret, testRefreshSecret, 0, 0)
	if svc.GetAccessTTL() != 60*time.Minute {
		t.Errorf("expected default 60m, got %v", svc.GetAccessTTL())
	}
	if svc.GetRefreshTTL() != 30*24*time.Hour {
		t.Errorf("expected default 720h, got %v", svc.GetRefreshTTL())
	}
}

func TestTokenUsesHS256(t *testing.T) {
	svc := newTestJWTService()
	tokenStr, _ := svc.GenerateAccessToken("user123", "user@test.com", "Test User")

	parser := jwt.NewParser()
	token, _, err := parser.ParseUnverified(tokenStr, &AccessTokenClaims{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token.Method.Alg() != "HS256" {
		t.Errorf("expected HS256, got %s", token.Method.Alg())
	}
}
