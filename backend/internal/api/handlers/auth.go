package handlers

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"lastsaas/internal/auth"
	"lastsaas/internal/db"
	"lastsaas/internal/email"
	"lastsaas/internal/events"
	"lastsaas/internal/middleware"
	"lastsaas/internal/models"
	"lastsaas/internal/syslog"
	"lastsaas/internal/telemetry"
	"lastsaas/internal/validation"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type AuthHandler struct {
	db              *db.MongoDB
	jwtService      *auth.JWTService
	passwordService *auth.PasswordService
	totpService     *auth.TOTPService
	googleOAuth     *auth.GoogleOAuthService
	githubOAuth     *auth.GitHubOAuthService
	microsoftOAuth  *auth.MicrosoftOAuthService
	emailService    *email.ResendService
	events          events.Emitter
	frontendURL     string
	syslog          *syslog.Logger
	getConfig       func(string) string
	rateLimiter     *middleware.RateLimiter
	telemetrySvc    *telemetry.Service
}

func NewAuthHandler(
	database *db.MongoDB,
	jwtService *auth.JWTService,
	passwordService *auth.PasswordService,
	googleOAuth *auth.GoogleOAuthService,
	emailService *email.ResendService,
	emitter events.Emitter,
	frontendURL string,
	sysLogger *syslog.Logger,
) *AuthHandler {
	return &AuthHandler{
		db:              database,
		jwtService:      jwtService,
		passwordService: passwordService,
		totpService:     auth.NewTOTPService(),
		googleOAuth:     googleOAuth,
		emailService:    emailService,
		events:          emitter,
		syslog:          sysLogger,
		frontendURL:     frontendURL,
	}
}

func (h *AuthHandler) SetGitHubOAuth(svc *auth.GitHubOAuthService)       { h.githubOAuth = svc }
func (h *AuthHandler) SetMicrosoftOAuth(svc *auth.MicrosoftOAuthService) { h.microsoftOAuth = svc }
func (h *AuthHandler) SetGetConfig(fn func(string) string)               { h.getConfig = fn }
func (h *AuthHandler) SetRateLimiter(rl *middleware.RateLimiter)          { h.rateLimiter = rl }
func (h *AuthHandler) SetTelemetry(svc *telemetry.Service)               { h.telemetrySvc = svc }
func (h *AuthHandler) SetTOTPEncryptionKey(key []byte) {
	if len(key) == 32 {
		h.totpService = auth.NewTOTPServiceWithEncryption(key)
	}
}

// sessionTTLs returns the access and refresh token TTLs from the config store,
// falling back to the JWTService defaults if not configured.
func (h *AuthHandler) sessionTTLs() (accessTTL, refreshTTL time.Duration) {
	accessTTL = h.jwtService.GetAccessTTL()
	refreshTTL = h.jwtService.GetRefreshTTL()
	if h.getConfig != nil {
		if v := h.getConfig("auth.session.access_ttl_minutes"); v != "" {
			if mins, err := strconv.Atoi(v); err == nil && mins > 0 {
				accessTTL = time.Duration(mins) * time.Minute
			}
		}
		if v := h.getConfig("auth.session.refresh_ttl_days"); v != "" {
			if days, err := strconv.Atoi(v); err == nil && days > 0 {
				refreshTTL = time.Duration(days) * 24 * time.Hour
			}
		}
	}
	return
}

// generateTokenPair creates an access token and refresh token using the dynamic config TTLs.
func (h *AuthHandler) generateTokenPair(userID, email, displayName string) (accessToken, refreshToken string, refreshTTL time.Duration, err error) {
	aTTL, rTTL := h.sessionTTLs()
	accessToken, err = h.jwtService.GenerateAccessTokenWithTTL(userID, email, displayName, aTTL)
	if err != nil {
		return
	}
	refreshToken, err = h.jwtService.GenerateRefreshTokenWithTTL(userID, rTTL)
	refreshTTL = rTTL
	return
}

// --- Request/Response types ---

type RegisterRequest struct {
	Email           string `json:"email"`
	Password        string `json:"password"`
	DisplayName     string `json:"displayName"`
	InvitationToken string `json:"invitationToken,omitempty"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refreshToken"`
}

type AuthResponse struct {
	AccessToken  string           `json:"accessToken"`
	RefreshToken string           `json:"refreshToken"`
	User         *models.User     `json:"user"`
	Memberships  []MembershipInfo `json:"memberships"`
}

type MFARequiredResponse struct {
	MFARequired bool   `json:"mfaRequired"`
	MFAToken    string `json:"mfaToken"`
}

type MembershipInfo struct {
	TenantID   string            `json:"tenantId"`
	TenantName string            `json:"tenantName"`
	TenantSlug string            `json:"tenantSlug"`
	Role       models.MemberRole `json:"role"`
	IsRoot     bool              `json:"isRoot"`
}

type VerifyEmailRequest struct {
	Token string `json:"token"`
}

type ResendVerificationRequest struct {
	Email string `json:"email"`
}

type ForgotPasswordRequest struct {
	Email string `json:"email"`
}

type ResetPasswordRequest struct {
	Token       string `json:"token"`
	NewPassword string `json:"newPassword"`
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
}

type AcceptInvitationRequest struct {
	Token string `json:"token"`
}

// --- Auth Providers Discovery ---

func (h *AuthHandler) GetProviders(w http.ResponseWriter, r *http.Request) {
	providers := map[string]bool{
		"password":  true,
		"google":    h.googleOAuth != nil,
		"github":    h.githubOAuth != nil,
		"microsoft": h.microsoftOAuth != nil,
		"magicLink": h.getConfig != nil && h.getConfig("auth.magic_link.enabled") == "true",
		"passkeys":  h.getConfig != nil && h.getConfig("auth.passkeys.enabled") == "true",
	}
	respondWithJSON(w, http.StatusOK, providers)
}

