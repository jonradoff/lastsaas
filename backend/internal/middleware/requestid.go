package middleware

import (
	"context"
	"crypto/rand"
	"fmt"
	"net/http"
	"time"
)

const RequestIDContextKey contextKey = "requestId"

// RequestID generates a unique ID for each request, sets it as a response header,
// and stores it in the request context for downstream logging.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := generateRequestID()
		w.Header().Set("X-Request-ID", id)
		ctx := context.WithValue(r.Context(), RequestIDContextKey, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetRequestID returns the request ID from the context, or empty string if not set.
func GetRequestID(ctx context.Context) string {
	id, _ := ctx.Value(RequestIDContextKey).(string)
	return id
}

func generateRequestID() string {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		// Fallback to timestamp-based ID on catastrophic rand failure
		return fmt.Sprintf("%x", time.Now().UnixNano())
	}
	return fmt.Sprintf("%x", b)
}
