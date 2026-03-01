package handlers

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"lastsaas/internal/auth"
	"lastsaas/internal/db"
	"lastsaas/internal/email"
	"lastsaas/internal/events"
	"lastsaas/internal/health"
	"lastsaas/internal/middleware"
	"lastsaas/internal/models"
	"lastsaas/internal/syslog"
	"lastsaas/internal/version"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/gorilla/mux"
)

type AdminHandler struct {
	db           *db.MongoDB
	events       events.Emitter
	syslog       *syslog.Logger
	health       *health.Service
	getConfig    func(string) string
	jwtService   *auth.JWTService
	emailService *email.ResendService
}

func NewAdminHandler(database *db.MongoDB, emitter events.Emitter, sysLogger *syslog.Logger) *AdminHandler {
	return &AdminHandler{
		db:     database,
		events: emitter,
		syslog: sysLogger,
	}
}

// isRootTenantOwner returns true if the given user is the owner of the root tenant.
func (h *AdminHandler) isRootTenantOwner(ctx context.Context, userID primitive.ObjectID) bool {
	count, _ := h.db.TenantMemberships().CountDocuments(ctx, bson.M{
		"userId": userID,
		"role":   models.RoleOwner,
	})
	if count == 0 {
		return false
	}
	// Verify the tenant is root
	var membership models.TenantMembership
	if err := h.db.TenantMemberships().FindOne(ctx, bson.M{"userId": userID, "role": models.RoleOwner}).Decode(&membership); err != nil {
		return false
	}
	var tenant models.Tenant
	if err := h.db.Tenants().FindOne(ctx, bson.M{"_id": membership.TenantID}).Decode(&tenant); err != nil {
		return false
	}
	return tenant.IsRoot
}

func (h *AdminHandler) SetHealthService(svc *health.Service, getConfig func(string) string) {
	h.health = svc
	h.getConfig = getConfig
}

func (h *AdminHandler) SetJWTService(svc *auth.JWTService) {
	h.jwtService = svc
}

func (h *AdminHandler) SetEmailService(svc *email.ResendService) {
	h.emailService = svc
}


type TenantListItem struct {
	ID                  string    `json:"id"`
	Name                string    `json:"name"`
	Slug                string    `json:"slug"`
	IsRoot              bool      `json:"isRoot"`
	IsActive            bool      `json:"isActive"`
	MemberCount         int       `json:"memberCount"`
	PlanName            string    `json:"planName"`
	BillingWaived       bool      `json:"billingWaived"`
	SubscriptionCredits int64     `json:"subscriptionCredits"`
	PurchasedCredits    int64     `json:"purchasedCredits"`
	BillingStatus       string    `json:"billingStatus"`
	CreatedAt           time.Time `json:"createdAt"`
}

type UserListItem struct {
	ID            string    `json:"id"`
	Email         string    `json:"email"`
	DisplayName   string    `json:"displayName"`
	EmailVerified bool      `json:"emailVerified"`
	IsActive      bool      `json:"isActive"`
	TenantCount   int       `json:"tenantCount"`
	CreatedAt     time.Time `json:"createdAt"`
	LastLoginAt   *time.Time `json:"lastLoginAt,omitempty"`
}

func (h *AdminHandler) ListTenants(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := r.URL.Query()

	// Pagination
	page, _ := strconv.Atoi(q.Get("page"))
	if page < 1 {
		page = 1
	}
	limit, _ := strconv.Atoi(q.Get("limit"))
	if limit < 1 || limit > 100 {
		limit = 25
	}
	skip := int64((page - 1) * limit)

	// Search filter
	filter := bson.M{}
	if search := strings.TrimSpace(q.Get("search")); search != "" {
		escaped := primitive.Regex{Pattern: "(?i)" + escapeRegexInput(search)}
		filter["$or"] = []bson.M{
			{"name": bson.M{"$regex": escaped.Pattern}},
			{"slug": bson.M{"$regex": escaped.Pattern}},
		}
	}

	// Status filter
	if status := q.Get("status"); status != "" {
		switch status {
		case "active":
			filter["isActive"] = true
		case "disabled":
			filter["isActive"] = false
		}
	}

	// Billing status filter
	if bs := q.Get("billingStatus"); bs != "" {
		switch models.BillingStatus(bs) {
		case models.BillingStatusNone, models.BillingStatusActive, models.BillingStatusPastDue, models.BillingStatusCanceled:
			filter["billingStatus"] = bs
		}
	}

	// Sort
	sortField := "createdAt"
	sortDir := -1
	switch q.Get("sort") {
	case "name":
		sortField = "name"
		sortDir = 1
	case "-name":
		sortField = "name"
		sortDir = -1
	case "createdAt":
		sortField = "createdAt"
		sortDir = 1
	case "-createdAt":
		sortField = "createdAt"
		sortDir = -1
	}

	// Total count for pagination
	total, err := h.db.Tenants().CountDocuments(ctx, filter)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to count tenants")
		return
	}

	// Fetch page
	opts := options.Find().
		SetSort(bson.D{{Key: sortField, Value: sortDir}}).
		SetSkip(skip).
		SetLimit(int64(limit))
	cursor, err := h.db.Tenants().Find(ctx, filter, opts)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to fetch tenants")
		return
	}
	defer cursor.Close(ctx)

	var tenants []models.Tenant
	if err := cursor.All(ctx, &tenants); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to decode tenants")
		return
	}

	// Batch member counts via aggregation
	tenantIDs := make([]primitive.ObjectID, len(tenants))
	for i, t := range tenants {
		tenantIDs[i] = t.ID
	}
	memberCounts := map[string]int{}
	if len(tenantIDs) > 0 {
		pipeline := bson.A{
			bson.M{"$match": bson.M{"tenantId": bson.M{"$in": tenantIDs}}},
			bson.M{"$group": bson.M{"_id": "$tenantId", "count": bson.M{"$sum": 1}}},
		}
		aggCursor, err := h.db.TenantMemberships().Aggregate(ctx, pipeline)
		if err == nil {
			defer aggCursor.Close(ctx)
			var results []struct {
				ID    primitive.ObjectID `bson:"_id"`
				Count int               `bson:"count"`
			}
			aggCursor.All(ctx, &results)
			for _, r := range results {
				memberCounts[r.ID.Hex()] = r.Count
			}
		}
	}

	// Build plan name lookup (bounded to 500 plans)
	planCursor, _ := h.db.Plans().Find(ctx, bson.M{}, options.Find().SetLimit(500))
	planNames := map[string]string{}
	var systemPlanName string
	if planCursor != nil {
		var plans []models.Plan
		planCursor.All(ctx, &plans)
		for _, p := range plans {
			planNames[p.ID.Hex()] = p.Name
			if p.IsSystem {
				systemPlanName = p.Name
			}
		}
	}
	if systemPlanName == "" {
		systemPlanName = "Free"
	}

	items := make([]TenantListItem, 0, len(tenants))
	for _, t := range tenants {
		pName := systemPlanName
		if t.PlanID != nil {
			if n, ok := planNames[t.PlanID.Hex()]; ok {
				pName = n
			}
		}
		items = append(items, TenantListItem{
			ID:                  t.ID.Hex(),
			Name:                t.Name,
			Slug:                t.Slug,
			IsRoot:              t.IsRoot,
			IsActive:            t.IsActive,
			MemberCount:         memberCounts[t.ID.Hex()],
			PlanName:            pName,
			BillingWaived:       t.BillingWaived,
			SubscriptionCredits: t.SubscriptionCredits,
			PurchasedCredits:    t.PurchasedCredits,
			BillingStatus:       string(t.BillingStatus),
			CreatedAt:           t.CreatedAt,
		})
	}

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"tenants": items,
		"total":   total,
		"page":    page,
		"limit":   limit,
	})
}