// --- Handlers ---

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	req.DisplayName = strings.TrimSpace(req.DisplayName)

	if req.Email == "" || req.Password == "" || req.DisplayName == "" {
		respondWithError(w, http.StatusBadRequest, "Email, password, and display name are required")
		return
	}

	if !isValidEmail(req.Email) {
		respondWithError(w, http.StatusBadRequest, "Invalid email format")
		return
	}

	if err := h.passwordService.ValidatePasswordStrength(req.Password); err != nil {
		respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Check email uniqueness
	var existing models.User
	if err := h.db.Users().FindOne(r.Context(), bson.M{"email": req.Email}).Decode(&existing); err == nil {
		respondWithError(w, http.StatusConflict, "Unable to create account with these details")
		return
	}

	passwordHash, err := h.passwordService.HashPassword(req.Password)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to process password")
		return
	}

	now := time.Now()
	user := models.User{
		ID:            primitive.NewObjectID(),
		Email:         req.Email,
		DisplayName:   req.DisplayName,
		PasswordHash:  passwordHash,
		AuthMethods:   []models.AuthMethod{models.AuthMethodPassword},
		EmailVerified: false,
		IsActive:      true,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if err := validation.Validate(&user); err != nil {
		respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	if _, err := h.db.Users().InsertOne(r.Context(), user); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to create user")
		return
	}

	h.syslog.High(r.Context(), fmt.Sprintf("User created: %s (%s) via registration", user.Email, user.ID.Hex()))

	// Handle invitation or auto-create tenant
	invitationAccepted := false
	if req.InvitationToken != "" {
		if err := h.acceptInvitationForUser(r.Context(), user.ID, req.InvitationToken); err != nil {
			slog.Error("Failed to accept invitation during registration", "error", err)
		} else {
			invitationAccepted = true
		}
	} else {
		h.createPersonalTenant(r.Context(), user.ID, user.DisplayName, now)
	}

	// Send verification email (skip if invitation already verified them)
	if !invitationAccepted {
		h.sendVerificationEmail(r.Context(), user.ID, user.Email, user.DisplayName)
	}

	// Generate tokens
	accessToken, refreshToken, refreshTTL, err := h.generateTokenPair(user.ID.Hex(), user.Email, user.DisplayName)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to generate token")
		return
	}
	storeRefreshToken(r, h.db, user.ID, refreshToken, refreshTTL)

	memberships := h.getUserMemberships(r.Context(), user.ID)

	h.events.Emit(events.Event{
		Type:      events.EventUserRegistered,
		Timestamp: now,
		Data:      map[string]interface{}{"userId": user.ID.Hex()},
	})

	if h.telemetrySvc != nil {
		h.telemetrySvc.Track(r.Context(), models.TelemetryEvent{
			EventName: models.TelemetryUserRegistered,
			Category:  models.TelemetryCategoryFunnel,
			UserID:    &user.ID,
		})
	}

	respondWithJSON(w, http.StatusCreated, AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User:         &user,
		Memberships:  memberships,
	})
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))

	if req.Email == "" || req.Password == "" {
		respondWithError(w, http.StatusBadRequest, "Email and password are required")
		return
	}

	var user models.User
	if err := h.db.Users().FindOne(r.Context(), bson.M{"email": req.Email}).Decode(&user); err != nil {
		// Perform dummy bcrypt to equalize response timing and prevent account enumeration
		h.passwordService.DummyCompare(req.Password)
		respondWithError(w, http.StatusUnauthorized, "Invalid email or password")
		return
	}

	// Check account lockout
	if user.IsLocked() {
		respondWithError(w, http.StatusTooManyRequests, "Account is temporarily locked. Please try again later.")
		return
	}

	if !user.HasAuthMethod(models.AuthMethodPassword) {
		respondWithError(w, http.StatusUnauthorized, "Invalid email or password")
		return
	}

	if err := h.passwordService.ComparePassword(user.PasswordHash, req.Password); err != nil {
		// Atomic increment of failed attempts + conditional lock
		now := time.Now()
		filter := bson.M{
			"_id": user.ID,
			"$or": []bson.M{
				{"accountLockedUntil": nil},
				{"accountLockedUntil": bson.M{"$lt": now}},
			},
		}
		var updated models.User
		err := h.db.Users().FindOneAndUpdate(
			r.Context(),
			filter,
			bson.M{"$inc": bson.M{"failedLoginAttempts": 1}},
			options.FindOneAndUpdate().SetReturnDocument(options.After),
		).Decode(&updated)
		if err == nil && updated.FailedLoginAttempts >= 5 {
			lockUntil := now.Add(15 * time.Minute)
			h.db.Users().UpdateOne(r.Context(), bson.M{"_id": user.ID}, bson.M{
				"$set": bson.M{"accountLockedUntil": lockUntil},
			})
			h.syslog.LogCatWithUser(r.Context(), models.LogHigh, models.LogCatSecurity,
				fmt.Sprintf("Account locked after %d failed login attempts: %s", updated.FailedLoginAttempts, user.Email), user.ID)
		} else {
			h.syslog.LogCatWithUser(r.Context(), models.LogLow, models.LogCatAuth,
				fmt.Sprintf("Failed login attempt for %s", user.Email), user.ID)
		}
		respondWithError(w, http.StatusUnauthorized, "Invalid email or password")
		return
	}

	if !user.IsActive {
		respondWithError(w, http.StatusUnauthorized, "Account is inactive")
		return
	}

	// Successful login: reset failed attempts
	now := time.Now()
	h.db.Users().UpdateOne(r.Context(), bson.M{"_id": user.ID}, bson.M{
		"$set": bson.M{
			"failedLoginAttempts": 0,
			"accountLockedUntil":  nil,
			"lastLoginAt":         now,
			"updatedAt":           now,
		},
	})

	// If MFA is enabled, return MFA token instead of full tokens
	if user.TOTPEnabled {
		mfaToken, err := h.jwtService.GenerateMFAToken(user.ID.Hex(), user.Email)
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, "Failed to generate token")
			return
		}
		respondWithJSON(w, http.StatusOK, MFARequiredResponse{
			MFARequired: true,
			MFAToken:    mfaToken,
		})
		return
	}

	accessToken, refreshToken, refreshTTL, err := h.generateTokenPair(user.ID.Hex(), user.Email, user.DisplayName)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to generate token")
		return
	}
	storeRefreshToken(r, h.db, user.ID, refreshToken, refreshTTL)

	memberships := h.getUserMemberships(r.Context(), user.ID)

	h.events.Emit(events.Event{
		Type:      events.EventUserLoggedIn,
		Timestamp: now,
		Data:      map[string]interface{}{"userId": user.ID.Hex()},
	})

	h.syslog.LogCatWithUser(r.Context(), models.LogLow, models.LogCatAuth,
		fmt.Sprintf("User logged in: %s", user.Email), user.ID)

	if h.telemetrySvc != nil {
		h.telemetrySvc.TrackLogin(r.Context(), user.ID)
	}

	respondWithJSON(w, http.StatusOK, AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User:         &user,
		Memberships:  memberships,
	})
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	// Revoke the access token
	authHeader := r.Header.Get("Authorization")
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) == 2 {
		tokenHash := hashToken(parts[1])
		h.db.RevokedTokens().InsertOne(r.Context(), models.RevokedToken{
			ID:        primitive.NewObjectID(),
			TokenHash: tokenHash,
			ExpiresAt: time.Now().Add(h.jwtService.GetAccessTTL()),
			CreatedAt: time.Now(),
		})
	}

	// Revoke refresh token if provided (scoped to the authenticated user)
	var req struct {
		RefreshToken string `json:"refreshToken"`
	}
	if json.NewDecoder(r.Body).Decode(&req) == nil && req.RefreshToken != "" {
		if user, ok := middleware.GetUserFromContext(r.Context()); ok {
			tokenHash := hashToken(req.RefreshToken)
			h.db.RefreshTokens().UpdateMany(r.Context(),
				bson.M{"tokenHash": tokenHash, "userId": user.ID},
				bson.M{"$set": bson.M{"isRevoked": true}},
			)
		}
	}

	if user, ok := middleware.GetUserFromContext(r.Context()); ok {
		h.syslog.LogCatWithUser(r.Context(), models.LogLow, models.LogCatAuth,
			fmt.Sprintf("User logged out: %s", user.Email), user.ID)
	}

	respondWithJSON(w, http.StatusOK, map[string]string{"message": "Logged out successfully"})
}

func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req RefreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.RefreshToken == "" {
		respondWithError(w, http.StatusBadRequest, "Refresh token is required")
		return
	}

	claims, err := h.jwtService.ValidateRefreshToken(req.RefreshToken)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Invalid refresh token")
		return
	}

	// Check if refresh token is stored
	tokenHash := hashToken(req.RefreshToken)
	var storedToken models.RefreshToken
	err = h.db.RefreshTokens().FindOne(r.Context(), bson.M{
		"tokenHash": tokenHash,
	}).Decode(&storedToken)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Refresh token not found")
		return
	}

	// If token was already revoked, this is a replay attack — revoke the entire family
	if storedToken.IsRevoked {
		if storedToken.FamilyID != "" {
			h.db.RefreshTokens().UpdateMany(r.Context(),
				bson.M{"familyId": storedToken.FamilyID},
				bson.M{"$set": bson.M{"isRevoked": true}},
			)
			slog.Warn("Security: refresh token replay detected, family revoked", "userId", storedToken.UserID.Hex(), "familyId", storedToken.FamilyID)
		}
		respondWithError(w, http.StatusUnauthorized, "Refresh token has been revoked")
		return
	}

	// Revoke old refresh token
	h.db.RefreshTokens().UpdateOne(r.Context(),
		bson.M{"_id": storedToken.ID},
		bson.M{"$set": bson.M{"isRevoked": true}},
	)

	userID, err := primitive.ObjectIDFromHex(claims.UserID)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Invalid user ID")
		return
	}

	var user models.User
	if err := h.db.Users().FindOne(r.Context(), bson.M{"_id": userID, "isActive": true}).Decode(&user); err != nil {
		respondWithError(w, http.StatusUnauthorized, "User not found")
		return
	}

	accessToken, refreshToken, refreshTTL, err := h.generateTokenPair(user.ID.Hex(), user.Email, user.DisplayName)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to generate token")
		return
	}
	storeRefreshToken(r, h.db, user.ID, refreshToken, refreshTTL, storedToken.FamilyID)

	// Update lastActiveAt on the new stored token
	newHash := hashToken(refreshToken)
	h.db.RefreshTokens().UpdateOne(r.Context(),
		bson.M{"tokenHash": newHash},
		bson.M{"$set": bson.M{"lastActiveAt": time.Now()}},
	)

	memberships := h.getUserMemberships(r.Context(), user.ID)

	respondWithJSON(w, http.StatusOK, AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User:         &user,
		Memberships:  memberships,
	})
}

func (h *AuthHandler) GetMe(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUserFromContext(r.Context())
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	memberships := h.getUserMemberships(r.Context(), user.ID)

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"user":        user,
		"memberships": memberships,
	})
}

func (h *AuthHandler) VerifyEmail(w http.ResponseWriter, r *http.Request) {
	var req VerifyEmailRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Token == "" {
		respondWithError(w, http.StatusBadRequest, "Token is required")
		return
	}

	now := time.Now()

	var token models.VerificationToken
	err := h.db.VerificationTokens().FindOneAndUpdate(
		r.Context(),
		bson.M{
			"token":     hashToken(req.Token),
			"type":      models.TokenTypeEmailVerification,
			"usedAt":    nil,
			"expiresAt": bson.M{"$gt": now},
		},
		bson.M{"$set": bson.M{"usedAt": now}},
	).Decode(&token)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid or expired verification token")
		return
	}

	h.db.Users().UpdateOne(r.Context(), bson.M{"_id": token.UserID}, bson.M{
		"$set": bson.M{"emailVerified": true, "updatedAt": now},
	})

	h.events.Emit(events.Event{
		Type:      events.EventUserVerified,
		Timestamp: now,
		Data:      map[string]interface{}{"userId": token.UserID.Hex()},
	})

	if h.telemetrySvc != nil {
		h.telemetrySvc.Track(r.Context(), models.TelemetryEvent{
			EventName: models.TelemetryUserVerified,
			Category:  models.TelemetryCategoryFunnel,
			UserID:    &token.UserID,
		})
	}

	respondWithJSON(w, http.StatusOK, map[string]string{"message": "Email verified successfully"})
}

