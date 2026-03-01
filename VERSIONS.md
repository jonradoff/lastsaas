# LastSaaS Version Notes

## v1.2 — March 1, 2026

### Product Analytics & Telemetry
- **Conversion funnel dashboard** — visualize the customer journey from visitor to paid subscriber with conversion rates at each step (Visitors → Signups → Plan Page Views → Checkouts → Paid Conversions → Upgrades)
- **SaaS KPIs** — MRR, ARR, ARPU, LTV, churn rate, trial-to-paid conversion rate, median time to first purchase, active subscriber count with trend sparklines
- **Retention cohort analysis** — weekly or monthly cohort retention heatmap tracking user engagement over time
- **Engagement metrics** — DAU/WAU/MAU for paying subscribers, average sessions per user, top features by usage, credit consumption trend
- **Custom event explorer** — browse all telemetry event types, view trend charts, filter by name and time range
- **Telemetry Go SDK** — `telemetry.Track()`, `TrackBatch()`, `TrackPageView()`, `TrackCheckoutStarted()`, `TrackLogin()` for zero-overhead in-process event recording
- **Telemetry REST API** — anonymous endpoint for page views (rate-limited at 60/min per IP) and authenticated endpoints for custom events (120/min per user)
- **Auto-instrumentation** — registration, email verification, login, checkout, subscription activation/cancellation, and plan changes tracked automatically with no configuration
- **365-day retention** with MongoDB TTL auto-expiration

### MCP Server (AI Admin Access)
- **26 read-only tools** across 13 categories for AI-powered admin access
- **2 resources** — `lastsaas://dashboard` and `lastsaas://health` for automatic context
- Tool categories: About, Dashboard, Tenants, Users, Financial, Logs, Health, Config, Plans, Announcements, Promotions, Security, Webhooks
- API key authentication with root-tenant scope
- Stdio transport compatible with Claude Desktop and Claude Code
- MCP registry manifests for discoverability

### System Health Monitoring
- Automatic node registration with 30-second heartbeat
- Metrics collection every 60 seconds: CPU, memory, disk, network, HTTP request stats, MongoDB stats, Go runtime
- HTTP metrics middleware with percentile latency tracking (p50/p95/p99)
- Threshold-based alerting with configurable warning/critical levels
- 30-day data retention via MongoDB TTL indexes
- Real-time dashboard with 8 time-series charts (Recharts)
- Aggregate, all-nodes overlay, and single-node filter modes
- Time range selection: 1h, 6h, 24h, 7d, 30d
- Integration health panel (MongoDB, Stripe, Resend, Google OAuth connectivity)
- Send Test Email button for Resend integration verification

### Authentication Enhancements
- **MFA/TOTP** two-factor authentication with setup wizard and recovery codes
- **Magic link** passwordless login via email
- **Google, GitHub, and Microsoft OAuth** with automatic account linking
- **Passkey/WebAuthn** support for passwordless authentication
- **Dark/light theme** preference per user
- **Session management** — list active sessions, revoke individual or all sessions
- Password reset for OAuth-only accounts
- Auto-verify email when accepting a team invitation
- Account lockout after failed login attempts

### Billing & Commerce
- **Per-seat pricing model** with included seats, min/max seat limits
- **Free trials** with configurable trial days per plan and trial abuse prevention
- **Stripe Tax** integration for automatic tax calculation
- **Promotion codes and coupons** — create and manage via admin UI with expiration dates and product restrictions
- **Credit bundles** for one-time credit purchases
- **PDF invoice generation** with company name, address, and tax breakdown
- **Multi-currency support** with configurable default currency
- **Refund and dispute handling** via webhook handlers

### White-Label Branding
- Custom app name, tagline, and logo (text, image, or both modes)
- Theme colors with auto-generated shade palettes
- Custom fonts (body and heading), favicon, media library
- Custom landing page, custom pages at `/p/{slug}` with SEO metadata
- CSS injection, head HTML injection (analytics, meta tags)
- Configurable navigation sidebar with entitlement-gated items
- Auth page customization (login/signup headings and subtext)
- Dashboard HTML customization, Open Graph image support

### API Keys & Outgoing Webhooks
- `lsk_`-prefixed API keys with admin and user authority scopes
- SHA-256 hashed storage, last-used tracking
- 19 webhook event types across billing, team lifecycle, user lifecycle, credits, and security
- HMAC-SHA256 payload signing with `whsec_`-prefixed secrets
- Delivery tracking, test events, secret regeneration, event type filtering

### Admin Interface Improvements
- **Three-tier admin access**: user (read-only), admin (read-write), owner (destructive)
- **Root Members management** — manage the admin team with invitations and role changes
- **Admin impersonation** — log in as any user for debugging
- **Financial dashboard** — revenue, ARR, DAU, MAU time-series with charting
- **Onboarding flow** for new users
- **Announcements** — publish system-wide announcements
- **In-app messaging** — send messages to individual users
- **CSV export** for users and tenants
- Multi-select severity toggles in log viewer

### Built-in API Documentation
- Interactive HTML reference at `/api/docs` with expandable endpoint cards
- Markdown reference at `/api/docs/markdown` for external documentation
- Comprehensive webhook event reference with payload descriptions
- Auto-versioned from the VERSION file
- Embeddable via iframe (CSP configured)

### CI/CD & Testing
- GitHub Actions CI workflow with Go build, lint, and test
- Codecov integration with coverage badges
- Comprehensive backend test suite (handlers, services, middleware)
- Frontend test setup with Vitest
- MongoDB JSON Schema validation for data integrity
- Hybrid validation: Go struct tags + MongoDB schema enforcement

### Infrastructure & Performance
- Scalability improvements: batch queries, bounded concurrency, fail-open rate limiting
- Server-side app name injection into index.html (eliminates title flicker)
- Compile-time version embedding via ldflags
- CLI tools: `setup`, `start`/`stop`/`restart`, `change-password`, `send-message`, `transfer-root-owner`, `config`, `version`, `status`, `mcp`

### Security Hardening
- Security headers (CSP, HSTS, X-Frame-Options, X-Content-Type-Options, Referrer-Policy, Permissions-Policy)
- Rate limiting on authentication and telemetry endpoints
- NoSQL injection protection with regex input escaping
- XSS protection via DOMPurify for injected HTML
- Webhook signature verification (Stripe inbound, HMAC-SHA256 outbound)
- System log injection detection with automatic critical alerts
- Refresh token rotation with family-based revocation
- Billing attack vector hardening

---

## v1.0 — February 22, 2026

### Initial Public Release
- Multi-tenant architecture with role-based access control (owner/admin/user)
- Email/password authentication with bcrypt hashing and JWT tokens
- Email verification via Resend
- Stripe Checkout integration for subscription billing
- Stripe Billing Portal for customer self-service
- Plan management with entitlements (boolean and numeric)
- Billing enforcement middleware
- Dual credit buckets (subscription + purchased) with configurable reset policies
- Team invitations and member management
- Ownership transfer between members
- Per-tenant activity logs
- User profile management and account deletion
- Admin dashboard with user and tenant management
- Configuration variable editor (strings, numbers, enums, templates)
- Dockerized deployment (Go + React + Alpine)
- Fly.io deployment configuration
- Graceful shutdown with connection draining
- Auto-versioning with database migration on startup
