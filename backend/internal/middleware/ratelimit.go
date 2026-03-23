package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type RateLimiter struct {
	// In-memory fallback
	mu       sync.RWMutex
	requests map[string]*rateLimitEntry
	cleanup  *time.Ticker
	done     chan bool
	stopOnce sync.Once

	// MongoDB-backed (distributed)
	collection *mongo.Collection
}

type rateLimitEntry struct {
	count     int
	windowEnd time.Time
}

// rateLimitDoc is the MongoDB document for distributed rate limiting.
type rateLimitDoc struct {
	Key       string    `bson:"_id"`
	Count     int       `bson:"count"`
	WindowEnd time.Time `bson:"windowEnd"`
	ExpiresAt time.Time `bson:"expiresAt"`
}

type RateLimitConfig struct {
	MaxRequests int
	Window      time.Duration
}

var (
	AccountCreationLimit    = RateLimitConfig{MaxRequests: 5, Window: time.Hour}
	LoginAttemptLimit       = RateLimitConfig{MaxRequests: 10, Window: 15 * time.Minute}
	PasswordResetLimit      = RateLimitConfig{MaxRequests: 5, Window: time.Hour}
	ResendVerificationLimit = RateLimitConfig{MaxRequests: 3, Window: 60 * time.Second}
	OAuthInitLimit          = RateLimitConfig{MaxRequests: 10, Window: time.Minute}
	EmailVerificationLimit  = RateLimitConfig{MaxRequests: 10, Window: time.Hour}
	TokenRefreshLimit       = RateLimitConfig{MaxRequests: 30, Window: time.Minute}
	InvitationLimit         = RateLimitConfig{MaxRequests: 20, Window: time.Hour}
	MFAChallengeLimit       = RateLimitConfig{MaxRequests: 3, Window: 5 * time.Minute}
	MagicLinkLimit          = RateLimitConfig{MaxRequests: 5, Window: 15 * time.Minute}
	EmailPasswordResetLimit = RateLimitConfig{MaxRequests: 3, Window: time.Hour}
	EmailMagicLinkLimit     = RateLimitConfig{MaxRequests: 3, Window: time.Hour}
	UsageRecordLimit        = RateLimitConfig{MaxRequests: 120, Window: time.Minute}
	ResetTokenVerifyLimit   = RateLimitConfig{MaxRequests: 10, Window: 15 * time.Minute}
	MagicLinkVerifyLimit    = RateLimitConfig{MaxRequests: 10, Window: 15 * time.Minute}
	CSVExportLimit              = RateLimitConfig{MaxRequests: 5, Window: time.Hour}
	TelemetryAnonymousLimit     = RateLimitConfig{MaxRequests: 60, Window: time.Minute}
	TelemetryAuthenticatedLimit = RateLimitConfig{MaxRequests: 120, Window: time.Minute}
	// Scanner limits
	ScanPublicLimit        = RateLimitConfig{MaxRequests: 5, Window: time.Hour}
	ScanAuthenticatedLimit = RateLimitConfig{MaxRequests: 20, Window: time.Hour}
)

// NewRateLimiter creates an in-memory-only rate limiter (fallback mode).
func NewRateLimiter() *RateLimiter {
	rl := &RateLimiter{
		requests: make(map[string]*rateLimitEntry),
		cleanup:  time.NewTicker(5 * time.Minute),
		done:     make(chan bool),
	}
	go func() {
		for {
			select {
			case <-rl.cleanup.C:
				rl.cleanupExpired()
			case <-rl.done:
				return
			}
		}
	}()
	return rl
}

// NewDistributedRateLimiter creates a rate limiter backed by MongoDB.
// Falls back to in-memory if MongoDB operations fail.
func NewDistributedRateLimiter(db *mongo.Database) *RateLimiter {
	rl := NewRateLimiter()
	rl.collection = db.Collection("rate_limits")

	// Ensure TTL index for automatic cleanup.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, _ = rl.collection.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "expiresAt", Value: 1}},
		Options: options.Index().SetExpireAfterSeconds(0),
	})

	return rl
}

func (rl *RateLimiter) Stop() {
	rl.cleanup.Stop()
	rl.stopOnce.Do(func() { close(rl.done) })
}

func (rl *RateLimiter) cleanupExpired() {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	now := time.Now()
	for key, entry := range rl.requests {
		if now.After(entry.windowEnd) {
			delete(rl.requests, key)
		}
	}
}

