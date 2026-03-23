package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"lastsaas/internal/middleware"
	"lastsaas/internal/scanner"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ScannerHandler handles MCP store scanning endpoints.
type ScannerHandler struct {
	service *scanner.Service
}

// NewScannerHandler creates a new ScannerHandler.
func NewScannerHandler(svc *scanner.Service) *ScannerHandler {
	return &ScannerHandler{service: svc}
}

// TriggerScan handles POST /api/scan.
//
// Body: {"domain": "allbirds.com"}
// Public endpoint (no auth required). Rate-limited by the caller.
func (h *ScannerHandler) TriggerScan(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Domain string `json:"domain"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if req.Domain == "" {
		respondWithError(w, http.StatusBadRequest, "domain is required")
		return
	}

	ctx := r.Context()

	// Optionally attach tenant ID when the request is authenticated, for paid scan history.
	var tenantID *primitive.ObjectID
	if tenant, ok := middleware.GetTenantFromContext(ctx); ok {
		id := tenant.ID
		tenantID = &id
	}

	stored, err := h.service.ScanStore(ctx, req.Domain, tenantID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Scan failed: "+err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, stored)
}

// GetScan handles GET /api/scan/{id}.
func (h *ScannerHandler) GetScan(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	scanID := vars["id"]
	if scanID == "" {
		respondWithError(w, http.StatusBadRequest, "scan ID is required")
		return
	}

	scan, err := h.service.GetScan(r.Context(), scanID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to retrieve scan")
		return
	}
	if scan == nil {
		respondWithError(w, http.StatusNotFound, "Scan not found")
		return
	}

	respondWithJSON(w, http.StatusOK, scan)
}

// GetLatestDomainScan handles GET /api/scan/domain/{domain}.
func (h *ScannerHandler) GetLatestDomainScan(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	domain := vars["domain"]
	if domain == "" {
		respondWithError(w, http.StatusBadRequest, "domain is required")
		return
	}

	scan, err := h.service.GetLatestScan(r.Context(), domain)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to retrieve scan")
		return
	}
	if scan == nil {
		respondWithError(w, http.StatusNotFound, "No scan found for domain")
		return
	}

	respondWithJSON(w, http.StatusOK, scan)
}

// ListScans handles GET /api/scans.
// Requires auth + tenant context (paid feature).
func (h *ScannerHandler) ListScans(w http.ResponseWriter, r *http.Request) {
	tenant, ok := middleware.GetTenantFromContext(r.Context())
	if !ok {
		respondWithError(w, http.StatusBadRequest, "Tenant context required")
		return
	}

	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	limit, _ := strconv.Atoi(q.Get("limit"))
	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 20
	}

	scans, total, err := h.service.ListScans(r.Context(), tenant.ID, page, limit)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to list scans")
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"scans": scans,
		"total": total,
		"page":  page,
		"limit": limit,
	})
}