// ExportTenantsCSV streams all tenants matching filters as a CSV download.
func (h *AdminHandler) ExportTenantsCSV(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := r.URL.Query()

	filter := bson.M{}
	if search := strings.TrimSpace(q.Get("search")); search != "" {
		escaped := primitive.Regex{Pattern: "(?i)" + escapeRegexInput(search)}
		filter["$or"] = []bson.M{
			{"name": bson.M{"$regex": escaped.Pattern}},
			{"slug": bson.M{"$regex": escaped.Pattern}},
		}
	}
	if status := q.Get("status"); status != "" {
		switch status {
		case "active":
			filter["isActive"] = true
		case "disabled":
			filter["isActive"] = false
		}
	}
	if bs := q.Get("billingStatus"); bs != "" {
		switch models.BillingStatus(bs) {
		case models.BillingStatusNone, models.BillingStatusActive, models.BillingStatusPastDue, models.BillingStatusCanceled:
			filter["billingStatus"] = bs
		}
	}

	opts := options.Find().
		SetSort(bson.D{{Key: "createdAt", Value: -1}}).
		SetLimit(10000)

	cursor, err := h.db.Tenants().Find(ctx, filter, opts)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to query tenants")
		return
	}
	defer cursor.Close(ctx)

	var tenants []models.Tenant
	if err := cursor.All(ctx, &tenants); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to decode tenants")
		return
	}

	// Batch member counts
	tenantIDs := make([]primitive.ObjectID, len(tenants))
	for i, t := range tenants {
		tenantIDs[i] = t.ID
	}
	memberCounts := map[string]int{}
	if len(tenantIDs) > 0 {
		pipeline := bson.A{
			bson.M{"$match": bson.M{"tenantId": bson.M{"$in": tenantIDs}}},
			bson.M{"$group": bson.M{"_id": "$tenantId", "count": bson.M{"$sum": 1}}},
		}
		aggCursor, err := h.db.TenantMemberships().Aggregate(ctx, pipeline)
		if err == nil {
			defer aggCursor.Close(ctx)
			var results []struct {
				ID    primitive.ObjectID `bson:"_id"`
				Count int               `bson:"count"`
			}
			aggCursor.All(ctx, &results)
			for _, r := range results {
				memberCounts[r.ID.Hex()] = r.Count
			}
		}
	}

	// Batch plan names
	planCursor, _ := h.db.Plans().Find(ctx, bson.M{}, options.Find().SetLimit(500))
	planNames := map[string]string{}
	var systemPlanName string
	if planCursor != nil {
		var plans []models.Plan
		planCursor.All(ctx, &plans)
		planCursor.Close(ctx)
		for _, p := range plans {
			planNames[p.ID.Hex()] = p.Name
			if p.IsSystem {
				systemPlanName = p.Name
			}
		}
	}
	if systemPlanName == "" {
		systemPlanName = "Free"
	}

	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment; filename=tenants.csv")

	writer := csv.NewWriter(w)
	writer.Write([]string{"ID", "Name", "Slug", "IsRoot", "IsActive", "MemberCount", "PlanName", "BillingStatus", "Credits", "CreatedAt"})

	for _, t := range tenants {
		pName := systemPlanName
		if t.PlanID != nil {
			if n, ok := planNames[t.PlanID.Hex()]; ok {
				pName = n
			}
		}
		totalCredits := t.SubscriptionCredits + t.PurchasedCredits
		writer.Write([]string{
			t.ID.Hex(),
			sanitizeCSVField(t.Name),
			sanitizeCSVField(t.Slug),
			strconv.FormatBool(t.IsRoot),
			strconv.FormatBool(t.IsActive),
			strconv.Itoa(memberCounts[t.ID.Hex()]),
			sanitizeCSVField(pName),
			string(t.BillingStatus),
			strconv.FormatInt(totalCredits, 10),
			t.CreatedAt.Format(time.RFC3339),
		})
	}
	writer.Flush()
}

func (h *AdminHandler) GetTenant(w http.ResponseWriter, r *http.Request) {
	tenantIDStr := mux.Vars(r)["tenantId"]
	tenantID, err := primitive.ObjectIDFromHex(tenantIDStr)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid tenant ID")
		return
	}

	var tenant models.Tenant
	if err := h.db.Tenants().FindOne(r.Context(), bson.M{"_id": tenantID}).Decode(&tenant); err != nil {
		respondWithError(w, http.StatusNotFound, "Tenant not found")
		return
	}

	// Get members
	cursor, err := h.db.TenantMemberships().Find(r.Context(), bson.M{"tenantId": tenantID})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to fetch members")
		return
	}
	defer cursor.Close(r.Context())

	var memberships []models.TenantMembership
	cursor.All(r.Context(), &memberships)

	// Batch-fetch all member users in a single query
	userIDs := make([]primitive.ObjectID, len(memberships))
	for i, m := range memberships {
		userIDs[i] = m.UserID
	}
	userMap := map[primitive.ObjectID]models.User{}
	if len(userIDs) > 0 {
		userCursor, err := h.db.Users().Find(r.Context(), bson.M{"_id": bson.M{"$in": userIDs}})
		if err == nil {
			defer userCursor.Close(r.Context())
			var users []models.User
			if err := userCursor.All(r.Context(), &users); err == nil {
				for _, u := range users {
					userMap[u.ID] = u
				}
			}
		}
	}

	var members []MemberResponse
	for _, m := range memberships {
		user, ok := userMap[m.UserID]
		if !ok {
			continue
		}
		members = append(members, MemberResponse{
			UserID:      user.ID.Hex(),
			Email:       user.Email,
			DisplayName: user.DisplayName,
			Role:        m.Role,
			JoinedAt:    m.JoinedAt,
		})
	}

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"tenant":  tenant,
		"members": members,
	})
}

func (h *AdminHandler) UpdateTenantStatus(w http.ResponseWriter, r *http.Request) {
	tenantIDStr := mux.Vars(r)["tenantId"]
	tenantID, err := primitive.ObjectIDFromHex(tenantIDStr)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid tenant ID")
		return
	}

	var tenant models.Tenant
	if err := h.db.Tenants().FindOne(r.Context(), bson.M{"_id": tenantID}).Decode(&tenant); err != nil {
		respondWithError(w, http.StatusNotFound, "Tenant not found")
		return
	}

	// Cannot deactivate the root tenant
	if tenant.IsRoot {
		respondWithError(w, http.StatusForbidden, "Cannot modify the root tenant status")
		return
	}

	var req struct {
		IsActive bool `json:"isActive"`
	}
	if err := decodeJSON(r, &req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	h.db.Tenants().UpdateOne(r.Context(),
		bson.M{"_id": tenantID},
		bson.M{"$set": bson.M{"isActive": req.IsActive, "updatedAt": time.Now()}},
	)

	if !req.IsActive {
		h.events.Emit(events.Event{
			Type:      events.EventTenantDeactivated,
			Timestamp: time.Now(),
			Data: map[string]interface{}{
				"tenantId":   tenantID.Hex(),
				"tenantName": tenant.Name,
			},
		})
	}

	respondWithJSON(w, http.StatusOK, map[string]string{"message": "Tenant status updated"})
}

