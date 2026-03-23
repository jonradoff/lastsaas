# MCPLens Development Rules

## Validation

MCPLens uses hybrid validation: Go-side (`validate` struct tags via go-playground/validator) and MongoDB JSON Schema (`internal/db/schema.go`).

**When modifying model structs in `internal/models/`:**
1. Update `validate` struct tags on the model
2. Update the corresponding MongoDB JSON Schema in `internal/db/schema.go`
3. Keep both in sync — the Go tags and MongoDB schema must enforce the same constraints
4. Run `cd backend && go test ./internal/validation/...` to verify

**When adding a new collection that accepts user/API writes:**
1. Add `validate` tags to the model struct
2. Add a schema function to `internal/db/schema.go` and include it in `AllSchemas()`
3. Add tests in `internal/validation/validate_test.go`

## System Logging

Use `syslog.Logger` for all significant system events. Severity levels: critical, high, medium, low, debug.

## Build Verification

Always verify after changes:
```bash
cd backend && go build ./...
cd frontend && npx tsc --noEmit
```

## Dependent Project Deployment (CRITICAL)

MCPLens **MUST** deploy using the SaaS Dockerfile (`Dockerfile.saas`) and the corresponding Fly config (`fly.saas.toml`). Never use bare `fly deploy`.

**Why this matters:** The SaaS Dockerfile runs both the product backend AND the MCPLens backend behind Caddy (via supervisord). The MCPLens backend serves all auth endpoints (`/api/auth/*`), bootstrap status (`/api/bootstrap/status`), OAuth providers (Google, etc.), billing, and admin APIs. Without it, login breaks silently — the product backend has no auth routes, so API calls return HTML from the SPA catch-all, causing mysterious redirects to `/setup` or broken login forms with missing OAuth buttons.

**Correct deploy command:**
```bash
fly deploy -c fly.saas.toml
```

**Propagation rule:** When setting up or working on any dependent project, ensure:
1. The project has a `deploy.md` at its root with full deployment instructions and the "why" behind the multi-process architecture
2. The project's Claude Code memory (MEMORY.md or CLAUDE.md) contains a cross-reference: "See `deploy.md` — never bare `fly deploy`"
3. If the project doesn't have these yet, create them before the first deployment
