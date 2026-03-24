package apierror

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWrite(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)

	Write(w, http.StatusBadRequest, CodeBadRequest, "invalid input", r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}

	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type = %q, want 'application/json'", ct)
	}

	var resp Response
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Error != "invalid input" {
		t.Errorf("Error = %q, want 'invalid input'", resp.Error)
	}
	if resp.Code != CodeBadRequest {
		t.Errorf("Code = %q, want %q", resp.Code, CodeBadRequest)
	}
}

func TestHelperFunctions(t *testing.T) {
	tests := []struct {
		name           string
		fn             func(http.ResponseWriter, *http.Request, string)
		expectedStatus int
		expectedCode   Code
	}{
		{"BadRequest", BadRequest, 400, CodeBadRequest},
		{"Unauthorized", Unauthorized, 401, CodeUnauthorized},
		{"Forbidden", Forbidden, 403, CodeForbidden},
		{"NotFound", NotFound, 404, CodeNotFound},
		{"Conflict", Conflict, 409, CodeConflict},
		{"Validation", Validation, 400, CodeValidation},
		{"Internal", Internal, 500, CodeInternal},
		{"RateLimited", RateLimited, 429, CodeRateLimited},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest("GET", "/test", nil)

			tt.fn(w, r, "test message")

			if w.Code != tt.expectedStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.expectedStatus)
			}

			var resp Response
			if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
				t.Fatalf("failed to decode: %v", err)
			}
			if resp.Code != tt.expectedCode {
				t.Errorf("code = %q, want %q", resp.Code, tt.expectedCode)
			}
			if resp.Error != "test message" {
				t.Errorf("error = %q, want 'test message'", resp.Error)
			}
		})
	}
}

func TestCodeConstants(t *testing.T) {
	codes := map[Code]string{
		CodeBadRequest:       "BAD_REQUEST",
		CodeUnauthorized:     "UNAUTHORIZED",
		CodeForbidden:        "FORBIDDEN",
		CodeNotFound:         "NOT_FOUND",
		CodeConflict:         "CONFLICT",
		CodeRateLimited:      "RATE_LIMITED",
		CodeValidation:       "VALIDATION_ERROR",
		CodePaymentRequired:  "PAYMENT_REQUIRED",
		CodeNotInitialized:   "NOT_INITIALIZED",
		CodeInternal:         "INTERNAL_ERROR",
		CodeServiceUnavail:   "SERVICE_UNAVAILABLE",
		CodeMFARequired:      "MFA_REQUIRED",
		CodeAccountLocked:    "ACCOUNT_LOCKED",
		CodeAccountInactive:  "ACCOUNT_INACTIVE",
		CodeTokenExpired:     "TOKEN_EXPIRED",
		CodeEmailNotVerified: "EMAIL_NOT_VERIFIED",
		CodePlanLimit:        "PLAN_LIMIT",
	}

	for code, expected := range codes {
		if string(code) != expected {
			t.Errorf("code %q != expected %q", code, expected)
		}
	}
}
