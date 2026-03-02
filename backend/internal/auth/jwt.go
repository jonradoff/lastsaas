package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrInvalidToken = errors.New("invalid token")
	ErrExpiredToken = errors.New("token has expired")
)

type JWTService struct {
	accessSecret  []byte
	refreshSecret []byte
	accessTTL     time.Duration
	refreshTTL    time.Duration
}

type AccessTokenClaims struct {
	UserID         string `json:"userId"`
	Email          string `json:"email"`
	DisplayName    string `json:"displayName"`
	TokenType      string `json:"tokenType,omitempty"` // "access", "mfa", "impersonation"
	MFAPending     bool   `json:"mfaPending,omitempty"`
	ImpersonatedBy string `json:"impersonatedBy,omitempty"`
	jwt.RegisteredClaims
}

type RefreshTokenClaims struct {
	UserID string `json:"userId"`
	jwt.RegisteredClaims
}

func NewJWTService(accessSecret, refreshSecret string, accessTTLMin, refreshTTLDay int) *JWTService {
	accessTTL := 60 * time.Minute
	refreshTTL := 30 * 24 * time.Hour
	if accessTTLMin > 0 {
		accessTTL = time.Duration(accessTTLMin) * time.Minute
	}
	if refreshTTLDay > 0 {
		refreshTTL = time.Duration(refreshTTLDay) * 24 * time.Hour
	}
	return &JWTService{
		accessSecret:  []byte(accessSecret),
		refreshSecret: []byte(refreshSecret),
		accessTTL:     accessTTL,
		refreshTTL:    refreshTTL,
	}
}

func (s *JWTService) GenerateAccessToken(userID, email, displayName string) (string, error) {
	return s.GenerateAccessTokenWithTTL(userID, email, displayName, s.accessTTL)
}

func (s *JWTService) GenerateAccessTokenWithTTL(userID, email, displayName string, ttl time.Duration) (string, error) {
	if ttl <= 0 {
		ttl = s.accessTTL
	}
	claims := AccessTokenClaims{
		UserID:      userID,
		Email:       email,
		DisplayName: displayName,
		TokenType:   "access",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.accessSecret)
}

func (s *JWTService) GenerateMFAToken(userID, email string) (string, error) {
	claims := AccessTokenClaims{
		UserID:     userID,
		Email:      email,
		TokenType:  "mfa",
		MFAPending: true,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(5 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.accessSecret)
}

func (s *JWTService) GenerateImpersonationToken(userID, email, displayName, impersonatedBy string) (string, error) {
	claims := AccessTokenClaims{
		UserID:         userID,
		Email:          email,
		DisplayName:    displayName,
		TokenType:      "impersonation",
		ImpersonatedBy: impersonatedBy,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(5 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.accessSecret)
}

func (s *JWTService) GenerateRefreshToken(userID string) (string, error) {
	return s.GenerateRefreshTokenWithTTL(userID, s.refreshTTL)
}

func (s *JWTService) GenerateRefreshTokenWithTTL(userID string, ttl time.Duration) (string, error) {
	if ttl <= 0 {
		ttl = s.refreshTTL
	}
	claims := RefreshTokenClaims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.refreshSecret)
}

func (s *JWTService) ValidateAccessToken(tokenString string) (*AccessTokenClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &AccessTokenClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return s.accessSecret, nil
	})
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredToken
		}
		return nil, ErrInvalidToken
	}
	claims, ok := token.Claims.(*AccessTokenClaims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}
	return claims, nil
}

func (s *JWTService) ValidateRefreshToken(tokenString string) (*RefreshTokenClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &RefreshTokenClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return s.refreshSecret, nil
	})
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredToken
		}
		return nil, ErrInvalidToken
	}
	claims, ok := token.Claims.(*RefreshTokenClaims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}
	return claims, nil
}

func (s *JWTService) GetAccessTTL() time.Duration {
	return s.accessTTL
}

func (s *JWTService) GetRefreshTTL() time.Duration {
	return s.refreshTTL
}