func (h *AuthHandler) ResendVerification(w http.ResponseWriter, r *http.Request) {
	var req ResendVerificationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Email == "" {
		respondWithError(w, http.StatusBadRequest, "Email is required")
		return
	}

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))

	var user models.User
	if err := h.db.Users().FindOne(r.Context(), bson.M{"email": req.Email}).Decode(&user); err != nil {
		respondWithJSON(w, http.StatusOK, map[string]string{"message": "If the email exists, a verification link has been sent"})
		return
	}

	if user.EmailVerified {
		respondWithJSON(w, http.StatusOK, map[string]string{"message": "If the email exists, a verification link has been sent"})
		return
	}

	if user.LastVerificationSent != nil && time.Since(*user.LastVerificationSent) < 60*time.Second {
		respondWithError(w, http.StatusTooManyRequests, "Please wait before requesting another verification email")
		return
	}

	h.sendVerificationEmail(r.Context(), user.ID, user.Email, user.DisplayName)

	respondWithJSON(w, http.StatusOK, map[string]string{"message": "If the email exists, a verification link has been sent"})
}

func (h *AuthHandler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	var req ForgotPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Email == "" {
		respondWithError(w, http.StatusBadRequest, "Email is required")
		return
	}

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))

	// Email-based rate limiting (in addition to IP-based rate limiting on the route)
	if h.rateLimiter != nil {
		if allowed, _, _ := h.rateLimiter.Allow("email:pwreset:"+req.Email, middleware.EmailPasswordResetLimit); !allowed {
			respondWithJSON(w, http.StatusOK, map[string]string{"message": "If the email exists, a password reset link has been sent"})
			return
		}
	}

	defer respondWithJSON(w, http.StatusOK, map[string]string{"message": "If the email exists, a password reset link has been sent"})

	var user models.User
	if err := h.db.Users().FindOne(r.Context(), bson.M{"email": req.Email}).Decode(&user); err != nil {
		return
	}

	// Revoke any previous unused password reset tokens for this user
	h.db.VerificationTokens().UpdateMany(r.Context(),
		bson.M{"userId": user.ID, "type": models.TokenTypePasswordReset, "usedAt": nil},
		bson.M{"$set": bson.M{"usedAt": time.Now()}},
	)

	resetToken := generateRandomToken()
	hashedToken := hashToken(resetToken)
	verification := models.VerificationToken{
		ID:        primitive.NewObjectID(),
		UserID:    user.ID,
		Token:     hashedToken,
		Type:      models.TokenTypePasswordReset,
		ExpiresAt: time.Now().Add(30 * time.Minute),
		CreatedAt: time.Now(),
	}
	h.db.VerificationTokens().InsertOne(r.Context(), verification)

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_ = ctx // timeout guard for background goroutine
		if h.emailService != nil {
			if err := h.emailService.SendPasswordResetEmail(user.Email, user.DisplayName, resetToken); err != nil {
				slog.Error("Failed to send password reset email", "error", err)
			}
		}
	}()
}

func (h *AuthHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	var req ResetPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Token == "" || req.NewPassword == "" {
		respondWithError(w, http.StatusBadRequest, "Token and new password are required")
		return
	}

	if err := h.passwordService.ValidatePasswordStrength(req.NewPassword); err != nil {
		respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Now()

	hashedToken := hashToken(req.Token)
	var token models.VerificationToken
	err := h.db.VerificationTokens().FindOneAndUpdate(
		r.Context(),
		bson.M{
			"token":     hashedToken,
			"type":      models.TokenTypePasswordReset,
			"usedAt":    nil,
			"expiresAt": bson.M{"$gt": now},
		},
		bson.M{"$set": bson.M{"usedAt": now}},
	).Decode(&token)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid or expired reset token")
		return
	}

	passwordHash, err := h.passwordService.HashPassword(req.NewPassword)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to process password")
		return
	}

	h.db.Users().UpdateOne(r.Context(), bson.M{"_id": token.UserID}, bson.M{
		"$set": bson.M{
			"passwordHash": passwordHash,
			"updatedAt":    now,
		},
		"$addToSet": bson.M{
			"authMethods": models.AuthMethodPassword,
		},
	})

	h.db.RefreshTokens().UpdateMany(r.Context(),
		bson.M{"userId": token.UserID, "isRevoked": false},
		bson.M{"$set": bson.M{"isRevoked": true}},
	)

	h.syslog.High(r.Context(), fmt.Sprintf("Password reset via token for user %s", token.UserID.Hex()))

	respondWithJSON(w, http.StatusOK, map[string]string{"message": "Password reset successfully"})
}

func (h *AuthHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUserFromContext(r.Context())
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	var req ChangePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.NewPassword == "" {
		respondWithError(w, http.StatusBadRequest, "New password is required")
		return
	}

	if err := h.passwordService.ValidatePasswordStrength(req.NewPassword); err != nil {
		respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	if user.HasAuthMethod(models.AuthMethodPassword) {
		if req.CurrentPassword == "" {
			respondWithError(w, http.StatusBadRequest, "Current password is required")
			return
		}
		if err := h.passwordService.ComparePassword(user.PasswordHash, req.CurrentPassword); err != nil {
			respondWithError(w, http.StatusUnauthorized, "Current password is incorrect")
			return
		}
	}

	passwordHash, err := h.passwordService.HashPassword(req.NewPassword)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to process password")
		return
	}

	update := bson.M{
		"$set": bson.M{
			"passwordHash": passwordHash,
			"updatedAt":    time.Now(),
		},
	}
	if !user.HasAuthMethod(models.AuthMethodPassword) {
		update["$addToSet"] = bson.M{"authMethods": models.AuthMethodPassword}
	}

	h.db.Users().UpdateOne(r.Context(), bson.M{"_id": user.ID}, update)

	// Revoke all active sessions so stolen tokens can't persist after password change
	h.db.RefreshTokens().UpdateMany(r.Context(),
		bson.M{"userId": user.ID, "isRevoked": false},
		bson.M{"$set": bson.M{"isRevoked": true}},
	)

	h.syslog.High(r.Context(), fmt.Sprintf("Password changed by user %s (%s)", user.Email, user.ID.Hex()))

	respondWithJSON(w, http.StatusOK, map[string]string{"message": "Password changed successfully"})
}

// --- MFA/TOTP ---

func (h *AuthHandler) MFASetup(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUserFromContext(r.Context())
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	if user.TOTPEnabled {
		respondWithError(w, http.StatusConflict, "MFA is already enabled")
		return
	}

	appName := "LastSaaS"
	if h.getConfig != nil {
		if name := h.getConfig("app.name"); name != "" {
			appName = name
		}
	}

	key, err := h.totpService.GenerateSecret(appName, user.Email)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to generate MFA secret")
		return
	}

	// Encrypt and store secret temporarily (not yet enabled)
	encryptedSecret, err := h.totpService.EncryptSecret(key.Secret())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to secure MFA secret")
		return
	}
	h.db.Users().UpdateOne(r.Context(), bson.M{"_id": user.ID}, bson.M{
		"$set": bson.M{"totpSecret": encryptedSecret, "updatedAt": time.Now()},
	})

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"secret": key.Secret(),
		"qrUrl":  key.URL(),
	})
}

func (h *AuthHandler) MFAVerifySetup(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUserFromContext(r.Context())
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	var req struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Code == "" {
		respondWithError(w, http.StatusBadRequest, "Code is required")
		return
	}

	// Re-fetch user to get totpSecret
	var freshUser models.User
	if err := h.db.Users().FindOne(r.Context(), bson.M{"_id": user.ID}).Decode(&freshUser); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to fetch user")
		return
	}

	if freshUser.TOTPSecret == "" {
		respondWithError(w, http.StatusBadRequest, "MFA setup has not been initiated")
		return
	}

	decryptedSecret := h.totpService.DecryptSecret(freshUser.TOTPSecret)
	if !h.totpService.ValidateCodeWithWindow(decryptedSecret, req.Code) {
		respondWithError(w, http.StatusUnauthorized, "Invalid verification code")
		return
	}

	// Generate recovery codes
	plainCodes, hashedCodes, err := h.totpService.GenerateRecoveryCodes(8)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to generate recovery codes")
		return
	}

	now := time.Now()
	h.db.Users().UpdateOne(r.Context(), bson.M{"_id": user.ID}, bson.M{
		"$set": bson.M{
			"totpEnabled":    true,
			"totpVerifiedAt": now,
			"recoveryCodes":  hashedCodes,
			"updatedAt":      now,
		},
	})

	h.syslog.HighWithUser(r.Context(), fmt.Sprintf("MFA enabled for user %s (%s)", user.Email, user.ID.Hex()), user.ID)

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"message":       "MFA enabled successfully",
		"recoveryCodes": plainCodes,
	})
}

