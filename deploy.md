# MCPLens Deployment Guide

## Why `Dockerfile.saas` — Never Bare `fly deploy`

MCPLens is deployed using `Dockerfile.saas` and `fly.saas.toml`. The SaaS Dockerfile
builds three components and bundles them into a single container:

1. **Go binary** — serves all HTTP routes: auth (`/api/auth/*`), billing, admin, scanner API, bootstrap, OAuth
2. **React frontend** — compiled to `/app/static`, served as an SPA by the Go binary
3. **Node.js scanner** — built TypeScript CLI at `/app/scanner/dist/cli.js`, invoked by the Go binary at scan time

Without the scanner dist present, scan requests will fail with a "scanner CLI error" at runtime.
Without the correct Dockerfile, the `SCANNER_PATH` env var won't point to the compiled CLI.

**Correct deploy command — always:**
```bash
fly deploy -c fly.saas.toml
```

---

## Prerequisites

- Fly.io account (recommended) OR Railway account
- MongoDB Atlas cluster
- Stripe account with products/prices configured
- Resend account (transactional email)
- Cloudflare DNS access for transparentfunnel.com
- Docker installed locally (for local build verification)

---

## Option A: Fly.io (Recommended)

### 1. Install Fly CLI

```bash
# Windows
winget install flyctl

# macOS
brew install flyctl

# Linux
curl -L https://fly.io/install.sh | sh
```

### 2. Login

```bash
fly auth login
```

### 3. Create the app (first time only)

```bash
cd mcplens
fly apps create mcplens
```

### 4. Set Environment Variables

```bash
fly secrets set \
  MONGODB_URI="mongodb+srv://<user>:<password>@<cluster>.mongodb.net/" \
  DATABASE_NAME="mcplens" \
  JWT_ACCESS_SECRET="<generate: openssl rand -hex 32>" \
  JWT_REFRESH_SECRET="<generate: openssl rand -hex 32>" \
  STRIPE_SECRET_KEY="sk_live_..." \
  STRIPE_PUBLISHABLE_KEY="pk_live_..." \
  STRIPE_WEBHOOK_SECRET="whsec_..." \
  WEBHOOK_ENCRYPTION_KEY="<generate: openssl rand -hex 32>" \
  RESEND_API_KEY="re_..." \
  APP_NAME="MCPLens" \
  FROM_EMAIL="hello@transparentfunnel.com" \
  FROM_NAME="MCPLens" \
  FRONTEND_URL="https://transparentfunnel.com" \
  GOOGLE_CLIENT_ID="<optional>" \
  GOOGLE_CLIENT_SECRET="<optional>" \
  GOOGLE_REDIRECT_URL="https://transparentfunnel.com/api/auth/google/callback" \
  DATADOG_API_KEY="<optional — omit if not using Datadog>"
```

> `SCANNER_PATH` is baked into `Dockerfile.saas` as `/app/scanner/dist` — do not set it as a secret.

### 5. Deploy

```bash
fly deploy -c fly.saas.toml
```

This builds the image remotely on Fly's builders (no local Docker required).

### 6. Configure DNS in Cloudflare

After first deploy, get the Fly app hostname:

```bash
fly status -c fly.saas.toml
```

In Cloudflare DNS for `transparentfunnel.com`:

| Type  | Name | Target              | Proxy  |
|-------|------|---------------------|--------|
| CNAME | @    | mcplens.fly.dev     | ON (orange cloud) |

Or use Fly's custom domain feature to get a TLS cert provisioned automatically:

```bash
fly certs add transparentfunnel.com -c fly.saas.toml
fly certs show transparentfunnel.com -c fly.saas.toml
```

### 7. Update Stripe Webhook

In the Stripe dashboard, update (or create) the webhook endpoint:

- **URL:** `https://transparentfunnel.com/api/billing/webhook`
- **Events:** `checkout.session.completed`, `customer.subscription.updated`, `customer.subscription.deleted`, `invoice.payment_failed`

---

## Option B: Railway

### 1. Install Railway CLI

```bash
npm install -g @railway/cli
railway login
```

### 2. Create project

```bash
cd mcplens
railway init
```

### 3. Configure Railway to use Dockerfile.saas

In the Railway dashboard → your service → Settings → Build:
- Set **Dockerfile Path** to `Dockerfile.saas`

### 4. Set environment variables

Add the same variables listed in Option A via the Railway dashboard (Variables tab).

### 5. Deploy

```bash
railway up
```

### 6. Configure custom domain

In Railway dashboard → your service → Settings → Domains:
- Add `transparentfunnel.com`
- Copy the CNAME target and add it in Cloudflare

---

## Local Build Verification

To verify the Dockerfile builds correctly before deploying:

```bash
cd mcplens
docker build -f Dockerfile.saas -t mcplens:local .
```

Successful build confirms:
- Go binary compiles without errors
- React frontend builds without errors
- Node.js scanner TypeScript compiles without errors
- All three artifacts are present in the final image

---

## Post-Deploy Verification Checklist

1. `https://transparentfunnel.com` — landing page loads
2. `https://transparentfunnel.com/scan` — public scanner page loads
3. Try scanning a store (e.g. `allbirds.com`) — results appear
4. `https://transparentfunnel.com/login` — login form loads with correct branding
5. Test login with admin account
6. Test Stripe checkout flow
7. `https://transparentfunnel.com/health` — returns `{"status":"ok"}`

---

## Secrets Generation Reference

```bash
# JWT secrets
openssl rand -hex 32   # run twice: once for ACCESS, once for REFRESH

# Webhook encryption key
openssl rand -hex 32
```

---

## Ongoing Deploys

Every subsequent deploy is a single command:

```bash
fly deploy -c fly.saas.toml
```
