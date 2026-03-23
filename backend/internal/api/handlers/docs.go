package handlers

import (
	"fmt"
	"html"
	"net/http"
	"strings"

	"lastsaas/internal/models"
	"lastsaas/internal/version"
)

// --- Documentation data model ---

type apiParam struct {
	Name     string
	Type     string
	Required bool
	Desc     string
}

type apiEndpoint struct {
	Method   string
	Path     string
	Summary  string
	Detail   string // extended description (HTML-safe)
	Auth     string // "none", "jwt", "jwt+tenant", "admin", "owner", "stripe"
	Params   []apiParam
	Body     string // JSON request body example
	Response string // JSON response example
}

type apiSection struct {
	Title     string
	Endpoints []apiEndpoint
}

type webhookEventDoc struct {
	Type string
	Desc string
}

func webhookEventsDoc() []webhookEventDoc {
	return []webhookEventDoc{
		// Tier 1: Billing
		{string(models.WebhookEventSubscriptionActivated), "Fired when a subscription is activated after a successful checkout. Payload includes tenantId, planId, planName, billingInterval, and amountCents."},
		{string(models.WebhookEventSubscriptionCanceled), "Fired when a subscription is canceled (by user, admin, or Stripe). Payload includes tenantId, tenantName, and reason (user_initiated, cancel_at_period_end, or subscription_ended)."},
		{string(models.WebhookEventPaymentReceived), "Fired when a recurring subscription payment succeeds (excludes the first payment which triggers subscription.activated). Payload includes tenantId, amountCents, currency, and planName."},
		{string(models.WebhookEventPaymentFailed), "Fired when a subscription payment fails. The tenant is moved to past_due status. Payload includes tenantId and tenantName."},
		// Tier 2: Team lifecycle
		{string(models.WebhookEventMemberInvited), "Fired when a team member is invited. Payload includes tenantId, tenantName, email, role, and invitedBy."},
		{string(models.WebhookEventMemberJoined), "Fired when a user joins a tenant by accepting an invitation. Payload includes tenantId, tenantName, userId, and role."},
		{string(models.WebhookEventMemberRemoved), "Fired when a member is removed from a tenant by an admin. Payload includes tenantId, tenantName, userId, and removedBy."},
		{string(models.WebhookEventMemberRoleChanged), "Fired when a member's role is changed within a tenant. Payload includes tenantId, tenantName, userId, oldRole, and newRole."},
		{string(models.WebhookEventOwnershipTransferred), "Fired when tenant ownership is transferred to another member. Payload includes tenantId, tenantName, fromUserId, and toUserId."},
		// Tier 3: User lifecycle
		{string(models.WebhookEventUserRegistered), "Fired when a new user registers. Payload includes userId, email, and displayName."},
		{string(models.WebhookEventUserVerified), "Fired when a user verifies their email address. Payload includes userId and email."},
		{string(models.WebhookEventUserDeactivated), "Fired when an admin deactivates a user account. Payload includes userId."},
		// Tier 4: Credits & billing details
		{string(models.WebhookEventCreditsPurchased), "Fired when a credit bundle is purchased. Payload includes tenantId, bundleId, bundleName, credits, and amountCents."},
		{string(models.WebhookEventPlanChanged), "Fired when a tenant's plan changes (upgrade, downgrade, or subscription end). Payload includes tenantId, planId, and planName."},
		{string(models.WebhookEventTenantCreated), "Fired when a new tenant is created during registration. Payload includes tenantId, tenantName, tenantSlug, and userId."},
		{string(models.WebhookEventTenantDeactivated), "Fired when an admin deactivates a tenant. Payload includes tenantId and tenantName."},
		// Tier 5: Audit & security
		{string(models.WebhookEventUserDeleted), "Fired when an admin deletes a user account. Payload includes userId and email."},
		{string(models.WebhookEventTenantDeleted), "Fired when a tenant is deleted (e.g. sole owner deleted). Payload includes tenantId, tenantName, and reason."},
		{string(models.WebhookEventAPIKeyCreated), "Fired when a new API key is created. Payload includes keyId, name, authority, and createdBy."},
		{string(models.WebhookEventAPIKeyRevoked), "Fired when an API key is revoked/deleted. Payload includes keyId and revokedBy."},
	}
}

// --- Endpoint reference ---

