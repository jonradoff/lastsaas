// Package handlers implements HTTP request handlers for the API.
//
// Logging strategy: This package uses two logging mechanisms:
//   - syslog.Logger for auditable business events (user creation, ownership transfer, etc.)
//   - stdlib log.Printf for operational diagnostics (email delivery failures in background
//     goroutines, etc.) where structured logging adds overhead without audit value.
package handlers

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
)

type ErrorResponse struct {
	Error string `json:"error"`
}

func respondWithJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(payload)
}

func respondWithError(w http.ResponseWriter, status int, message string) {
	respondWithJSON(w, status, ErrorResponse{Error: message})
}

func generateRandomToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9.!#$%&'*+/=?^_` + "`" + `{|}~-]+@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$`)

func isValidEmail(email string) bool {
	if len(email) > 254 {
		return false
	}
	return emailRegex.MatchString(email)
}

var regexMetaReplacer = regexp.MustCompile(`[.*+?^${}()|[\]\\]`)

// sanitizeCSVField prevents CSV injection by prefixing formula-triggering characters with a single quote.
func sanitizeCSVField(s string) string {
	if len(s) > 0 && strings.ContainsRune("=+-@\t\r", rune(s[0])) {
		return "'" + s
	}
	return s
}

// escapeRegexInput escapes special regex characters in user input for safe use in MongoDB $regex queries.
func escapeRegexInput(s string) string {
	return regexMetaReplacer.ReplaceAllStringFunc(s, func(c string) string {
		return `\` + c
	})
}