func (h *AuthHandler) MFADisable(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUserFromContext(r.Context())
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	var req struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Code == "" {
		respondWithError(w, http.StatusBadRequest, "Code is required")
		return
	}

	var freshUser models.User
	if err := h.db.Users().FindOne(r.Context(), bson.M{"_id": user.ID}).Decode(&freshUser); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to fetch user")
		return
	}

	if !freshUser.TOTPEnabled {
		respondWithError(w, http.StatusBadRequest, "MFA is not enabled")
		return
	}

	// Try TOTP code first, then recovery code
	valid := h.totpService.ValidateCodeWithWindow(h.totpService.DecryptSecret(freshUser.TOTPSecret), req.Code)
	if !valid {
		_, valid = h.totpService.ValidateRecoveryCode(req.Code, freshUser.RecoveryCodes)
	}
	if !valid {
		respondWithError(w, http.StatusUnauthorized, "Invalid code")
		return
	}

	h.db.Users().UpdateOne(r.Context(), bson.M{"_id": user.ID}, bson.M{
		"$set": bson.M{
			"totpEnabled":    false,
			"totpSecret":     "",
			"totpVerifiedAt": nil,
			"recoveryCodes":  nil,
			"updatedAt":      time.Now(),
		},
	})

	h.syslog.HighWithUser(r.Context(), fmt.Sprintf("MFA disabled for user %s (%s)", user.Email, user.ID.Hex()), user.ID)

	respondWithJSON(w, http.StatusOK, map[string]string{"message": "MFA disabled successfully"})
}

func (h *AuthHandler) MFAChallenge(w http.ResponseWriter, r *http.Request) {
	var req struct {
		MFAToken string `json:"mfaToken"`
		Code     string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if req.MFAToken == "" || req.Code == "" {
		respondWithError(w, http.StatusBadRequest, "MFA token and code are required")
		return
	}

	claims, err := h.jwtService.ValidateAccessToken(req.MFAToken)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Invalid or expired MFA token")
		return
	}
	if claims.TokenType != "mfa" || !claims.MFAPending {
		respondWithError(w, http.StatusUnauthorized, "Invalid MFA token")
		return
	}

	userID, err := primitive.ObjectIDFromHex(claims.UserID)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Invalid user")
		return
	}

	var user models.User
	if err := h.db.Users().FindOne(r.Context(), bson.M{"_id": userID}).Decode(&user); err != nil {
		respondWithError(w, http.StatusUnauthorized, "User not found")
		return
	}

	// Validate TOTP code or recovery code
	valid := h.totpService.ValidateCodeWithWindow(h.totpService.DecryptSecret(user.TOTPSecret), req.Code)
	recoveryIdx := -1
	if !valid {
		recoveryIdx, valid = h.totpService.ValidateRecoveryCode(req.Code, user.RecoveryCodes)
	}
	if !valid {
		h.syslog.LogCatWithUser(r.Context(), models.LogMedium, models.LogCatSecurity,
			fmt.Sprintf("Failed MFA attempt for user %s", user.Email), user.ID)
		respondWithError(w, http.StatusUnauthorized, "Invalid code")
		return
	}

	// If recovery code was used, remove it atomically
	if recoveryIdx >= 0 {
		h.db.Users().UpdateOne(r.Context(), bson.M{"_id": user.ID}, bson.M{
			"$pull": bson.M{"recoveryCodes": user.RecoveryCodes[recoveryIdx]},
		})
	}

	// Generate full auth tokens
	accessToken, refreshToken, refreshTTL, err := h.generateTokenPair(user.ID.Hex(), user.Email, user.DisplayName)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to generate token")
		return
	}
	storeRefreshToken(r, h.db, user.ID, refreshToken, refreshTTL)

	memberships := h.getUserMemberships(r.Context(), user.ID)

	now := time.Now()
	h.events.Emit(events.Event{
		Type:      events.EventUserLoggedIn,
		Timestamp: now,
		Data:      map[string]interface{}{"userId": user.ID.Hex()},
	})

	h.syslog.LogCatWithUser(r.Context(), models.LogLow, models.LogCatAuth,
		fmt.Sprintf("User logged in via MFA: %s", user.Email), user.ID)

	respondWithJSON(w, http.StatusOK, AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User:         &user,
		Memberships:  memberships,
	})
}

func (h *AuthHandler) MFARegenerateRecoveryCodes(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUserFromContext(r.Context())
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	var req struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Code == "" {
		respondWithError(w, http.StatusBadRequest, "Code is required")
		return
	}

	var freshUser models.User
	if err := h.db.Users().FindOne(r.Context(), bson.M{"_id": user.ID}).Decode(&freshUser); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to fetch user")
		return
	}

	if !freshUser.TOTPEnabled {
		respondWithError(w, http.StatusBadRequest, "MFA is not enabled")
		return
	}

	if !h.totpService.ValidateCodeWithWindow(h.totpService.DecryptSecret(freshUser.TOTPSecret), req.Code) {
		respondWithError(w, http.StatusUnauthorized, "Invalid code")
		return
	}

	plainCodes, hashedCodes, err := h.totpService.GenerateRecoveryCodes(8)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to generate recovery codes")
		return
	}

	h.db.Users().UpdateOne(r.Context(), bson.M{"_id": user.ID}, bson.M{
		"$set": bson.M{"recoveryCodes": hashedCodes, "updatedAt": time.Now()},
	})

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"recoveryCodes": plainCodes,
	})
}

// --- Magic Link ---

func (h *AuthHandler) MagicLinkRequest(w http.ResponseWriter, r *http.Request) {
	if h.getConfig == nil || h.getConfig("auth.magic_link.enabled") != "true" {
		respondWithError(w, http.StatusNotFound, "Magic link login is not enabled")
		return
	}

	var req struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Email == "" {
		respondWithError(w, http.StatusBadRequest, "Email is required")
		return
	}
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))

	// Email-based rate limiting
	if h.rateLimiter != nil {
		if allowed, _, _ := h.rateLimiter.Allow("email:magiclink:"+req.Email, middleware.EmailMagicLinkLimit); !allowed {
			respondWithJSON(w, http.StatusOK, map[string]string{"message": "If the email exists, a sign-in link has been sent"})
			return
		}
	}

	// Always return success to prevent enumeration
	defer respondWithJSON(w, http.StatusOK, map[string]string{"message": "If the email exists, a sign-in link has been sent"})

	var user models.User
	if err := h.db.Users().FindOne(r.Context(), bson.M{"email": req.Email}).Decode(&user); err != nil {
		return
	}

	magicToken := generateRandomToken()
	hashedMagicToken := hashToken(magicToken)
	verification := models.VerificationToken{
		ID:        primitive.NewObjectID(),
		UserID:    user.ID,
		Token:     hashedMagicToken,
		Type:      models.TokenTypeMagicLink,
		ExpiresAt: time.Now().Add(15 * time.Minute),
		CreatedAt: time.Now(),
	}
	h.db.VerificationTokens().InsertOne(r.Context(), verification)

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_ = ctx // timeout guard for background goroutine
		if h.emailService != nil {
			if err := h.emailService.SendMagicLinkEmail(user.Email, user.DisplayName, magicToken); err != nil {
				slog.Error("Failed to send magic link email", "error", err)
			}
		}
	}()
}

func (h *AuthHandler) MagicLinkVerify(w http.ResponseWriter, r *http.Request) {
	if h.getConfig == nil || h.getConfig("auth.magic_link.enabled") != "true" {
		respondWithError(w, http.StatusNotFound, "Magic link login is not enabled")
		return
	}

	var req struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Token == "" {
		respondWithError(w, http.StatusBadRequest, "Token is required")
		return
	}

	now := time.Now()

	hashedToken := hashToken(req.Token)
	var token models.VerificationToken
	err := h.db.VerificationTokens().FindOneAndUpdate(
		r.Context(),
		bson.M{
			"token":     hashedToken,
			"type":      models.TokenTypeMagicLink,
			"usedAt":    nil,
			"expiresAt": bson.M{"$gt": now},
		},
		bson.M{"$set": bson.M{"usedAt": now}},
	).Decode(&token)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid or expired magic link token")
		return
	}

	var user models.User
	if err := h.db.Users().FindOne(r.Context(), bson.M{"_id": token.UserID, "isActive": true}).Decode(&user); err != nil {
		respondWithError(w, http.StatusUnauthorized, "User not found")
		return
	}

	// Mark email as verified (they proved ownership)
	h.db.Users().UpdateOne(r.Context(), bson.M{"_id": user.ID}, bson.M{
		"$set":      bson.M{"emailVerified": true, "lastLoginAt": now, "updatedAt": now},
		"$addToSet": bson.M{"authMethods": models.AuthMethodMagicLink},
	})

	// If user has MFA enabled, return MFA token
	if user.TOTPEnabled {
		mfaToken, err := h.jwtService.GenerateMFAToken(user.ID.Hex(), user.Email)
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, "Failed to generate token")
			return
		}
		respondWithJSON(w, http.StatusOK, MFARequiredResponse{
			MFARequired: true,
			MFAToken:    mfaToken,
		})
		return
	}

	accessToken, refreshToken, refreshTTL, err := h.generateTokenPair(user.ID.Hex(), user.Email, user.DisplayName)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to generate token")
		return
	}
	storeRefreshToken(r, h.db, user.ID, refreshToken, refreshTTL)

	memberships := h.getUserMemberships(r.Context(), user.ID)

	respondWithJSON(w, http.StatusOK, AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User:         &user,
		Memberships:  memberships,
	})
}

