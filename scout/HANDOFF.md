# Scout - Builder - Master Coordination

This file is the coordination protocol between three Claude Code instances:

- **Scout** — monitors the live site, audits source code, researches improvements
- **Builder** — implements fixes for scout findings (via PRs)
- **Master** — builds features, reviews and merges all PRs, deploys

## Workflow

1. **Scout** discovers an issue → creates a finding in `scout/findings/` → adds a row here with status `new` → pushes directly to `master` (scout ONLY touches `scout/` files)
2. **Builder** pulls, sees `new` items → picks one, changes status to `implementing` → creates a branch `fix/{id}-{slug}` → opens a PR when done → changes status to `pr-open` with PR link
3. **Master** reviews the PR → merges to `master` → deploys with `fly deploy -c fly.saas.toml` → changes status to `done`
4. **Scout** on next cycle → verifies `done` items on the live site → marks `verified` or `regression`

## Branch Rules

- **Scout** pushes directly to `master` but ONLY modifies files in `scout/`
- **Builder** NEVER pushes to `master` — always creates `fix/{id}-{slug}` branches and opens PRs
- **Master** NEVER pushes to `master` — always creates feature branches and opens PRs. Master is the only instance that merges PRs.

## Status Values

- `new` — Scout found this, not yet picked up
- `implementing` — Builder is working on it (on a branch)
- `pr-open` — Builder opened a PR (include PR URL in the finding file)
- `done` — Master merged the PR and deployed
- `verified` — Scout confirmed the fix on the live site
- `regression` — Scout found the issue returned after being marked done
- `wontfix` — Deliberately skipped (with reason in the finding file)
- `deferred` — Acknowledged but not prioritized now

## Active Findings

| ID | Finding | Severity | Status | Owner | Updated |
|----|---------|----------|--------|-------|---------|
| 001 | Resend email API returning 401 — all transactional emails broken | high | regression | scout | 2026-03-24 |
| 002 | Command injection vulnerability in scanner CLI exec() calls | critical | new | scout | 2026-03-24 |