func (h *AdminHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := r.URL.Query()

	// Pagination
	page, _ := strconv.Atoi(q.Get("page"))
	if page < 1 {
		page = 1
	}
	limit, _ := strconv.Atoi(q.Get("limit"))
	if limit < 1 || limit > 100 {
		limit = 25
	}
	skip := int64((page - 1) * limit)

	// Search filter
	filter := bson.M{}
	if search := strings.TrimSpace(q.Get("search")); search != "" {
		escaped := escapeRegexInput(search)
		filter["$or"] = []bson.M{
			{"email": bson.M{"$regex": "(?i)" + escaped}},
			{"displayName": bson.M{"$regex": "(?i)" + escaped}},
		}
	}

	// Status filter
	if status := q.Get("status"); status != "" {
		switch status {
		case "active":
			filter["isActive"] = true
		case "disabled":
			filter["isActive"] = false
		}
	}

	// Sort
	sortField := "createdAt"
	sortDir := -1
	switch q.Get("sort") {
	case "email":
		sortField = "email"
		sortDir = 1
	case "-email":
		sortField = "email"
		sortDir = -1
	case "displayName":
		sortField = "displayName"
		sortDir = 1
	case "-displayName":
		sortField = "displayName"
		sortDir = -1
	case "createdAt":
		sortField = "createdAt"
		sortDir = 1
	case "-createdAt":
		sortField = "createdAt"
		sortDir = -1
	}

	// Total count for pagination
	total, err := h.db.Users().CountDocuments(ctx, filter)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to count users")
		return
	}

	// Fetch page
	opts := options.Find().
		SetSort(bson.D{{Key: sortField, Value: sortDir}}).
		SetSkip(skip).
		SetLimit(int64(limit))
	cursor, err := h.db.Users().Find(ctx, filter, opts)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to fetch users")
		return
	}
	defer cursor.Close(ctx)

	var users []models.User
	if err := cursor.All(ctx, &users); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to decode users")
		return
	}

	// Batch tenant counts via aggregation
	userIDs := make([]primitive.ObjectID, len(users))
	for i, u := range users {
		userIDs[i] = u.ID
	}
	tenantCounts := map[string]int{}
	if len(userIDs) > 0 {
		pipeline := bson.A{
			bson.M{"$match": bson.M{"userId": bson.M{"$in": userIDs}}},
			bson.M{"$group": bson.M{"_id": "$userId", "count": bson.M{"$sum": 1}}},
		}
		aggCursor, err := h.db.TenantMemberships().Aggregate(ctx, pipeline)
		if err == nil {
			defer aggCursor.Close(ctx)
			var results []struct {
				ID    primitive.ObjectID `bson:"_id"`
				Count int               `bson:"count"`
			}
			aggCursor.All(ctx, &results)
			for _, r := range results {
				tenantCounts[r.ID.Hex()] = r.Count
			}
		}
	}

	items := make([]UserListItem, 0, len(users))
	for _, u := range users {
		items = append(items, UserListItem{
			ID:            u.ID.Hex(),
			Email:         u.Email,
			DisplayName:   u.DisplayName,
			EmailVerified: u.EmailVerified,
			IsActive:      u.IsActive,
			TenantCount:   tenantCounts[u.ID.Hex()],
			CreatedAt:     u.CreatedAt,
			LastLoginAt:   u.LastLoginAt,
		})
	}

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"users": items,
		"total": total,
		"page":  page,
		"limit": limit,
	})
}

// ExportUsersCSV streams all users matching filters as a CSV download.
func (h *AdminHandler) ExportUsersCSV(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := r.URL.Query()

	filter := bson.M{}
	if search := strings.TrimSpace(q.Get("search")); search != "" {
		escaped := escapeRegexInput(search)
		filter["$or"] = []bson.M{
			{"email": bson.M{"$regex": "(?i)" + escaped}},
			{"displayName": bson.M{"$regex": "(?i)" + escaped}},
		}
	}
	if status := q.Get("status"); status != "" {
		switch status {
		case "active":
			filter["isActive"] = true
		case "disabled":
			filter["isActive"] = false
		}
	}

	opts := options.Find().
		SetSort(bson.D{{Key: "createdAt", Value: -1}}).
		SetLimit(10000)

	cursor, err := h.db.Users().Find(ctx, filter, opts)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to query users")
		return
	}
	defer cursor.Close(ctx)

	var users []models.User
	if err := cursor.All(ctx, &users); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to decode users")
		return
	}

	// Batch tenant counts
	userIDs := make([]primitive.ObjectID, len(users))
	for i, u := range users {
		userIDs[i] = u.ID
	}
	tenantCounts := map[string]int{}
	if len(userIDs) > 0 {
		pipeline := bson.A{
			bson.M{"$match": bson.M{"userId": bson.M{"$in": userIDs}}},
			bson.M{"$group": bson.M{"_id": "$userId", "count": bson.M{"$sum": 1}}},
		}
		aggCursor, err := h.db.TenantMemberships().Aggregate(ctx, pipeline)
		if err == nil {
			defer aggCursor.Close(ctx)
			var results []struct {
				ID    primitive.ObjectID `bson:"_id"`
				Count int               `bson:"count"`
			}
			aggCursor.All(ctx, &results)
			for _, r := range results {
				tenantCounts[r.ID.Hex()] = r.Count
			}
		}
	}

	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment; filename=users.csv")

	writer := csv.NewWriter(w)
	writer.Write([]string{"ID", "Email", "DisplayName", "EmailVerified", "IsActive", "TenantCount", "CreatedAt", "LastLoginAt"})

	for _, u := range users {
		lastLogin := ""
		if u.LastLoginAt != nil {
			lastLogin = u.LastLoginAt.Format(time.RFC3339)
		}
		writer.Write([]string{
			u.ID.Hex(),
			sanitizeCSVField(u.Email),
			sanitizeCSVField(u.DisplayName),
			strconv.FormatBool(u.EmailVerified),
			strconv.FormatBool(u.IsActive),
			strconv.Itoa(tenantCounts[u.ID.Hex()]),
			u.CreatedAt.Format(time.RFC3339),
			lastLogin,
		})
	}
	writer.Flush()
}

func (h *AdminHandler) UpdateUserStatus(w http.ResponseWriter, r *http.Request) {
	userIDStr := mux.Vars(r)["userId"]
	userID, err := primitive.ObjectIDFromHex(userIDStr)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	// Non-owners cannot deactivate the root tenant owner
	actingMembership, _ := middleware.GetMembershipFromContext(r.Context())
	if actingMembership.Role != models.RoleOwner && h.isRootTenantOwner(r.Context(), userID) {
		respondWithError(w, http.StatusForbidden, "Cannot modify the root tenant owner")
		return
	}

	var req struct {
		IsActive bool `json:"isActive"`
	}
	if err := decodeJSON(r, &req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	result, err := h.db.Users().UpdateOne(r.Context(),
		bson.M{"_id": userID},
		bson.M{"$set": bson.M{"isActive": req.IsActive, "updatedAt": time.Now()}},
	)
	if err != nil || result.MatchedCount == 0 {
		respondWithError(w, http.StatusNotFound, "User not found")
		return
	}

	actingUser, _ := middleware.GetUserFromContext(r.Context())
	action := "activated"
	if !req.IsActive {
		action = "deactivated"
	}
	h.syslog.HighWithUser(r.Context(), fmt.Sprintf("User %s: %s (admin action)", action, userID.Hex()), actingUser.ID)

	if !req.IsActive {
		h.events.Emit(events.Event{
			Type:      events.EventUserDeactivated,
			Timestamp: time.Now(),
			Data: map[string]interface{}{
				"userId": userID.Hex(),
			},
		})
	}

	respondWithJSON(w, http.StatusOK, map[string]string{"message": "User status updated"})
}

func (h *AdminHandler) GetAbout(w http.ResponseWriter, r *http.Request) {
	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"version":   version.Current,
		"copyright": "\u00a92026 Metavert LLC, licensed under the MIT License",
	})
}