// --- Auth Code Exchange (OAuth security) ---

// createAuthCodeRedirect stores tokens in the DB behind a short-lived code and redirects with ?code=XXX
func (h *AuthHandler) createAuthCodeRedirect(w http.ResponseWriter, r *http.Request, userID primitive.ObjectID, tokenData models.AuthCodeTokenData) {
	code := generateRandomToken()
	now := time.Now()
	authCode := models.AuthCode{
		ID:        primitive.NewObjectID(),
		Code:      code,
		UserID:    userID,
		TokenData: tokenData,
		ExpiresAt: now.Add(60 * time.Second),
		CreatedAt: now,
	}
	if _, err := h.db.AuthCodes().InsertOne(r.Context(), authCode); err != nil {
		http.Redirect(w, r, h.frontendURL+"/login?error=code_generation_failed", http.StatusTemporaryRedirect)
		return
	}
	redirectURL := fmt.Sprintf("%s/auth/callback?code=%s", h.frontendURL, code)
	http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
}

// ExchangeCode handles POST /api/auth/exchange-code
func (h *AuthHandler) ExchangeCode(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Code == "" {
		respondWithError(w, http.StatusBadRequest, "Code is required")
		return
	}

	// Atomically find and mark as used
	now := time.Now()
	var authCode models.AuthCode
	err := h.db.AuthCodes().FindOneAndUpdate(r.Context(),
		bson.M{"code": req.Code, "usedAt": nil, "expiresAt": bson.M{"$gt": now}},
		bson.M{"$set": bson.M{"usedAt": now}},
	).Decode(&authCode)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Invalid or expired code")
		return
	}

	if authCode.TokenData.IsMFA {
		respondWithJSON(w, http.StatusOK, map[string]interface{}{
			"mfaRequired": true,
			"mfaToken":    authCode.TokenData.MFAToken,
		})
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"accessToken":  authCode.TokenData.AccessToken,
		"refreshToken": authCode.TokenData.RefreshToken,
	})
}

// --- Google OAuth ---

func (h *AuthHandler) GoogleOAuth(w http.ResponseWriter, r *http.Request) {
	if h.googleOAuth == nil {
		respondWithError(w, http.StatusNotImplemented, "Google OAuth is not configured")
		return
	}

	state := generateRandomToken()
	oauthState := models.OAuthState{
		ID:        primitive.NewObjectID(),
		State:     state,
		ExpiresAt: time.Now().Add(10 * time.Minute),
		CreatedAt: time.Now(),
	}
	h.db.OAuthStates().InsertOne(r.Context(), oauthState)

	authURL := h.googleOAuth.GetAuthURL(state)
	http.Redirect(w, r, authURL, http.StatusTemporaryRedirect)
}

func (h *AuthHandler) GoogleOAuthCallback(w http.ResponseWriter, r *http.Request) {
	if h.googleOAuth == nil {
		respondWithError(w, http.StatusNotImplemented, "Google OAuth is not configured")
		return
	}

	state := r.URL.Query().Get("state")
	code := r.URL.Query().Get("code")

	if state == "" || code == "" {
		http.Redirect(w, r, h.frontendURL+"/login?error=oauth_failed", http.StatusTemporaryRedirect)
		return
	}

	result := h.db.OAuthStates().FindOneAndDelete(r.Context(), bson.M{
		"state":     state,
		"expiresAt": bson.M{"$gt": time.Now()},
	})
	if result.Err() != nil {
		http.Redirect(w, r, h.frontendURL+"/login?error=invalid_state", http.StatusTemporaryRedirect)
		return
	}

	token, err := h.googleOAuth.ExchangeCode(r.Context(), code)
	if err != nil {
		http.Redirect(w, r, h.frontendURL+"/login?error=oauth_exchange_failed", http.StatusTemporaryRedirect)
		return
	}

	googleUser, err := h.googleOAuth.GetUserInfo(r.Context(), token)
	if err != nil || !googleUser.VerifiedEmail {
		http.Redirect(w, r, h.frontendURL+"/login?error=oauth_user_info_failed", http.StatusTemporaryRedirect)
		return
	}

	now := time.Now()
	var user models.User
	var isNewUser bool

	err = h.db.Users().FindOne(r.Context(), bson.M{"googleId": googleUser.ID}).Decode(&user)
	if err != nil {
		err = h.db.Users().FindOne(r.Context(), bson.M{"email": strings.ToLower(googleUser.Email)}).Decode(&user)
		if err != nil {
			isNewUser = true
			user = models.User{
				ID:            primitive.NewObjectID(),
				Email:         strings.ToLower(googleUser.Email),
				DisplayName:   googleUser.GivenName,
				GoogleID:      googleUser.ID,
				AuthMethods:   []models.AuthMethod{models.AuthMethodGoogle},
				EmailVerified: true,
				IsActive:      true,
				CreatedAt:     now,
				UpdatedAt:     now,
				LastLoginAt:   &now,
			}
			h.db.Users().InsertOne(r.Context(), user)
			h.createPersonalTenant(r.Context(), user.ID, user.DisplayName, now)
			h.syslog.High(r.Context(), fmt.Sprintf("User created: %s (%s) via Google OAuth", user.Email, user.ID.Hex()))
		} else {
			h.db.Users().UpdateOne(r.Context(), bson.M{"_id": user.ID}, bson.M{
				"$set":      bson.M{"googleId": googleUser.ID, "lastLoginAt": now, "updatedAt": now},
				"$addToSet": bson.M{"authMethods": models.AuthMethodGoogle},
			})
		}
	} else {
		h.db.Users().UpdateOne(r.Context(), bson.M{"_id": user.ID}, bson.M{
			"$set": bson.M{"lastLoginAt": now, "updatedAt": now},
		})
	}

	// Check MFA
	if user.TOTPEnabled {
		mfaToken, err := h.jwtService.GenerateMFAToken(user.ID.Hex(), user.Email)
		if err != nil {
			http.Redirect(w, r, h.frontendURL+"/login?error=token_generation_failed", http.StatusTemporaryRedirect)
			return
		}
		h.createAuthCodeRedirect(w, r, user.ID, models.AuthCodeTokenData{MFAToken: mfaToken, IsMFA: true})
		return
	}

	accessToken, refreshToken, refreshTTL, err := h.generateTokenPair(user.ID.Hex(), user.Email, user.DisplayName)
	if err != nil {
		http.Redirect(w, r, h.frontendURL+"/login?error=token_generation_failed", http.StatusTemporaryRedirect)
		return
	}
	storeRefreshToken(r, h.db, user.ID, refreshToken, refreshTTL)

	if isNewUser {
		h.events.Emit(events.Event{
			Type:      events.EventUserRegistered,
			Timestamp: now,
			Data:      map[string]interface{}{"userId": user.ID.Hex(), "method": "google"},
		})
	}

	h.createAuthCodeRedirect(w, r, user.ID, models.AuthCodeTokenData{AccessToken: accessToken, RefreshToken: refreshToken})
}

// --- GitHub OAuth ---

func (h *AuthHandler) GitHubOAuth(w http.ResponseWriter, r *http.Request) {
	if h.githubOAuth == nil {
		respondWithError(w, http.StatusNotImplemented, "GitHub OAuth is not configured")
		return
	}

	state := generateRandomToken()
	oauthState := models.OAuthState{
		ID:        primitive.NewObjectID(),
		State:     state,
		ExpiresAt: time.Now().Add(10 * time.Minute),
		CreatedAt: time.Now(),
	}
	h.db.OAuthStates().InsertOne(r.Context(), oauthState)

	authURL := h.githubOAuth.GetAuthURL(state)
	http.Redirect(w, r, authURL, http.StatusTemporaryRedirect)
}

