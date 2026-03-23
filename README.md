# MCPLens

**Scan any Shopify store's AI agent readiness. Lighthouse for agent commerce.**

MCPLens connects to any Shopify store's public MCP endpoint, runs agent commerce scenarios, and generates a scored report showing exactly how well AI buyer agents can discover, evaluate, and purchase products.

## Quick Start

```bash
# Scan any Shopify store (no setup required)
npx mcplens scan allbirds.com

# Compare stores
npx mcplens scan allbirds.com rothy.com --compare

# Use in CI/CD
npx mcplens scan mystore.com --non-interactive --fail-under 70
```

## How It Works

MCPLens connects to `https://{domain}/api/mcp` (Shopify's public MCP endpoint, available on all 5.5M+ Shopify stores) and simulates what an AI buyer agent would do:

1. **Discovers available tools** — what can agents do on this store?
2. **Runs 10 test scenarios** across 4 categories
3. **Scores each category** and generates a composite 0-100 score
4. **Generates a report** with specific issues and fix instructions

## Scoring

| Category | Weight | What It Tests |
|---|---|---|
| Data Quality | 35% | Price data, descriptions, images, structured attributes |
| Product Discovery | 30% | Search functionality, filtering, result completeness |
| Checkout Flow | 25% | Cart operations, checkout initiation |
| Protocol Compliance | 10% | MCP spec adherence, error handling |

Scores are color-coded: 🟢 80-100 (Good), 🟡 50-79 (Needs Work), 🔴 0-49 (Critical)

## Web Scanner

Try it without installing anything: [mcplens.dev/scan](https://mcplens.dev/scan)

## CLI Reference

```bash
# Basic scan
mcplens scan <domain>

# Multiple flags
mcplens scan <domain> [options]

Options:
  --format <html|json>    Output format (default: html)
  --out <path>            Output file path
  --non-interactive       CI/CD mode (no prompts)
  --fail-under <score>    Exit code 1 if below threshold
  --compare               Side-by-side comparison (multiple domains)
  --verbose               Show detailed scan logs
  --open                  Open report in browser

# Batch scanning
mcplens batch --domains list.txt --output results/ --delay 1000
```

## CI/CD Integration

Add to your GitHub Actions:

```yaml
name: Agent Readiness Check
on: [push]
jobs:
  scan:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
      - run: npx mcplens scan mystore.com --non-interactive --fail-under 70
```

## Pricing

MCPLens is **open source and free** for scanning. Paid tiers add fix instructions, tracking, buyer agent simulation, and agency features:

| | Free | Pro ($50/mo) | Max ($200/mo) | Agency ($600/mo) |
|---|---|---|---|---|
| Layer 1 scan + score | ✅ | ✅ | ✅ | ✅ |
| Shareable report URL | ✅ | ✅ | ✅ | ✅ |
| Fix instructions | — | ✅ | ✅ | ✅ |
| Store tracking | — | 3 stores | 15 stores | 50 stores |
| Scheduled scans | — | Weekly | Daily | Daily |
| CI/CD integration | — | ✅ | ✅ | ✅ |
| Email alerts | — | ✅ | ✅ | ✅ |
| Simulated Buyer Agent (Layer 3) | — | — | Early Access | ✅ |
| LLM Quality Assessment (Layer 2) | — | — | Early Access | ✅ |
| Revenue impact calculator | — | — | ✅ | ✅ |
| White-label reports | — | — | — | ✅ |
| API access | — | — | — | ✅ |
| Batch scanning | — | — | — | ✅ |
| Team/multi-user | — | — | — | ✅ |
| Agency bonuses | — | — | — | ✅ |

Annual billing saves 2 months on every plan. Founding user promos available for the first 100 users.

## Why MCPLens?

Every Shopify store now has a public MCP endpoint (`/api/mcp`). AI buyer agents from ChatGPT, Gemini, and others are using these endpoints to shop. But most merchants have no idea whether agents can actually use their store effectively.

MCPLens fills that gap: **scan → score → fix → track**.

- 🔍 **Scan** any store without their participation (public endpoints)
- 📊 **Score** across 4 categories with a 0-100 composite
- 🔧 **Fix** with specific, actionable recommendations
- 📈 **Track** improvements over time (paid feature)

## Tech Stack

- **Scanner:** TypeScript (Node.js)
- **Backend:** Go (LastSaaS framework)
- **Frontend:** React + TypeScript
- **Database:** MongoDB
- **Billing:** Stripe

## Contributing

Contributions welcome! See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## License

MIT — see [LICENSE](LICENSE) for details.

---

Built with [Claude Code](https://claude.ai/claude-code) | [Web Scanner](https://mcplens.dev/scan) | [GitHub](https://github.com/reesthomas212/mcplens)