func apiReference() []apiSection {
	return []apiSection{
		{
			Title: "Authentication",
			Endpoints: []apiEndpoint{
				{
					Method:  "POST",
					Path:    "/api/auth/register",
					Summary: "Register a new user",
					Detail:  "Creates a new user account with email and password. If an invitation token is provided, the user is automatically added to the inviting tenant. A personal tenant is always created for the new user. Returns access and refresh tokens plus the user profile and tenant memberships.",
					Auth:    "none",
					Body:    `{"email":"user@example.com","password":"secureP@ss1","displayName":"Jane Doe","invitationToken":"(optional)"}`,
					Response: `{"accessToken":"eyJ...","refreshToken":"eyJ...",
 "user":{"id":"...","email":"user@example.com","displayName":"Jane Doe","emailVerified":false,"isActive":true,"authMethods":[{"provider":"password"}],"createdAt":"...","updatedAt":"..."},
 "memberships":[{"tenantId":"...","tenantName":"Jane's Team","tenantSlug":"janes-team","role":"owner","isRoot":false}]}`,
				},
				{
					Method:   "POST",
					Path:     "/api/auth/login",
					Summary:  "Authenticate and receive tokens",
					Detail:   "Authenticates a user with email and password. Returns JWT access and refresh tokens. Account is locked for 15 minutes after 5 consecutive failed attempts.",
					Auth:     "none",
					Body:     `{"email":"user@example.com","password":"secureP@ss1"}`,
					Response: `{"accessToken":"eyJ...","refreshToken":"eyJ...","user":{...},"memberships":[...]}`,
				},
				{
					Method:   "POST",
					Path:     "/api/auth/refresh",
					Summary:  "Exchange refresh token for new access token",
					Detail:   "Exchanges a valid refresh token for a new access/refresh token pair. The old refresh token is revoked (rotation). Use this when the access token expires.",
					Auth:     "none",
					Body:     `{"refreshToken":"eyJ..."}`,
					Response: `{"accessToken":"eyJ...","refreshToken":"eyJ...","user":{...},"memberships":[...]}`,
				},
				{
					Method:   "POST",
					Path:     "/api/auth/verify-email",
					Summary:  "Verify email address",
					Detail:   "Confirms the user's email address using a token sent via email. The token is single-use and expires after 24 hours.",
					Auth:     "none",
					Body:     `{"token":"verification-token-from-email"}`,
					Response: `{"message":"Email verified successfully"}`,
				},
				{
					Method:   "POST",
					Path:     "/api/auth/resend-verification",
					Summary:  "Resend verification email",
					Detail:   "Sends a new verification email to the specified address. Rate-limited to one request per 60 seconds per email. Returns a success message regardless of whether the email exists (prevents enumeration).",
					Auth:     "none",
					Body:     `{"email":"user@example.com"}`,
					Response: `{"message":"If the email exists, a verification link has been sent"}`,
				},
				{
					Method:   "POST",
					Path:     "/api/auth/forgot-password",
					Summary:  "Request password reset email",
					Detail:   "Sends a password reset email to the specified address. Returns a success message regardless of whether the email exists (prevents enumeration). Only works for accounts with password authentication enabled.",
					Auth:     "none",
					Body:     `{"email":"user@example.com"}`,
					Response: `{"message":"If the email exists, a password reset link has been sent"}`,
				},
				{
					Method:   "POST",
					Path:     "/api/auth/reset-password",
					Summary:  "Reset password with token",
					Detail:   "Resets the user's password using a token from the reset email. All existing refresh tokens are revoked (logs out all sessions). The token is single-use.",
					Auth:     "none",
					Body:     `{"token":"reset-token-from-email","newPassword":"newSecureP@ss1"}`,
					Response: `{"message":"Password reset successfully"}`,
				},
				{
					Method:  "GET",
					Path:    "/api/auth/google",
					Summary: "Initiate Google OAuth flow",
					Detail:  "Redirects the user to Google's OAuth consent screen. After authorization, Google redirects back to the callback URL. Only available when Google OAuth is configured.",
					Auth:    "none",
				},
				{
					Method:  "GET",
					Path:    "/api/auth/google/callback",
					Summary: "Google OAuth callback",
					Detail:  "Handles the OAuth callback from Google. Links the Google account to an existing user (matched by email) or creates a new account. Redirects to the frontend with tokens in the URL fragment: <code>/auth/callback#access_token=...&amp;refresh_token=...</code>",
					Auth:    "none",
					Params: []apiParam{
						{"state", "string", true, "OAuth state parameter (verified against stored state)"},
						{"code", "string", true, "OAuth authorization code from Google"},
					},
				},
				{
					Method:   "GET",
					Path:     "/api/auth/me",
					Summary:  "Get current user profile",
					Detail:   "Returns the authenticated user's profile and all tenant memberships. Use this to hydrate the session after login or page refresh.",
					Auth:     "jwt",
					Response: `{"user":{"id":"...","email":"...","displayName":"...","emailVerified":true,"isActive":true,"authMethods":[...],"createdAt":"...","updatedAt":"...","lastLoginAt":"..."},"memberships":[{"tenantId":"...","tenantName":"...","tenantSlug":"...","role":"owner","isRoot":false}]}`,
				},
				{
					Method:   "POST",
					Path:     "/api/auth/logout",
					Summary:  "Revoke session tokens",
					Detail:   "Revokes the current access token. If a refresh token is provided in the body, it is also revoked.",
					Auth:     "jwt",
					Body:     `{"refreshToken":"eyJ... (optional)"}`,
					Response: `{"message":"Logged out successfully"}`,
				},
				{
					Method:   "POST",
					Path:     "/api/auth/change-password",
					Summary:  "Change password",
					Detail:   "Changes the authenticated user's password. If the user already has a password, the current password must be provided. For Google-only accounts adding a password for the first time, the current password field can be omitted.",
					Auth:     "jwt",
					Body:     `{"currentPassword":"oldP@ss (required if password exists)","newPassword":"newSecureP@ss1"}`,
					Response: `{"message":"Password changed successfully"}`,
				},
				{
					Method:   "POST",
					Path:     "/api/auth/accept-invitation",
					Summary:  "Accept a team invitation",
					Detail:   "Accepts a pending invitation to join a tenant. The invitation token comes from the invitation email. The user is added to the tenant with the role specified in the invitation. Returns updated memberships.",
					Auth:     "jwt",
					Body:     `{"token":"invitation-token-from-email"}`,
					Response: `{"message":"Invitation accepted","memberships":[{"tenantId":"...","tenantName":"...","tenantSlug":"...","role":"user","isRoot":false}]}`,
				},
			},
		},
		{
			Title: "Tenant Members",
			Endpoints: []apiEndpoint{
				{
					Method:   "GET",
					Path:     "/api/tenant/members",
					Summary:  "List tenant members",
					Detail:   "Returns all members of the current tenant with their roles and join dates. Any member of the tenant can call this endpoint.",
					Auth:     "jwt+tenant",
					Response: `{"members":[{"userId":"...","email":"user@example.com","displayName":"Jane Doe","role":"owner","joinedAt":"2025-01-15T..."}]}`,
				},
				{
					Method:  "POST",
					Path:    "/api/tenant/members/invite",
					Summary: "Invite a user by email",
					Detail:  "Sends an invitation email to join the tenant. If the email belongs to an existing user, they receive a join link. If not, they receive a signup-and-join link. Invitations expire after 7 days. Only owners can invite admins; admins can only invite users. Subject to the plan's user limit.",
					Auth:    "admin",
					Body:    `{"email":"newuser@example.com","role":"user"}`,
					Response: `{"message":"Invitation sent"}`,
				},
				{
					Method:  "DELETE",
					Path:    "/api/tenant/members/{userId}",
					Summary: "Remove a member",
					Detail:  "Removes a member from the tenant. You cannot remove the owner or yourself. Admins can only remove regular users (not other admins).",
					Auth:    "admin",
					Params: []apiParam{
						{"userId", "ObjectID", true, "The user's ID"},
					},
					Response: `{"message":"Member removed"}`,
				},
				{
					Method:  "PATCH",
					Path:    "/api/tenant/members/{userId}/role",
					Summary: "Change a member's role",
					Detail:  "Changes a member's role to admin or user. Only the tenant owner can change roles. To transfer ownership, use the dedicated transfer endpoint instead.",
					Auth:    "owner",
					Params:  []apiParam{{"userId", "ObjectID", true, "The target user's ID"}},
					Body:    `{"role":"admin"}`,
					Response: `{"message":"Role updated"}`,
				},
				{
					Method:  "POST",
					Path:    "/api/tenant/members/{userId}/transfer-ownership",
					Summary: "Transfer tenant ownership",
					Detail:  "Transfers ownership of the tenant to another member. The current owner is demoted to admin. The target user must already be a member of the tenant. This action cannot be undone by the previous owner.",
					Auth:    "owner",
					Params:  []apiParam{{"userId", "ObjectID", true, "The new owner's user ID"}},
					Response: `{"message":"Ownership transferred"}`,
				},
			},
		},
		{
			Title: "Messages",
			Endpoints: []apiEndpoint{
				{
					Method:   "GET",
					Path:     "/api/messages",
					Summary:  "List messages",
					Detail:   "Returns all messages for the authenticated user, sorted by creation date (newest first). Messages include system notifications like invitation alerts.",
					Auth:     "jwt",
					Response: `{"messages":[{"id":"...","userId":"...","type":"invitation","title":"...","body":"...","isRead":false,"createdAt":"..."}]}`,
				},
				{
					Method:   "GET",
					Path:     "/api/messages/unread-count",
					Summary:  "Get unread count",
					Detail:   "Returns the number of unread messages for the authenticated user. Use this for notification badges.",
					Auth:     "jwt",
					Response: `{"count":3}`,
				},
				{
					Method:  "PATCH",
					Path:    "/api/messages/{messageId}/read",
					Summary: "Mark as read",
					Detail:  "Marks a specific message as read. Only the message owner can mark it as read.",
					Auth:    "jwt",
					Params:  []apiParam{{"messageId", "ObjectID", true, "The message ID"}},
					Response: `{"message":"Marked as read"}`,
				},
			},
		},
		{
			Title: "Plans & Billing",
			Endpoints: []apiEndpoint{
				{
					Method:  "GET",
					Path:    "/api/plans",
					Summary: "List available plans",
					Detail:  "Returns all subscription plans visible to the current user, along with the tenant's current plan, billing status, credits, and subscription interval. Requires the <code>X-Tenant-ID</code> header to determine the tenant's current state.",
					Auth:    "jwt",
					Response: `{"plans":[{"id":"...","name":"Pro","description":"...","monthlyPriceCents":2900,"annualDiscountPct":20,"usageCreditsPerMonth":1000,"creditResetPolicy":"reset","bonusCredits":0,"userLimit":10,"entitlements":{...}}],
 "currentPlanId":"...","billingWaived":false,"tenantSubscriptionCredits":500,"tenantPurchasedCredits":0,
 "billingStatus":"active","billingInterval":"year","currentPeriodEnd":"2026-01-15T...","canceledAt":null}`,
				},
				{
					Method:   "GET",
					Path:     "/api/credit-bundles",
					Summary:  "List credit bundles",
					Detail:   "Returns all active credit bundles available for purchase, sorted by sort order.",
					Auth:     "jwt",
					Response: `{"bundles":[{"id":"...","name":"500 Credits","credits":500,"priceCents":4900,"isActive":true,"sortOrder":1}]}`,
				},
				{
					Method:  "POST",
					Path:    "/api/billing/checkout",
					Summary: "Start a checkout session",
					Detail:  "Creates a Stripe Checkout session for a plan subscription or credit bundle purchase. For free plans or billing-waived tenants, the plan is assigned immediately without Stripe. Specify either <code>planId</code> or <code>bundleId</code>, not both.",
					Auth:    "jwt+tenant",
					Body:    `{"planId":"ObjectID (or bundleId)","billingInterval":"year"}`,
					Response: `{"checkoutUrl":"https://checkout.stripe.com/..."}`,
				},
				{
					Method:   "POST",
					Path:     "/api/billing/portal",
					Summary:  "Open billing portal",
					Detail:   "Creates a Stripe Billing Portal session URL where the customer can manage payment methods, view invoices, and update billing details. The tenant must have an existing Stripe customer ID.",
					Auth:     "jwt+tenant",
					Response: `{"portalUrl":"https://billing.stripe.com/..."}`,
				},
				{
					Method:  "GET",
					Path:    "/api/billing/transactions",
					Summary: "List billing transactions",
					Detail:  "Returns paginated billing transactions for the current tenant, sorted by date (newest first).",
					Auth:    "jwt+tenant",
					Params: []apiParam{
						{"page", "int", false, "Page number (default: 1)"},
						{"perPage", "int", false, "Items per page, 1-100 (default: 20)"},
					},
					Response: `{"transactions":[{"id":"...","tenantId":"...","description":"Pro Plan (Annual)","type":"subscription","amountCents":29900,"currency":"usd","invoiceNumber":"INV-0001","createdAt":"..."}],
 "total":15,"page":1,"perPage":20}`,
				},
				{
					Method:  "GET",
					Path:    "/api/billing/transactions/{id}/invoice",
					Summary: "Get invoice details",
					Detail:  "Returns the full transaction record and tenant name for rendering an invoice view.",
					Auth:    "jwt+tenant",
					Params:  []apiParam{{"id", "ObjectID", true, "Transaction ID"}},
					Response: `{"transaction":{...},"tenant":{"name":"Acme Corp"}}`,
				},
				{
					Method:  "GET",
					Path:    "/api/billing/transactions/{id}/invoice/pdf",
					Summary: "Download invoice PDF",
					Detail:  "Generates and returns a PDF invoice for the specified transaction. The response Content-Type is <code>application/pdf</code>.",
					Auth:    "jwt+tenant",
					Params:  []apiParam{{"id", "ObjectID", true, "Transaction ID"}},
				},
				{
					Method:   "POST",
					Path:     "/api/billing/cancel",
					Summary:  "Cancel subscription",
					Detail:   "Cancels the tenant's current subscription at the end of the billing period. The tenant retains access until the period ends. Returns the period end date.",
					Auth:     "jwt+tenant",
					Response: `{"message":"Subscription will cancel at end of billing period","currentPeriodEnd":"2026-02-15T..."}`,
				},
				{
					Method:   "GET",
					Path:     "/api/billing/config",
					Summary:  "Get billing configuration",
					Detail:   "Returns the Stripe publishable key for initializing Stripe.js on the frontend. Returns an empty string if Stripe is not configured.",
					Auth:     "jwt+tenant",
					Response: `{"publishableKey":"pk_live_..."}`,
				},
			},
		},
		{
			Title: "Admin — Dashboard & Monitoring",
			Endpoints: []apiEndpoint{
				{
					Method:   "GET",
					Path:     "/api/admin/about",
					Summary:  "Get system information",
					Detail:   "Returns the current version and copyright information.",
					Auth:     "admin",
					Response: `{"version":"1.00","copyright":"..."}`,
				},
				{
					Method:   "GET",
					Path:     "/api/admin/dashboard",
					Summary:  "Get dashboard metrics",
					Detail:   "Returns high-level system metrics including total user count, tenant count, and overall health status with any active issues.",
					Auth:     "admin",
					Response: `{"users":142,"tenants":38,"health":{"healthy":true,"issues":[]}}`,
				},
				{
					Method:  "GET",
					Path:    "/api/admin/logs",
					Summary: "List system logs",
					Detail:  "Returns paginated system audit logs with optional filtering by severity, user, or text search. Logs record authentication events, configuration changes, billing actions, and other system activity.",
					Auth:    "admin",
					Params: []apiParam{
						{"page", "int", false, "Page number (default: 1)"},
						{"perPage", "int", false, "Items per page, 1-100 (default: 50)"},
						{"severity", "string", false, "Filter by severity: critical, high, medium, low, debug"},
						{"userId", "ObjectID", false, "Filter by user ID"},
						{"search", "string", false, "Full-text search in log messages"},
					},
					Response: `{"logs":[{"id":"...","severity":"high","message":"Webhook created: Test → https://...","userId":"...","createdAt":"..."}],"total":256}`,
				},
				{
					Method:   "GET",
					Path:     "/api/admin/health/nodes",
					Summary:  "List server nodes",
					Detail:   "Returns all known server nodes and their current status. In a multi-machine deployment, each machine registers as a separate node.",
					Auth:     "admin",
					Response: `{"nodes":[{"id":"...","hostname":"d892610f630968","region":"iad","lastSeen":"...","isHealthy":true}]}`,
				},
				{
					Method:  "GET",
					Path:    "/api/admin/health/metrics",
					Summary: "Get performance metrics",
					Detail:  "Returns time-series performance metrics (CPU, memory, request rate, latency) for a specific node or aggregated across all nodes.",
					Auth:    "admin",
					Params: []apiParam{
						{"node", "ObjectID", false, "Node ID (omit for aggregate)"},
						{"range", "string", false, "Time range: 1h, 6h, 24h, 7d, 30d (default: 24h)"},
					},
					Response: `{"metrics":[{"timestamp":"...","cpu":23.5,"memoryMB":128,"requestsPerMin":45,"avgLatencyMs":12}],"from":"...","to":"..."}`,
				},
				{
					Method:   "GET",
					Path:     "/api/admin/health/current",
					Summary:  "Get current node health",
					Detail:   "Returns the latest health snapshot for each active node. Use this for real-time monitoring dashboards.",
					Auth:     "admin",
					Response: `{"metrics":[{"nodeId":"...","cpu":15.2,"memoryMB":96,"requestsPerMin":30,"avgLatencyMs":8}]}`,
				},
				{
					Method:   "GET",
					Path:     "/api/admin/health/integrations",
					Summary:  "Check integration health",
					Detail:   "Checks the connectivity and status of all external integrations: MongoDB, Stripe, Resend (email), and Google OAuth. Returns the check status and last 24h call count for each.",
					Auth:     "admin",
					Response: `{"integrations":[{"name":"mongodb","status":"healthy","lastCheck":"...","calls24h":1520},{"name":"stripe","status":"healthy",...},{"name":"resend","status":"not_configured",...}]}`,
				},
			},
		},
		{
			Title: "Admin — Configuration",
			Endpoints: []apiEndpoint{
				{
					Method:   "GET",
					Path:     "/api/admin/config",
					Summary:  "List all config variables",
					Detail:   "Returns all configuration variables as a map keyed by variable name. Includes system variables (read-only name/type) and user-created variables.",
					Auth:     "admin",
					Response: `{"configs":{"app.name":{"name":"app.name","type":"string","value":"MCPLens","description":"Application name","isSystem":true,"options":""},...}}`,
				},
				{
					Method:   "POST",
					Path:     "/api/admin/config",
					Summary:  "Create a config variable",
					Detail:   "Creates a new user-defined configuration variable. Variable names must be unique. Types: <code>string</code>, <code>numeric</code>, <code>enum</code> (pipe-separated options), <code>template</code> (supports placeholders).",
					Auth:     "admin",
					Body:     `{"name":"feature.max_uploads","description":"Maximum uploads per user","type":"numeric","value":"100","options":""}`,
					Response: `{"name":"feature.max_uploads","type":"numeric","value":"100","description":"Maximum uploads per user","isSystem":false,"options":""}`,
				},
				{
					Method:  "GET",
					Path:    "/api/admin/config/{name}",
					Summary: "Get a config variable",
					Detail:  "Returns a single configuration variable by name.",
					Auth:    "admin",
					Params:  []apiParam{{"name", "string", true, "Config variable name"}},
					Response: `{"name":"app.name","type":"string","value":"MCPLens","description":"Application name","isSystem":true,"options":""}`,
				},
				{
					Method:  "PUT",
					Path:    "/api/admin/config/{name}",
					Summary: "Update a config variable",
					Detail:  "Updates the value (and optionally description/options) of a configuration variable. System variables only allow value changes. Enum variables validate against the options list.",
					Auth:    "admin",
					Params:  []apiParam{{"name", "string", true, "Config variable name"}},
					Body:    `{"value":"200","description":"Updated description (optional)"}`,
					Response: `{"name":"feature.max_uploads","type":"numeric","value":"200",...}`,
				},
				{
					Method:   "DELETE",
					Path:     "/api/admin/config/{name}",
					Summary:  "Delete a config variable",
					Detail:   "Deletes a user-created configuration variable. System variables cannot be deleted.",
					Auth:     "admin",
					Params:   []apiParam{{"name", "string", true, "Config variable name"}},
					Response: `{"message":"Config variable deleted"}`,
				},
			},
		},
		{
			Title: "Admin — Tenants",
			Endpoints: []apiEndpoint{
				{
					Method:   "GET",
					Path:     "/api/admin/tenants",
					Summary:  "List all tenants",
					Detail:   "Returns all tenants with member counts and billing information. Includes the plan name, billing waived status, and credit balances.",
					Auth:     "admin",
					Response: `{"tenants":[{"id":"...","name":"Acme Corp","slug":"acme-corp","isRoot":false,"isActive":true,"memberCount":5,"planName":"Pro","billingWaived":false,"subscriptionCredits":1000,"purchasedCredits":200,"createdAt":"..."}]}`,
				},
				{
					Method:  "GET",
					Path:    "/api/admin/tenants/{tenantId}",
					Summary: "Get tenant details",
					Detail:  "Returns full tenant details including all members with roles and join dates.",
					Auth:    "admin",
					Params:  []apiParam{{"tenantId", "ObjectID", true, "Tenant ID"}},
					Response: `{"tenant":{"id":"...","name":"Acme Corp","slug":"acme-corp","isRoot":false,"isActive":true,"planId":"...","billingWaived":false,"subscriptionCredits":1000,"purchasedCredits":200,"stripeCustomerId":"cus_...","billingStatus":"active","billingInterval":"year","currentPeriodEnd":"...","createdAt":"...","updatedAt":"..."},
 "members":[{"userId":"...","email":"jane@acme.com","displayName":"Jane Doe","role":"owner","joinedAt":"..."}]}`,
				},
				{
					Method:   "PUT",
					Path:     "/api/admin/tenants/{tenantId}",
					Summary:  "Update tenant",
					Detail:   "Updates tenant properties. All fields are optional — only provided fields are changed. Can modify name, billing waived status, and credit balances.",
					Auth:     "owner",
					Params:   []apiParam{{"tenantId", "ObjectID", true, "Tenant ID"}},
					Body:     `{"name":"New Name (optional)","billingWaived":true,"subscriptionCredits":5000,"purchasedCredits":100}`,
					Response: `{"message":"Tenant updated"}`,
				},
				{
					Method:   "PATCH",
					Path:     "/api/admin/tenants/{tenantId}/status",
					Summary:  "Activate or deactivate tenant",
					Detail:   "Sets a tenant's active status. Deactivated tenants cannot access the application. The root tenant cannot be deactivated.",
					Auth:     "owner",
					Params:   []apiParam{{"tenantId", "ObjectID", true, "Tenant ID"}},
					Body:     `{"isActive":false}`,
					Response: `{"message":"Tenant deactivated"}`,
				},
				{
					Method:   "PATCH",
					Path:     "/api/admin/tenants/{tenantId}/plan",
					Summary:  "Assign plan to tenant",
					Detail:   "Directly assigns a plan to a tenant (bypasses Stripe). Can also toggle billing waived status. Send an empty <code>planId</code> or omit it to remove the plan.",
					Auth:     "owner",
					Params:   []apiParam{{"tenantId", "ObjectID", true, "Tenant ID"}},
					Body:     `{"planId":"ObjectID (optional)","billingWaived":true}`,
					Response: `{"status":"updated"}`,
				},
				{
					Method:   "POST",
					Path:     "/api/admin/tenants/{tenantId}/cancel-subscription",
					Summary:  "Cancel subscription (admin override)",
					Detail:   "Cancels a tenant's Stripe subscription. Set <code>immediate</code> to true to cancel now; otherwise cancels at the end of the billing period.",
					Auth:     "owner",
					Params:   []apiParam{{"tenantId", "ObjectID", true, "Tenant ID"}},
					Body:     `{"immediate":false}`,
					Response: `{"message":"Subscription canceled"}`,
				},
				{
					Method:   "PATCH",
					Path:     "/api/admin/tenants/{tenantId}/subscription",
					Summary:  "Update subscription details",
					Detail:   "Manually updates subscription metadata such as the current period end date. Use this for correcting billing records.",
					Auth:     "owner",
					Params:   []apiParam{{"tenantId", "ObjectID", true, "Tenant ID"}},
					Body:     `{"currentPeriodEnd":"2026-03-15T00:00:00Z"}`,
					Response: `{"message":"Subscription updated"}`,
				},
			},
		},
		{
			Title: "Admin — Users",
			Endpoints: []apiEndpoint{
				{
					Method:   "GET",
					Path:     "/api/admin/users",
					Summary:  "List all users",
					Detail:   "Returns all users with summary information including tenant count and last login time.",
					Auth:     "owner",
					Response: `{"users":[{"id":"...","email":"jane@example.com","displayName":"Jane Doe","emailVerified":true,"isActive":true,"tenantCount":2,"createdAt":"...","lastLoginAt":"..."}]}`,
				},
				{
					Method:  "GET",
					Path:    "/api/admin/users/{userId}",
					Summary: "Get user details",
					Detail:  "Returns full user profile including authentication methods and all tenant memberships with billing details for each tenant.",
					Auth:    "owner",
					Params:  []apiParam{{"userId", "ObjectID", true, "User ID"}},
					Response: `{"user":{"id":"...","email":"jane@example.com","displayName":"Jane Doe","emailVerified":true,"isActive":true,"authMethods":[{"provider":"password"},{"provider":"google"}],"createdAt":"...","lastLoginAt":"..."},
 "memberships":[{"tenantId":"...","tenantName":"Acme Corp","tenantSlug":"acme-corp","isRoot":false,"role":"owner","joinedAt":"...","planId":"...","planName":"Pro","billingWaived":false,"subscriptionCredits":1000,"purchasedCredits":200}]}`,
				},
				{
					Method:   "PUT",
					Path:     "/api/admin/users/{userId}",
					Summary:  "Update user",
					Detail:   "Updates a user's email or display name. Both fields are optional — only provided fields are changed.",
					Auth:     "owner",
					Params:   []apiParam{{"userId", "ObjectID", true, "User ID"}},
					Body:     `{"email":"new@example.com","displayName":"New Name"}`,
					Response: `{"message":"User updated"}`,
				},
				{
					Method:   "PATCH",
					Path:     "/api/admin/users/{userId}/status",
					Summary:  "Activate or deactivate user",
					Detail:   "Sets a user's active status. Deactivated users cannot log in. Active sessions are not immediately terminated but will fail on the next API call.",
					Auth:     "owner",
					Params:   []apiParam{{"userId", "ObjectID", true, "User ID"}},
					Body:     `{"isActive":false}`,
					Response: `{"message":"User deactivated"}`,
				},
				{
					Method:   "PATCH",
					Path:     "/api/admin/users/{userId}/role/{tenantId}",
					Summary:  "Change user's role in tenant",
					Detail:   "Changes a user's role within a specific tenant. Can set to owner, admin, or user. When changing to owner, the current owner is demoted to admin.",
					Auth:     "owner",
					Params: []apiParam{
						{"userId", "ObjectID", true, "User ID"},
						{"tenantId", "ObjectID", true, "Tenant ID"},
					},
					Body:     `{"role":"admin"}`,
					Response: `{"message":"Role updated"}`,
				},
				{
					Method:  "GET",
					Path:    "/api/admin/users/{userId}/preflight-delete",
					Summary: "Preview delete effects",
					Detail:  "Returns a preview of what would happen if the user were deleted. Shows all tenants where the user is the owner and lists other members who could take ownership. Returns <code>canDelete: false</code> if the user is the sole owner of the root tenant.",
					Auth:    "owner",
					Params:  []apiParam{{"userId", "ObjectID", true, "User ID"}},
					Response: `{"canDelete":true,"ownerships":[{"tenantId":"...","tenantName":"Acme Corp","isRoot":false,"otherMembers":[{"userId":"...","email":"bob@acme.com","displayName":"Bob","role":"admin","joinedAt":"..."}]}]}`,
				},
				{
					Method:  "DELETE",
					Path:    "/api/admin/users/{userId}",
					Summary: "Delete user",
					Detail:  "Permanently deletes a user account. For tenants where the user is the owner, specify a replacement owner or confirm tenant deletion. The request body must resolve all ownership conflicts identified by the preflight endpoint.",
					Auth:    "owner",
					Params:  []apiParam{{"userId", "ObjectID", true, "User ID"}},
					Body:    `{"replacementOwners":{"tenantId":"newOwnerUserId"},"confirmTenantDeletions":["tenantId"]}`,
					Response: `{"message":"User deleted"}`,
				},
			},
		},
		{
			Title: "Admin — Plans",
			Endpoints: []apiEndpoint{
				{
					Method:   "GET",
					Path:     "/api/admin/plans",
					Summary:  "List all plans",
					Detail:   "Returns all subscription plans with subscriber counts.",
					Auth:     "admin",
					Response: `{"plans":[{"id":"...","name":"Pro","description":"...","monthlyPriceCents":2900,"annualDiscountPct":20,"usageCreditsPerMonth":1000,"creditResetPolicy":"reset","bonusCredits":0,"userLimit":10,"entitlements":{"feature_x":{"type":"bool","boolValue":true,"description":"..."}},"isSystem":false,"createdAt":"..."}]}`,
				},
				{
					Method:   "GET",
					Path:     "/api/admin/plans/{planId}",
					Summary:  "Get plan details",
					Detail:   "Returns full details for a single plan.",
					Auth:     "admin",
					Params:   []apiParam{{"planId", "ObjectID", true, "Plan ID"}},
					Response: `{"id":"...","name":"Pro","description":"...","monthlyPriceCents":2900,...}`,
				},
				{
					Method:   "GET",
					Path:     "/api/admin/entitlement-keys",
					Summary:  "List entitlement keys",
					Detail:   "Returns all unique entitlement keys currently in use across all plans, with their types and descriptions.",
					Auth:     "admin",
					Response: `{"keys":[{"key":"feature_x","type":"bool","description":"Enable feature X"}]}`,
				},
				{
					Method:  "POST",
					Path:    "/api/admin/plans",
					Summary: "Create a plan",
					Detail:  "Creates a new subscription plan. Plan names must be unique. Credit reset policy can be <code>reset</code> (credits reset each month) or <code>accrue</code> (unused credits roll over). Set <code>userLimit</code> to 0 for unlimited users.",
					Auth:    "owner",
					Body:    `{"name":"Enterprise","description":"For large teams","monthlyPriceCents":9900,"annualDiscountPct":25,"usageCreditsPerMonth":5000,"creditResetPolicy":"accrue","bonusCredits":1000,"userLimit":0,"entitlements":{"feature_x":{"type":"bool","boolValue":true,"description":"Enable feature X"}}}`,
					Response: `{"id":"...","name":"Enterprise",...}`,
				},
				{
					Method:   "PUT",
					Path:     "/api/admin/plans/{planId}",
					Summary:  "Update a plan",
					Detail:   "Updates an existing plan. System plans (Free) cannot be renamed. All fields from the create endpoint are accepted.",
					Auth:     "owner",
					Params:   []apiParam{{"planId", "ObjectID", true, "Plan ID"}},
					Body:     `{"name":"Enterprise Plus","monthlyPriceCents":14900,...}`,
					Response: `{"id":"...","name":"Enterprise Plus",...}`,
				},
				{
					Method:   "DELETE",
					Path:     "/api/admin/plans/{planId}",
					Summary:  "Delete a plan",
					Detail:   "Deletes a plan. System plans and plans with active subscribers cannot be deleted. Reassign subscribers first.",
					Auth:     "owner",
					Params:   []apiParam{{"planId", "ObjectID", true, "Plan ID"}},
					Response: `{"status":"deleted"}`,
				},
			},
		},
		{
			Title: "Admin — Credit Bundles",
			Endpoints: []apiEndpoint{
				{
					Method:   "GET",
					Path:     "/api/admin/credit-bundles",
					Summary:  "List all credit bundles",
					Detail:   "Returns all credit bundles (active and inactive), sorted by sort order.",
					Auth:     "admin",
					Response: `{"bundles":[{"id":"...","name":"500 Credits","credits":500,"priceCents":4900,"isActive":true,"sortOrder":1,"createdAt":"..."}]}`,
				},
				{
					Method:   "POST",
					Path:     "/api/admin/credit-bundles",
					Summary:  "Create a credit bundle",
					Detail:   "Creates a new credit bundle for purchase. Bundle names must be unique. Credits and price must be positive values.",
					Auth:     "owner",
					Body:     `{"name":"1000 Credits","credits":1000,"priceCents":8900,"isActive":true,"sortOrder":2}`,
					Response: `{"id":"...","name":"1000 Credits","credits":1000,...}`,
				},
				{
					Method:   "PUT",
					Path:     "/api/admin/credit-bundles/{bundleId}",
					Summary:  "Update a credit bundle",
					Detail:   "Updates an existing credit bundle.",
					Auth:     "owner",
					Params:   []apiParam{{"bundleId", "ObjectID", true, "Bundle ID"}},
					Body:     `{"name":"1000 Credits","credits":1000,"priceCents":7900,...}`,
					Response: `{"id":"...","name":"1000 Credits",...}`,
				},
				{
					Method:   "DELETE",
					Path:     "/api/admin/credit-bundles/{bundleId}",
					Summary:  "Delete a credit bundle",
					Detail:   "Permanently deletes a credit bundle.",
					Auth:     "owner",
					Params:   []apiParam{{"bundleId", "ObjectID", true, "Bundle ID"}},
					Response: `{"status":"deleted"}`,
				},
			},
		},
		{
			Title: "Admin — Financial",
			Endpoints: []apiEndpoint{
				{
					Method:  "GET",
					Path:    "/api/admin/financial/transactions",
					Summary: "List all transactions",
					Detail:  "Returns paginated billing transactions across all tenants. Supports filtering by tenant and text search across description, invoice number, plan name, and bundle name.",
					Auth:    "admin",
					Params: []apiParam{
						{"page", "int", false, "Page number (default: 1)"},
						{"perPage", "int", false, "Items per page, 1-100 (default: 50)"},
						{"tenantId", "ObjectID", false, "Filter by tenant"},
						{"search", "string", false, "Search description, invoice number, plan/bundle name"},
					},
					Response: `{"transactions":[{"id":"...","tenantId":"...","description":"Pro Plan (Annual)","type":"subscription","amountCents":29900,"currency":"usd","invoiceNumber":"INV-0001","planName":"Pro","createdAt":"..."}],
 "total":150,"page":1,"perPage":50}`,
				},
				{
					Method:  "GET",
					Path:    "/api/admin/financial/metrics",
					Summary: "Get financial metrics",
					Detail:  "Returns time-series financial data for charting. Supported metrics: <code>revenue</code> (daily revenue), <code>arr</code> (annualized recurring revenue), <code>dau</code> (daily active users), <code>mau</code> (monthly active users).",
					Auth:    "admin",
					Params: []apiParam{
						{"range", "string", false, "Time range: 7d, 30d, 1y (default: 30d)"},
						{"metric", "string", false, "Metric type: revenue, arr, dau, mau (default: revenue)"},
					},
					Response: `{"data":[{"date":"2026-02-01","value":15000},{"date":"2026-02-02","value":18500},...]}`,
				},
			},
		},
		{
			Title: "Admin — API Keys",
			Endpoints: []apiEndpoint{
				{
					Method:   "GET",
					Path:     "/api/admin/api-keys",
					Summary:  "List active API keys",
					Detail:   "Returns all active API keys with metadata. The key hash is never returned — only the preview (last 8 characters) is shown.",
					Auth:     "admin",
					Response: `{"apiKeys":[{"id":"...","name":"CI/CD Pipeline","keyPreview":"x7k9m2pq","authority":"admin","createdBy":"...","createdAt":"...","lastUsedAt":"...","isActive":true}]}`,
				},
				{
					Method:  "POST",
					Path:    "/api/admin/api-keys",
					Summary: "Create an API key",
					Detail:  "Creates a new API key and returns the raw key value. <strong>The raw key is only returned once</strong> — it is stored as a SHA-256 hash and cannot be retrieved later. Authority levels: <code>admin</code> keys auto-resolve the root tenant and get admin-level access; <code>user</code> keys require an <code>X-Tenant-ID</code> header.",
					Auth:    "admin",
					Body:    `{"name":"CI/CD Pipeline","authority":"admin"}`,
					Response: `{"apiKey":{"id":"...","name":"CI/CD Pipeline","keyPreview":"x7k9m2pq","authority":"admin",...},"rawKey":"lsk_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmno"}`,
				},
				{
					Method:   "DELETE",
					Path:     "/api/admin/api-keys/{keyId}",
					Summary:  "Revoke an API key",
					Detail:   "Soft-deletes an API key. The key immediately stops working for authentication. This cannot be undone.",
					Auth:     "admin",
					Params:   []apiParam{{"keyId", "ObjectID", true, "API key ID"}},
					Response: `{"status":"deleted"}`,
				},
			},
		},
		{
			Title: "Admin — Webhooks",
			Endpoints: []apiEndpoint{
				{
					Method:   "GET",
					Path:     "/api/admin/webhooks",
					Summary:  "List active webhooks",
					Detail:   "Returns all active webhook configurations sorted by creation date (newest first).",
					Auth:     "admin",
					Response: `{"webhooks":[{"id":"...","name":"Provisioning","description":"...","url":"https://example.com/webhook","secretPreview":"k9m2pqx7","events":["tenant.created"],"isActive":true,"createdBy":"...","createdAt":"..."}]}`,
				},
				{
					Method:   "GET",
					Path:     "/api/admin/webhooks/event-types",
					Summary:  "List available event types",
					Detail:   "Returns all webhook event types that can be subscribed to, with descriptions.",
					Auth:     "admin",
					Response: `{"eventTypes":[{"type":"tenant.created","description":"Fired when a new tenant is created..."}]}`,
				},
				{
					Method:  "POST",
					Path:    "/api/admin/webhooks",
					Summary: "Create a webhook",
					Detail:  "Creates a new webhook with an auto-generated signing secret (prefixed <code>whsec_</code>). The full secret is returned in the response — you can also retrieve it later from the detail endpoint. All deliveries include an <code>X-Webhook-Signature</code> header containing the HMAC-SHA256 signature of the payload.",
					Auth:    "admin",
					Body:    `{"name":"Provisioning","description":"Provision new tenants","url":"https://example.com/webhook","events":["tenant.created"]}`,
					Response: `{"webhook":{"id":"...","name":"Provisioning",...},"secret":"whsec_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdef"}`,
				},
				{
					Method:  "GET",
					Path:    "/api/admin/webhooks/{webhookId}",
					Summary: "Get webhook details",
					Detail:  "Returns full webhook configuration including the signing secret and the 20 most recent delivery attempts with their payloads and response details.",
					Auth:    "admin",
					Params:  []apiParam{{"webhookId", "ObjectID", true, "Webhook ID"}},
					Response: `{"webhook":{"id":"...","name":"Provisioning",...},"secret":"whsec_...","deliveries":[{"id":"...","eventType":"tenant.created","payload":"{...}","responseCode":200,"responseBody":"ok","success":true,"durationMs":120,"createdAt":"..."}]}`,
				},
				{
					Method:   "PUT",
					Path:     "/api/admin/webhooks/{webhookId}",
					Summary:  "Update webhook",
					Detail:   "Updates the webhook's name, description, URL, or subscribed events. The signing secret is not affected.",
					Auth:     "admin",
					Params:   []apiParam{{"webhookId", "ObjectID", true, "Webhook ID"}},
					Body:     `{"name":"Updated Name","description":"...","url":"https://new-url.com/webhook","events":["tenant.created"]}`,
					Response: `{"webhook":{"id":"...","name":"Updated Name",...}}`,
				},
				{
					Method:   "DELETE",
					Path:     "/api/admin/webhooks/{webhookId}",
					Summary:  "Delete webhook",
					Detail:   "Soft-deletes a webhook. It immediately stops receiving event deliveries.",
					Auth:     "admin",
					Params:   []apiParam{{"webhookId", "ObjectID", true, "Webhook ID"}},
					Response: `{"status":"deleted"}`,
				},
				{
					Method:   "POST",
					Path:     "/api/admin/webhooks/{webhookId}/test",
					Summary:  "Send test event",
					Detail:   "Delivers a test <code>tenant.created</code> event with sample data to the webhook URL. The delivery includes an <code>X-Webhook-Test: true</code> header so your handler can distinguish test deliveries. Returns the delivery result.",
					Auth:     "admin",
					Params:   []apiParam{{"webhookId", "ObjectID", true, "Webhook ID"}},
					Response: `{"delivery":{"id":"...","eventType":"tenant.created","success":true,"responseCode":200,"durationMs":85,"createdAt":"..."}}`,
				},
				{
					Method:   "POST",
					Path:     "/api/admin/webhooks/{webhookId}/regenerate-secret",
					Summary:  "Regenerate signing secret",
					Detail:   "Generates a new signing secret for the webhook. The old secret immediately stops working. Returns the new secret and preview.",
					Auth:     "admin",
					Params:   []apiParam{{"webhookId", "ObjectID", true, "Webhook ID"}},
					Response: `{"secret":"whsec_NEWsecretABCDEFGHIJKLMNOPQRSTUV","secretPreview":"QRSTUV12"}`,
				},
			},
		},
		{
			Title: "System",
			Endpoints: []apiEndpoint{
				{
					Method:   "GET",
					Path:     "/api/version",
					Summary:  "Get API version",
					Detail:   "Returns the current application version. All API responses also include the version in the <code>X-API-Version</code> response header and a unique <code>X-Request-ID</code> header for tracing.",
					Auth:     "none",
					Response: `{"version":"1.00"}`,
				},
				{
					Method:   "GET",
					Path:     "/health",
					Summary:  "Health check",
					Detail:   "Returns a simple health status. Used by load balancers and monitoring services to verify the server is running.",
					Auth:     "none",
					Response: `{"status":"ok"}`,
				},
				{
					Method: "POST",
					Path:   "/api/billing/webhook",
					Summary: "Stripe webhook",
					Detail: "Receives and processes Stripe webhook events. Authenticated via Stripe's webhook signature verification — not accessible with API keys or JWT tokens. Handles checkout completion, subscription updates, cancellations, and invoice events.",
					Auth:   "stripe",
				},
			},
		},
	}
}