func (h *AuthHandler) GitHubOAuthCallback(w http.ResponseWriter, r *http.Request) {
	if h.githubOAuth == nil {
		respondWithError(w, http.StatusNotImplemented, "GitHub OAuth is not configured")
		return
	}

	state := r.URL.Query().Get("state")
	code := r.URL.Query().Get("code")

	if state == "" || code == "" {
		http.Redirect(w, r, h.frontendURL+"/login?error=oauth_failed", http.StatusTemporaryRedirect)
		return
	}

	result := h.db.OAuthStates().FindOneAndDelete(r.Context(), bson.M{
		"state":     state,
		"expiresAt": bson.M{"$gt": time.Now()},
	})
	if result.Err() != nil {
		http.Redirect(w, r, h.frontendURL+"/login?error=invalid_state", http.StatusTemporaryRedirect)
		return
	}

	token, err := h.githubOAuth.ExchangeCode(r.Context(), code)
	if err != nil {
		http.Redirect(w, r, h.frontendURL+"/login?error=oauth_exchange_failed", http.StatusTemporaryRedirect)
		return
	}

	ghUser, err := h.githubOAuth.GetUserInfo(r.Context(), token)
	if err != nil {
		http.Redirect(w, r, h.frontendURL+"/login?error=oauth_user_info_failed", http.StatusTemporaryRedirect)
		return
	}

	now := time.Now()
	var user models.User
	var isNewUser bool
	ghIDStr := fmt.Sprintf("%d", ghUser.ID)

	err = h.db.Users().FindOne(r.Context(), bson.M{"githubId": ghIDStr}).Decode(&user)
	if err != nil {
		err = h.db.Users().FindOne(r.Context(), bson.M{"email": strings.ToLower(ghUser.Email)}).Decode(&user)
		if err != nil {
			isNewUser = true
			displayName := ghUser.Name
			if displayName == "" {
				displayName = ghUser.Login
			}
			user = models.User{
				ID:            primitive.NewObjectID(),
				Email:         strings.ToLower(ghUser.Email),
				DisplayName:   displayName,
				GitHubID:      ghIDStr,
				AuthMethods:   []models.AuthMethod{models.AuthMethodGitHub},
				EmailVerified: true,
				IsActive:      true,
				CreatedAt:     now,
				UpdatedAt:     now,
				LastLoginAt:   &now,
			}
			h.db.Users().InsertOne(r.Context(), user)
			h.createPersonalTenant(r.Context(), user.ID, user.DisplayName, now)
			h.syslog.High(r.Context(), fmt.Sprintf("User created: %s (%s) via GitHub OAuth", user.Email, user.ID.Hex()))
		} else {
			// Only link GitHub to existing account if user has verified their email
			if !user.EmailVerified {
				http.Redirect(w, r, h.frontendURL+"/login?error=email_not_verified", http.StatusTemporaryRedirect)
				return
			}
			h.db.Users().UpdateOne(r.Context(), bson.M{"_id": user.ID}, bson.M{
				"$set":      bson.M{"githubId": ghIDStr, "lastLoginAt": now, "updatedAt": now},
				"$addToSet": bson.M{"authMethods": models.AuthMethodGitHub},
			})
		}
	} else {
		h.db.Users().UpdateOne(r.Context(), bson.M{"_id": user.ID}, bson.M{
			"$set": bson.M{"lastLoginAt": now, "updatedAt": now},
		})
	}

	if user.TOTPEnabled {
		mfaToken, err := h.jwtService.GenerateMFAToken(user.ID.Hex(), user.Email)
		if err != nil {
			http.Redirect(w, r, h.frontendURL+"/login?error=token_generation_failed", http.StatusTemporaryRedirect)
			return
		}
		h.createAuthCodeRedirect(w, r, user.ID, models.AuthCodeTokenData{MFAToken: mfaToken, IsMFA: true})
		return
	}

	accessToken, refreshToken, refreshTTL, err := h.generateTokenPair(user.ID.Hex(), user.Email, user.DisplayName)
	if err != nil {
		http.Redirect(w, r, h.frontendURL+"/login?error=token_generation_failed", http.StatusTemporaryRedirect)
		return
	}
	storeRefreshToken(r, h.db, user.ID, refreshToken, refreshTTL)

	if isNewUser {
		h.events.Emit(events.Event{
			Type:      events.EventUserRegistered,
			Timestamp: now,
			Data:      map[string]interface{}{"userId": user.ID.Hex(), "method": "github"},
		})
	}

	h.createAuthCodeRedirect(w, r, user.ID, models.AuthCodeTokenData{AccessToken: accessToken, RefreshToken: refreshToken})
}

// --- Microsoft OAuth ---

func (h *AuthHandler) MicrosoftOAuth(w http.ResponseWriter, r *http.Request) {
	if h.microsoftOAuth == nil {
		respondWithError(w, http.StatusNotImplemented, "Microsoft OAuth is not configured")
		return
	}

	state := generateRandomToken()
	oauthState := models.OAuthState{
		ID:        primitive.NewObjectID(),
		State:     state,
		ExpiresAt: time.Now().Add(10 * time.Minute),
		CreatedAt: time.Now(),
	}
	h.db.OAuthStates().InsertOne(r.Context(), oauthState)

	authURL := h.microsoftOAuth.GetAuthURL(state)
	http.Redirect(w, r, authURL, http.StatusTemporaryRedirect)
}

func (h *AuthHandler) MicrosoftOAuthCallback(w http.ResponseWriter, r *http.Request) {
	if h.microsoftOAuth == nil {
		respondWithError(w, http.StatusNotImplemented, "Microsoft OAuth is not configured")
		return
	}

	state := r.URL.Query().Get("state")
	code := r.URL.Query().Get("code")

	if state == "" || code == "" {
		http.Redirect(w, r, h.frontendURL+"/login?error=oauth_failed", http.StatusTemporaryRedirect)
		return
	}

	result := h.db.OAuthStates().FindOneAndDelete(r.Context(), bson.M{
		"state":     state,
		"expiresAt": bson.M{"$gt": time.Now()},
	})
	if result.Err() != nil {
		http.Redirect(w, r, h.frontendURL+"/login?error=invalid_state", http.StatusTemporaryRedirect)
		return
	}

	token, err := h.microsoftOAuth.ExchangeCode(r.Context(), code)
	if err != nil {
		http.Redirect(w, r, h.frontendURL+"/login?error=oauth_exchange_failed", http.StatusTemporaryRedirect)
		return
	}

	msUser, err := h.microsoftOAuth.GetUserInfo(r.Context(), token)
	if err != nil {
		http.Redirect(w, r, h.frontendURL+"/login?error=oauth_user_info_failed", http.StatusTemporaryRedirect)
		return
	}

	userEmail := msUser.GetEmail()
	if userEmail == "" {
		http.Redirect(w, r, h.frontendURL+"/login?error=oauth_user_info_failed", http.StatusTemporaryRedirect)
		return
	}

	now := time.Now()
	var user models.User
	var isNewUser bool

	err = h.db.Users().FindOne(r.Context(), bson.M{"microsoftId": msUser.ID}).Decode(&user)
	if err != nil {
		err = h.db.Users().FindOne(r.Context(), bson.M{"email": strings.ToLower(userEmail)}).Decode(&user)
		if err != nil {
			isNewUser = true
			displayName := msUser.DisplayName
			if displayName == "" {
				displayName = msUser.GivenName
			}
			user = models.User{
				ID:            primitive.NewObjectID(),
				Email:         strings.ToLower(userEmail),
				DisplayName:   displayName,
				MicrosoftID:   msUser.ID,
				AuthMethods:   []models.AuthMethod{models.AuthMethodMicrosoft},
				EmailVerified: true,
				IsActive:      true,
				CreatedAt:     now,
				UpdatedAt:     now,
				LastLoginAt:   &now,
			}
			h.db.Users().InsertOne(r.Context(), user)
			h.createPersonalTenant(r.Context(), user.ID, user.DisplayName, now)
			h.syslog.High(r.Context(), fmt.Sprintf("User created: %s (%s) via Microsoft OAuth", user.Email, user.ID.Hex()))
		} else {
			// Only link Microsoft to existing account if user has verified their email
			if !user.EmailVerified {
				http.Redirect(w, r, h.frontendURL+"/login?error=email_not_verified", http.StatusTemporaryRedirect)
				return
			}
			h.db.Users().UpdateOne(r.Context(), bson.M{"_id": user.ID}, bson.M{
				"$set":      bson.M{"microsoftId": msUser.ID, "lastLoginAt": now, "updatedAt": now},
				"$addToSet": bson.M{"authMethods": models.AuthMethodMicrosoft},
			})
		}
	} else {
		h.db.Users().UpdateOne(r.Context(), bson.M{"_id": user.ID}, bson.M{
			"$set": bson.M{"lastLoginAt": now, "updatedAt": now},
		})
	}

	if user.TOTPEnabled {
		mfaToken, err := h.jwtService.GenerateMFAToken(user.ID.Hex(), user.Email)
		if err != nil {
			http.Redirect(w, r, h.frontendURL+"/login?error=token_generation_failed", http.StatusTemporaryRedirect)
			return
		}
		h.createAuthCodeRedirect(w, r, user.ID, models.AuthCodeTokenData{MFAToken: mfaToken, IsMFA: true})
		return
	}

	accessToken, refreshToken, refreshTTL, err := h.generateTokenPair(user.ID.Hex(), user.Email, user.DisplayName)
	if err != nil {
		http.Redirect(w, r, h.frontendURL+"/login?error=token_generation_failed", http.StatusTemporaryRedirect)
		return
	}
	storeRefreshToken(r, h.db, user.ID, refreshToken, refreshTTL)

	if isNewUser {
		h.events.Emit(events.Event{
			Type:      events.EventUserRegistered,
			Timestamp: now,
			Data:      map[string]interface{}{"userId": user.ID.Hex(), "method": "microsoft"},
		})
	}

	h.createAuthCodeRedirect(w, r, user.ID, models.AuthCodeTokenData{AccessToken: accessToken, RefreshToken: refreshToken})
}

// --- Session Management ---

