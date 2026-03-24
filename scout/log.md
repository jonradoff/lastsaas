# Scout Run Log

<!-- Append-only log of each scout cycle -->

## Run 2026-03-24 20:00
- Phase completed: health
- Finding: 001 - Resend email API returning 401 Unauthorized
- Site status: up (machine auto-stopped earlier, cold-start caused brief 502)
- Also noted: Google/GitHub/Microsoft OAuth not configured, DataDog not configured (lower priority)

## Run 2026-03-24 20:45
- Phase completed: health + audit (scanner)
- Finding: 002 - Command injection vulnerability in scanner CLI exec() calls (CRITICAL)
- Verification: 001 marked regression — Resend 401 still in logs at 20:37:32, no new deployment
- Site status: up
- Scanner audit found 26 total issues (1 critical, 6 high, 10 medium, 2 low) — will file remaining in future cycles