// Allow checks the rate limit for the given key. Uses MongoDB if available.
// When distributed mode is configured but fails, falls back to the local
// in-memory limiter so transient DB issues don't block all requests.
func (rl *RateLimiter) Allow(key string, config RateLimitConfig) (bool, int, time.Time) {
	if rl.collection != nil {
		allowed, remaining, resetTime, err := rl.allowDistributed(key, config)
		if err == nil {
			return allowed, remaining, resetTime
		}
		// Fall back to local in-memory limiter when distributed check fails.
		slog.Warn("distributed rate-limit check failed, falling back to local", "key", key, "error", err)
	}
	return rl.allowLocal(key, config)
}

func (rl *RateLimiter) allowDistributed(key string, config RateLimitConfig) (bool, int, time.Time, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	now := time.Now()
	windowEnd := now.Add(config.Window)
	// expiresAt gives MongoDB TTL some buffer to clean up after the window.
	expiresAt := windowEnd.Add(time.Minute)

	// Atomically increment counter, creating the doc if it doesn't exist.
	// If the window has expired, reset the counter.
	opts := options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After)

	var doc rateLimitDoc

	// First, try to increment within an existing valid window.
	err := rl.collection.FindOneAndUpdate(ctx,
		bson.M{"_id": key, "windowEnd": bson.M{"$gt": now}},
		bson.M{"$inc": bson.M{"count": 1}},
		opts,
	).Decode(&doc)

	if err != nil {
		if err == mongo.ErrNoDocuments {
			// No valid window exists — reset/create with count=1.
			err = rl.collection.FindOneAndUpdate(ctx,
				bson.M{"_id": key},
				bson.M{"$set": bson.M{
					"count":     1,
					"windowEnd": windowEnd,
					"expiresAt": expiresAt,
				}},
				opts,
			).Decode(&doc)
			if err != nil {
				return false, 0, now, err
			}
		} else {
			return false, 0, now, err
		}
	}

	if doc.Count > config.MaxRequests {
		return false, 0, doc.WindowEnd, nil
	}

	remaining := config.MaxRequests - doc.Count
	return true, remaining, doc.WindowEnd, nil
}

func (rl *RateLimiter) allowLocal(key string, config RateLimitConfig) (bool, int, time.Time) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	entry, exists := rl.requests[key]

	if !exists || now.After(entry.windowEnd) {
		rl.requests[key] = &rateLimitEntry{
			count:     1,
			windowEnd: now.Add(config.Window),
		}
		return true, config.MaxRequests - 1, now.Add(config.Window)
	}

	if entry.count >= config.MaxRequests {
		return false, 0, entry.windowEnd
	}

	entry.count++
	return true, config.MaxRequests - entry.count, entry.windowEnd
}

func GetClientIP(r *http.Request) string {
	// Fly-Client-IP is set by the Fly.io proxy and cannot be spoofed by clients.
	// Prefer this over X-Forwarded-For which clients can forge.
	if flyIP := r.Header.Get("Fly-Client-IP"); flyIP != "" {
		if net.ParseIP(flyIP) != nil {
			return flyIP
		}
	}

	if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
		if net.ParseIP(strings.TrimSpace(realIP)) != nil {
			return strings.TrimSpace(realIP)
		}
	}

	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Use the first (leftmost) IP — that's the original client
		parts := strings.SplitN(xff, ",", 2)
		ip := strings.TrimSpace(parts[0])
		if net.ParseIP(ip) != nil {
			return ip
		}
	}

	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

func (rl *RateLimiter) RateLimitHandler(config RateLimitConfig, keyFunc func(*http.Request) string, handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := keyFunc(r)
		allowed, remaining, resetTime := rl.Allow(key, config)

		w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", config.MaxRequests))
		w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))
		w.Header().Set("X-RateLimit-Reset", resetTime.Format(time.RFC3339))

		if !allowed {
			retryAfter := int(time.Until(resetTime).Seconds())
			if retryAfter < 1 {
				retryAfter = 1
			}
			w.Header().Set("Retry-After", fmt.Sprintf("%d", retryAfter))
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error":      "Rate limit exceeded",
				"retryAfter": retryAfter,
			})
			return
		}

		handler(w, r)
	}
}