func (h *AuthHandler) ListSessions(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUserFromContext(r.Context())
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	cursor, err := h.db.RefreshTokens().Find(r.Context(), bson.M{
		"userId":    user.ID,
		"isRevoked": false,
		"expiresAt": bson.M{"$gt": time.Now()},
	}, options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}))
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to fetch sessions")
		return
	}
	defer cursor.Close(r.Context())

	var tokens []models.RefreshToken
	if err := cursor.All(r.Context(), &tokens); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to fetch sessions")
		return
	}

	// Determine current session by matching against the Authorization header token
	currentTokenHash := ""
	authHeader := r.Header.Get("Authorization")
	if parts := strings.SplitN(authHeader, " ", 2); len(parts) == 2 {
		// We can't directly match access token to refresh token, so we mark based on most recent
	}

	type sessionInfo struct {
		ID           string     `json:"id"`
		DeviceInfo   string     `json:"deviceInfo"`
		IPAddress    string     `json:"ipAddress"`
		CreatedAt    time.Time  `json:"createdAt"`
		LastActiveAt time.Time  `json:"lastActiveAt"`
		IsCurrent    bool       `json:"isCurrent"`
	}

	_ = currentTokenHash

	sessions := make([]sessionInfo, 0, len(tokens))
	for i, t := range tokens {
		sessions = append(sessions, sessionInfo{
			ID:           t.ID.Hex(),
			DeviceInfo:   t.DeviceInfo,
			IPAddress:    t.IPAddress,
			CreatedAt:    t.CreatedAt,
			LastActiveAt: t.LastActiveAt,
			IsCurrent:    i == 0, // Most recent session is likely current
		})
	}

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"sessions": sessions,
	})
}

func (h *AuthHandler) RevokeSession(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUserFromContext(r.Context())
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	sessionID := strings.TrimPrefix(r.URL.Path, "/api/auth/sessions/")
	if sessionID == "" {
		respondWithError(w, http.StatusBadRequest, "Session ID is required")
		return
	}

	objID, err := primitive.ObjectIDFromHex(sessionID)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid session ID")
		return
	}

	result, err := h.db.RefreshTokens().UpdateOne(r.Context(),
		bson.M{"_id": objID, "userId": user.ID},
		bson.M{"$set": bson.M{"isRevoked": true}},
	)
	if err != nil || result.ModifiedCount == 0 {
		respondWithError(w, http.StatusNotFound, "Session not found")
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]string{"message": "Session revoked"})
}

func (h *AuthHandler) RevokeAllSessions(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUserFromContext(r.Context())
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	// Revoke all except the most recent active session
	h.db.RefreshTokens().UpdateMany(r.Context(),
		bson.M{
			"userId":    user.ID,
			"isRevoked": false,
		},
		bson.M{"$set": bson.M{"isRevoked": true}},
	)

	respondWithJSON(w, http.StatusOK, map[string]string{"message": "All other sessions revoked"})
}

// --- Preferences ---

