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
- **Master** pushes features directly to `master` and is the only instance that merges PRs and deploys

## Builder Rules (Critical)

1. **Keep PRs small and fast.** One finding = one PR. Fix only what the finding describes — do not refactor surrounding code, add features, or "improve" things you notice along the way.
2. **Rebase before opening a PR.** Master pushes feature commits to `master` frequently. Before you open your PR, always run:
   ```bash
   git fetch origin master && git rebase origin/master
   ```
   If there are conflicts, resolve them on your branch. Never ask Master to resolve your conflicts.
3. **Touch minimal files.** The fewer files your PR changes, the lower the conflict risk. If a fix requires changes across 5+ files, break it into smaller PRs or coordinate with the user first.
4. **Don't modify `scout/` files beyond status updates.** You may update the status field in finding files and HANDOFF.md. Do not create new findings — that's the scout's job.
5. **Test before opening PR.** Run `cd backend && go build ./cmd/server/... && go build ./internal/...` and `cd frontend && npx tsc --noEmit`. If either fails, fix it before opening the PR.

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
| 002 | Command injection vulnerability in scanner CLI exec() calls | critical | implementing | builder | 2026-03-24 |