func (h *AdminHandler) GetDashboard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userCount, _ := h.db.Users().CountDocuments(ctx, bson.M{})
	tenantCount, _ := h.db.Tenants().CountDocuments(ctx, bson.M{})

	// Health status
	healthy := true
	var issues []string

	if h.health != nil && h.getConfig != nil {
		metrics, err := h.health.GetCurrentMetrics(ctx)
		if err == nil && len(metrics) > 0 {
			cpuWarn, _ := strconv.ParseFloat(h.getConfig("health.cpu.warning_threshold"), 64)
			cpuCrit, _ := strconv.ParseFloat(h.getConfig("health.cpu.critical_threshold"), 64)
			memWarn, _ := strconv.ParseFloat(h.getConfig("health.memory.warning_threshold"), 64)
			memCrit, _ := strconv.ParseFloat(h.getConfig("health.memory.critical_threshold"), 64)
			diskWarn, _ := strconv.ParseFloat(h.getConfig("health.disk.warning_threshold"), 64)
			diskCrit, _ := strconv.ParseFloat(h.getConfig("health.disk.critical_threshold"), 64)

			for _, m := range metrics {
				node := m.NodeID
				if m.CPU.UsagePercent >= cpuCrit {
					healthy = false
					issues = append(issues, fmt.Sprintf("CPU critical on %s: %.1f%%", node, m.CPU.UsagePercent))
				} else if m.CPU.UsagePercent >= cpuWarn {
					issues = append(issues, fmt.Sprintf("CPU warning on %s: %.1f%%", node, m.CPU.UsagePercent))
				}
				if m.Memory.UsedPercent >= memCrit {
					healthy = false
					issues = append(issues, fmt.Sprintf("Memory critical on %s: %.1f%%", node, m.Memory.UsedPercent))
				} else if m.Memory.UsedPercent >= memWarn {
					issues = append(issues, fmt.Sprintf("Memory warning on %s: %.1f%%", node, m.Memory.UsedPercent))
				}
				if m.Disk.UsedPercent >= diskCrit {
					healthy = false
					issues = append(issues, fmt.Sprintf("Disk critical on %s: %.1f%%", node, m.Disk.UsedPercent))
				} else if m.Disk.UsedPercent >= diskWarn {
					issues = append(issues, fmt.Sprintf("Disk warning on %s: %.1f%%", node, m.Disk.UsedPercent))
				}
			}
		}
	}

	// Check integration health
	if h.health != nil {
		intHealthy, intIssues := h.health.IntegrationsHealthy()
		if !intHealthy {
			healthy = false
			issues = append(issues, intIssues...)
		}
	}

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"users":   userCount,
		"tenants": tenantCount,
		"health": map[string]interface{}{
			"healthy": healthy,
			"issues":  issues,
		},
	})
}

func decodeJSON(r *http.Request, v interface{}) error {
	return json.NewDecoder(r.Body).Decode(v)
}

// --- User Profile Handlers ---

type UserDetail struct {
	ID            string              `json:"id"`
	Email         string              `json:"email"`
	DisplayName   string              `json:"displayName"`
	EmailVerified bool                `json:"emailVerified"`
	IsActive      bool                `json:"isActive"`
	AuthMethods   []models.AuthMethod `json:"authMethods"`
	CreatedAt     time.Time           `json:"createdAt"`
	UpdatedAt     time.Time           `json:"updatedAt"`
	LastLoginAt   *time.Time          `json:"lastLoginAt,omitempty"`
}

type UserMembershipDetail struct {
	TenantID            string            `json:"tenantId"`
	TenantName          string            `json:"tenantName"`
	TenantSlug          string            `json:"tenantSlug"`
	IsRoot              bool              `json:"isRoot"`
	Role                models.MemberRole `json:"role"`
	JoinedAt            time.Time         `json:"joinedAt"`
	PlanID              string            `json:"planId"`
	PlanName            string            `json:"planName"`
	BillingWaived       bool              `json:"billingWaived"`
	SubscriptionCredits int64             `json:"subscriptionCredits"`
	PurchasedCredits    int64             `json:"purchasedCredits"`
}

func (h *AdminHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	userID, err := primitive.ObjectIDFromHex(mux.Vars(r)["userId"])
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	var user models.User
	if err := h.db.Users().FindOne(r.Context(), bson.M{"_id": userID}).Decode(&user); err != nil {
		respondWithError(w, http.StatusNotFound, "User not found")
		return
	}

	cursor, err := h.db.TenantMemberships().Find(r.Context(), bson.M{"userId": userID})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to fetch memberships")
		return
	}
	defer cursor.Close(r.Context())

	var memberships []models.TenantMembership
	cursor.All(r.Context(), &memberships)

	// Build plan name lookup for membership details (bounded to 500 plans)
	planCursor, _ := h.db.Plans().Find(r.Context(), bson.M{}, options.Find().SetLimit(500))
	planNameMap := map[string]string{}
	planIDMap := map[string]string{} // planOID hex -> planOID hex (for convenience)
	var systemPlanName string
	var systemPlanID string
	if planCursor != nil {
		var allPlans []models.Plan
		planCursor.All(r.Context(), &allPlans)
		for _, p := range allPlans {
			planNameMap[p.ID.Hex()] = p.Name
			planIDMap[p.ID.Hex()] = p.ID.Hex()
			if p.IsSystem {
				systemPlanName = p.Name
				systemPlanID = p.ID.Hex()
			}
		}
	}
	if systemPlanName == "" {
		systemPlanName = "Free"
	}

	// Batch-fetch all tenants in a single query
	tenantIDs := make([]primitive.ObjectID, len(memberships))
	for i, m := range memberships {
		tenantIDs[i] = m.TenantID
	}
	tenantMap := map[primitive.ObjectID]models.Tenant{}
	if len(tenantIDs) > 0 {
		tenantCursor, err := h.db.Tenants().Find(r.Context(), bson.M{"_id": bson.M{"$in": tenantIDs}})
		if err == nil {
			defer tenantCursor.Close(r.Context())
			var tenants []models.Tenant
			if err := tenantCursor.All(r.Context(), &tenants); err == nil {
				for _, t := range tenants {
					tenantMap[t.ID] = t
				}
			}
		}
	}

	var memberDetails []UserMembershipDetail
	for _, m := range memberships {
		tenant, ok := tenantMap[m.TenantID]
		if !ok {
			continue
		}
		pName := systemPlanName
		pID := systemPlanID
		if tenant.PlanID != nil {
			if n, ok := planNameMap[tenant.PlanID.Hex()]; ok {
				pName = n
				pID = tenant.PlanID.Hex()
			}
		}
		memberDetails = append(memberDetails, UserMembershipDetail{
			TenantID:            tenant.ID.Hex(),
			TenantName:          tenant.Name,
			TenantSlug:          tenant.Slug,
			IsRoot:              tenant.IsRoot,
			Role:                m.Role,
			JoinedAt:            m.JoinedAt,
			PlanID:              pID,
			PlanName:            pName,
			BillingWaived:       tenant.BillingWaived,
			SubscriptionCredits: tenant.SubscriptionCredits,
			PurchasedCredits:    tenant.PurchasedCredits,
		})
	}

	detail := UserDetail{
		ID:            user.ID.Hex(),
		Email:         user.Email,
		DisplayName:   user.DisplayName,
		EmailVerified: user.EmailVerified,
		IsActive:      user.IsActive,
		AuthMethods:   user.AuthMethods,
		CreatedAt:     user.CreatedAt,
		UpdatedAt:     user.UpdatedAt,
		LastLoginAt:   user.LastLoginAt,
	}

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"user":        detail,
		"memberships": memberDetails,
	})
}

