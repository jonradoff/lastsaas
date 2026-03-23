package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"lastsaas/internal/version"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// ---------------------------------------------------------------------------
// HTTP client for proxying read-only requests to the MCPLens API
// ---------------------------------------------------------------------------

type mcpClient struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

func newMCPClient(baseURL, apiKey string) *mcpClient {
	return &mcpClient{
		baseURL: baseURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *mcpClient) get(ctx context.Context, path string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("API error (HTTP %d): %s", resp.StatusCode, string(body))
	}
	return body, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func prettyJSON(data []byte) string {
	var buf bytes.Buffer
	if err := json.Indent(&buf, data, "", "  "); err != nil {
		return string(data)
	}
	return buf.String()
}

func buildQuery(params map[string]string) string {
	v := url.Values{}
	for key, val := range params {
		if val != "" {
			v.Set(key, val)
		}
	}
	if len(v) == 0 {
		return ""
	}
	return "?" + v.Encode()
}

// ---------------------------------------------------------------------------
// Entry point
// ---------------------------------------------------------------------------

func cmdMCP() {
	baseURL := os.Getenv("MCPLENS_URL")
	apiKey := os.Getenv("MCPLENS_API_KEY")

	if baseURL == "" {
		fmt.Fprintln(os.Stderr, "MCPLENS_URL environment variable is required (e.g. http://localhost:3000)")
		os.Exit(1)
	}
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "MCPLENS_API_KEY environment variable is required (e.g. lsk_xxxxx)")
		os.Exit(1)
	}

	client := newMCPClient(baseURL, apiKey)

	s := server.NewMCPServer(
		"mcplens-admin",
		version.Current,
		server.WithToolCapabilities(false),
		server.WithResourceCapabilities(false, false),
		server.WithRecovery(),
	)

	registerAboutTools(s, client)
	registerDashboardTools(s, client)
	registerTenantTools(s, client)
	registerUserTools(s, client)
	registerFinancialTools(s, client)
	registerLogTools(s, client)
	registerHealthTools(s, client)
	registerConfigTools(s, client)
	registerPlanTools(s, client)
	registerAnnouncementTools(s, client)
	registerPromotionTools(s, client)
	registerSecurityTools(s, client)
	registerWebhookTools(s, client)
	registerPMTools(s, client)
	registerResources(s, client)

	if err := server.ServeStdio(s); err != nil {
		fmt.Fprintf(os.Stderr, "MCP server error: %v\n", err)
		os.Exit(1)
	}
}

// ---------------------------------------------------------------------------
// About tools (1)
// ---------------------------------------------------------------------------

func registerAboutTools(s *server.MCPServer, client *mcpClient) {
	s.AddTool(
		mcp.NewTool("get_about",
			mcp.WithDescription("Get system info: software version and environment details"),
			mcp.WithReadOnlyHintAnnotation(true),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			data, err := client.get(ctx, "/api/admin/about")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(prettyJSON(data)), nil
		},
	)
}

// ---------------------------------------------------------------------------
// Dashboard tools (1)
// ---------------------------------------------------------------------------

func registerDashboardTools(s *server.MCPServer, client *mcpClient) {
	s.AddTool(
		mcp.NewTool("dashboard_stats",
			mcp.WithDescription("Get admin dashboard summary: total user count, tenant count, and system health status with any active issues"),
			mcp.WithReadOnlyHintAnnotation(true),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			data, err := client.get(ctx, "/api/admin/dashboard")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(prettyJSON(data)), nil
		},
	)
}

// ---------------------------------------------------------------------------
// Tenant tools (2)
// ---------------------------------------------------------------------------