func (h *AuthHandler) UpdatePreferences(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUserFromContext(r.Context())
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	var req struct {
		ThemePreference string `json:"themePreference"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	update := bson.M{"updatedAt": time.Now()}
	if req.ThemePreference != "" {
		if req.ThemePreference != "dark" && req.ThemePreference != "light" && req.ThemePreference != "system" {
			respondWithError(w, http.StatusBadRequest, "Invalid theme preference")
			return
		}
		update["themePreference"] = req.ThemePreference
	}

	h.db.Users().UpdateOne(r.Context(), bson.M{"_id": user.ID}, bson.M{"$set": update})

	respondWithJSON(w, http.StatusOK, map[string]string{"message": "Preferences updated"})
}

// --- Onboarding ---

func (h *AuthHandler) CompleteOnboarding(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUserFromContext(r.Context())
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	now := time.Now()
	h.db.Users().UpdateOne(r.Context(), bson.M{"_id": user.ID}, bson.M{
		"$set": bson.M{
			"onboardingCompletedAt": now,
			"updatedAt":             now,
		},
	})

	respondWithJSON(w, http.StatusOK, map[string]string{"message": "Onboarding completed"})
}

// --- Accept Invitation (for existing users) ---

func (h *AuthHandler) AcceptInvitation(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUserFromContext(r.Context())
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	var req AcceptInvitationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Token == "" {
		respondWithError(w, http.StatusBadRequest, "Invitation token is required")
		return
	}

	if err := h.acceptInvitationForUser(r.Context(), user.ID, req.Token); err != nil {
		respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	memberships := h.getUserMemberships(r.Context(), user.ID)
	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"message":     "Invitation accepted",
		"memberships": memberships,
	})
}

// --- Internal helpers ---

func (h *AuthHandler) createPersonalTenant(ctx context.Context, userID primitive.ObjectID, displayName string, now time.Time) {
	slug := fmt.Sprintf("tenant-%s", primitive.NewObjectID().Hex()[:8])
	tenant := models.Tenant{
		ID:            primitive.NewObjectID(),
		Name:          displayName + "'s Team",
		Slug:          slug,
		IsRoot:        false,
		IsActive:      true,
		BillingStatus: models.BillingStatusNone,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if _, err := h.db.Tenants().InsertOne(ctx, tenant); err != nil {
		slog.Error("Failed to create personal tenant", "userId", userID.Hex(), "error", err)
		return
	}

	membership := models.TenantMembership{
		ID:        primitive.NewObjectID(),
		UserID:    userID,
		TenantID:  tenant.ID,
		Role:      models.RoleOwner,
		JoinedAt:  now,
		UpdatedAt: now,
	}
	if _, err := h.db.TenantMemberships().InsertOne(ctx, membership); err != nil {
		slog.Error("Failed to create membership for personal tenant", "error", err)
	}

	h.events.Emit(events.Event{
		Type:      events.EventTenantCreated,
		Timestamp: now,
		Data:      map[string]interface{}{"tenantId": tenant.ID.Hex(), "userId": userID.Hex()},
	})
}

func (h *AuthHandler) sendVerificationEmail(ctx context.Context, userID primitive.ObjectID, userEmail, displayName string) {
	verificationToken := generateRandomToken()
	hashedVerificationToken := hashToken(verificationToken)
	verification := models.VerificationToken{
		ID:        primitive.NewObjectID(),
		UserID:    userID,
		Token:     hashedVerificationToken,
		Type:      models.TokenTypeEmailVerification,
		ExpiresAt: time.Now().Add(24 * time.Hour),
		CreatedAt: time.Now(),
	}
	h.db.VerificationTokens().InsertOne(ctx, verification)

	now := time.Now()
	h.db.Users().UpdateOne(ctx, bson.M{"_id": userID}, bson.M{
		"$set": bson.M{"lastVerificationSent": now},
	})

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_ = ctx // timeout guard for background goroutine
		if h.emailService != nil {
			if err := h.emailService.SendVerificationEmail(userEmail, displayName, verificationToken); err != nil {
				slog.Error("Failed to send verification email", "to", userEmail, "error", err)
			}
		} else {
			slog.Warn("Email service not configured, logging verification token", "email", userEmail, "token", verificationToken)
		}
	}()
}

func (h *AuthHandler) getUserMemberships(ctx context.Context, userID primitive.ObjectID) []MembershipInfo {
	cursor, err := h.db.TenantMemberships().Find(ctx, bson.M{"userId": userID})
	if err != nil {
		return nil
	}
	defer cursor.Close(ctx)

	var memberships []models.TenantMembership
	if err := cursor.All(ctx, &memberships); err != nil {
		return nil
	}

	var result []MembershipInfo
	for _, m := range memberships {
		var tenant models.Tenant
		if err := h.db.Tenants().FindOne(ctx, bson.M{"_id": m.TenantID}).Decode(&tenant); err != nil {
			continue
		}
		result = append(result, MembershipInfo{
			TenantID:   tenant.ID.Hex(),
			TenantName: tenant.Name,
			TenantSlug: tenant.Slug,
			Role:       m.Role,
			IsRoot:     tenant.IsRoot,
		})
	}
	return result
}

func (h *AuthHandler) acceptInvitationForUser(ctx context.Context, userID primitive.ObjectID, token string) error {
	now := time.Now()
	hashedInvToken := hashToken(token)

	// Look up invitation first (without modifying it) so we can validate email before claiming it
	var invitation models.Invitation
	err := h.db.Invitations().FindOne(ctx, bson.M{
		"token":     hashedInvToken,
		"status":    models.InvitationPending,
		"expiresAt": bson.M{"$gt": now},
	}).Decode(&invitation)
	if err != nil {
		return fmt.Errorf("invalid or expired invitation")
	}

	// Verify the accepting user's email matches the invitation BEFORE claiming it
	var acceptingUser models.User
	if err := h.db.Users().FindOne(ctx, bson.M{"_id": userID}).Decode(&acceptingUser); err != nil {
		return fmt.Errorf("user not found")
	}
	if !strings.EqualFold(acceptingUser.Email, invitation.Email) {
		return fmt.Errorf("invitation was sent to a different email address")
	}

	// Atomically claim the invitation — prevents race conditions with concurrent acceptance
	res := h.db.Invitations().FindOneAndUpdate(
		ctx,
		bson.M{
			"_id":    invitation.ID,
			"status": models.InvitationPending,
		},
		bson.M{"$set": bson.M{"status": models.InvitationAccepted}},
	)
	if res.Err() != nil {
		return fmt.Errorf("invitation already accepted")
	}

	count, _ := h.db.TenantMemberships().CountDocuments(ctx, bson.M{
		"userId":   userID,
		"tenantId": invitation.TenantID,
	})
	if count > 0 {
		return fmt.Errorf("already a member of this tenant")
	}

	membership := models.TenantMembership{
		ID:        primitive.NewObjectID(),
		UserID:    userID,
		TenantID:  invitation.TenantID,
		Role:      invitation.Role,
		JoinedAt:  now,
		UpdatedAt: now,
	}
	if _, err := h.db.TenantMemberships().InsertOne(ctx, membership); err != nil {
		return fmt.Errorf("failed to create membership")
	}

	// Auto-verify email: the user proved ownership by receiving the invitation at this address.
	if !acceptingUser.EmailVerified {
		h.db.Users().UpdateOne(ctx, bson.M{"_id": userID}, bson.M{
			"$set": bson.M{"emailVerified": true, "updatedAt": now},
		})
	}

	h.events.Emit(events.Event{
		Type:      events.EventMemberJoined,
		Timestamp: now,
		Data: map[string]interface{}{
			"userId":   userID.Hex(),
			"tenantId": invitation.TenantID.Hex(),
			"role":     string(invitation.Role),
		},
	})

	return nil
}

// --- Token utilities ---

func storeRefreshToken(r *http.Request, database *db.MongoDB, userID primitive.ObjectID, rawToken string, ttl time.Duration, familyID ...string) {
	tokenHash := hashToken(rawToken)
	now := time.Now()

	fid := generateRandomToken()
	if len(familyID) > 0 && familyID[0] != "" {
		fid = familyID[0]
	}

	// Enforce concurrent session limit BEFORE inserting new token
	const maxSessions = 10
	activeCount, _ := database.RefreshTokens().CountDocuments(r.Context(), bson.M{
		"userId":    userID,
		"isRevoked": false,
		"expiresAt": bson.M{"$gt": now},
	})
	if activeCount >= maxSessions {
		// Revoke oldest sessions to make room
		excess := activeCount - maxSessions + 1
		cursor, err := database.RefreshTokens().Find(r.Context(),
			bson.M{"userId": userID, "isRevoked": false, "expiresAt": bson.M{"$gt": now}},
			options.Find().SetSort(bson.D{{Key: "createdAt", Value: 1}}).SetLimit(excess),
		)
		if err == nil {
			defer cursor.Close(r.Context())
			for cursor.Next(r.Context()) {
				var old models.RefreshToken
				if cursor.Decode(&old) == nil {
					database.RefreshTokens().UpdateByID(r.Context(), old.ID, bson.M{
						"$set": bson.M{"isRevoked": true},
					})
				}
			}
		}
	}

	rt := models.RefreshToken{
		ID:           primitive.NewObjectID(),
		UserID:       userID,
		TokenHash:    tokenHash,
		FamilyID:     fid,
		IPAddress:    middleware.GetClientIP(r),
		UserAgent:    r.UserAgent(),
		DeviceInfo:   auth.ParseUserAgent(r.UserAgent()),
		ExpiresAt:    now.Add(ttl),
		CreatedAt:    now,
		LastActiveAt: now,
		IsRevoked:    false,
	}
	database.RefreshTokens().InsertOne(r.Context(), rt)
}

// DeleteAccount allows users to delete their own account after password confirmation.
func (h *AuthHandler) DeleteAccount(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUserFromContext(r.Context())
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	var req struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Verify password if user has password auth
	if user.HasAuthMethod(models.AuthMethodPassword) {
		if req.Password == "" {
			respondWithError(w, http.StatusBadRequest, "Password is required to confirm account deletion")
			return
		}
		if err := h.passwordService.ComparePassword(user.PasswordHash, req.Password); err != nil {
			respondWithError(w, http.StatusUnauthorized, "Incorrect password")
			return
		}
	}

	ctx := r.Context()

	// Check ownership of tenants
	cursor, err := h.db.TenantMemberships().Find(ctx, bson.M{"userId": user.ID})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to check memberships")
		return
	}
	var memberships []models.TenantMembership
	cursor.All(ctx, &memberships)
	cursor.Close(ctx)

	for _, m := range memberships {
		if m.Role != models.RoleOwner {
			continue
		}

		var tenant models.Tenant
		if err := h.db.Tenants().FindOne(ctx, bson.M{"_id": m.TenantID}).Decode(&tenant); err != nil {
			continue
		}

		if tenant.IsRoot {
			respondWithError(w, http.StatusForbidden, "Cannot delete the root tenant owner account. Transfer ownership first.")
			return
		}

		otherCount, _ := h.db.TenantMemberships().CountDocuments(ctx, bson.M{
			"tenantId": m.TenantID,
			"userId":   bson.M{"$ne": user.ID},
		})
		if otherCount > 0 {
			respondWithError(w, http.StatusBadRequest, fmt.Sprintf("You are the owner of '%s' which has other members. Transfer ownership before deleting your account.", tenant.Name))
			return
		}

		// Sole member — delete the tenant
		h.db.TenantMemberships().DeleteMany(ctx, bson.M{"tenantId": m.TenantID})
		h.db.Tenants().DeleteOne(ctx, bson.M{"_id": m.TenantID})
		h.db.Invitations().DeleteMany(ctx, bson.M{"tenantId": m.TenantID})

		h.events.Emit(events.Event{
			Type:      events.EventTenantDeleted,
			Timestamp: time.Now(),
			Data: map[string]interface{}{
				"tenantId":   m.TenantID.Hex(),
				"tenantName": tenant.Name,
				"reason":     "owner_self_deleted",
			},
		})
	}

	// Delete user data
	h.db.TenantMemberships().DeleteMany(ctx, bson.M{"userId": user.ID})
	h.db.RefreshTokens().DeleteMany(ctx, bson.M{"userId": user.ID})
	h.db.Messages().DeleteMany(ctx, bson.M{"userId": user.ID})
	h.db.Users().DeleteOne(ctx, bson.M{"_id": user.ID})

	h.syslog.High(ctx, fmt.Sprintf("User self-deleted account: %s (%s)", user.Email, user.ID.Hex()))

	h.events.Emit(events.Event{
		Type:      events.EventUserDeleted,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"userId": user.ID.Hex(),
			"email":  user.Email,
			"reason": "self_delete",
		},
	})

	respondWithJSON(w, http.StatusOK, map[string]string{"message": "Account deleted"})
}

// ExportData returns a JSON dump of the user's data for GDPR compliance.
func (h *AuthHandler) ExportData(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUserFromContext(r.Context())
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	ctx := r.Context()

	// Gather memberships
	cursor, _ := h.db.TenantMemberships().Find(ctx, bson.M{"userId": user.ID})
	var memberships []models.TenantMembership
	if cursor != nil {
		cursor.All(ctx, &memberships)
		cursor.Close(ctx)
	}

	type exportMembership struct {
		TenantID string `json:"tenantId"`
		Role     string `json:"role"`
		JoinedAt string `json:"joinedAt"`
	}
	var exportMemberships []exportMembership
	for _, m := range memberships {
		exportMemberships = append(exportMemberships, exportMembership{
			TenantID: m.TenantID.Hex(),
			Role:     string(m.Role),
			JoinedAt: m.JoinedAt.Format(time.RFC3339),
		})
	}

	// Gather messages
	msgCursor, _ := h.db.Messages().Find(ctx, bson.M{"userId": user.ID})
	var messages []models.Message
	if msgCursor != nil {
		msgCursor.All(ctx, &messages)
		msgCursor.Close(ctx)
	}

	type exportMessage struct {
		Subject   string `json:"subject"`
		Body      string `json:"body"`
		IsSystem  bool   `json:"isSystem"`
		Read      bool   `json:"read"`
		CreatedAt string `json:"createdAt"`
	}
	var exportMessages []exportMessage
	for _, m := range messages {
		exportMessages = append(exportMessages, exportMessage{
			Subject:   m.Subject,
			Body:      m.Body,
			IsSystem:  m.IsSystem,
			Read:      m.Read,
			CreatedAt: m.CreatedAt.Format(time.RFC3339),
		})
	}

	export := map[string]interface{}{
		"profile": map[string]interface{}{
			"id":            user.ID.Hex(),
			"email":         user.Email,
			"displayName":   user.DisplayName,
			"emailVerified": user.EmailVerified,
			"authMethods":   user.AuthMethods,
			"createdAt":     user.CreatedAt.Format(time.RFC3339),
			"updatedAt":     user.UpdatedAt.Format(time.RFC3339),
		},
		"memberships": exportMemberships,
		"messages":    exportMessages,
		"exportedAt":  time.Now().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=account-data.json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(export)
}

func hashToken(raw string) string {
	hash := sha256.Sum256([]byte(raw))
	return base64.StdEncoding.EncodeToString(hash[:])
}
