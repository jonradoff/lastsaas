package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"lastsaas/internal/db"
	"lastsaas/internal/middleware"
	"lastsaas/internal/models"
	"lastsaas/internal/scanner"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// AgencyBranding stores white-label branding for Agency plan tenants.
type AgencyBranding struct {
	ID          primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	TenantID    primitive.ObjectID `json:"tenantId" bson:"tenantId"`
	Logo        string             `json:"logo" bson:"logo"`
	CompanyName string             `json:"companyName" bson:"companyName"`
	AccentColor string             `json:"accentColor" bson:"accentColor"`
	UpdatedAt   time.Time          `json:"updatedAt" bson:"updatedAt"`
}

// AgencyBrandingHandler manages agency white-label branding.
type AgencyBrandingHandler struct {
	db      *db.MongoDB
	scanner *scanner.Service
}

// NewAgencyBrandingHandler creates a new AgencyBrandingHandler.
func NewAgencyBrandingHandler(database *db.MongoDB, svc *scanner.Service) *AgencyBrandingHandler {
	return &AgencyBrandingHandler{db: database, scanner: svc}
}

// hasAgencyBranding checks whether the tenant's plan grants the agency_branding entitlement.
func (h *AgencyBrandingHandler) hasAgencyBranding(r *http.Request, tenant *models.Tenant) bool {
	// Root tenants and billing-waived tenants get all features
	if tenant.IsRoot || tenant.BillingWaived {
		return true
	}
	if tenant.PlanID == nil {
		return false
	}
	var plan models.Plan
	if err := h.db.Plans().FindOne(r.Context(), bson.M{"_id": *tenant.PlanID}).Decode(&plan); err != nil {
		return false
	}
	if ent, ok := plan.Entitlements["agency_branding"]; ok {
		if ent.Type == models.EntitlementTypeBool && ent.BoolValue {
			return true
		}
	}
	return false
}

// GetAgencyBranding handles GET /api/branding/agency.
// Returns the current agency branding for the authenticated tenant.
// Requires auth + tenant + agency_branding entitlement.
func (h *AgencyBrandingHandler) GetAgencyBranding(w http.ResponseWriter, r *http.Request) {
	tenant, ok := middleware.GetTenantFromContext(r.Context())
	if !ok {
		respondWithError(w, http.StatusBadRequest, "Tenant context required")
		return
	}

	if !h.hasAgencyBranding(r, tenant) {
		respondWithError(w, http.StatusPaymentRequired, "Agency branding requires an Agency plan. Please upgrade.")
		return
	}

	var branding AgencyBranding
	err := h.db.AgencyBrandings().FindOne(r.Context(), bson.M{"tenantId": tenant.ID}).Decode(&branding)
	if err == mongo.ErrNoDocuments {
		// Return empty defaults
		respondWithJSON(w, http.StatusOK, AgencyBranding{
			TenantID: tenant.ID,
		})
		return
	}
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to load agency branding")
		return
	}

	respondWithJSON(w, http.StatusOK, branding)
}

