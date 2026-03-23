# MCPLens

[![CI](https://github.com/jonradoff/lastsaas/actions/workflows/ci.yml/badge.svg)](https://github.com/jonradoff/lastsaas/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/jonradoff/lastsaas/branch/master/graph/badge.svg)](https://codecov.io/gh/jonradoff/lastsaas)
[![Go Report Card](https://goreportcard.com/badge/github.com/jonradoff/lastsaas)](https://goreportcard.com/report/github.com/jonradoff/lastsaas)

**The last SaaS boilerplate you'll ever need.**

LastSaaS is a complete, production-ready SaaS foundation built entirely through conversation with [Claude Code](https://claude.ai/claude-code). It gives you multi-tenant account management, authentication, role-based access control, white-label branding, Stripe billing, API keys, outgoing webhooks, a full admin interface, system health monitoring, credit-based usage tracking, and product analytics with telemetry — everything you need to launch a SaaS business, ready to customize for your specific product.

The bottleneck for building software isn't engineering capacity anymore — it's imagination. LastSaaS proves it: a single person with a clear vision and an AI agent can stand up what used to require a team and months of work. And because it was built with [Claude Code](https://claude.ai/claude-code), the codebase is fork-ready for agentic engineering — point an AI agent at it and keep building your product through conversation.

**[Project Page](https://metavert.io/lastsaas)**

---

## Why LastSaaS Exists

Every SaaS product needs the same boring foundation: user accounts, teams, roles, authentication, admin dashboards, billing, usage limits, branding, webhooks, API keys. Historically, building that foundation meant weeks of plumbing before you could write a single line of your actual product.

LastSaaS eliminates that. Fork it, point an AI agent at it, and start building your product on top of a foundation that already handles:

- Multi-tenant isolation with role-based access
- JWT authentication with refresh token rotation
- Google, GitHub, and Microsoft OAuth integration
- Magic link passwordless authentication
- MFA/TOTP with recovery codes
- Email verification and password resets
- Team invitations and member management
- Stripe billing with subscriptions, per-seat pricing, trials, and credit bundles
- Plan entitlements and billing enforcement middleware
- White-label branding with custom themes, logos, landing pages, and custom pages
- API key authentication (admin and user scopes)
- Outgoing webhooks with 19 event types and HMAC-SHA256 signing
- Credit-based usage tracking (subscription + purchased buckets)
- Promotion codes and coupon management via Stripe
- Product analytics dashboard (conversion funnel, KPIs, retention cohorts, engagement metrics)
- Telemetry event system with Go SDK and REST API for custom event tracking
- A full admin interface for managing everything
- Built-in API documentation (HTML and Markdown)
- Real-time system health monitoring
- Financial metrics dashboard (revenue, ARR, DAU, MAU)
- MCP (Model Context Protocol) server for AI-powered admin access
- CLI tools for server administration
- Auto-versioning with database migrations
- Production deployment on Fly.io

This is open-source infrastructure for the agentic era of software — where the person with the idea is also the person who ships it. The codebase follows consistent patterns that AI agents navigate fluently, so you can keep evolving it the same way it was built.

---

## How It Compares

If you're evaluating SaaS boilerplates, you've probably looked at ShipFast, Supastarter, MakerKit, SaaS Pegasus, and Gravity. Here's why technical founders choose LastSaaS instead.

**Free and open-source.** ShipFast costs $169, MakerKit runs $199–599, Supastarter starts at $299, SaaS Pegasus charges $249/year, and Gravity is under $1K. LastSaaS is MIT-licensed — fork it, ship it, never pay a license fee. You own the code completely.

**Go backend, not another Next.js project.** ShipFast, Supastarter, MakerKit, and Gravity are all JavaScript/TypeScript stacks. SaaS Pegasus uses Django. LastSaaS pairs a Go backend with a React + TypeScript frontend — giving you compiled-binary deployment, low memory footprint (a 14MB Alpine container), and the concurrency model that Go is known for. If your SaaS will handle real traffic or you want a backend that isn't a Node.js monolith, this matters.

**Genuine multi-tenancy.** ShipFast has no multi-tenancy at all. SaaS Pegasus and Gravity offer basic team features but not true tenant isolation. LastSaaS gives you full multi-tenant architecture: tenant-scoped data isolation, three-tier RBAC (owner/admin/user), team invitations, ownership transfer, per-tenant activity logs, and per-tenant billing. This is the difference between "users can collaborate" and "each customer gets their own isolated workspace."

**White-label branding built in.** Most boilerplates give you a theme toggle at best. LastSaaS includes a full white-label system: custom app name, logo, colors, fonts, landing page, custom pages, CSS injection, favicon, configurable navigation with entitlement gating, and auth page customization. If you're building a platform where customers see your brand (not yours-plus-a-framework), this saves weeks.

**Outgoing webhooks, not just Stripe webhooks.** None of the alternatives — ShipFast, Supastarter, MakerKit, SaaS Pegasus, or Gravity — include an outgoing webhook system. LastSaaS ships with 19 event types across billing, team lifecycle, user lifecycle, credits, and security events, with HMAC-SHA256 signing, delivery tracking, and test events. Your customers can integrate with your platform from day one.

**API keys with scoped access.** ShipFast, Supastarter, and MakerKit don't include API key management. LastSaaS provides `lsk_`-prefixed API keys with admin and user authority scopes, SHA-256 hashed storage, and last-used tracking — ready for your customers to build integrations.

**Health monitoring and financial dashboards.** No competing boilerplate includes system health monitoring. LastSaaS collects CPU, memory, disk, HTTP, and MongoDB metrics every 60 seconds across all nodes, with 8 real-time charts, threshold alerting, and 30-day retention. The financial dashboard gives you revenue, ARR, DAU, and MAU time-series out of the box.

**Product analytics with telemetry.** No competing boilerplate includes product analytics. LastSaaS auto-instruments the customer journey — visitor → signup → plan page → checkout → paid conversion → upgrade — and visualizes it as a conversion funnel. The PM dashboard includes SaaS KPIs (MRR, ARR, ARPU, LTV, churn rate, trial conversion), retention cohort analysis, engagement metrics (DAU/WAU/MAU for paying subscribers), and a custom event explorer. A Go SDK and REST API let you track your own events with zero configuration.

**MCP server for AI-native operations.** This is unique to LastSaaS. A built-in Model Context Protocol server with 32 read-only tools lets you connect Claude (or any MCP-compatible AI) directly to your running application. Query your ARR trend, investigate error spikes, audit API keys, or review system health — all in natural language. No other SaaS boilerplate offers agentic admin access.

**Built for AI-assisted development.** LastSaaS was built entirely through conversation with Claude Code, and the codebase is designed to keep being built that way. Consistent patterns, clear naming, and a structure AI agents navigate fluently. Fork it, point an agent at it, describe your product, and keep going. The competing boilerplates were built for manual development — LastSaaS is built for the way software is made now.

| | LastSaaS | ShipFast | Supastarter | MakerKit | Pegasus | Gravity |
|---|---|---|---|---|---|---|
| **Price** | **Free (MIT)** | $169 | $299+ | $199–599 | $249/yr | <$1K |
| **Stack** | Go + React | Next.js | Next.js / Nuxt | Next.js | Django | Node + React |
| **Multi-Tenancy** | Full RBAC | — | ✓ | ✓ | Teams only | Teams only |
| **White-Label** | Full | — | Partial | Partial | — | — |
| **Webhooks** | 19 events | — | — | — | — | — |
| **API Keys** | Scoped | — | — | — | Basic | — |
| **Health Monitoring** | 8 charts | — | — | — | — | — |
| **Product Analytics** | 5-tab PM dashboard | — | — | — | — | — |
| **Credit System** | Dual buckets | — | — | Basic | — | — |
| **MCP Server** | 26 tools | — | — | — | — | — |
| **Admin Dashboard** | Full | — | ✓ | ✓ | ✓ | Basic |
| **Stripe Billing** | Full | ✓ | ✓ | ✓ | ✓ | ✓ |

---

## Features

### Authentication & Identity
- Email/password registration with bcrypt hashing
- Email verification via [Resend](https://resend.com)
- Google, GitHub, and Microsoft OAuth with automatic account linking
- Magic link passwordless login via email
- MFA/TOTP two-factor authentication with setup wizard
- Recovery codes for MFA backup access
- JWT access tokens (30min) + refresh tokens (7 days) with rotation
- Account lockout after failed login attempts
- Password reset flow with secure tokens
- Password strength enforcement
- Session management — list active sessions, revoke individual or all sessions
- Session revocation on password change

### Multi-Tenancy
- Root tenant (system admin) + customer tenants
- Users belong to tenants via memberships
- Roles: **owner**, **admin**, **user** with hierarchical permissions
- Team invitations with email notifications
- Ownership transfer between members
- Per-tenant activity log
- Tenant settings self-service

### Billing & Credits (Stripe)
- **Subscription plans** with monthly and annual billing (configurable annual discount %)
- **Pricing models**: flat-rate or per-seat (with included seats, min/max seat limits)
- **Free trials** with configurable trial days per plan and trial abuse prevention
- **Credit bundles** for one-time purchases
- **Dual credit buckets**: subscription credits (reset or accrue) + purchased credits
- **Stripe Checkout** (redirect-based) for payment collection
- **Stripe Billing Portal** for customer self-service (payment methods, invoices)
- **Multi-currency support** with configurable default currency
- **Stripe Tax** automatic tax calculation
- **Promotion codes and coupons** — create and manage via admin UI, linked to Stripe
- **Invoice generation** — sequential invoice numbers, PDF download, tax breakdown
- **Transaction history** — per-tenant and admin-wide with search and filtering
- **Financial metrics** — revenue, ARR, DAU, MAU time-series with charting
- **Billing enforcement middleware** — blocks expired subscriptions from paid features
- **Entitlement middleware** — gate features based on plan (boolean and numeric entitlements)
- **Billing waiver** for special accounts (root tenant, demo accounts)
- **Admin subscription management** — cancel, modify, reassign plans
- **Refund and dispute handling** — webhook handlers for `charge.refunded`, `charge.dispute.created`, `charge.dispute.closed`

### White-Label Branding
- Custom app name, tagline, and logo (text, image, or both modes)
- Theme colors (primary, accent, background, surface, text) with auto-generated shade palettes
- Custom fonts (body and heading)
- Custom landing page with configurable HTML
- Custom pages served at `/p/{slug}` with SEO metadata
- Custom CSS injection
- Custom head HTML injection (analytics, meta tags)
- Favicon upload
- Media library for image/asset management
- Configurable navigation sidebar with entitlement-gated items
- Auth page customization (login/signup headings and subtext)
- Dashboard HTML customization
- Open Graph image support

### API Keys
- Create API keys with `lsk_` prefix
- Two authority levels: **admin** (auto-resolves root tenant) and **user** (requires X-Tenant-ID)
- SHA-256 hashed storage — raw key shown only at creation
- Last-used timestamp tracking
- Admin UI for key management
- Supports both JWT and API key authentication on all endpoints

### Outgoing Webhooks
- 19 event types across 5 tiers:
  - **Billing**: subscription.activated, subscription.canceled, payment.received, payment.failed
  - **Team lifecycle**: member.invited, member.joined, member.removed, member.role_changed, ownership.transferred
  - **User lifecycle**: user.registered, user.verified, user.deactivated
  - **Credits & billing**: credits.purchased, plan.changed, tenant.created, tenant.deactivated
  - **Audit & security**: user.deleted, tenant.deleted, api_key.created, api_key.revoked
- HMAC-SHA256 payload signing with `whsec_`-prefixed secrets
- Delivery tracking with response codes, response bodies, and duration
- Test event delivery
- Secret regeneration
- Event type filtering per webhook

### Admin Interface
- **Dashboard** with user/tenant counts, health overview, and business metrics
- **User management** — list, search, view profiles, edit, suspend, impersonate, delete with ownership preflight
- **Tenant management** — list, view, edit, plan assignment, status control, subscription management
- **Financial overview** — transaction history across all tenants, revenue/ARR/DAU/MAU charts
- **Product analytics** — conversion funnel, SaaS KPIs, retention cohorts, engagement metrics, custom event explorer
- **Plan management** — create, edit, archive, entitlements, per-seat configuration, trial days
- **Credit bundle management** — create, edit, sort, activate/deactivate
- **Promotions** — create and manage Stripe promotion codes and coupons
- **Branding editor** — theme colors, logos, fonts, landing page, custom pages, CSS, navigation
- **API key management** — create, view, revoke
- **Webhook management** — create, edit, delete, test, view delivery history
- **Announcements** — publish system-wide announcements
- **System log viewer** with severity filtering, search, and user filtering
- **Configuration variable editor** (strings, numbers, enums, templates)
- **In-app messaging** — send messages to individual users
- **Root members** — manage the admin team with invitations and role changes
- **CSV export** for users and tenants
- **Admin impersonation** — log in as any user for debugging
- **System health monitoring** (see below)
- **Integration health checks** — MongoDB, Stripe, Resend, Google OAuth status
- Three-tier admin access: **user** (read-only), **admin** (read-write), **owner** (destructive operations)

### System Health Monitoring
- Automatic node registration with heartbeat (30s interval)
- Metrics collection every 60s: CPU, memory, disk, network, HTTP request stats, MongoDB stats, Go runtime
- HTTP metrics middleware with percentile latency tracking (p50/p95/p99)
- Threshold-based alerting (configurable warning/critical levels)
- 30-day automatic data retention via MongoDB TTL indexes
- Real-time dashboard with 8 time-series charts (Recharts)
- Aggregate, all-nodes overlay, and single-node filter modes
- Time range selection: 1h, 6h, 24h, 7d, 30d
- Integration health panel (MongoDB, Stripe, Resend, Google OAuth connectivity)

### Product Analytics & Telemetry
- **Conversion funnel** — Visitors → Signups → Plan Page Views → Checkouts → Paid Conversions → Upgrades, with conversion rates at each step
- **SaaS KPIs** — MRR, ARR, ARPU, LTV, churn rate, trial-to-paid conversion rate, median time to first purchase, active subscriber count
- **Retention cohorts** — weekly or monthly cohort retention table with color-coded heatmap (tracks `lastLoginAt` over time)
- **Engagement metrics** — DAU/WAU/MAU for paying subscribers, average sessions per user, top features by usage, credit consumption trend
- **Custom event explorer** — browse all event types, view trend charts, filter by name and time range
- **Telemetry Go SDK** — `telemetry.Track()` / `TrackBatch()` / `TrackPageView()` / `TrackCheckoutStarted()` / `TrackLogin()` for direct in-process event recording (no HTTP overhead)
- **Telemetry REST API** — `POST /telemetry/track` (anonymous, rate-limited) for page views; `POST /telemetry/events` and `/telemetry/events/batch` (authenticated) for custom events
- **Auto-instrumentation** — registration, email verification, login, checkout, subscription activation/cancellation, and plan changes are tracked automatically
- **365-day retention** — telemetry events auto-expire via MongoDB TTL index
- **Rate limiting** — 60 req/min per IP for anonymous tracking, 120 req/min per user for authenticated events

### User Self-Service
- Profile editing (display name, email)
- Theme preference (light/dark)
- Password management
- MFA setup and management
- Session viewer with remote revocation
- Account deletion with data cleanup
- Data export (GDPR-friendly)
- Billing management (plan selection, credit purchases, invoice history, PDF download)
- Onboarding flow

### Built-in API Documentation
- Interactive HTML API reference at `/api/docs` with expandable endpoint details
- Markdown API reference at `/api/docs/markdown` for integration in external docs
- Comprehensive webhook event reference with payload descriptions
- Auto-versioned from the VERSION file

### CLI Administration
- `lastsaas setup` — Initialize the system (create root tenant + owner)
- `lastsaas start` / `stop` / `restart` — Server process management
- `lastsaas change-password` — Reset any user's password
- `lastsaas send-message` — Send system messages to users
- `lastsaas transfer-root-owner` — Transfer root tenant ownership
- `lastsaas config list|get|set` — Manage configuration variables
- `lastsaas version` — Show binary and database versions
- `lastsaas status` — Check system health
- `lastsaas mcp` — Start the MCP server (see [MCP Server](#mcp-server-ai-admin-access) below)

### MCP Server (AI Admin Access)

A built-in [Model Context Protocol](https://modelcontextprotocol.io) server gives AI assistants like Claude read-only access to your admin data — dashboards, users, tenants, financials, logs, health, and more. Useful for asking questions like "what's our ARR trend?" or "show me critical logs from the last hour" in natural language.

- **32 read-only tools** across 14 categories — no write operations, safe by design
- **2 resources** — `lastsaas://dashboard` and `lastsaas://health` for automatic context
- **API key authentication** — requires a root-tenant API key, same auth as the admin API
- **Stdio transport** — runs locally, compatible with Claude Desktop and Claude Code

**Tool categories:**
- **About** — software version and environment
- **Dashboard** — user/tenant counts, health overview
- **Tenants** — list with filtering, detailed view with members
- **Users** — list with search, detailed view with auth methods and memberships
- **Financial** — transaction history, revenue/ARR/DAU/MAU time-series metrics
- **Logs** — full-text search with severity/category/date filters, severity counts
- **Health** — current system metrics, time-series health data, node list, integration status
- **Config** — list and inspect runtime configuration variables
- **Plans** — plan details, entitlement keys, credit bundles
- **Announcements** — list published and draft announcements
- **Promotions** — Stripe promotion codes with coupon details
- **Security** — API key inventory (previews only), root tenant members
- **Webhooks** — webhook configs, event type reference, delivery history

### Security
- Security headers (CSP, HSTS, X-Frame-Options, X-Content-Type-Options, Referrer-Policy, Permissions-Policy)
- Rate limiting on authentication endpoints
- Request body size limits
- NoSQL injection protection (regex input escaping)
- XSS protection via DOMPurify for injected HTML
- Trusted proxy IP resolution (Fly-Client-IP)
- Webhook signature verification (Stripe inbound, HMAC-SHA256 outbound)
- Idempotent webhook processing via unique event ID index
- System log injection detection with automatic critical alerts
- Refresh token rotation with family-based revocation

### Production Ready
- Dockerized multi-stage build (Go + Node + Alpine)
- Fly.io deployment with auto-stop/auto-start machines
- SPA serving from the Go binary (no separate web server needed)
- CORS, security headers, rate limiting
- Graceful shutdown with connection draining
- Auto-versioning with database migration on startup
- Version notification messages to admin users after upgrades

---

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Backend | Go 1.25, gorilla/mux |
| Frontend | React 19, TypeScript, Vite 7, Tailwind CSS 4 |
| Database | MongoDB (Atlas or local) |
| Auth | JWT (access + refresh), bcrypt, Google/GitHub/Microsoft OAuth, Magic Links, TOTP MFA |
| Billing | Stripe (stripe-go v82) — Checkout, Billing Portal, Webhooks, Tax |
| Email | Resend |
| Charts | Recharts |
| Metrics | gopsutil v4 |
| PDF | gofpdf (invoice generation) |
| Security | DOMPurify (frontend), HMAC-SHA256 (webhooks) |
| Deployment | Docker, Fly.io |

---

## Prerequisites

- **Go 1.25+** — [install](https://go.dev/doc/install)
- **Node.js 22+** — [install](https://nodejs.org)
- **MongoDB** — either:
  - [MongoDB Atlas](https://www.mongodb.com/atlas) (free M0 tier works fine)
  - Local MongoDB Community Edition
- **Git**

### Optional
- [Resend](https://resend.com) API key — for email verification, password resets, and invitations
- Google OAuth credentials — for Google sign-in
- GitHub OAuth credentials — for GitHub sign-in
- Microsoft OAuth credentials — for Microsoft sign-in
- [Stripe](https://stripe.com) account — for billing (subscriptions, credit purchases, invoices)
- [Fly.io](https://fly.io) account — for production deployment

---

## Quick Start

### 1. Clone the repository

```bash
git clone https://github.com/jonradoff/lastsaas.git
cd lastsaas
```

### 2. Run the setup script

```bash
./scripts/setup.sh
```

This will prompt you for:
- **Database name** — the project identity (two projects sharing a name share the same user base)
- **MongoDB URI** — your Atlas connection string or `mongodb://localhost:27017`
- **JWT secrets** — auto-generated
- **Google OAuth credentials** — optional, press Enter to skip
- **Resend API key** — optional, press Enter to skip
- **App name and email settings**

It writes a `.env` file and copies the config template.

### 3. Start the backend

```bash
set -a && source .env && set +a
cd backend
go run ./cmd/server
```

The server starts on `http://localhost:4290`.

### 4. Start the frontend

In a separate terminal:

```bash
set -a && source .env && set +a
cd frontend
npm install
npm run dev
```

The frontend starts on `http://localhost:4280`.

### 5. Initialize the system

Run the CLI setup to create the root tenant and admin account:

```bash
cd backend
go run ./cmd/lastsaas setup
```

This creates the root tenant (your admin organization) and the owner account. You can now log in at `http://localhost:4280`.

---

## Setting Up Stripe Billing

Stripe integration is optional but required for paid subscriptions, credit bundle purchases, and invoice generation. If you skip this section, LastSaaS works as a free-tier-only platform.

### 1. Create a Stripe account

Sign up at [stripe.com](https://stripe.com) and complete onboarding. You can use **test mode** during development.

### 2. Get your API keys

Go to **Stripe Dashboard → Developers → API keys** and copy:
- **Publishable key** (starts with `pk_test_` or `pk_live_`)
- **Secret key** (starts with `sk_test_` or `sk_live_`)

### 3. Create a webhook endpoint

Go to **Stripe Dashboard → Developers → Webhooks → Add endpoint**:

- **Endpoint URL**: `https://your-domain.com/api/billing/webhook`
  - For local development with the Stripe CLI: `stripe listen --forward-to localhost:4290/api/billing/webhook`
- **Events to subscribe to** — select these 8 events:
  - `checkout.session.completed`
  - `invoice.paid`
  - `invoice.payment_failed`
  - `customer.subscription.updated`
  - `customer.subscription.deleted`
  - `charge.refunded`
  - `charge.dispute.created`
  - `charge.dispute.closed`

After creating the endpoint, copy the **Signing secret** (starts with `whsec_`).

### 4. Set environment variables

Add these to your `.env` file:

```bash
STRIPE_SECRET_KEY=sk_test_...
STRIPE_PUBLISHABLE_KEY=pk_test_...
STRIPE_WEBHOOK_SECRET=whsec_...
```

### 5. Create plans in the admin UI

Once the backend is running with Stripe configured:
1. Log in as the root tenant owner
2. Go to **Admin → Plans** and create your subscription plans
3. Set pricing, billing intervals, trial days, entitlements, and credit allocations
4. Optionally create **credit bundles** under Admin → Credit Bundles
5. Optionally create **promotion codes** under Admin → Promotions

Stripe Products and Prices are created automatically when customers check out — you don't need to configure anything in the Stripe Dashboard beyond the API keys and webhook.

### 6. Go live

When you're ready for production:
1. Switch to **live mode** in the Stripe Dashboard
2. Create a new webhook endpoint with your production URL and the same 8 events
3. Update your production environment variables with the live keys and webhook secret

---

## Configuration

Config files live in `backend/config/`:
- `dev.example.yaml` / `prod.example.yaml` — committed templates
- `dev.yaml` / `prod.yaml` — your actual configs (gitignored)

Set `LASTSAAS_ENV=dev` or `LASTSAAS_ENV=prod` to select which config to load. Defaults to `dev`.

Secrets are referenced as `${ENV_VAR}` in YAML and expanded from environment variables at load time. Default values use `${VAR:default}` syntax.

### Environment Variables

| Variable | Required | Description |
|----------|----------|------------|
| `DATABASE_NAME` | Yes | Project identity — shared name = shared user base |
| `MONGODB_URI` | Yes | MongoDB connection string |
| `JWT_ACCESS_SECRET` | Yes | Secret for signing access tokens |
| `JWT_REFRESH_SECRET` | Yes | Secret for signing refresh tokens |
| `FRONTEND_URL` | Yes | Frontend URL for CORS and email links |
| `APP_NAME` | Yes | Your application name (used in emails, UI) |
| `STRIPE_SECRET_KEY` | No | Stripe secret API key |
| `STRIPE_PUBLISHABLE_KEY` | No | Stripe publishable key (sent to frontend) |
| `STRIPE_WEBHOOK_SECRET` | No | Stripe webhook signing secret |
| `GOOGLE_CLIENT_ID` | No | Google OAuth client ID |
| `GOOGLE_CLIENT_SECRET` | No | Google OAuth secret |
| `GOOGLE_REDIRECT_URL` | No | Google OAuth redirect URL |
| `RESEND_API_KEY` | No | Resend email service API key |
| `FROM_EMAIL` | No | Sender email address (default: noreply@yourdomain.com) |
| `FROM_NAME` | No | Sender name (default: LastSaaS) |

---

## Project Structure

```
lastsaas/
  backend/
    cmd/
      server/main.go              Entry point (HTTP server, route wiring)
      lastsaas/main.go            CLI administration tool + MCP server
    config/                       YAML config files
    internal/
      api/handlers/               HTTP handlers (auth, admin, tenant, billing, branding, webhooks, etc.)
      apicounter/                  API call counters for integration health
      auth/                       JWT, password hashing, Google/GitHub/Microsoft OAuth, TOTP MFA
      config/                     Config loader with env variable expansion
      configstore/                Runtime configuration (DB-backed, cached)
      db/                         MongoDB connection, collections, indexes
      email/                      Resend email service with templates
      events/                     Internal event emitter (drives webhook deliveries)
      health/                     System health monitoring service
      middleware/                  Auth, tenant, RBAC, rate limiting, metrics, security, billing enforcement
      models/                     All data models
      planstore/                  Plan seeding
      stripe/                     Stripe service (Checkout, Billing Portal, Customers, Prices, Subscriptions)
      syslog/                     System logging service with injection detection
      telemetry/                  Telemetry event collection, Go SDK, and PM analytics queries
      version/                    Version management and auto-migration
  frontend/
    src/
      api/client.ts               Axios API client with token refresh
      components/                 Layout, AdminLayout, BrandingThemeInjector, shared components
      contexts/                   Auth, Tenant, and Branding React contexts
      pages/
        admin/                    Admin interface (dashboard, users, tenants, plans, billing, branding, etc.)
        admin/health/             Health monitoring components and charts
        app/                      Customer-facing pages (dashboard, billing, team, settings, activity)
        app/settings/             User settings tabs (profile, security, MFA, sessions, billing)
        auth/                     Login, signup, MFA challenge, magic link, verification, password reset
        public/                   Landing page and custom pages
      types/index.ts              TypeScript type definitions
  scripts/
    setup.sh                      Interactive setup script
  Dockerfile                      Multi-stage production build
  fly.toml                        Fly.io deployment config
  VERSION                         Current version number
```

---

## API Documentation

LastSaaS includes built-in, self-hosted API documentation:

- **Interactive HTML reference**: `GET /api/docs` — expandable endpoint cards with request/response examples
- **Markdown reference**: `GET /api/docs/markdown` — for embedding in external documentation

The documentation is generated from code and always matches the running version. It covers all endpoints, parameters, request/response formats, and all 19 webhook event types with payload descriptions.

---

## MCP Server Setup

The MCP server lets AI assistants query your admin data in natural language. It proxies read-only requests to the LastSaaS admin API using an API key.

### Prerequisites

1. A running LastSaaS instance (local or deployed)
2. A root-tenant API key — create one in **Admin → API Keys** with **admin** authority

### Usage with Claude Desktop

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json` on macOS):

```json
{
  "mcpServers": {
    "lastsaas": {
      "command": "/path/to/lastsaas",
      "args": ["mcp"],
      "env": {
        "LASTSAAS_URL": "https://your-app.fly.dev",
        "LASTSAAS_API_KEY": "lsk_your_api_key_here"
      }
    }
  }
}
```

### Usage with Claude Code

Add to your project's `.mcp.json`:

```json
{
  "mcpServers": {
    "lastsaas": {
      "command": "/path/to/lastsaas",
      "args": ["mcp"],
      "env": {
        "LASTSAAS_URL": "https://your-app.fly.dev",
        "LASTSAAS_API_KEY": "lsk_your_api_key_here"
      }
    }
  }
}
```

### Build the CLI binary

```bash
cd backend
go build -o lastsaas ./cmd/lastsaas
```

### Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `LASTSAAS_URL` | Yes | Base URL of the LastSaaS instance (e.g. `http://localhost:4290` or `https://your-app.fly.dev`) |
| `LASTSAAS_API_KEY` | Yes | Root-tenant API key with admin authority (starts with `lsk_`) |

### Available Tools (26)

| Tool | Description |
|------|-------------|
| `get_about` | Software version and environment |
| `dashboard_stats` | User/tenant counts and health overview |
| `list_tenants` | Paginated tenant list with search and filters |
| `get_tenant` | Detailed tenant with plan, billing, and members |
| `list_users` | Paginated user list with search and filters |
| `get_user` | Detailed user with auth methods and memberships |
| `list_transactions` | Financial transactions with search and filters |
| `get_financial_metrics` | Revenue, ARR, DAU, MAU time-series |
| `search_logs` | Full-text log search with severity/category/date filters |
| `get_log_severity_counts` | Log counts by severity level |
| `get_system_health` | Current CPU, memory, disk, HTTP stats |
| `get_health_metrics` | Time-series health data (per-node or aggregate) |
| `list_nodes` | Server nodes with status and version |
| `get_integrations` | Third-party integration health |
| `list_config` | All runtime configuration variables |
| `get_config` | Single config variable details |
| `list_plans` | Subscription plans with pricing and entitlements |
| `get_plan` | Detailed plan info |
| `list_entitlement_keys` | Entitlement key catalog |
| `list_credit_bundles` | Credit bundle pricing |
| `list_announcements` | Published and draft announcements |
| `list_promotions` | Stripe promotion codes and coupons |
| `list_api_keys` | API key inventory (previews only) |
| `list_root_members` | Admin team and pending invitations |
| `list_webhooks` | Outbound webhook configurations |
| `list_webhook_event_types` | Available webhook event types |
| `get_webhook` | Webhook detail with delivery history |

### MCP Examples

These examples show how the MCP tools work in practice. Each example lists the user prompt and the exact tool calls that execute behind the scenes.

#### Example 1: Check Business Overview

**Prompt:** "How's my SaaS doing today?"

**Tool calls:**
1. `dashboard_stats` — returns a snapshot:
   ```
   Users: 1,284 (12 new today)
   Tenants: 89 (3 new this week)
   Health: All systems healthy
   Active alerts: 0
   ```

#### Example 2: Investigate Revenue Trend

**Prompt:** "Show me our revenue trend for the last 30 days"

**Tool calls:**
1. `get_financial_metrics` — queries time-series data:
   ```json
   { "range": "30d", "metric": "revenue" }
   ```
   Returns daily revenue data points (values in cents) for charting, e.g.:
   ```
   2026-01-27: $1,250.00
   2026-01-28: $890.00
   ...
   2026-02-25: $2,100.00
   ```
2. `get_financial_metrics` — follow up with ARR:
   ```json
   { "range": "30d", "metric": "arr" }
   ```

#### Example 3: Find a Specific Tenant

**Prompt:** "Look up the Acme Corp tenant and show me their details"

**Tool calls:**
1. `list_tenants` — searches by name:
   ```json
   { "search": "Acme Corp" }
   ```
   Returns matching tenants with IDs, plan names, and billing status.
2. `get_tenant` — fetches full details using the tenant ID:
   ```json
   { "id": "6789abc..." }
   ```
   Returns plan info, billing status, credit balances, subscription dates, and a full member list with roles.

#### Example 4: Search for Critical Errors

**Prompt:** "Were there any critical errors in the last hour?"

**Tool calls:**
1. `search_logs` — filters by severity and time:
   ```json
   {
     "severity": "critical",
     "fromDate": "2026-02-26T14:00:00Z",
     "perPage": "50"
   }
   ```
   Returns log entries with timestamps, categories, messages, and metadata. For example:
   ```
   [critical] 14:23:05 billing — Stripe webhook signature verification failed
   [critical] 14:31:12 security — Log injection pattern detected in user input
   ```
2. `get_log_severity_counts` — provides overall distribution:
   ```
   critical: 2, high: 15, medium: 89, low: 342, debug: 1,204
   ```

#### Example 5: Monitor System Health

**Prompt:** "How's the system health? Are all integrations working?"

**Tool calls:**
1. `get_system_health` — returns current metrics:
   ```
   CPU: 12.3%
   Memory: 245 MB / 512 MB (47.8%)
   Disk: 1.2 GB / 8 GB (15.0%)
   HTTP: 1,234 requests (p95 latency: 45ms)
   MongoDB: 12 active connections
   ```
2. `get_integrations` — checks third-party connectivity:
   ```
   MongoDB: healthy (3ms response)
   Stripe: healthy (120ms response, 89 calls/24h)
   Resend: healthy (95ms response, 12 calls/24h)
   Google OAuth: healthy
   ```

#### Example 6: Audit API Key Usage

**Prompt:** "Show me all active API keys and when they were last used"

**Tool calls:**
1. `list_api_keys` — returns inventory (full keys are never exposed):
   ```
   "Production Automation" (admin) — lsk_...a3f2 — last used: 2 hours ago
   "CI/CD Pipeline" (admin) — lsk_...b7e1 — last used: 15 minutes ago
   "Customer Integration" (user) — lsk_...c9d4 — last used: 3 days ago
   "Old Test Key" (admin) — lsk_...d1e5 — last used: never
   ```

#### Example 7: Review Subscription Plans

**Prompt:** "What plans do we offer and how many subscribers does each have?"

**Tool calls:**
1. `list_plans` — returns all plans with pricing and subscriber counts:
   ```
   Starter: $9/mo ($86/yr) — 45 subscribers — 5 seat limit
   Pro: $29/mo ($278/yr) — 31 subscribers — 25 seat limit, per-seat pricing
   Enterprise: $99/mo ($950/yr) — 8 subscribers — unlimited seats
   ```
2. `list_credit_bundles` — shows available credit purchases:
   ```
   Small Pack: 100 credits — $5.00 — active
   Medium Pack: 500 credits — $20.00 — active
   Large Pack: 2000 credits — $60.00 — active
   ```

#### Example 8: Check Webhook Delivery

**Prompt:** "Are our webhooks delivering successfully? Show me the recent history for the production webhook"

**Tool calls:**
1. `list_webhooks` — shows all configured webhooks:
   ```
   "Production" — https://api.example.com/webhooks — 12 events — active
   "Staging" — https://staging.example.com/hooks — 5 events — active
   ```
2. `get_webhook` — fetches delivery history for the production webhook:
   ```json
   { "id": "abc123..." }
   ```
   Returns recent delivery attempts with status codes, response times, and any failures:
   ```
   2026-02-26 14:30:01 — subscription.activated — 200 OK (120ms)
   2026-02-26 14:15:22 — payment.received — 200 OK (95ms)
   2026-02-26 13:45:10 — member.joined — 500 Error (340ms)
   ```

#### Example 9: User Lookup and Membership

**Prompt:** "Find the user with email alice@example.com and show me their tenant memberships"

**Tool calls:**
1. `list_users` — searches by email:
   ```json
   { "search": "alice@example.com" }
   ```
   Returns the matching user with ID, verification status, and last login.
2. `get_user` — fetches full details:
   ```json
   { "id": "def456..." }
   ```
   Returns:
   ```
   Email: alice@example.com (verified)
   Auth: password + Google OAuth
   MFA: TOTP enabled
   Memberships:
     — Acme Corp (owner) — joined 2026-01-15
     — Side Project Inc (admin) — joined 2026-02-10
   Last login: 2 hours ago
   ```

#### Example 10: Health Metrics Deep Dive

**Prompt:** "Show me the CPU and memory trends across all nodes for the last 24 hours"

**Tool calls:**
1. `list_nodes` — identifies all server nodes:
   ```
   node-1 (v1.0.0) — healthy — last seen: 30s ago
   node-2 (v1.0.0) — healthy — last seen: 28s ago
   ```
2. `get_health_metrics` — fetches aggregate time-series data:
   ```json
   { "range": "24h" }
   ```
   Returns data points every 60 seconds with CPU %, memory %, disk %, HTTP request counts, and latency percentiles — ready for charting.
3. `get_health_metrics` — optionally drill into a specific node:
   ```json
   { "node": "node-1", "range": "24h" }
   ```

---

## Deployment

### Fly.io

LastSaaS includes a Dockerfile and Fly.io configuration for production deployment.

```bash
# Install flyctl if needed
curl -L https://fly.io/install.sh | sh

# Create the app (first time only)
flyctl apps create your-app-name --org your-org

# Set production secrets
flyctl secrets set \
  DATABASE_NAME="your-db-name" \
  MONGODB_URI="mongodb+srv://..." \
  JWT_ACCESS_SECRET="$(openssl rand -hex 32)" \
  JWT_REFRESH_SECRET="$(openssl rand -hex 32)" \
  FRONTEND_URL="https://your-app-name.fly.dev" \
  APP_NAME="YourApp" \
  STRIPE_SECRET_KEY="sk_live_..." \
  STRIPE_PUBLISHABLE_KEY="pk_live_..." \
  STRIPE_WEBHOOK_SECRET="whsec_..."

# Deploy
flyctl deploy
```

The Dockerfile builds both the Go backend and React frontend into a single ~14MB Alpine container. The Go binary serves the frontend SPA directly — no nginx or separate web server required.

### Other Platforms

The Docker image works anywhere containers run. The only external dependency is MongoDB. Set the environment variables listed above and expose port 8080.

---

## Fork It and Keep Building with AI

LastSaaS was built entirely through conversation with [Claude Code](https://claude.ai/claude-code) — every feature, every handler, every component was described in natural language and implemented by an AI agent. But the real point isn't that it *was* built this way — it's that it's designed to *keep* being built this way.

The codebase follows consistent patterns, uses clear naming, and maintains a structure that AI agents navigate fluently. Fork it, point Claude Code at it, and start describing your product. The agent already understands the patterns — authentication, tenancy, billing, middleware, events — and builds on top of them naturally. You're not starting from scratch; you're continuing a conversation.

Here's what's already wired up for you:

1. **Add your product's data models** in `backend/internal/models/`
2. **Add API handlers** in `backend/internal/api/handlers/`
3. **Wire routes** in `backend/cmd/server/main.go`
4. **Add frontend pages** in `frontend/src/pages/`
5. **Use the tenant context** — every authenticated request carries the user's tenant, so your product logic gets multi-tenancy for free
6. **Use the credit system** — check and deduct credits for usage-based features
7. **Use entitlements** — gate features with `middleware.RequireEntitlement(db, "feature_name")` and `middleware.RequireActiveBilling()`
8. **Use the config store** — add runtime-configurable settings without redeployment
9. **Use the event emitter** — emit events from your handlers and they'll automatically be delivered to configured webhooks
10. **Use API keys** — your endpoints automatically support both JWT and API key authentication
11. **Use the branding system** — your UI inherits the white-label theme automatically via the BrandingContext
12. **Use the telemetry system** — track custom events with `telemetry.Track()` in Go or `POST /telemetry/events` from external clients, and they'll automatically appear in the PM dashboard

---

## Privacy Policy

LastSaaS is self-hosted software — you control your database, your hosting, and your data. The MCP server connects to your LastSaaS instance via the HTTP API using read-only tools — no data is transmitted to Metavert LLC or any third party.

For the full privacy policy, see: https://www.metavert.io/lastsaas-privacy-policy

---

## License

[MIT](LICENSE) - Copyright 2026 Metavert LLC
