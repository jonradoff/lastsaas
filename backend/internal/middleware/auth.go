package middleware

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"log"
	"net/http"
	"strings"
	"time"

	"lastsaas/internal/auth"
	"lastsaas/internal/db"
	"lastsaas/internal/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type contextKey string

const (
	UserContextKey           contextKey = "user"
	APIKeyContextKey         contextKey = "apikey"
	ImpersonatedByContextKey contextKey = "impersonatedBy"
)

type AuthMiddleware struct {
	jwtService *auth.JWTService
	db         *db.MongoDB
}

func NewAuthMiddleware(jwtService *auth.JWTService, database *db.MongoDB) *AuthMiddleware {
	return &AuthMiddleware{
		jwtService: jwtService,
		db:         database,
	}
}

func (m *AuthMiddleware) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, `{"error":"Authorization header required"}`, http.StatusUnauthorized)
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			http.Error(w, `{"error":"Invalid authorization header format"}`, http.StatusUnauthorized)
			return
		}

		tokenString := parts[1]

		// API key authentication
		if strings.HasPrefix(tokenString, "lsk_") {
			m.authenticateAPIKey(w, r, next, tokenString)
			return
		}

		// JWT authentication
		m.authenticateJWT(w, r, next, tokenString)
	})
}

func (m *AuthMiddleware) authenticateJWT(w http.ResponseWriter, r *http.Request, next http.Handler, tokenString string) {
	claims, err := m.jwtService.ValidateAccessToken(tokenString)
	if err != nil {
		if err == auth.ErrExpiredToken {
			http.Error(w, `{"error":"Token has expired"}`, http.StatusUnauthorized)
			return
		}
		http.Error(w, `{"error":"Invalid token"}`, http.StatusUnauthorized)
		return
	}

	if m.isTokenRevoked(r.Context(), tokenString) {
		http.Error(w, `{"error":"Token has been revoked"}`, http.StatusUnauthorized)
		return
	}

	userID, err := primitive.ObjectIDFromHex(claims.UserID)
	if err != nil {
		http.Error(w, `{"error":"Invalid user ID"}`, http.StatusUnauthorized)
		return
	}

	var user models.User
	err = m.db.Users().FindOne(r.Context(), bson.M{"_id": userID}).Decode(&user)
	if err != nil {
		http.Error(w, `{"error":"User not found"}`, http.StatusUnauthorized)
		return
	}

	if !user.IsActive {
		http.Error(w, `{"error":"User account is inactive"}`, http.StatusUnauthorized)
		return
	}

	ctx := context.WithValue(r.Context(), UserContextKey, &user)
	if claims.ImpersonatedBy != "" {
		ctx = context.WithValue(ctx, ImpersonatedByContextKey, claims.ImpersonatedBy)
	}
	next.ServeHTTP(w, r.WithContext(ctx))
}

func (m *AuthMiddleware) authenticateAPIKey(w http.ResponseWriter, r *http.Request, next http.Handler, rawKey string) {
	hash := sha256.Sum256([]byte(rawKey))
	keyHash := base64.RawURLEncoding.EncodeToString(hash[:])

	var apiKey models.APIKey
	err := m.db.APIKeys().FindOne(r.Context(), bson.M{
		"keyHash":  keyHash,
		"isActive": true,
	}).Decode(&apiKey)
	if err != nil {
		http.Error(w, `{"error":"Invalid API key"}`, http.StatusUnauthorized)
		return
	}

	// Look up key creator
	var user models.User
	err = m.db.Users().FindOne(r.Context(), bson.M{"_id": apiKey.CreatedBy}).Decode(&user)
	if err != nil || !user.IsActive {
		http.Error(w, `{"error":"API key owner account is inactive"}`, http.StatusUnauthorized)
		return
	}

	ctx := context.WithValue(r.Context(), UserContextKey, &user)

	// Admin keys: auto-resolve root tenant + admin membership
	if apiKey.Authority == models.APIKeyAuthorityAdmin {
		var rootTenant models.Tenant
		err := m.db.Tenants().FindOne(r.Context(), bson.M{"isRoot": true}).Decode(&rootTenant)
		if err != nil {
			http.Error(w, `{"error":"System configuration error"}`, http.StatusInternalServerError)
			return
		}
		ctx = context.WithValue(ctx, TenantContextKey, &rootTenant)
		ctx = context.WithValue(ctx, MembershipContextKey, &models.TenantMembership{
			UserID:   user.ID,
			TenantID: rootTenant.ID,
			Role:     models.RoleAdmin,
		})
	}

	ctx = context.WithValue(ctx, APIKeyContextKey, &apiKey)

	// Update lastUsedAt asynchronously
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		now := time.Now()
		_, _ = m.db.APIKeys().UpdateByID(ctx, apiKey.ID,
			bson.M{"$set": bson.M{"lastUsedAt": now}})
	}()

	next.ServeHTTP(w, r.WithContext(ctx))
}

func (m *AuthMiddleware) isTokenRevoked(ctx context.Context, rawToken string) bool {
	hash := sha256.Sum256([]byte(rawToken))
	tokenHash := base64.StdEncoding.EncodeToString(hash[:])

	count, err := m.db.RevokedTokens().CountDocuments(ctx, bson.M{"tokenHash": tokenHash})
	if err != nil {
		log.Printf("Warning: revoked-token lookup failed (denying access): %v", err)
		return true
	}
	return count > 0
}

func GetUserFromContext(ctx context.Context) (*models.User, bool) {
	user, ok := ctx.Value(UserContextKey).(*models.User)
	return user, ok
}

func GetAPIKeyFromContext(ctx context.Context) (*models.APIKey, bool) {
	key, ok := ctx.Value(APIKeyContextKey).(*models.APIKey)
	return key, ok
}

func GetImpersonatedBy(ctx context.Context) string {
	v, _ := ctx.Value(ImpersonatedByContextKey).(string)
	return v
}
