package configstore

import (
	"context"
	"log/slog"
	"os"
	"time"

	"lastsaas/internal/db"
	"lastsaas/internal/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

func appNameDefault() string {
	if name := os.Getenv("APP_NAME"); name != "" {
		return name
	}
	return "MCPLens"
}

// SystemDefaults defines the system-level configuration variables that must always exist.
var SystemDefaults = []models.ConfigVar{
	{
		Name:        "app.name",
		Description: "Application name used in email templates and other system text (referenced as {{.AppName}} in templates)",
		Type:        models.ConfigTypeString,
		Value:       appNameDefault(),
		IsSystem:    true,
	},
	{
		Name:        "email.verification.subject",
		Description: "Subject line for the email verification message",
		Type:        models.ConfigTypeTemplate,
		Value:       `Verify your {{.AppName}} account`,
		IsSystem:    true,
	},
	{
		Name:        "email.verification.body",
		Description: "HTML body for the email verification message",
		Type:        models.ConfigTypeTemplate,
		Value: `<!DOCTYPE html>
<html>
<head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1.0"></head>
<body style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; background-color: #f8fafc; color: #1e293b; margin: 0; padding: 40px 20px;">
    <div style="max-width: 600px; margin: 0 auto; background: #ffffff; border: 1px solid #e2e8f0; border-radius: 12px; padding: 40px;">
        <h1 style="color: #0f172a; margin: 0 0 8px 0; font-size: 24px;">{{.AppName}}</h1>
        <hr style="border: none; border-top: 1px solid #e2e8f0; margin: 20px 0;">
        <h2 style="color: #1e293b; margin-bottom: 16px;">Verify Your Email</h2>
        <p style="color: #475569; line-height: 1.6;">Hi {{.DisplayName}},</p>
        <p style="color: #475569; line-height: 1.6;">Thanks for signing up! Please verify your email address by clicking the button below:</p>
        <div style="text-align: center; margin: 30px 0;">
            <a href="{{.VerifyURL}}" style="display: inline-block; background: #2563eb; color: white; text-decoration: none; padding: 14px 32px; border-radius: 8px; font-weight: 600; font-size: 16px;">Verify Email Address</a>
        </div>
        <p style="color: #94a3b8; font-size: 14px;">If you didn't create an account, you can safely ignore this email.</p>
        <p style="color: #94a3b8; font-size: 14px;">This link will expire in 24 hours.</p>
    </div>
</body>
</html>`,
		IsSystem: true,
	},
	{
		Name:        "email.password_reset.subject",
		Description: "Subject line for the password reset email",
		Type:        models.ConfigTypeTemplate,
		Value:       `Reset your {{.AppName}} password`,
		IsSystem:    true,
	},
	{
		Name:        "email.password_reset.body",
		Description: "HTML body for the password reset email",
		Type:        models.ConfigTypeTemplate,
		Value: `<!DOCTYPE html>
<html>
<head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1.0"></head>
<body style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; background-color: #f8fafc; color: #1e293b; margin: 0; padding: 40px 20px;">
    <div style="max-width: 600px; margin: 0 auto; background: #ffffff; border: 1px solid #e2e8f0; border-radius: 12px; padding: 40px;">
        <h1 style="color: #0f172a; margin: 0 0 8px 0; font-size: 24px;">{{.AppName}}</h1>
        <hr style="border: none; border-top: 1px solid #e2e8f0; margin: 20px 0;">
        <h2 style="color: #1e293b; margin-bottom: 16px;">Reset Your Password</h2>
        <p style="color: #475569; line-height: 1.6;">Hi {{.DisplayName}},</p>
        <p style="color: #475569; line-height: 1.6;">We received a request to reset your password. Click the button below to choose a new password:</p>
        <div style="text-align: center; margin: 30px 0;">
            <a href="{{.ResetURL}}" style="display: inline-block; background: #2563eb; color: white; text-decoration: none; padding: 14px 32px; border-radius: 8px; font-weight: 600; font-size: 16px;">Reset Password</a>
        </div>
        <p style="color: #94a3b8; font-size: 14px;">If you didn't request a password reset, you can safely ignore this email.</p>
        <p style="color: #94a3b8; font-size: 14px;">This link will expire in 1 hour.</p>
    </div>
</body>
</html>`,
		IsSystem: true,
	},
	{
		Name:        "email.invitation.subject",
		Description: "Subject line for the team invitation email",
		Type:        models.ConfigTypeTemplate,
		Value:       `You've been invited to {{.TenantName}}`,
		IsSystem:    true,
	},
	{
		Name:        "email.invitation.body",
		Description: "HTML body for the team invitation email",
		Type:        models.ConfigTypeTemplate,
		Value: `<!DOCTYPE html>
<html>
<head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1.0"></head>
<body style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; background-color: #f8fafc; color: #1e293b; margin: 0; padding: 40px 20px;">
    <div style="max-width: 600px; margin: 0 auto; background: #ffffff; border: 1px solid #e2e8f0; border-radius: 12px; padding: 40px;">
        <h1 style="color: #0f172a; margin: 0 0 8px 0; font-size: 24px;">{{.AppName}}</h1>
        <hr style="border: none; border-top: 1px solid #e2e8f0; margin: 20px 0;">
        <h2 style="color: #1e293b; margin-bottom: 16px;">You've Been Invited</h2>
        <p style="color: #475569; line-height: 1.6;">{{.InviterName}} has invited you to join <strong>{{.TenantName}}</strong>.</p>
        <div style="text-align: center; margin: 30px 0;">
            <a href="{{.InviteURL}}" style="display: inline-block; background: #2563eb; color: white; text-decoration: none; padding: 14px 32px; border-radius: 8px; font-weight: 600; font-size: 16px;">Accept Invitation</a>
        </div>
        <p style="color: #94a3b8; font-size: 14px;">This invitation will expire in 7 days.</p>
    </div>
</body>
</html>`,
		IsSystem: true,
	},
	{
		Name:        "health.cpu.warning_threshold",
		Description: "CPU usage percentage that triggers a warning status indicator",
		Type:        models.ConfigTypeNumeric,
		Value:       "70",
		IsSystem:    true,
	},
	{
		Name:        "health.cpu.critical_threshold",
		Description: "CPU usage percentage that triggers a critical status indicator",
		Type:        models.ConfigTypeNumeric,
		Value:       "90",
		IsSystem:    true,
	},
	{
		Name:        "health.memory.warning_threshold",
		Description: "Memory usage percentage that triggers a warning status indicator",
		Type:        models.ConfigTypeNumeric,
		Value:       "75",
		IsSystem:    true,
	},
	{
		Name:        "health.memory.critical_threshold",
		Description: "Memory usage percentage that triggers a critical status indicator",
		Type:        models.ConfigTypeNumeric,
		Value:       "90",
		IsSystem:    true,
	},
	{
		Name:        "health.disk.warning_threshold",
		Description: "Disk usage percentage that triggers a warning status indicator",
		Type:        models.ConfigTypeNumeric,
		Value:       "80",
		IsSystem:    true,
	},
	{
		Name:        "health.disk.critical_threshold",
		Description: "Disk usage percentage that triggers a critical status indicator",
		Type:        models.ConfigTypeNumeric,
		Value:       "95",
		IsSystem:    true,
	},
	{
		Name:        "health.node.stale_timeout_seconds",
		Description: "Seconds without heartbeat before a node is considered stale",
		Type:        models.ConfigTypeNumeric,
		Value:       "180",
		IsSystem:    true,
	},
	{
		Name:        "health.metrics.retention_days",
		Description: "Number of days to retain system metrics before automatic deletion",
		Type:        models.ConfigTypeNumeric,
		Value:       "30",
		IsSystem:    true,
	},
	{
		Name:        "log.min_level",
		Description: "Minimum severity level for system log entries. Messages below this level are discarded.",
		Type:        models.ConfigTypeEnum,
		Value:       "debug",
		Options:     `[{"label":"No Logging","value":"none"},{"label":"Critical","value":"critical"},{"label":"High","value":"high"},{"label":"Medium","value":"medium"},{"label":"Low","value":"low"},{"label":"Debug","value":"debug"}]`,
		IsSystem:    true,
	},
	{
		Name:        "team.upgrade_prompt.title",
		Description: "Title shown when a user tries to invite a member but has reached their plan's user limit. Supports {{.UserLimit}}.",
		Type:        models.ConfigTypeTemplate,
		Value:       "Team limit reached",
		IsSystem:    true,
	},
	{
		Name:        "team.upgrade_prompt.body",
		Description: "Body text for the upgrade prompt shown when the user limit is reached. Supports {{.UserLimit}}, {{.PlanName}}, {{.AppName}}.",
		Type:        models.ConfigTypeTemplate,
		Value:       "Your current plan allows up to {{.UserLimit}} team members. Upgrade your plan to invite more people to your team.",
		IsSystem:    true,
	},
	{
		Name:        "entitlement.upgrade_prompt.title",
		Description: "Title shown when testing an entitlement the current plan does not include. Supports {{.EntitlementName}}, {{.PlanName}}, {{.AppName}}.",
		Type:        models.ConfigTypeTemplate,
		Value:       "Upgrade required",
		IsSystem:    true,
	},
	{
		Name:        "entitlement.upgrade_prompt.body",
		Description: "Body text shown when a boolean entitlement is not included in the current plan. Supports {{.EntitlementName}}, {{.PlanName}}, {{.AppName}}, {{.RecommendedPlanName}}.",
		Type:        models.ConfigTypeTemplate,
		Value:       "Your current {{.PlanName}} plan does not include {{.EntitlementName}}. Upgrade to {{.RecommendedPlanName}} to unlock this feature.",
		IsSystem:    true,
	},
	{
		Name:        "entitlement.upgrade_prompt.numeric_body",
		Description: "Body text shown when a numeric entitlement limit is exceeded. Supports {{.EntitlementName}}, {{.PlanName}}, {{.AppName}}, {{.RecommendedPlanName}}, {{.CurrentValue}}, {{.RequestedValue}}.",
		Type:        models.ConfigTypeTemplate,
		Value:       "Your current {{.PlanName}} plan allows a maximum of {{.CurrentValue}} {{.EntitlementName}}. Upgrade to {{.RecommendedPlanName}} for more.",
		IsSystem:    true,
	},
	{
		Name:        "analytics.head_snippet",
		Description: "HTML snippet injected into <head> on all app pages (not admin). Use for analytics tracking codes such as Google Analytics.",
		Type:        models.ConfigTypeString,
		Value:       "<!-- analytics snippet: paste your tracking code here -->",
		IsSystem:    true,
	},
	{
		Name:        "billing.failed_charge.message_subject",
		Description: "Subject for the in-app message sent when a subscription payment fails. Supports {{.AppName}}.",
		Type:        models.ConfigTypeTemplate,
		Value:       `Action Required: Payment Failed for {{.AppName}}`,
		IsSystem:    true,
	},
	{
		Name:        "billing.failed_charge.message_body",
		Description: "Body for the in-app message sent when a subscription payment fails. Supports {{.AppName}} and {{.BillingURL}}.",
		Type:        models.ConfigTypeTemplate,
		Value:       `Your most recent payment for {{.AppName}} was unsuccessful. Please update your billing information to avoid any interruption to your service. You can update your payment method by visiting your <a href="{{.BillingURL}}">billing settings</a>.`,
		IsSystem:    true,
	},
	{
		Name:        "auth.session.access_ttl_minutes",
		Description: "JWT access token lifetime in minutes. Short-lived tokens are silently refreshed in the background.",
		Type:        models.ConfigTypeNumeric,
		Value:       "60",
		IsSystem:    true,
	},
	{
		Name:        "auth.session.refresh_ttl_days",
		Description: "Refresh token lifetime in days. This controls how long a user stays logged in without re-authenticating.",
		Type:        models.ConfigTypeNumeric,
		Value:       "30",
		IsSystem:    true,
	},
	{
		Name:        "auth.magic_link.enabled",
		Description: "Enable or disable magic link (passwordless) login. When enabled, users can sign in via an emailed link.",
		Type:        models.ConfigTypeEnum,
		Value:       "false",
		Options:     `[{"label":"Enabled","value":"true"},{"label":"Disabled","value":"false"}]`,
		IsSystem:    true,
	},
	{
		Name:        "auth.passkeys.enabled",
		Description: "Enable or disable passkey/WebAuthn authentication. When enabled, users can register and sign in with passkeys.",
		Type:        models.ConfigTypeEnum,
		Value:       "false",
		Options:     `[{"label":"Enabled","value":"true"},{"label":"Disabled","value":"false"}]`,
		IsSystem:    true,
	},
	{
		Name:        "auth.mfa.enabled",
		Description: "Enable or disable TOTP two-factor authentication. When enabled, users can set up MFA via authenticator apps.",
		Type:        models.ConfigTypeEnum,
		Value:       "false",
		Options:     `[{"label":"Enabled","value":"true"},{"label":"Disabled","value":"false"}]`,
		IsSystem:    true,
	},
	{
		Name:        "auth.sso.enabled",
		Description: "Enable or disable SAML SSO authentication. When enabled, tenants can configure SAML identity providers.",
		Type:        models.ConfigTypeEnum,
		Value:       "false",
		Options:     `[{"label":"Enabled","value":"true"},{"label":"Disabled","value":"false"}]`,
		IsSystem:    true,
	},
	{
		Name:        "onboarding.enabled",
		Description: "Enable or disable the onboarding wizard for new users.",
		Type:        models.ConfigTypeEnum,
		Value:       "true",
		Options:     `[{"label":"Enabled","value":"true"},{"label":"Disabled","value":"false"}]`,
		IsSystem:    true,
	},
	{
		Name:        "onboarding.steps",
		Description: "JSON array of onboarding steps to show. Valid steps: profile, team, plan.",
		Type:        models.ConfigTypeString,
		Value:       `["profile","team","plan"]`,
		IsSystem:    true,
	},
	{
		Name:        "billing.company_name",
		Description: "Company name displayed on invoices. Leave blank to omit.",
		Type:        models.ConfigTypeString,
		Value:       "",
		IsSystem:    true,
	},
	{
		Name:        "billing.company_address",
		Description: "Company address displayed on invoices. Use \\n for line breaks (e.g. \"123 Main St\\nNew York, NY 10001\").",
		Type:        models.ConfigTypeString,
		Value:       "",
		IsSystem:    true,
	},
	{
		Name:        "billing.default_currency",
		Description: "Default currency for Stripe billing (e.g. usd, eur, gbp). Must be a valid Stripe currency code.",
		Type:        models.ConfigTypeString,
		Value:       "usd",
		IsSystem:    true,
	},
	{
		Name:        "billing.tax.enabled",
		Description: "Enable Stripe Tax for automatic tax calculation on checkout. Requires Stripe Tax to be configured in the Stripe Dashboard.",
		Type:        models.ConfigTypeEnum,
		Value:       "false",
		Options:     `[{"label":"Enabled","value":"true"},{"label":"Disabled","value":"false"}]`,
		IsSystem:    true,
	},
	{
		Name:        "billing.tax.id",
		Description: "Company tax/VAT registration number to display on invoices (e.g. \"VAT: GB123456789\"). Leave blank to omit.",
		Type:        models.ConfigTypeString,
		Value:       "",
		IsSystem:    true,
	},
	{
		Name:        "email.magic_link.subject",
		Description: "Subject line for the magic link login email. Supports {{.AppName}}.",
		Type:        models.ConfigTypeTemplate,
		Value:       `Sign in to {{.AppName}}`,
		IsSystem:    true,
	},
	{
		Name:        "email.magic_link.body",
		Description: "HTML body for the magic link login email. Supports {{.AppName}}, {{.DisplayName}}, {{.MagicLinkURL}}.",
		Type:        models.ConfigTypeTemplate,
		Value: `<!DOCTYPE html>
<html>
<head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1.0"></head>
<body style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; background-color: #f8fafc; color: #1e293b; margin: 0; padding: 40px 20px;">
    <div style="max-width: 600px; margin: 0 auto; background: #ffffff; border: 1px solid #e2e8f0; border-radius: 12px; padding: 40px;">
        <h1 style="color: #0f172a; margin: 0 0 8px 0; font-size: 24px;">{{.AppName}}</h1>
        <hr style="border: none; border-top: 1px solid #e2e8f0; margin: 20px 0;">
        <h2 style="color: #1e293b; margin-bottom: 16px;">Sign In</h2>
        <p style="color: #475569; line-height: 1.6;">Hi {{.DisplayName}},</p>
        <p style="color: #475569; line-height: 1.6;">Click the button below to sign in to your account:</p>
        <div style="text-align: center; margin: 30px 0;">
            <a href="{{.MagicLinkURL}}" style="display: inline-block; background: #2563eb; color: white; text-decoration: none; padding: 14px 32px; border-radius: 8px; font-weight: 600; font-size: 16px;">Sign In</a>
        </div>
        <p style="color: #94a3b8; font-size: 14px;">If you didn't request this link, you can safely ignore this email.</p>
        <p style="color: #94a3b8; font-size: 14px;">This link will expire in 15 minutes.</p>
    </div>
</body>
</html>`,
		IsSystem: true,
	},
}

// Seed inserts any missing system-defined variables into the database.
// Existing variables are not overwritten.
func Seed(ctx context.Context, database *db.MongoDB) error {
	col := database.ConfigVars()
	now := time.Now()

	for _, def := range SystemDefaults {
		err := col.FindOne(ctx, bson.M{"name": def.Name}).Err()
		if err == mongo.ErrNoDocuments {
			def.CreatedAt = now
			def.UpdatedAt = now
			if _, insertErr := col.InsertOne(ctx, def); insertErr != nil {
				return insertErr
			}
			slog.Info("Seeded system config variable", "name", def.Name)
		} else if err != nil {
			return err
		}
	}
	return nil
}
