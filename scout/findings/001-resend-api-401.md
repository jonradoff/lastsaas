---
id: 001
type: bug
severity: high
status: new
found: 2026-03-24
phase: health
---

## Resend email integration returning 401 Unauthorized

**What:** The Resend API integration is failing with HTTP 401 on every health check cycle (every 5 minutes). This means all transactional emails are broken — email verification, password resets, team invitations, rescan alerts, etc. Users who sign up cannot verify their email.

**Where:** Fly.io logs for app `mcplens`, health monitoring integration check

**Evidence:**
```
2026/03/24 19:56:31 WARN health: integration unhealthy integration=resend error="resend API returned status 401"
```
This appears on every health check cycle (19:48:02, 19:53:03, 19:56:31).

**Suggested Fix:** The `RESEND_API_KEY` environment variable on Fly.io is either missing, expired, or invalid. Steps:
1. Check the key: `flyctl secrets list -a mcplens` — verify `RESEND_API_KEY` exists
2. Go to https://resend.com/api-keys and generate a new key (or verify the existing one)
3. Update: `flyctl secrets set RESEND_API_KEY=re_xxxx -a mcplens`
4. The app will auto-restart and the health check should pass
