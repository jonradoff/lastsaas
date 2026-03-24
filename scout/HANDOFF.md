# Scout - Builder Handoff

This file is the coordination protocol between the **Scout** (monitors site, finds issues, researches improvements) and the **Builder** (implements fixes and features).

## How It Works

1. **Scout** discovers an issue or improvement → creates a finding in `scout/findings/` → adds a row here with status `new`
2. **Builder** pulls, sees `new` items → picks one, changes status to `implementing`, pushes
3. **Builder** finishes work and deploys → changes status to `done`, pushes
4. **Scout** on next cycle → pulls, verifies `done` items on the live site → marks `verified` or `regression`

## Status Values

- `new` — Scout found this, not yet picked up
- `implementing` — Builder is working on it
- `done` — Builder says it's fixed/shipped
- `verified` — Scout confirmed the fix on the live site
- `regression` — Scout found the issue returned after being marked done
- `wontfix` — Deliberately skipped (with reason in the finding file)
- `deferred` — Acknowledged but not prioritized now

## Active Findings

| ID | Finding | Severity | Status | Owner | Updated |
|----|---------|----------|--------|-------|---------|