// --- Auth helpers ---

func authLabel(auth string) string {
	switch auth {
	case "none":
		return "Public"
	case "jwt":
		return "Bearer token"
	case "jwt+tenant":
		return "Bearer token + X-Tenant-ID"
	case "admin":
		return "Admin (Bearer + root tenant)"
	case "owner":
		return "Owner (Bearer + root tenant)"
	case "stripe":
		return "Stripe signature"
	default:
		return auth
	}
}

func authBadge(auth string) string {
	switch auth {
	case "none":
		return `<span class="badge badge-public">Public</span>`
	case "jwt":
		return `<span class="badge badge-jwt">Bearer</span>`
	case "jwt+tenant":
		return `<span class="badge badge-tenant">Bearer + Tenant</span>`
	case "admin":
		return `<span class="badge badge-admin">Admin</span>`
	case "owner":
		return `<span class="badge badge-owner">Owner</span>`
	case "stripe":
		return `<span class="badge badge-stripe">Stripe</span>`
	default:
		return html.EscapeString(auth)
	}
}

// --- HTML documentation ---

// DocsHTML handles GET /api/docs — serves human-readable HTML documentation.
func DocsHTML(w http.ResponseWriter, r *http.Request) {
	sections := apiReference()
	events := webhookEventsDoc()
	var sb strings.Builder

	sb.WriteString(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>MCPLens API Reference</title>
<style>
*{box-sizing:border-box;margin:0;padding:0}
body{font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;background:#0a0a0f;color:#c8c8d0;line-height:1.6;padding:2rem 1rem;max-width:1000px;margin:0 auto}
h1{color:#fff;font-size:1.75rem;margin-bottom:.25rem}
.subtitle{color:#666;font-size:.875rem;margin-bottom:2rem}
h2{color:#fff;font-size:1.125rem;margin-top:2.5rem;margin-bottom:1rem;padding-bottom:.5rem;border-bottom:1px solid #1a1a2e}
h3{color:#e0e0e8;font-size:.9375rem;margin-top:1.5rem;margin-bottom:.5rem}
.endpoint{border:1px solid #1a1a2e;border-radius:.5rem;margin-bottom:.5rem;overflow:hidden}
.ep-header{display:flex;align-items:center;gap:.75rem;padding:.625rem .875rem;cursor:pointer;transition:background .15s}
.ep-header:hover{background:#111118}
.method{font-family:'SF Mono',SFMono-Regular,Consolas,monospace;font-size:.6875rem;font-weight:700;padding:.1875rem .5rem;border-radius:.25rem;min-width:4rem;text-align:center;display:inline-block;letter-spacing:.02em}
.GET{background:#0d3320;color:#34d399}.POST{background:#0c2d4a;color:#60a5fa}.PUT{background:#332b00;color:#fbbf24}.PATCH{background:#2d1f00;color:#fb923c}.DELETE{background:#3b0f0f;color:#f87171}
.ep-path{font-family:'SF Mono',SFMono-Regular,Consolas,monospace;font-size:.8125rem;color:#a78bfa;white-space:nowrap}
.ep-summary{color:#888;font-size:.8125rem;flex:1;margin-left:.25rem}
.badge{font-size:.625rem;padding:.125rem .375rem;border-radius:.1875rem;font-weight:600;letter-spacing:.03em;text-transform:uppercase;white-space:nowrap}
.badge-public{background:#1a2e1a;color:#4ade80}.badge-jwt{background:#1a1a2e;color:#a78bfa}.badge-tenant{background:#1a2a3e;color:#60a5fa}.badge-admin{background:#2e2a1a;color:#fbbf24}.badge-owner{background:#2e1a1a;color:#f87171}.badge-stripe{background:#1a1a2e;color:#818cf8}
.ep-toggle{color:#444;font-size:.75rem;margin-left:auto;transition:transform .2s}
.ep-toggle.open{transform:rotate(180deg)}
.ep-detail{display:none;padding:.75rem .875rem;border-top:1px solid #111;background:#08080d;font-size:.8125rem}
.ep-detail.open{display:block}
.ep-detail p{color:#888;margin-bottom:.75rem;line-height:1.5}
.ep-detail p code,.ep-detail p strong{color:#c8c8d0}
.detail-label{color:#555;font-size:.6875rem;text-transform:uppercase;letter-spacing:.05em;margin-bottom:.25rem;font-weight:600}
.detail-section{margin-bottom:.75rem}
.param-table{width:100%;border-collapse:collapse;font-size:.75rem}
.param-table th{text-align:left;color:#555;font-weight:600;padding:.25rem .5rem;border-bottom:1px solid #1a1a2e;text-transform:uppercase;letter-spacing:.05em;font-size:.625rem}
.param-table td{padding:.25rem .5rem;border-bottom:1px solid #111;color:#aaa}
.param-table td:first-child{color:#a78bfa;font-family:monospace}
.param-table .required{color:#f87171;font-size:.625rem}
pre.json{background:#0d0d14;border:1px solid #1a1a2e;border-radius:.375rem;padding:.5rem .75rem;overflow-x:auto;font-size:.75rem;line-height:1.4;color:#8b8ba0;font-family:'SF Mono',SFMono-Regular,Consolas,monospace;white-space:pre-wrap;word-break:break-all}
.note{background:#111;border:1px solid #1a1a2e;border-radius:.5rem;padding:1rem;margin:1.5rem 0;font-size:.8125rem;color:#888}
.note strong{color:#c8c8d0}
code{background:#1a1a2e;padding:.125rem .375rem;border-radius:.25rem;font-size:.8125rem;color:#a78bfa;font-family:'SF Mono',SFMono-Regular,Consolas,monospace}
a{color:#60a5fa;text-decoration:none}a:hover{text-decoration:underline}
.toc{margin:1.5rem 0;padding:1rem;background:#0d0d14;border:1px solid #1a1a2e;border-radius:.5rem}
.toc a{display:inline-block;margin:.125rem .5rem .125rem 0;font-size:.8125rem;color:#888}
.toc a:hover{color:#60a5fa}
.event-card{padding:.75rem;border:1px solid #1a1a2e;border-radius:.375rem;margin-bottom:.5rem}
.event-type{font-family:monospace;color:#a78bfa;font-size:.8125rem;font-weight:600}
.event-desc{color:#888;font-size:.8125rem;margin-top:.25rem}
</style>
</head>
<body>
`)
	sb.WriteString(fmt.Sprintf(`<h1>MCPLens API Reference</h1><p class="subtitle">Version %s</p>`, html.EscapeString(version.Current)))

	// Auth note
	sb.WriteString(`<div class="note">
<strong>Authentication:</strong> Include an access token or API key as <code>Authorization: Bearer &lt;token&gt;</code>.
Tenant-scoped routes require an <code>X-Tenant-ID</code> header.
Admin keys (authority=admin) auto-resolve the root tenant. User keys require <code>X-Tenant-ID</code>.
Admin routes require membership in the root tenant with admin or owner role.
</div>`)

	// Table of contents
	sb.WriteString(`<div class="toc"><strong style="color:#c8c8d0">Sections:</strong><br>`)
	for _, s := range sections {
		anchor := strings.ReplaceAll(strings.ToLower(s.Title), " ", "-")
		anchor = strings.ReplaceAll(anchor, "—", "")
		anchor = strings.ReplaceAll(anchor, "&", "")
		sb.WriteString(fmt.Sprintf(`<a href="#%s">%s</a>`, anchor, html.EscapeString(s.Title)))
	}
	sb.WriteString(`<a href="#webhook-events">Webhook Events</a>`)
	sb.WriteString(`</div>`)

	// Endpoint sections
	for _, s := range sections {
		anchor := strings.ReplaceAll(strings.ToLower(s.Title), " ", "-")
		anchor = strings.ReplaceAll(anchor, "—", "")
		anchor = strings.ReplaceAll(anchor, "&", "")
		sb.WriteString(fmt.Sprintf(`<h2 id="%s">%s</h2>`, anchor, html.EscapeString(s.Title)))

		for i, e := range s.Endpoints {
			eid := fmt.Sprintf("%s-%d", anchor, i)
			sb.WriteString(fmt.Sprintf(`<div class="endpoint"><div class="ep-header" onclick="toggleDetail('%s')">`, eid))
			sb.WriteString(fmt.Sprintf(`<span class="method %s">%s</span>`, e.Method, e.Method))
			sb.WriteString(fmt.Sprintf(`<span class="ep-path">%s</span>`, html.EscapeString(e.Path)))
			sb.WriteString(fmt.Sprintf(`<span class="ep-summary">%s</span>`, html.EscapeString(e.Summary)))
			sb.WriteString(authBadge(e.Auth))
			sb.WriteString(fmt.Sprintf(`<span class="ep-toggle" id="toggle-%s">▼</span>`, eid))
			sb.WriteString(`</div>`)

			// Detail panel
			sb.WriteString(fmt.Sprintf(`<div class="ep-detail" id="detail-%s">`, eid))
			sb.WriteString(fmt.Sprintf(`<p>%s</p>`, e.Detail)) // Detail may contain HTML

			if len(e.Params) > 0 {
				sb.WriteString(`<div class="detail-section"><div class="detail-label">Parameters</div>`)
				sb.WriteString(`<table class="param-table"><thead><tr><th>Name</th><th>Type</th><th>Required</th><th>Description</th></tr></thead><tbody>`)
				for _, p := range e.Params {
					req := ""
					if p.Required {
						req = `<span class="required">required</span>`
					} else {
						req = "optional"
					}
					sb.WriteString(fmt.Sprintf(`<tr><td>%s</td><td>%s</td><td>%s</td><td>%s</td></tr>`,
						html.EscapeString(p.Name), html.EscapeString(p.Type), req, html.EscapeString(p.Desc)))
				}
				sb.WriteString(`</tbody></table></div>`)
			}

			if e.Body != "" {
				sb.WriteString(`<div class="detail-section"><div class="detail-label">Request Body</div>`)
				sb.WriteString(fmt.Sprintf(`<pre class="json">%s</pre></div>`, html.EscapeString(e.Body)))
			}

			if e.Response != "" {
				sb.WriteString(`<div class="detail-section"><div class="detail-label">Response</div>`)
				sb.WriteString(fmt.Sprintf(`<pre class="json">%s</pre></div>`, html.EscapeString(e.Response)))
			}

			sb.WriteString(`</div></div>`)
		}
	}

	// Webhook events section
	sb.WriteString(`<h2 id="webhook-events">Webhook Events</h2>`)
	sb.WriteString(`<p style="color:#888;font-size:.8125rem;margin-bottom:1rem">Events that can be subscribed to via webhooks. Each delivery includes an <code>X-Webhook-Signature</code> header containing the HMAC-SHA256 hex digest of the JSON payload, computed with your webhook's signing secret.</p>`)
	for _, ev := range events {
		sb.WriteString(`<div class="event-card">`)
		sb.WriteString(fmt.Sprintf(`<div class="event-type">%s</div>`, html.EscapeString(ev.Type)))
		sb.WriteString(fmt.Sprintf(`<div class="event-desc">%s</div>`, html.EscapeString(ev.Desc)))
		sb.WriteString(`</div>`)
	}

	// JavaScript for toggles
	sb.WriteString(`
<script>
function toggleDetail(id){
  var d=document.getElementById('detail-'+id);
  var t=document.getElementById('toggle-'+id);
  if(d.classList.contains('open')){d.classList.remove('open');t.classList.remove('open')}
  else{d.classList.add('open');t.classList.add('open')}
}
</script>`)

	sb.WriteString(`</body></html>`)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Write([]byte(sb.String()))
}

// --- Markdown documentation ---

// DocsMarkdown handles GET /api/docs/markdown — serves markdown API reference.
func DocsMarkdown(w http.ResponseWriter, r *http.Request) {
	sections := apiReference()
	events := webhookEventsDoc()
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# MCPLens API Reference\n\n**Version:** %s\n\n", version.Current))

	sb.WriteString("## Authentication\n\n")
	sb.WriteString("Include an access token or API key as `Authorization: Bearer <token>`.\n")
	sb.WriteString("Tenant-scoped routes require an `X-Tenant-ID` header.\n")
	sb.WriteString("Admin keys (authority=admin) auto-resolve the root tenant. User keys require `X-Tenant-ID`.\n")
	sb.WriteString("Admin routes require membership in the root tenant with admin or owner role.\n\n")

	sb.WriteString("## Auth Levels\n\n")
	sb.WriteString("| Level | Description |\n|---|---|\n")
	sb.WriteString("| Public | No authentication required |\n")
	sb.WriteString("| Bearer token | JWT access token or API key |\n")
	sb.WriteString("| Bearer token + X-Tenant-ID | Token plus tenant context |\n")
	sb.WriteString("| Admin | Root tenant membership with admin+ role |\n")
	sb.WriteString("| Owner | Root tenant membership with owner role |\n\n")

	for _, s := range sections {
		sb.WriteString(fmt.Sprintf("## %s\n\n", s.Title))

		for _, e := range s.Endpoints {
			sb.WriteString(fmt.Sprintf("### `%s` %s\n\n", e.Method, e.Path))
			sb.WriteString(fmt.Sprintf("**%s** — %s\n\n", e.Summary, authLabel(e.Auth)))
			sb.WriteString(fmt.Sprintf("%s\n\n", stripHTML(e.Detail)))

			if len(e.Params) > 0 {
				sb.WriteString("**Parameters:**\n\n")
				sb.WriteString("| Name | Type | Required | Description |\n|---|---|---|---|\n")
				for _, p := range e.Params {
					req := "No"
					if p.Required {
						req = "Yes"
					}
					sb.WriteString(fmt.Sprintf("| `%s` | %s | %s | %s |\n", p.Name, p.Type, req, p.Desc))
				}
				sb.WriteString("\n")
			}

			if e.Body != "" {
				sb.WriteString("**Request Body:**\n\n```json\n")
				sb.WriteString(e.Body)
				sb.WriteString("\n```\n\n")
			}

			if e.Response != "" {
				sb.WriteString("**Response:**\n\n```json\n")
				sb.WriteString(e.Response)
				sb.WriteString("\n```\n\n")
			}

			sb.WriteString("---\n\n")
		}
	}

	// Webhook events
	sb.WriteString("## Webhook Events\n\n")
	sb.WriteString("Events that can be subscribed to via webhooks. Each delivery includes an `X-Webhook-Signature` header containing the HMAC-SHA256 hex digest of the JSON payload, computed with your webhook's signing secret.\n\n")
	sb.WriteString("| Event | Description |\n|---|---|\n")
	for _, ev := range events {
		sb.WriteString(fmt.Sprintf("| `%s` | %s |\n", ev.Type, ev.Desc))
	}
	sb.WriteString("\n")

	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Write([]byte(sb.String()))
}

// stripHTML removes simple HTML tags for the markdown output.
func stripHTML(s string) string {
	result := s
	result = strings.ReplaceAll(result, "<code>", "`")
	result = strings.ReplaceAll(result, "</code>", "`")
	result = strings.ReplaceAll(result, "<strong>", "**")
	result = strings.ReplaceAll(result, "</strong>", "**")
	result = strings.ReplaceAll(result, "&amp;", "&")
	result = strings.ReplaceAll(result, "&lt;", "<")
	result = strings.ReplaceAll(result, "&gt;", ">")
	return result
}
