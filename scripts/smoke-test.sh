#!/usr/bin/env bash
# Production smoke tests for mcplens.dev
# Run: bash scripts/smoke-test.sh [BASE_URL]
#
# These are non-destructive GET requests that verify critical endpoints are up.
# Exit code 0 = all passed, 1 = at least one failure.

set -euo pipefail

BASE_URL="${1:-https://mcplens.dev}"
FAILURES=0
TOTAL=0

check() {
  local name="$1"
  local method="$2"
  local path="$3"
  local expected_status="$4"
  local body="${5:-}"

  TOTAL=$((TOTAL + 1))

  local args=(-s -o /dev/null -w "%{http_code}" -X "$method")
  if [[ -n "$body" ]]; then
    args+=(-H "Content-Type: application/json" -d "$body")
  fi
  args+=("${BASE_URL}${path}")

  local status
  status=$(curl "${args[@]}" 2>/dev/null || echo "000")

  if [[ "$status" == "$expected_status" ]]; then
    echo "  PASS  $name ($status)"
  else
    echo "  FAIL  $name — expected $expected_status, got $status"
    FAILURES=$((FAILURES + 1))
  fi
}

echo "=== MCPLens Production Smoke Tests ==="
echo "Target: $BASE_URL"
echo ""

echo "--- Health & Bootstrap ---"
check "Bootstrap status"          GET  "/api/bootstrap/status"     200
check "Landing page"              GET  "/"                         200

echo ""
echo "--- Public Pages ---"
check "Scan page"                 GET  "/scan"                     200
check "Terms page"                GET  "/terms"                    200
check "Privacy page"              GET  "/privacy"                  200
check "Login page"                GET  "/login"                    200

echo ""
echo "--- Auth Discovery ---"
check "Auth providers"            GET  "/api/auth/providers"       200

echo ""
echo "--- Public API (scan) ---"
check "Get scan (nonexistent)"    GET  "/api/scan/domain/test-nonexistent-domain-12345.com"  404

echo ""
echo "--- Auth Gates (should reject) ---"
check "Get me (no auth)"          GET  "/api/auth/me"              401
check "Tracked stores (no auth)"  GET  "/api/tracked-stores"       401
check "Billing config (no auth)"  GET  "/api/billing/config"       401
check "Admin dashboard (no auth)" GET  "/api/admin/dashboard"      401

echo ""
echo "--- Branding ---"
check "Branding config"           GET  "/api/branding"             200

echo ""
echo "--- Input Validation ---"
check "Register empty body"       POST "/api/auth/register"       400 '{}'
check "Login empty body"          POST "/api/auth/login"          400 '{}'
check "Scan empty domain"         POST "/api/scan"                400 '{"domain":""}'

echo ""
echo "=== Results: $((TOTAL - FAILURES))/$TOTAL passed ==="

if [[ "$FAILURES" -gt 0 ]]; then
  echo "FAILED: $FAILURES test(s) failed"
  exit 1
else
  echo "ALL PASSED"
  exit 0
fi