func registerTenantTools(s *server.MCPServer, client *mcpClient) {
	// list_tenants
	s.AddTool(
		mcp.NewTool("list_tenants",
			mcp.WithDescription("List tenants with pagination, search, and filtering. Returns tenant name, slug, plan, credits, member count, billing status, and creation date."),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithNumber("page", mcp.Description("Page number (default 1)")),
			mcp.WithNumber("limit", mcp.Description("Items per page, 1-100 (default 25)")),
			mcp.WithString("search", mcp.Description("Search by tenant name or slug")),
			mcp.WithString("sort", mcp.Description("Sort: name, -name, createdAt, -createdAt (default -createdAt)")),
			mcp.WithString("status", mcp.Description("Filter: active or disabled")),
			mcp.WithString("billingStatus", mcp.Description("Filter: none, active, past_due, or canceled")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			q := map[string]string{
				"search":        req.GetString("search", ""),
				"sort":          req.GetString("sort", ""),
				"status":        req.GetString("status", ""),
				"billingStatus": req.GetString("billingStatus", ""),
			}
			if v := req.GetInt("page", 0); v > 0 {
				q["page"] = fmt.Sprintf("%d", v)
			}
			if v := req.GetInt("limit", 0); v > 0 {
				q["limit"] = fmt.Sprintf("%d", v)
			}
			data, err := client.get(ctx, "/api/admin/tenants"+buildQuery(q))
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(prettyJSON(data)), nil
		},
	)

	// get_tenant
	s.AddTool(
		mcp.NewTool("get_tenant",
			mcp.WithDescription("Get detailed tenant information including plan, billing, credits, and member list with roles"),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithString("id", mcp.Required(), mcp.Description("Tenant ID (MongoDB ObjectID hex)")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			id, err := req.RequireString("id")
			if err != nil {
				return mcp.NewToolResultError("id is required"), nil
			}
			data, err := client.get(ctx, "/api/admin/tenants/"+id)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(prettyJSON(data)), nil
		},
	)
}

// ---------------------------------------------------------------------------
// User tools (2)
// ---------------------------------------------------------------------------

func registerUserTools(s *server.MCPServer, client *mcpClient) {
	// list_users
	s.AddTool(
		mcp.NewTool("list_users",
			mcp.WithDescription("List users with pagination, search, and status filtering. Returns email, display name, verification status, tenant count, and last login."),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithNumber("page", mcp.Description("Page number (default 1)")),
			mcp.WithNumber("limit", mcp.Description("Items per page, 1-100 (default 25)")),
			mcp.WithString("search", mcp.Description("Search by email or display name")),
			mcp.WithString("sort", mcp.Description("Sort: email, -email, displayName, -displayName, createdAt, -createdAt")),
			mcp.WithString("status", mcp.Description("Filter: active or disabled")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			q := map[string]string{
				"search": req.GetString("search", ""),
				"sort":   req.GetString("sort", ""),
				"status": req.GetString("status", ""),
			}
			if v := req.GetInt("page", 0); v > 0 {
				q["page"] = fmt.Sprintf("%d", v)
			}
			if v := req.GetInt("limit", 0); v > 0 {
				q["limit"] = fmt.Sprintf("%d", v)
			}
			data, err := client.get(ctx, "/api/admin/users"+buildQuery(q))
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(prettyJSON(data)), nil
		},
	)

	// get_user
	s.AddTool(
		mcp.NewTool("get_user",
			mcp.WithDescription("Get detailed user info including auth methods, MFA status, and all tenant memberships with roles"),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithString("id", mcp.Required(), mcp.Description("User ID (MongoDB ObjectID hex)")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			id, err := req.RequireString("id")
			if err != nil {
				return mcp.NewToolResultError("id is required"), nil
			}
			data, err := client.get(ctx, "/api/admin/users/"+id)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(prettyJSON(data)), nil
		},
	)
}

// ---------------------------------------------------------------------------
// Financial tools (2)
// ---------------------------------------------------------------------------

func registerFinancialTools(s *server.MCPServer, client *mcpClient) {
	// list_transactions
	s.AddTool(
		mcp.NewTool("list_transactions",
			mcp.WithDescription("List financial transactions (subscriptions, credit purchases, refunds) with pagination and filtering"),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithNumber("page", mcp.Description("Page number (default 1)")),
			mcp.WithNumber("perPage", mcp.Description("Items per page, 1-100 (default 50)")),
			mcp.WithString("tenantId", mcp.Description("Filter by tenant ID")),
			mcp.WithString("search", mcp.Description("Search by description, invoice number, plan name, or bundle name")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			q := map[string]string{
				"tenantId": req.GetString("tenantId", ""),
				"search":   req.GetString("search", ""),
			}
			if v := req.GetInt("page", 0); v > 0 {
				q["page"] = fmt.Sprintf("%d", v)
			}
			if v := req.GetInt("perPage", 0); v > 0 {
				q["perPage"] = fmt.Sprintf("%d", v)
			}
			data, err := client.get(ctx, "/api/admin/financial/transactions"+buildQuery(q))
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(prettyJSON(data)), nil
		},
	)

	// get_financial_metrics
	s.AddTool(
		mcp.NewTool("get_financial_metrics",
			mcp.WithDescription("Get time-series financial/business metrics. Returns date/value data points for charts. Revenue and ARR are in cents."),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithString("range", mcp.Description("Time range: 7d, 30d, or 1y (default 30d)")),
			mcp.WithString("metric", mcp.Description("Metric: revenue, arr, dau, or mau (default revenue)")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			q := map[string]string{
				"range":  req.GetString("range", ""),
				"metric": req.GetString("metric", ""),
			}
			data, err := client.get(ctx, "/api/admin/financial/metrics"+buildQuery(q))
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(prettyJSON(data)), nil
		},
	)
}

// ---------------------------------------------------------------------------
// Log tools (2)
// ---------------------------------------------------------------------------

func registerLogTools(s *server.MCPServer, client *mcpClient) {
	// search_logs
	s.AddTool(
		mcp.NewTool("search_logs",
			mcp.WithDescription("Search and filter system logs. Returns log entries with severity, category, message, and metadata."),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithNumber("page", mcp.Description("Page number (default 1)")),
			mcp.WithNumber("perPage", mcp.Description("Items per page, 1-100 (default 50)")),
			mcp.WithString("severity", mcp.Description("Filter by severity: critical, high, medium, low, debug (comma-separated for multiple)")),
			mcp.WithString("category", mcp.Description("Filter by category: auth, billing, admin, system, security, tenant")),
			mcp.WithString("search", mcp.Description("Full-text search across log messages")),
			mcp.WithString("fromDate", mcp.Description("Start date filter (RFC3339 format)")),
			mcp.WithString("toDate", mcp.Description("End date filter (RFC3339 format)")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			q := map[string]string{
				"severity": req.GetString("severity", ""),
				"category": req.GetString("category", ""),
				"search":   req.GetString("search", ""),
				"fromDate": req.GetString("fromDate", ""),
				"toDate":   req.GetString("toDate", ""),
			}
			if v := req.GetInt("page", 0); v > 0 {
				q["page"] = fmt.Sprintf("%d", v)
			}
			if v := req.GetInt("perPage", 0); v > 0 {
				q["perPage"] = fmt.Sprintf("%d", v)
			}
			data, err := client.get(ctx, "/api/admin/logs"+buildQuery(q))
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(prettyJSON(data)), nil
		},
	)

	// get_log_severity_counts
	s.AddTool(
		mcp.NewTool("get_log_severity_counts",
			mcp.WithDescription("Get counts of logs grouped by severity level (critical, high, medium, low, debug)"),
			mcp.WithReadOnlyHintAnnotation(true),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			data, err := client.get(ctx, "/api/admin/logs/severity-counts")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(prettyJSON(data)), nil
		},
	)
}

// ---------------------------------------------------------------------------
// Health tools (4)
// ---------------------------------------------------------------------------

func registerHealthTools(s *server.MCPServer, client *mcpClient) {
	// get_system_health
	s.AddTool(
		mcp.NewTool("get_system_health",
			mcp.WithDescription("Get current system health: CPU, memory, disk usage, HTTP request stats, MongoDB connections, and Go runtime info"),
			mcp.WithReadOnlyHintAnnotation(true),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			data, err := client.get(ctx, "/api/admin/health/current")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(prettyJSON(data)), nil
		},
	)

	// get_health_metrics
	s.AddTool(
		mcp.NewTool("get_health_metrics",
			mcp.WithDescription("Get time-series health metrics (CPU, memory, disk) for a node or aggregated across all nodes. Returns data points for charting."),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithString("node", mcp.Description("Node ID to filter by (omit for aggregate across all nodes)")),
			mcp.WithString("range", mcp.Description("Time range: 1h, 6h, 24h, 7d, or 30d (default 24h)")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			q := map[string]string{
				"node":  req.GetString("node", ""),
				"range": req.GetString("range", ""),
			}
			data, err := client.get(ctx, "/api/admin/health/metrics"+buildQuery(q))
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(prettyJSON(data)), nil
		},
	)

	// list_nodes
	s.AddTool(
		mcp.NewTool("list_nodes",
			mcp.WithDescription("List all registered server nodes with hostname, status (healthy/stale), version, and last seen time"),
			mcp.WithReadOnlyHintAnnotation(true),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			data, err := client.get(ctx, "/api/admin/health/nodes")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(prettyJSON(data)), nil
		},
	)

	// get_integrations
	s.AddTool(
		mcp.NewTool("get_integrations",
			mcp.WithDescription("Get health status of third-party integrations (MongoDB, Stripe, Resend, OAuth providers) with response times and 24h call counts"),
			mcp.WithReadOnlyHintAnnotation(true),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			data, err := client.get(ctx, "/api/admin/health/integrations")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(prettyJSON(data)), nil
		},
	)
}

// ---------------------------------------------------------------------------
// Config tools (2)
// ---------------------------------------------------------------------------

func registerConfigTools(s *server.MCPServer, client *mcpClient) {
	// list_config
	s.AddTool(
		mcp.NewTool("list_config",
			mcp.WithDescription("List all configuration variables with names, types (string/numeric/enum/template), current values, and descriptions"),
			mcp.WithReadOnlyHintAnnotation(true),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			data, err := client.get(ctx, "/api/admin/config")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(prettyJSON(data)), nil
		},
	)

	// get_config
	s.AddTool(
		mcp.NewTool("get_config",
			mcp.WithDescription("Get a single configuration variable by name with its type, value, description, and available options"),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithString("name", mcp.Required(), mcp.Description("Config variable name (e.g. log.min_level, app.name)")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			name, err := req.RequireString("name")
			if err != nil {
				return mcp.NewToolResultError("name is required"), nil
			}
			data, err := client.get(ctx, "/api/admin/config/"+url.PathEscape(name))
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(prettyJSON(data)), nil
		},
	)
}

// ---------------------------------------------------------------------------
// Plan tools (4)
// ---------------------------------------------------------------------------

func registerPlanTools(s *server.MCPServer, client *mcpClient) {
	// list_plans
	s.AddTool(
		mcp.NewTool("list_plans",
			mcp.WithDescription("List all subscription plans with pricing, entitlements, credit allocations, and active subscriber counts"),
			mcp.WithReadOnlyHintAnnotation(true),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			data, err := client.get(ctx, "/api/admin/plans")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(prettyJSON(data)), nil
		},
	)

	// get_plan
	s.AddTool(
		mcp.NewTool("get_plan",
			mcp.WithDescription("Get detailed plan info: pricing model, monthly/annual prices, seat limits, credits, trial days, and entitlements"),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithString("id", mcp.Required(), mcp.Description("Plan ID (MongoDB ObjectID hex)")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			id, err := req.RequireString("id")
			if err != nil {
				return mcp.NewToolResultError("id is required"), nil
			}
			data, err := client.get(ctx, "/api/admin/plans/"+id)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(prettyJSON(data)), nil
		},
	)

	// list_entitlement_keys
	s.AddTool(
		mcp.NewTool("list_entitlement_keys",
			mcp.WithDescription("List all unique entitlement keys defined across plans, with their types (bool/numeric) and descriptions"),
			mcp.WithReadOnlyHintAnnotation(true),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			data, err := client.get(ctx, "/api/admin/entitlement-keys")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(prettyJSON(data)), nil
		},
	)

	// list_credit_bundles
	s.AddTool(
		mcp.NewTool("list_credit_bundles",
			mcp.WithDescription("List all credit bundles with name, credit amount, price in cents, active status, and sort order"),
			mcp.WithReadOnlyHintAnnotation(true),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			data, err := client.get(ctx, "/api/admin/credit-bundles")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(prettyJSON(data)), nil
		},
	)
}

// ---------------------------------------------------------------------------
// Announcement tools (1)
// ---------------------------------------------------------------------------

func registerAnnouncementTools(s *server.MCPServer, client *mcpClient) {
	// list_announcements
	s.AddTool(
		mcp.NewTool("list_announcements",
			mcp.WithDescription("List all announcements including drafts and published items with their titles, bodies, and publish dates"),
			mcp.WithReadOnlyHintAnnotation(true),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			data, err := client.get(ctx, "/api/admin/announcements")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(prettyJSON(data)), nil
		},
	)
}

// ---------------------------------------------------------------------------
// Promotion tools (1)
// ---------------------------------------------------------------------------

func registerPromotionTools(s *server.MCPServer, client *mcpClient) {
	// list_promotions
	s.AddTool(
		mcp.NewTool("list_promotions",
			mcp.WithDescription("List all Stripe promotion codes with coupon details, discount amounts, redemption counts, and expiration dates"),
			mcp.WithReadOnlyHintAnnotation(true),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			data, err := client.get(ctx, "/api/admin/promotions")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(prettyJSON(data)), nil
		},
	)
}

// ---------------------------------------------------------------------------
// Security tools (2)
// ---------------------------------------------------------------------------

func registerSecurityTools(s *server.MCPServer, client *mcpClient) {
	// list_api_keys
	s.AddTool(
		mcp.NewTool("list_api_keys",
			mcp.WithDescription("List all API keys with name, authority level (admin/user), key preview, creation date, and last used time. Full keys are never exposed."),
			mcp.WithReadOnlyHintAnnotation(true),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			data, err := client.get(ctx, "/api/admin/api-keys")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(prettyJSON(data)), nil
		},
	)

	// list_root_members
	s.AddTool(
		mcp.NewTool("list_root_members",
			mcp.WithDescription("List root tenant members (admin team) with roles and join dates, plus pending invitations"),
			mcp.WithReadOnlyHintAnnotation(true),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			data, err := client.get(ctx, "/api/admin/members")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(prettyJSON(data)), nil
		},
	)
}

// ---------------------------------------------------------------------------
// Webhook tools (3)
// ---------------------------------------------------------------------------

func registerWebhookTools(s *server.MCPServer, client *mcpClient) {
	// list_webhooks
	s.AddTool(
		mcp.NewTool("list_webhooks",
			mcp.WithDescription("List all outbound webhooks with URL, subscribed events, active status, and creation date"),
			mcp.WithReadOnlyHintAnnotation(true),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			data, err := client.get(ctx, "/api/admin/webhooks")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(prettyJSON(data)), nil
		},
	)

	// list_webhook_event_types
	s.AddTool(
		mcp.NewTool("list_webhook_event_types",
			mcp.WithDescription("List all available webhook event types organized by category (Billing, Team, User, Credits, Audit) with descriptions"),
			mcp.WithReadOnlyHintAnnotation(true),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			data, err := client.get(ctx, "/api/admin/webhooks/event-types")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(prettyJSON(data)), nil
		},
	)

	// get_webhook
	s.AddTool(
		mcp.NewTool("get_webhook",
			mcp.WithDescription("Get webhook details including URL, subscribed events, and recent delivery attempts with status codes"),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithString("id", mcp.Required(), mcp.Description("Webhook ID (MongoDB ObjectID hex)")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			id, err := req.RequireString("id")
			if err != nil {
				return mcp.NewToolResultError("id is required"), nil
			}
			data, err := client.get(ctx, "/api/admin/webhooks/"+id)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(prettyJSON(data)), nil
		},
	)
}

// ---------------------------------------------------------------------------
// PM / Telemetry tools (6)
// ---------------------------------------------------------------------------

func registerPMTools(s *server.MCPServer, client *mcpClient) {
	// get_funnel
	s.AddTool(
		mcp.NewTool("get_funnel",
			mcp.WithDescription("Get conversion funnel metrics: unique visitors, registrations, plan page views, checkouts started, paid conversions, and upgrades with step-by-step conversion rates"),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithString("range", mcp.Description("Time range: 7d, 30d, 90d, or 1y (default 30d)")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			q := map[string]string{
				"range": req.GetString("range", ""),
			}
			data, err := client.get(ctx, "/api/admin/pm/funnel"+buildQuery(q))
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(prettyJSON(data)), nil
		},
	)

	// get_kpis
	s.AddTool(
		mcp.NewTool("get_kpis",
			mcp.WithDescription("Get SaaS KPI snapshot: MRR, ARR, ARPU, LTV (all in cents), churn rate, trial conversion rate, time to first purchase, active subscribers, plan distribution, and 30-day MRR/subscriber trends"),
			mcp.WithReadOnlyHintAnnotation(true),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			data, err := client.get(ctx, "/api/admin/pm/kpis")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(prettyJSON(data)), nil
		},
	)

	// get_retention
	s.AddTool(
		mcp.NewTool("get_retention",
			mcp.WithDescription("Get cohort retention analysis showing what percentage of users return in subsequent periods after signup"),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithString("granularity", mcp.Description("Cohort granularity: weekly or monthly (default weekly)")),
			mcp.WithNumber("periods", mcp.Description("Number of retention periods to compute, 1-52 (default 12)")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			q := map[string]string{
				"granularity": req.GetString("granularity", ""),
			}
			if v := req.GetInt("periods", 0); v > 0 {
				q["periods"] = fmt.Sprintf("%d", v)
			}
			data, err := client.get(ctx, "/api/admin/pm/retention"+buildQuery(q))
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(prettyJSON(data)), nil
		},
	)

	// get_engagement
	s.AddTool(
		mcp.NewTool("get_engagement",
			mcp.WithDescription("Get engagement metrics for paying subscribers: DAU/WAU/MAU time series, average sessions per user per week, top features by usage, and credit consumption trend"),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithString("range", mcp.Description("Time range: 7d, 30d, 90d, or 1y (default 30d)")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			q := map[string]string{
				"range": req.GetString("range", ""),
			}
			data, err := client.get(ctx, "/api/admin/pm/engagement"+buildQuery(q))
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(prettyJSON(data)), nil
		},
	)

	// get_custom_events
	s.AddTool(
		mcp.NewTool("get_custom_events",
			mcp.WithDescription("Get trend data and total count for a specific telemetry event type over a time range. Use list_event_types to discover available event names."),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithString("name", mcp.Required(), mcp.Description("Event type name (e.g. page.view, user.login, checkout.started)")),
			mcp.WithString("range", mcp.Description("Time range: 7d, 30d, 90d, or 1y (default 30d)")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			name, err := req.RequireString("name")
			if err != nil {
				return mcp.NewToolResultError("name is required"), nil
			}
			q := map[string]string{
				"name":  name,
				"range": req.GetString("range", ""),
			}
			data, err := client.get(ctx, "/api/admin/pm/events"+buildQuery(q))
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(prettyJSON(data)), nil
		},
	)

	// list_event_types
	s.AddTool(
		mcp.NewTool("list_event_types",
			mcp.WithDescription("List all distinct telemetry event types with category, total count, and last seen timestamp"),
			mcp.WithReadOnlyHintAnnotation(true),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			data, err := client.get(ctx, "/api/admin/pm/events/types")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(prettyJSON(data)), nil
		},
	)
}

// ---------------------------------------------------------------------------
// Resources (2)
// ---------------------------------------------------------------------------

func registerResources(s *server.MCPServer, client *mcpClient) {
	// mcplens://dashboard
	s.AddResource(
		mcp.NewResource(
			"mcplens://dashboard",
			"Dashboard Summary",
			mcp.WithResourceDescription("MCPLens admin dashboard: user count, tenant count, and system health status with issues"),
			mcp.WithMIMEType("application/json"),
		),
		func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			data, err := client.get(ctx, "/api/admin/dashboard")
			if err != nil {
				return nil, fmt.Errorf("failed to fetch dashboard: %w", err)
			}
			return []mcp.ResourceContents{
				mcp.TextResourceContents{
					URI:      "lastsaas://dashboard",
					MIMEType: "application/json",
					Text:     prettyJSON(data),
				},
			}, nil
		},
	)

	// mcplens://health
	s.AddResource(
		mcp.NewResource(
			"mcplens://health",
			"System Health",
			mcp.WithResourceDescription("Current system health: CPU, memory, disk, HTTP stats, MongoDB connections, and node status"),
			mcp.WithMIMEType("application/json"),
		),
		func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			data, err := client.get(ctx, "/api/admin/health/current")
			if err != nil {
				return nil, fmt.Errorf("failed to fetch health: %w", err)
			}
			return []mcp.ResourceContents{
				mcp.TextResourceContents{
					URI:      "lastsaas://health",
					MIMEType: "application/json",
					Text:     prettyJSON(data),
				},
			}, nil
		},
	)
}
