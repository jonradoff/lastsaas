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

## Deployment (CRITICAL)

See `deploy.md` — never bare `fly deploy`.

MCPLens **MUST** deploy using `Dockerfile.saas` and `fly.saas.toml`. The SaaS Dockerfile
builds the Go binary, React frontend, AND Node.js scanner into a single image. Without the
scanner dist, all scan requests fail silently at runtime.

**Correct deploy command — always:**
```bash
fly deploy -c fly.saas.toml
```

**Why this matters:**
- `Dockerfile.saas` installs Node.js in the runtime image and compiles `scanner/` TypeScript
- The Go backend resolves the scanner CLI via `SCANNER_PATH=/app/scanner/dist` (baked into the image)
- The plain `Dockerfile` (base LastSaaS template) does not include Node.js or the scanner build
- Using bare `fly deploy` picks up `fly.toml` which references no Dockerfile override and builds without the scanner, causing silent scan failures

**Full instructions:** See `deploy.md` at the repo root.