func (h *AdminHandler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	userID, err := primitive.ObjectIDFromHex(mux.Vars(r)["userId"])
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	// Non-owners cannot modify the root tenant owner
	actingMembership, _ := middleware.GetMembershipFromContext(r.Context())
	if actingMembership.Role != models.RoleOwner && h.isRootTenantOwner(r.Context(), userID) {
		respondWithError(w, http.StatusForbidden, "Cannot modify the root tenant owner")
		return
	}

	var req struct {
		Email       *string `json:"email"`
		DisplayName *string `json:"displayName"`
	}
	if err := decodeJSON(r, &req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	var user models.User
	if err := h.db.Users().FindOne(r.Context(), bson.M{"_id": userID}).Decode(&user); err != nil {
		respondWithError(w, http.StatusNotFound, "User not found")
		return
	}

	actingUser, _ := middleware.GetUserFromContext(r.Context())
	updates := bson.M{"updatedAt": time.Now()}

	if req.Email != nil {
		newEmail := strings.TrimSpace(strings.ToLower(*req.Email))
		if newEmail == "" {
			respondWithError(w, http.StatusBadRequest, "Email cannot be empty")
			return
		}
		if newEmail != user.Email {
			count, _ := h.db.Users().CountDocuments(r.Context(), bson.M{"email": newEmail, "_id": bson.M{"$ne": userID}})
			if count > 0 {
				respondWithError(w, http.StatusConflict, "Email already in use by another account")
				return
			}
			updates["email"] = newEmail
			h.syslog.HighWithUser(r.Context(),
				fmt.Sprintf("User email changed: %s -> %s (user %s, admin action)", user.Email, newEmail, userID.Hex()),
				actingUser.ID)
		}
	}

	if req.DisplayName != nil {
		name := strings.TrimSpace(*req.DisplayName)
		if name != "" {
			updates["displayName"] = name
		}
	}

	h.db.Users().UpdateOne(r.Context(), bson.M{"_id": userID}, bson.M{"$set": updates})
	respondWithJSON(w, http.StatusOK, map[string]string{"message": "User updated"})
}

func (h *AdminHandler) UpdateUserRole(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userID, err := primitive.ObjectIDFromHex(vars["userId"])
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}
	tenantID, err := primitive.ObjectIDFromHex(vars["tenantId"])
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid tenant ID")
		return
	}

	// Non-owners cannot change the root tenant owner's role
	actingMembership, _ := middleware.GetMembershipFromContext(r.Context())
	if actingMembership.Role != models.RoleOwner && h.isRootTenantOwner(r.Context(), userID) {
		respondWithError(w, http.StatusForbidden, "Cannot modify the root tenant owner's role")
		return
	}

	var req struct {
		Role models.MemberRole `json:"role"`
	}
	if err := decodeJSON(r, &req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if !models.ValidRole(req.Role) {
		respondWithError(w, http.StatusBadRequest, "Invalid role")
		return
	}

	var tenant models.Tenant
	if err := h.db.Tenants().FindOne(r.Context(), bson.M{"_id": tenantID}).Decode(&tenant); err != nil {
		respondWithError(w, http.StatusNotFound, "Tenant not found")
		return
	}

	// Block root tenant ownership transfer via API
	if tenant.IsRoot && req.Role == models.RoleOwner {
		respondWithError(w, http.StatusForbidden, "Root tenant ownership can only be transferred via the CLI command: lastsaas transfer-root-owner")
		return
	}

	ctx := r.Context()
	now := time.Now()

	// If promoting to owner, demote current owner to admin
	if req.Role == models.RoleOwner {
		var currentOwner models.TenantMembership
		if err := h.db.TenantMemberships().FindOne(ctx, bson.M{
			"tenantId": tenantID,
			"role":     models.RoleOwner,
		}).Decode(&currentOwner); err == nil {
			h.db.TenantMemberships().UpdateOne(ctx,
				bson.M{"_id": currentOwner.ID},
				bson.M{"$set": bson.M{"role": models.RoleAdmin, "updatedAt": now}},
			)
		}
	}

	result, err := h.db.TenantMemberships().UpdateOne(ctx,
		bson.M{"userId": userID, "tenantId": tenantID},
		bson.M{"$set": bson.M{"role": req.Role, "updatedAt": now}},
	)
	if err != nil || result.MatchedCount == 0 {
		respondWithError(w, http.StatusNotFound, "Membership not found")
		return
	}

	actingUser, _ := middleware.GetUserFromContext(ctx)
	h.syslog.HighWithUser(ctx,
		fmt.Sprintf("User role changed: user %s in tenant %s (%s) -> %s (admin action)", userID.Hex(), tenant.Name, tenantID.Hex(), req.Role),
		actingUser.ID)

	respondWithJSON(w, http.StatusOK, map[string]string{"message": "Role updated"})
}

type tenantDeletionInfo struct {
	TenantID     string           `json:"tenantId"`
	TenantName   string           `json:"tenantName"`
	IsRoot       bool             `json:"isRoot"`
	OtherMembers []MemberResponse `json:"otherMembers"`
}

func (h *AdminHandler) PreflightDeleteUser(w http.ResponseWriter, r *http.Request) {
	userID, err := primitive.ObjectIDFromHex(mux.Vars(r)["userId"])
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	ctx := r.Context()
	actingUser, _ := middleware.GetUserFromContext(ctx)

	if actingUser.ID == userID {
		respondWithJSON(w, http.StatusOK, map[string]interface{}{
			"canDelete": false,
			"reason":    "Cannot delete your own account",
		})
		return
	}

	cursor, err := h.db.TenantMemberships().Find(ctx, bson.M{"userId": userID, "role": models.RoleOwner})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to query memberships")
		return
	}
	var ownerships []models.TenantMembership
	if err := cursor.All(ctx, &ownerships); err != nil {
		cursor.Close(ctx)
		respondWithError(w, http.StatusInternalServerError, "Failed to read memberships")
		return
	}
	cursor.Close(ctx)

	// Batch-fetch tenants and members to avoid N+1 queries.
	tenantIDs := make([]primitive.ObjectID, len(ownerships))
	for i, m := range ownerships {
		tenantIDs[i] = m.TenantID
	}

	tenantMap := map[primitive.ObjectID]models.Tenant{}
	if len(tenantIDs) > 0 {
		tCursor, err := h.db.Tenants().Find(ctx, bson.M{"_id": bson.M{"$in": tenantIDs}})
		if err == nil {
			var tenants []models.Tenant
			if err := tCursor.All(ctx, &tenants); err == nil {
				for _, t := range tenants {
					tenantMap[t.ID] = t
				}
			}
			tCursor.Close(ctx)
		}
	}

	var ownershipInfo []tenantDeletionInfo
	for _, m := range ownerships {
		tenant := tenantMap[m.TenantID]

		memberCursor, err := h.db.TenantMemberships().Find(ctx, bson.M{
			"tenantId": m.TenantID,
			"userId":   bson.M{"$ne": userID},
		})
		if err != nil {
			continue
		}
		var otherMemberships []models.TenantMembership
		memberCursor.All(ctx, &otherMemberships)
		memberCursor.Close(ctx)

		// Batch-fetch users for this tenant's other members.
		otherUserIDs := make([]primitive.ObjectID, len(otherMemberships))
		for i, om := range otherMemberships {
			otherUserIDs[i] = om.UserID
		}
		userMap := map[primitive.ObjectID]models.User{}
		if len(otherUserIDs) > 0 {
			uCursor, err := h.db.Users().Find(ctx, bson.M{"_id": bson.M{"$in": otherUserIDs}})
			if err == nil {
				var users []models.User
				if err := uCursor.All(ctx, &users); err == nil {
					for _, u := range users {
						userMap[u.ID] = u
					}
				}
				uCursor.Close(ctx)
			}
		}

		var otherMembers []MemberResponse
		for _, om := range otherMemberships {
			if u, ok := userMap[om.UserID]; ok {
				otherMembers = append(otherMembers, MemberResponse{
					UserID:      u.ID.Hex(),
					Email:       u.Email,
					DisplayName: u.DisplayName,
					Role:        om.Role,
					JoinedAt:    om.JoinedAt,
				})
			}
		}

		ownershipInfo = append(ownershipInfo, tenantDeletionInfo{
			TenantID:     tenant.ID.Hex(),
			TenantName:   tenant.Name,
			IsRoot:       tenant.IsRoot,
			OtherMembers: otherMembers,
		})
	}

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"canDelete":  true,
		"ownerships": ownershipInfo,
	})
}