// UpdateAgencyBranding handles PUT /api/branding/agency.
// Updates agency branding for the authenticated tenant.
// Requires auth + tenant + agency_branding entitlement.
func (h *AgencyBrandingHandler) UpdateAgencyBranding(w http.ResponseWriter, r *http.Request) {
	tenant, ok := middleware.GetTenantFromContext(r.Context())
	if !ok {
		respondWithError(w, http.StatusBadRequest, "Tenant context required")
		return
	}

	if !h.hasAgencyBranding(r, tenant) {
		respondWithError(w, http.StatusPaymentRequired, "Agency branding requires an Agency plan. Please upgrade.")
		return
	}

	var req struct {
		Logo        string `json:"logo"`
		CompanyName string `json:"companyName"`
		AccentColor string `json:"accentColor"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	now := time.Now().UTC()
	update := bson.M{
		"$set": bson.M{
			"tenantId":    tenant.ID,
			"logo":        req.Logo,
			"companyName": req.CompanyName,
			"accentColor": req.AccentColor,
			"updatedAt":   now,
		},
	}

	opts := options.Update().SetUpsert(true)
	_, err := h.db.AgencyBrandings().UpdateOne(r.Context(), bson.M{"tenantId": tenant.ID}, update, opts)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to update agency branding")
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// GetScanReport handles GET /api/scan/{id}/report.
// Returns an HTML report for the scan. When ?branding=true and the tenant has agency
// branding configured, the report is customised with their logo and company name.
func (h *AgencyBrandingHandler) GetScanReport(w http.ResponseWriter, r *http.Request) {
	// Extract scan ID from path — this handler is mounted at /api/scan/{id}/report
	// so we need to parse the id from the URL path.
	path := r.URL.Path
	// Path is like /api/scan/<id>/report — extract <id>
	parts := strings.Split(strings.TrimSuffix(path, "/report"), "/")
	scanID := parts[len(parts)-1]
	if scanID == "" || scanID == "report" {
		respondWithError(w, http.StatusBadRequest, "scan ID is required")
		return
	}

	scan, err := h.scanner.GetScan(r.Context(), scanID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to retrieve scan")
		return
	}
	if scan == nil {
		respondWithError(w, http.StatusNotFound, "Scan not found")
		return
	}

	// Build a minimal HTML report from the scan data
	html := buildScanReportHTML(scan)

	// Apply agency branding if requested and tenant context is available
	applyBranding := r.URL.Query().Get("branding") == "true"
	if applyBranding {
		if tenant, ok := middleware.GetTenantFromContext(r.Context()); ok {
			var branding AgencyBranding
			if err := h.db.AgencyBrandings().FindOne(r.Context(), bson.M{"tenantId": tenant.ID}).Decode(&branding); err == nil {
				if branding.CompanyName != "" {
					html = strings.ReplaceAll(html, "MCPLens", escapeHTMLEntities(branding.CompanyName))
				}
				if branding.AccentColor != "" {
					html = strings.ReplaceAll(html, "#6366f1", branding.AccentColor)
				}
				if branding.Logo != "" {
					logoTag := fmt.Sprintf(`<img src="%s" alt="%s logo" style="height:32px;vertical-align:middle;margin-right:8px;">`,
						escapeHTMLAttr(branding.Logo), escapeHTMLAttr(branding.CompanyName))
					html = strings.Replace(html, `<span class="brand-name">`, logoTag+`<span class="brand-name">`, 1)
				}
			}
		}
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Write([]byte(html))
}

// buildScanReportHTML creates a minimal self-contained HTML report from a stored scan.
func buildScanReportHTML(scan *scanner.StoredScan) string {
	scoreColor := "#ef4444"
	if scan.CompositeScore >= 80 {
		scoreColor = "#22c55e"
	} else if scan.CompositeScore >= 50 {
		scoreColor = "#f59e0b"
	}

	categoriesHTML := ""
	for _, cat := range scan.Categories {
		catScore := int(cat.CappedScore)
		catColor := "#ef4444"
		if catScore >= 80 {
			catColor = "#22c55e"
		} else if catScore >= 50 {
			catColor = "#f59e0b"
		}
		testedStr := "Tested"
		if !cat.Tested {
			testedStr = "Not tested"
			catColor = "#94a3b8"
		}
		catLabel := strings.ReplaceAll(string(cat.Category), "-", " ")
		catLabel = strings.Title(catLabel)
		categoriesHTML += fmt.Sprintf(`
		<div class="category">
			<div class="cat-name">%s</div>
			<div class="cat-score" style="color:%s">%d</div>
			<div class="cat-status">%s</div>
		</div>`, escapeHTMLEntities(catLabel), catColor, catScore, testedStr)
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>MCPLens Report — %s</title>
<style>
  * { box-sizing: border-box; margin: 0; padding: 0; }
  body { font-family: ui-sans-serif, system-ui, sans-serif; background: #0f172a; color: #f1f5f9; padding: 2rem; }
  .header { display: flex; align-items: center; margin-bottom: 2rem; }
  .brand-name { font-size: 1.5rem; font-weight: 700; color: #6366f1; }
  .report-title { margin-left: auto; font-size: 0.875rem; color: #94a3b8; }
  .score-card { background: #1e293b; border-radius: 1rem; padding: 2rem; text-align: center; margin-bottom: 2rem; }
  .domain { font-size: 1.25rem; font-weight: 600; margin-bottom: 0.5rem; }
  .score { font-size: 5rem; font-weight: 800; color: %s; line-height: 1; }
  .score-label { font-size: 1rem; color: #94a3b8; margin-top: 0.5rem; }
  .categories { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 1rem; margin-bottom: 2rem; }
  .category { background: #1e293b; border-radius: 0.75rem; padding: 1.25rem; }
  .cat-name { font-size: 0.875rem; color: #94a3b8; margin-bottom: 0.5rem; text-transform: capitalize; }
  .cat-score { font-size: 2rem; font-weight: 700; }
  .cat-status { font-size: 0.75rem; color: #64748b; margin-top: 0.25rem; }
  .meta { color: #64748b; font-size: 0.8rem; margin-top: 1rem; text-align: center; }
</style>
</head>
<body>
<div class="header">
  <span class="brand-name">MCPLens</span>
  <span class="report-title">Agent Readiness Report</span>
</div>
<div class="score-card">
  <div class="domain">%s</div>
  <div class="score">%d</div>
  <div class="score-label">/ 100 — Agent Readiness Score</div>
</div>
<div class="categories">%s
</div>
<p class="meta">Scan ID: %s &bull; Scanned: %s</p>
</body>
</html>`,
		escapeHTMLEntities(scan.Domain),
		scoreColor,
		escapeHTMLEntities(scan.Domain),
		scan.CompositeScore,
		categoriesHTML,
		scan.ID.Hex(),
		scan.CreatedAt.Format("2006-01-02 15:04 UTC"),
	)
}

func escapeHTMLEntities(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	return s
}

func escapeHTMLAttr(s string) string {
	return strings.ReplaceAll(escapeHTMLEntities(s), "'", "&#39;")
}
