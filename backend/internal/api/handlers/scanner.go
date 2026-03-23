package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"lastsaas/internal/db"
	"lastsaas/internal/middleware"
	"lastsaas/internal/models"
	"lastsaas/internal/scanner"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ScannerHandler handles MCP store scanning endpoints.
type ScannerHandler struct {
	service *scanner.Service
	db      *db.MongoDB
}

// NewScannerHandler creates a new ScannerHandler.
func NewScannerHandler(svc *scanner.Service) *ScannerHandler {
	return &ScannerHandler{service: svc}
}

// SetDB attaches the MongoDB instance for entitlement checks.
func (h *ScannerHandler) SetDB(database *db.MongoDB) {
	h.db = database
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

// --- Tracked Stores ---

// getMaxTrackedStores returns the max_tracked_stores entitlement for the tenant's plan.
// Returns 0 if the tenant has no plan or the entitlement is not set.
func (h *ScannerHandler) getMaxTrackedStores(ctx context.Context, tenant *models.Tenant) int64 {
	if h.db == nil || tenant.PlanID == nil {
		return 0
	}
	var plan models.Plan
	if err := h.db.Plans().FindOne(ctx, bson.M{"_id": *tenant.PlanID}).Decode(&plan); err != nil {
		return 0
	}
	if ent, ok := plan.Entitlements["max_tracked_stores"]; ok && ent.Type == models.EntitlementTypeNumeric {
		return ent.NumericValue
	}
	return 0
}

// AddTrackedStore handles POST /api/tracked-stores.
// Body: {"domain": "example.com"}
// Requires auth + tenant. Enforces max_tracked_stores entitlement.
func (h *ScannerHandler) AddTrackedStore(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenant, ok := middleware.GetTenantFromContext(ctx)
	if !ok {
		respondWithError(w, http.StatusBadRequest, "Tenant context required")
		return
	}

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

	// Check entitlement: max_tracked_stores
	maxStores := h.getMaxTrackedStores(ctx, tenant)
	if maxStores == 0 {
		respondWithError(w, http.StatusPaymentRequired, "Your plan does not include store tracking. Please upgrade.")
		return
	}

	currentCount, err := h.service.TrackedStores().CountTrackedStores(ctx, tenant.ID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to count tracked stores")
		return
	}
	if currentCount >= maxStores {
		respondWithError(w, http.StatusPaymentRequired, "You have reached the maximum number of tracked stores for your plan. Please upgrade to track more stores.")
		return
	}

	store, err := h.service.TrackedStores().AddTrackedStore(ctx, tenant.ID, req.Domain)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondWithJSON(w, http.StatusCreated, store)
}

// RemoveTrackedStore handles DELETE /api/tracked-stores/{id}.
// Requires auth + tenant.
func (h *ScannerHandler) RemoveTrackedStore(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenant, ok := middleware.GetTenantFromContext(ctx)
	if !ok {
		respondWithError(w, http.StatusBadRequest, "Tenant context required")
		return
	}

	vars := mux.Vars(r)
	storeID, err := primitive.ObjectIDFromHex(vars["id"])
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid store ID")
		return
	}

	if err := h.service.TrackedStores().RemoveTrackedStore(ctx, tenant.ID, storeID); err != nil {
		if err.Error() == "tracked store not found" {
			respondWithError(w, http.StatusNotFound, "Tracked store not found")
			return
		}
		respondWithError(w, http.StatusInternalServerError, "Failed to remove tracked store")
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]string{"message": "Store removed successfully"})
}

// ListTrackedStores handles GET /api/tracked-stores.
// Requires auth + tenant.
func (h *ScannerHandler) ListTrackedStores(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenant, ok := middleware.GetTenantFromContext(ctx)
	if !ok {
		respondWithError(w, http.StatusBadRequest, "Tenant context required")
		return
	}

	stores, err := h.service.TrackedStores().ListTrackedStores(ctx, tenant.ID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to list tracked stores")
		return
	}

	// Include entitlement info so the frontend can show upgrade prompts
	maxStores := h.getMaxTrackedStores(ctx, tenant)

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"stores":    stores,
		"total":     len(stores),
		"maxStores": maxStores,
	})
}

// GetTrackedStoreHistory handles GET /api/tracked-stores/{id}/history.
// Returns scan history for the tracked store.
func (h *ScannerHandler) GetTrackedStoreHistory(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenant, ok := middleware.GetTenantFromContext(ctx)
	if !ok {
		respondWithError(w, http.StatusBadRequest, "Tenant context required")
		return
	}

	vars := mux.Vars(r)
	storeID, err := primitive.ObjectIDFromHex(vars["id"])
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid store ID")
		return
	}

	store, err := h.service.TrackedStores().GetTrackedStore(ctx, tenant.ID, storeID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to retrieve tracked store")
		return
	}
	if store == nil {
		respondWithError(w, http.StatusNotFound, "Tracked store not found")
		return
	}

	limitStr := r.URL.Query().Get("limit")
	limit, _ := strconv.Atoi(limitStr)

	history, err := h.service.TrackedStores().GetScanHistory(ctx, tenant.ID, store.Domain, limit)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to retrieve scan history")
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"store":   store,
		"history": history,
	})
}