func (h *AdminHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	userID, err := primitive.ObjectIDFromHex(mux.Vars(r)["userId"])
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	actingUser, _ := middleware.GetUserFromContext(r.Context())
	if actingUser.ID == userID {
		respondWithError(w, http.StatusForbidden, "Cannot delete your own account")
		return
	}

	var user models.User
	if err := h.db.Users().FindOne(r.Context(), bson.M{"_id": userID}).Decode(&user); err != nil {
		respondWithError(w, http.StatusNotFound, "User not found")
		return
	}

	var req struct {
		ReplacementOwners      map[string]string `json:"replacementOwners"`
		ConfirmTenantDeletions []string          `json:"confirmTenantDeletions"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.ReplacementOwners == nil {
		req.ReplacementOwners = map[string]string{}
	}

	ctx := r.Context()

	// Find all memberships
	cursor, err := h.db.TenantMemberships().Find(ctx, bson.M{"userId": userID})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to query memberships")
		return
	}
	var memberships []models.TenantMembership
	if err := cursor.All(ctx, &memberships); err != nil {
		cursor.Close(ctx)
		respondWithError(w, http.StatusInternalServerError, "Failed to read memberships")
		return
	}
	cursor.Close(ctx)

	// Handle owner memberships
	for _, m := range memberships {
		if m.Role != models.RoleOwner {
			continue
		}

		var tenant models.Tenant
		if err := h.db.Tenants().FindOne(ctx, bson.M{"_id": m.TenantID}).Decode(&tenant); err != nil {
			respondWithError(w, http.StatusInternalServerError, "Failed to look up tenant")
			return
		}

		if tenant.IsRoot {
			respondWithError(w, http.StatusForbidden, "Cannot delete the root tenant owner. Transfer ownership first via CLI.")
			return
		}

		if replacementStr, ok := req.ReplacementOwners[m.TenantID.Hex()]; ok {
			replacementID, err := primitive.ObjectIDFromHex(replacementStr)
			if err != nil {
				respondWithError(w, http.StatusBadRequest, "Invalid replacement owner ID")
				return
			}
			result, _ := h.db.TenantMemberships().UpdateOne(ctx,
				bson.M{"userId": replacementID, "tenantId": m.TenantID},
				bson.M{"$set": bson.M{"role": models.RoleOwner, "updatedAt": time.Now()}},
			)
			if result.MatchedCount == 0 {
				respondWithError(w, http.StatusBadRequest, fmt.Sprintf("Replacement owner is not a member of tenant '%s'", tenant.Name))
				return
			}
			h.syslog.HighWithUser(ctx,
				fmt.Sprintf("Tenant '%s' ownership transferred to %s (prior owner %s being deleted)", tenant.Name, replacementStr, user.Email),
				actingUser.ID)
		} else {
			otherCount, _ := h.db.TenantMemberships().CountDocuments(ctx, bson.M{
				"tenantId": m.TenantID,
				"userId":   bson.M{"$ne": userID},
			})
			if otherCount > 0 {
				respondWithError(w, http.StatusBadRequest, fmt.Sprintf("User is owner of tenant '%s' which has other members. Provide a replacement owner.", tenant.Name))
				return
			}
			// No other members — confirm tenant deletion
			confirmed := false
			for _, tid := range req.ConfirmTenantDeletions {
				if tid == m.TenantID.Hex() {
					confirmed = true
					break
				}
			}
			if !confirmed {
				respondWithError(w, http.StatusBadRequest, fmt.Sprintf("User is the sole member of tenant '%s'. Confirm tenant deletion.", tenant.Name))
				return
			}
			h.db.TenantMemberships().DeleteMany(ctx, bson.M{"tenantId": m.TenantID})
			h.db.Tenants().DeleteOne(ctx, bson.M{"_id": m.TenantID})
			h.db.Invitations().DeleteMany(ctx, bson.M{"tenantId": m.TenantID})
			h.syslog.HighWithUser(ctx,
				fmt.Sprintf("Tenant '%s' deleted (sole member %s was deleted)", tenant.Name, user.Email),
				actingUser.ID)
			h.events.Emit(events.Event{
				Type:      events.EventTenantDeleted,
				Timestamp: time.Now(),
				Data: map[string]interface{}{
					"tenantId":   m.TenantID.Hex(),
					"tenantName": tenant.Name,
					"reason":     "owner_deleted",
				},
			})
		}
	}

	// Delete user's data
	h.db.TenantMemberships().DeleteMany(ctx, bson.M{"userId": userID})
	h.db.RefreshTokens().DeleteMany(ctx, bson.M{"userId": userID})
	h.db.Messages().DeleteMany(ctx, bson.M{"userId": userID})
	h.db.Users().DeleteOne(ctx, bson.M{"_id": userID})

	h.syslog.HighWithUser(ctx,
		fmt.Sprintf("User deleted: %s (%s) (admin action)", user.Email, userID.Hex()),
		actingUser.ID)

	h.events.Emit(events.Event{
		Type:      events.EventUserDeleted,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"userId": userID.Hex(),
			"email":  user.Email,
		},
	})

	respondWithJSON(w, http.StatusOK, map[string]string{"message": "User deleted"})
}

func (h *AdminHandler) UpdateTenant(w http.ResponseWriter, r *http.Request) {
	tenantID, err := primitive.ObjectIDFromHex(mux.Vars(r)["tenantId"])
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid tenant ID")
		return
	}

	var req struct {
		Name                *string `json:"name"`
		BillingWaived       *bool   `json:"billingWaived"`
		SubscriptionCredits *int64  `json:"subscriptionCredits"`
		PurchasedCredits    *int64  `json:"purchasedCredits"`
	}
	if err := decodeJSON(r, &req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	var tenant models.Tenant
	if err := h.db.Tenants().FindOne(r.Context(), bson.M{"_id": tenantID}).Decode(&tenant); err != nil {
		respondWithError(w, http.StatusNotFound, "Tenant not found")
		return
	}

	actingUser, _ := middleware.GetUserFromContext(r.Context())
	updates := bson.M{"updatedAt": time.Now()}

	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			respondWithError(w, http.StatusBadRequest, "Tenant name cannot be empty")
			return
		}
		if name != tenant.Name {
			updates["name"] = name
			h.syslog.LogWithUser(r.Context(), models.LogMedium,
				fmt.Sprintf("Tenant name changed: '%s' -> '%s' (%s, admin action)", tenant.Name, name, tenantID.Hex()),
				actingUser.ID)
		}
	}

	if req.BillingWaived != nil && *req.BillingWaived != tenant.BillingWaived {
		updates["billingWaived"] = *req.BillingWaived
	}

	creditChanged := false
	oldSub := tenant.SubscriptionCredits
	oldPurch := tenant.PurchasedCredits

	if req.SubscriptionCredits != nil && *req.SubscriptionCredits != tenant.SubscriptionCredits {
		updates["subscriptionCredits"] = *req.SubscriptionCredits
		creditChanged = true
	}
	if req.PurchasedCredits != nil && *req.PurchasedCredits != tenant.PurchasedCredits {
		updates["purchasedCredits"] = *req.PurchasedCredits
		creditChanged = true
	}

	if creditChanged {
		newSub := oldSub
		if req.SubscriptionCredits != nil {
			newSub = *req.SubscriptionCredits
		}
		newPurch := oldPurch
		if req.PurchasedCredits != nil {
			newPurch = *req.PurchasedCredits
		}
		h.syslog.HighWithUser(r.Context(),
			fmt.Sprintf("Tenant credits adjusted for '%s' (%s): subscription %d -> %d, purchased %d -> %d (admin action)",
				tenant.Name, tenantID.Hex(), oldSub, newSub, oldPurch, newPurch),
			actingUser.ID)
	}

	h.db.Tenants().UpdateOne(r.Context(), bson.M{"_id": tenantID}, bson.M{"$set": updates})
	respondWithJSON(w, http.StatusOK, map[string]string{"message": "Tenant updated"})
}

func (h *AdminHandler) ImpersonateUser(w http.ResponseWriter, r *http.Request) {
	if h.jwtService == nil {
		respondWithError(w, http.StatusInternalServerError, "JWT service not configured")
		return
	}

	actingUser, ok := middleware.GetUserFromContext(r.Context())
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	targetUserID, err := primitive.ObjectIDFromHex(mux.Vars(r)["userId"])
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	// Cannot impersonate self
	if targetUserID == actingUser.ID {
		respondWithError(w, http.StatusBadRequest, "Cannot impersonate yourself")
		return
	}

	var targetUser models.User
	if err := h.db.Users().FindOne(r.Context(), bson.M{"_id": targetUserID}).Decode(&targetUser); err != nil {
		respondWithError(w, http.StatusNotFound, "User not found")
		return
	}

	// Cannot impersonate other root tenant owners
	var rootTenant models.Tenant
	if err := h.db.Tenants().FindOne(r.Context(), bson.M{"isRoot": true}).Decode(&rootTenant); err == nil {
		var membership models.TenantMembership
		err := h.db.TenantMemberships().FindOne(r.Context(), bson.M{
			"userId":   targetUserID,
			"tenantId": rootTenant.ID,
			"role":     models.RoleOwner,
		}).Decode(&membership)
		if err == nil {
			respondWithError(w, http.StatusForbidden, "Cannot impersonate root tenant owners")
			return
		}
	}

	accessToken, err := h.jwtService.GenerateImpersonationToken(
		targetUser.ID.Hex(),
		targetUser.Email,
		targetUser.DisplayName,
		actingUser.ID.Hex(),
	)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to generate impersonation token")
		return
	}

	h.syslog.LogCatWithUser(r.Context(), models.LogCritical, models.LogCatSecurity,
		fmt.Sprintf("Admin %s (%s) started impersonating user %s (%s)",
			actingUser.Email, actingUser.ID.Hex(), targetUser.Email, targetUser.ID.Hex()),
		actingUser.ID)

	// Persistent impersonation audit record
	h.db.ImpersonationLogs().InsertOne(r.Context(), bson.M{
		"adminId":     actingUser.ID,
		"adminEmail":  actingUser.Email,
		"targetId":    targetUser.ID,
		"targetEmail": targetUser.Email,
		"ipAddress":   middleware.GetClientIP(r),
		"startedAt":   time.Now(),
		"expiresAt":   time.Now().Add(5 * time.Minute),
	})

	// Get target user's memberships
	cursor, err := h.db.TenantMemberships().Find(r.Context(), bson.M{"userId": targetUserID})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to fetch memberships")
		return
	}
	defer cursor.Close(r.Context())

	var memberships []models.TenantMembership
	cursor.All(r.Context(), &memberships)

	var membershipInfos []MembershipInfo
	for _, m := range memberships {
		var tenant models.Tenant
		if err := h.db.Tenants().FindOne(r.Context(), bson.M{"_id": m.TenantID}).Decode(&tenant); err != nil {
			continue
		}
		membershipInfos = append(membershipInfos, MembershipInfo{
			TenantID:   tenant.ID.Hex(),
			TenantName: tenant.Name,
			TenantSlug: tenant.Slug,
			Role:       m.Role,
			IsRoot:     tenant.IsRoot,
		})
	}

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"accessToken": accessToken,
		"user":        targetUser,
		"memberships": membershipInfos,
	})
}

// ===== Root Member Management =====

func (h *AdminHandler) getRootTenant(ctx context.Context) (*models.Tenant, error) {
	var tenant models.Tenant
	if err := h.db.Tenants().FindOne(ctx, bson.M{"isRoot": true}).Decode(&tenant); err != nil {
		return nil, err
	}
	return &tenant, nil
}

// ListRootMembers handles GET /api/admin/members
func (h *AdminHandler) ListRootMembers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rootTenant, err := h.getRootTenant(ctx)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Root tenant not found")
		return
	}

	// Fetch memberships
	cursor, err := h.db.TenantMemberships().Find(ctx, bson.M{"tenantId": rootTenant.ID})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to fetch members")
		return
	}
	defer cursor.Close(ctx)

	var memberships []models.TenantMembership
	if err := cursor.All(ctx, &memberships); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to decode members")
		return
	}

	// Batch-fetch users
	userIDs := make([]primitive.ObjectID, len(memberships))
	for i, m := range memberships {
		userIDs[i] = m.UserID
	}
	userMap := map[primitive.ObjectID]models.User{}
	if len(userIDs) > 0 {
		userCursor, err := h.db.Users().Find(ctx, bson.M{"_id": bson.M{"$in": userIDs}})
		if err == nil {
			defer userCursor.Close(ctx)
			var users []models.User
			if userCursor.All(ctx, &users) == nil {
				for _, u := range users {
					userMap[u.ID] = u
				}
			}
		}
	}

	var members []MemberResponse
	for _, m := range memberships {
		user, ok := userMap[m.UserID]
		if !ok {
			continue
		}
		members = append(members, MemberResponse{
			UserID:      user.ID.Hex(),
			Email:       user.Email,
			DisplayName: user.DisplayName,
			Role:        m.Role,
			JoinedAt:    m.JoinedAt,
		})
	}
	if members == nil {
		members = []MemberResponse{}
	}

	// Fetch pending invitations
	invCursor, err := h.db.Invitations().Find(ctx, bson.M{
		"tenantId":  rootTenant.ID,
		"status":    models.InvitationPending,
		"expiresAt": bson.M{"$gt": time.Now()},
	}, options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}))

	var invitations []models.Invitation
	if err == nil {
		defer invCursor.Close(ctx)
		invCursor.All(ctx, &invitations)
	}
	if invitations == nil {
		invitations = []models.Invitation{}
	}

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"members":     members,
		"invitations": invitations,
	})
}

// InviteRootMember handles POST /api/admin/members/invite
func (h *AdminHandler) InviteRootMember(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rootTenant, err := h.getRootTenant(ctx)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Root tenant not found")
		return
	}

	membership, ok := middleware.GetMembershipFromContext(ctx)
	if !ok {
		respondWithError(w, http.StatusForbidden, "Membership context missing")
		return
	}
	user, ok := middleware.GetUserFromContext(ctx)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	var req InviteMemberRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	if req.Email == "" {
		respondWithError(w, http.StatusBadRequest, "Email is required")
		return
	}
	if req.Role == models.RoleOwner {
		respondWithError(w, http.StatusBadRequest, "Cannot invite as owner. Use transfer ownership instead.")
		return
	}
	if !models.ValidRole(req.Role) {
		respondWithError(w, http.StatusBadRequest, "Invalid role")
		return
	}
	if req.Role == models.RoleAdmin && membership.Role != models.RoleOwner {
		respondWithError(w, http.StatusForbidden, "Only owners can invite admins")
		return
	}

	// Check if already a member
	var existingUser models.User
	if h.db.Users().FindOne(ctx, bson.M{"email": req.Email}).Decode(&existingUser) == nil {
		count, _ := h.db.TenantMemberships().CountDocuments(ctx, bson.M{
			"userId":   existingUser.ID,
			"tenantId": rootTenant.ID,
		})
		if count > 0 {
			respondWithError(w, http.StatusConflict, "User is already a member of the root tenant")
			return
		}
	}

	// Check if invitation already pending
	count, _ := h.db.Invitations().CountDocuments(ctx, bson.M{
		"tenantId":  rootTenant.ID,
		"email":     req.Email,
		"status":    models.InvitationPending,
		"expiresAt": bson.M{"$gt": time.Now()},
	})
	if count > 0 {
		respondWithError(w, http.StatusConflict, "An invitation has already been sent to this email")
		return
	}

	// No plan seat limit checks for root tenant

	now := time.Now()
	token := generateRandomToken()
	invitation := models.Invitation{
		ID:        primitive.NewObjectID(),
		TenantID:  rootTenant.ID,
		Email:     req.Email,
		Role:      req.Role,
		Token:     token,
		Status:    models.InvitationPending,
		InvitedBy: user.ID,
		ExpiresAt: now.Add(7 * 24 * time.Hour),
		CreatedAt: now,
	}

	if _, err := h.db.Invitations().InsertOne(ctx, invitation); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to create invitation")
		return
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_ = ctx // timeout guard for background goroutine
		if h.emailService != nil {
			if err := h.emailService.SendInvitationEmail(req.Email, user.DisplayName, rootTenant.Name, token); err != nil {
				log.Printf("Failed to send root member invitation email to %s: %v", req.Email, err)
			}
		}
	}()

	h.events.Emit(events.Event{
		Type:      events.EventMemberInvited,
		Timestamp: now,
		Data: map[string]interface{}{
			"tenantId": rootTenant.ID.Hex(),
			"email":    req.Email,
			"role":     string(req.Role),
		},
	})

	h.syslog.LogTenantActivity(ctx, models.LogHigh,
		fmt.Sprintf("Root member invited: %s as %s", req.Email, req.Role),
		user.ID, rootTenant.ID, "admin.root_member_invited",
		map[string]interface{}{"email": req.Email, "role": string(req.Role)})

	respondWithJSON(w, http.StatusCreated, map[string]string{"message": "Invitation sent"})
}

// RemoveRootMember handles DELETE /api/admin/members/{userId}
func (h *AdminHandler) RemoveRootMember(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rootTenant, err := h.getRootTenant(ctx)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Root tenant not found")
		return
	}

	currentMembership, ok := middleware.GetMembershipFromContext(ctx)
	if !ok {
		respondWithError(w, http.StatusForbidden, "Membership context missing")
		return
	}

	targetUserID, err := primitive.ObjectIDFromHex(mux.Vars(r)["userId"])
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	if targetUserID == currentMembership.UserID {
		respondWithError(w, http.StatusBadRequest, "Cannot remove yourself")
		return
	}

	var targetMembership models.TenantMembership
	if err := h.db.TenantMemberships().FindOne(ctx, bson.M{
		"userId":   targetUserID,
		"tenantId": rootTenant.ID,
	}).Decode(&targetMembership); err != nil {
		respondWithError(w, http.StatusNotFound, "Member not found")
		return
	}

	if targetMembership.Role == models.RoleOwner {
		respondWithError(w, http.StatusForbidden, "Cannot remove the owner. Transfer ownership first.")
		return
	}

	if currentMembership.Role == models.RoleAdmin && targetMembership.Role != models.RoleUser {
		respondWithError(w, http.StatusForbidden, "Admins can only remove users")
		return
	}

	if _, err := h.db.TenantMemberships().DeleteOne(ctx, bson.M{"_id": targetMembership.ID}); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to remove member")
		return
	}

	h.syslog.LogTenantActivity(ctx, models.LogHigh,
		fmt.Sprintf("Root member removed: user %s", targetUserID.Hex()),
		currentMembership.UserID, rootTenant.ID, "admin.root_member_removed",
		map[string]interface{}{"targetUserId": targetUserID.Hex()})

	h.events.Emit(events.Event{
		Type:      events.EventMemberRemoved,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"tenantId": rootTenant.ID.Hex(),
			"userId":   targetUserID.Hex(),
		},
	})

	respondWithJSON(w, http.StatusOK, map[string]string{"message": "Member removed"})
}

// ChangeRootMemberRole handles PATCH /api/admin/members/{userId}/role
func (h *AdminHandler) ChangeRootMemberRole(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rootTenant, err := h.getRootTenant(ctx)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Root tenant not found")
		return
	}

	currentMembership, ok := middleware.GetMembershipFromContext(ctx)
	if !ok {
		respondWithError(w, http.StatusForbidden, "Membership context missing")
		return
	}

	if currentMembership.Role != models.RoleOwner {
		respondWithError(w, http.StatusForbidden, "Only the owner can change roles")
		return
	}

	targetUserID, err := primitive.ObjectIDFromHex(mux.Vars(r)["userId"])
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	if targetUserID == currentMembership.UserID {
		respondWithError(w, http.StatusBadRequest, "Cannot change your own role")
		return
	}

	var req ChangeRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Role == models.RoleOwner {
		respondWithError(w, http.StatusBadRequest, "Cannot set role to owner. Use transfer ownership instead.")
		return
	}
	if !models.ValidRole(req.Role) {
		respondWithError(w, http.StatusBadRequest, "Invalid role")
		return
	}

	result, err := h.db.TenantMemberships().UpdateOne(ctx,
		bson.M{"userId": targetUserID, "tenantId": rootTenant.ID},
		bson.M{"$set": bson.M{"role": req.Role, "updatedAt": time.Now()}},
	)
	if err != nil || result.MatchedCount == 0 {
		respondWithError(w, http.StatusNotFound, "Member not found")
		return
	}

	h.events.Emit(events.Event{
		Type:      events.EventMemberRoleChanged,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"tenantId": rootTenant.ID.Hex(),
			"userId":   targetUserID.Hex(),
			"newRole":  string(req.Role),
		},
	})

	h.syslog.LogTenantActivity(ctx, models.LogHigh,
		fmt.Sprintf("Root member role changed: user %s to %s", targetUserID.Hex(), req.Role),
		currentMembership.UserID, rootTenant.ID, "admin.root_member_role_changed",
		map[string]interface{}{"targetUserId": targetUserID.Hex(), "newRole": string(req.Role)})

	respondWithJSON(w, http.StatusOK, map[string]string{"message": "Role updated"})
}

// CancelRootInvitation handles DELETE /api/admin/members/invitations/{invitationId}
func (h *AdminHandler) CancelRootInvitation(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rootTenant, err := h.getRootTenant(ctx)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Root tenant not found")
		return
	}

	invID, err := primitive.ObjectIDFromHex(mux.Vars(r)["invitationId"])
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid invitation ID")
		return
	}

	result, err := h.db.Invitations().DeleteOne(ctx, bson.M{
		"_id":      invID,
		"tenantId": rootTenant.ID,
		"status":   models.InvitationPending,
	})
	if err != nil || result.DeletedCount == 0 {
		respondWithError(w, http.StatusNotFound, "Invitation not found")
		return
	}

	if user, ok := middleware.GetUserFromContext(ctx); ok {
		h.syslog.LogTenantActivity(ctx, models.LogMedium,
			fmt.Sprintf("Root member invitation canceled: %s", invID.Hex()),
			user.ID, rootTenant.ID, "admin.root_invitation_canceled",
			map[string]interface{}{"invitationId": invID.Hex()})
	}

	respondWithJSON(w, http.StatusOK, map[string]string{"message": "Invitation canceled"})
}
