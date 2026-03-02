package email

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"math"
	"net/http"
	"time"

	"lastsaas/internal/apicounter"
)

type ResendService struct {
	apiKey      string
	fromEmail   string
	fromName    string
	appName     string
	baseURL     string
	frontendURL string
	getConfig   func(string) string
	httpClient  *http.Client
}

type emailRequest struct {
	From    string   `json:"from"`
	To      []string `json:"to"`
	Subject string   `json:"subject"`
	HTML    string   `json:"html"`
}

func NewResendService(apiKey, fromEmail, fromName, appName, frontendURL string, getConfig func(string) string) *ResendService {
	return &ResendService{
		apiKey:      apiKey,
		fromEmail:   fromEmail,
		fromName:    fromName,
		appName:     appName,
		baseURL:     "https://api.resend.com",
		frontendURL: frontendURL,
		getConfig:   getConfig,
		httpClient:  &http.Client{Timeout: 30 * time.Second},
	}
}

func (s *ResendService) from() string {
	if s.fromName == "" {
		return s.fromEmail
	}
	return fmt.Sprintf("%s <%s>", s.fromName, s.fromEmail)
}

func (s *ResendService) SendEmail(to, subject, html string) error {
	reqBody := emailRequest{
		From:    s.from(),
		To:      []string{to},
		Subject: subject,
		HTML:    html,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal email request: %w", err)
	}

	const maxRetries = 3
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(math.Pow(2, float64(attempt))) * 500 * time.Millisecond
			time.Sleep(backoff)
			slog.Warn("email retry attempt", "attempt", attempt+1, "maxRetries", maxRetries, "to", to)
		}

		req, err := http.NewRequest("POST", s.baseURL+"/emails", bytes.NewBuffer(jsonBody))
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Authorization", "Bearer "+s.apiKey)
		req.Header.Set("Content-Type", "application/json")

		resp, err := s.httpClient.Do(req)
		if err != nil {
			if attempt < maxRetries-1 {
				slog.Warn("email network error, will retry", "error", err)
				continue
			}
			return fmt.Errorf("failed to send email after %d attempts: %w", maxRetries, err)
		}

		if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
			resp.Body.Close()
			apicounter.ResendEmails.Add(1)
			slog.Info("email sent successfully", "to", to)
			return nil
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		// Retry on transient errors (429 rate limit, 5xx server errors)
		if (resp.StatusCode == 429 || resp.StatusCode >= 500) && attempt < maxRetries-1 {
			slog.Warn("email API transient error, will retry", "status", resp.StatusCode, "body", string(body))
			continue
		}

		slog.Error("email API error", "status", resp.StatusCode, "body", string(body))
		return fmt.Errorf("email API returned status %d", resp.StatusCode)
	}
	return fmt.Errorf("email send failed after %d attempts", maxRetries)
}

// resolveAppName returns the app name from the config system, falling back to the constructor value.
func (s *ResendService) resolveAppName() string {
	if s.getConfig != nil {
		if name := s.getConfig("app.name"); name != "" {
			return name
		}
	}
	return s.appName
}

// executeTemplate loads a template from config, parses it, and executes with the given data.
// If the config value is empty or parsing/execution fails, the fallback string is returned.
func (s *ResendService) executeTemplate(configKey string, data map[string]string, fallback string) string {
	tmplStr := ""
	if s.getConfig != nil {
		tmplStr = s.getConfig(configKey)
	}
	if tmplStr == "" {
		tmplStr = fallback
	}

	t, err := template.New(configKey).Parse(tmplStr)
	if err != nil {
		slog.Error("email: failed to parse template, using fallback", "template", configKey, "error", err)
		return fallback
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		slog.Error("email: failed to execute template, using fallback", "template", configKey, "error", err)
		return fallback
	}
	return buf.String()
}

func (s *ResendService) SendVerificationEmail(to, displayName, token string) error {
	verifyURL := fmt.Sprintf("%s/verify-email?token=%s", s.frontendURL, token)
	appName := s.resolveAppName()

	data := map[string]string{
		"AppName":     appName,
		"DisplayName": displayName,
		"VerifyURL":   verifyURL,
	}

	subject := s.executeTemplate("email.verification.subject", data,
		fmt.Sprintf("Verify your %s account", appName))

	fallbackBody := `<!DOCTYPE html>
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
</html>`

	html := s.executeTemplate("email.verification.body", data, fallbackBody)

	return s.SendEmail(to, subject, html)
}

func (s *ResendService) SendPasswordResetEmail(to, displayName, token string) error {
	resetURL := fmt.Sprintf("%s/reset-password?token=%s", s.frontendURL, token)
	appName := s.resolveAppName()

	data := map[string]string{
		"AppName":     appName,
		"DisplayName": displayName,
		"ResetURL":    resetURL,
	}

	subject := s.executeTemplate("email.password_reset.subject", data,
		fmt.Sprintf("Reset your %s password", appName))

	fallbackBody := `<!DOCTYPE html>
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
</html>`

	html := s.executeTemplate("email.password_reset.body", data, fallbackBody)

	return s.SendEmail(to, subject, html)
}

func (s *ResendService) SendMagicLinkEmail(to, displayName, token string) error {
	magicLinkURL := fmt.Sprintf("%s/auth/magic-link?token=%s", s.frontendURL, token)
	appName := s.resolveAppName()

	data := map[string]string{
		"AppName":      appName,
		"DisplayName":  displayName,
		"MagicLinkURL": magicLinkURL,
	}

	subject := s.executeTemplate("email.magic_link.subject", data,
		fmt.Sprintf("Sign in to %s", appName))

	fallbackBody := `<!DOCTYPE html>
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
</html>`

	html := s.executeTemplate("email.magic_link.body", data, fallbackBody)

	return s.SendEmail(to, subject, html)
}

func (s *ResendService) SendInvitationEmail(to, inviterName, tenantName, token string) error {
	inviteURL := fmt.Sprintf("%s/signup?invitation=%s", s.frontendURL, token)
	appName := s.resolveAppName()

	data := map[string]string{
		"AppName":     appName,
		"InviterName": inviterName,
		"TenantName":  tenantName,
		"InviteURL":   inviteURL,
	}

	subject := s.executeTemplate("email.invitation.subject", data,
		fmt.Sprintf("You've been invited to %s", tenantName))

	fallbackBody := `<!DOCTYPE html>
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
</html>`

	html := s.executeTemplate("email.invitation.body", data, fallbackBody)

	return s.SendEmail(to, subject, html)
}
