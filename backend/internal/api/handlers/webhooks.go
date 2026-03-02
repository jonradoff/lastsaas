package handlers

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"lastsaas/internal/db"
	"lastsaas/internal/middleware"
	"lastsaas/internal/models"
	"lastsaas/internal/syslog"
	"lastsaas/internal/validation"
	"lastsaas/internal/webhooks"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/gorilla/mux"
)

type WebhooksHandler struct {
	db         *db.MongoDB
	syslog     *syslog.Logger
	dispatcher *webhooks.Dispatcher
}

func NewWebhooksHandler(database *db.MongoDB, sysLogger *syslog.Logger, dispatcher *webhooks.Dispatcher) *WebhooksHandler {
	return &WebhooksHandler{
		db:         database,
		syslog:     sysLogger,
		dispatcher: dispatcher,
	}
}

// ListWebhooks handles GET /api/admin/webhooks
func (h *WebhooksHandler) ListWebhooks(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	opts := options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}).SetLimit(100)
	cursor, err := h.db.Webhooks().Find(ctx, bson.M{"isActive": true}, opts)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to list webhooks")
		return
	}
	defer cursor.Close(ctx)

	var hooks []models.Webhook
	if err := cursor.All(ctx, &hooks); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to decode webhooks")
		return
	}
	if hooks == nil {
		hooks = []models.Webhook{}
	}

	// Enrich with delivery stats
	type webhookWithStats struct {
		models.Webhook
		Deliveries24h int        `json:"deliveries24h"`
		LastDelivery  *time.Time `json:"lastDelivery"`
	}
	since := time.Now().Add(-24 * time.Hour)
	result := make([]webhookWithStats, len(hooks))
	for i, hook := range hooks {
		result[i].Webhook = hook
		count, _ := h.db.WebhookDeliveries().CountDocuments(ctx, bson.M{
			"webhookId": hook.ID,
			"createdAt": bson.M{"$gte": since},
		})
		result[i].Deliveries24h = int(count)

		var lastDel models.WebhookDelivery
		lastOpts := options.FindOne().SetSort(bson.D{{Key: "createdAt", Value: -1}})
		if h.db.WebhookDeliveries().FindOne(ctx, bson.M{"webhookId": hook.ID}, lastOpts).Decode(&lastDel) == nil {
			result[i].LastDelivery = &lastDel.CreatedAt
		}
	}

	total, _ := h.db.Webhooks().CountDocuments(ctx, bson.M{"isActive": true})
	respondWithJSON(w, http.StatusOK, map[string]interface{}{"webhooks": result, "total": total})
}

// GetWebhook handles GET /api/admin/webhooks/{webhookId}
func (h *WebhooksHandler) GetWebhook(w http.ResponseWriter, r *http.Request) {
	whID, err := primitive.ObjectIDFromHex(mux.Vars(r)["webhookId"])
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid webhook ID")
		return
	}

	var hook models.Webhook
	if err := h.db.Webhooks().FindOne(r.Context(), bson.M{"_id": whID, "isActive": true}).Decode(&hook); err != nil {
		respondWithError(w, http.StatusNotFound, "Webhook not found")
		return
	}

	// Get recent deliveries
	cursor, err := h.db.WebhookDeliveries().Find(r.Context(),
		bson.M{"webhookId": whID},
		options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}).SetLimit(20),
	)
	if err != nil {
		respondWithJSON(w, http.StatusOK, map[string]interface{}{
			"webhook":    hook,
			"deliveries": []models.WebhookDelivery{},
		})
		return
	}
	defer cursor.Close(r.Context())

	var deliveries []models.WebhookDelivery
	if err := cursor.All(r.Context(), &deliveries); err != nil {
		deliveries = []models.WebhookDelivery{}
	}
	if deliveries == nil {
		deliveries = []models.WebhookDelivery{}
	}

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"webhook":    hook,
		"deliveries": deliveries,
	})
}

type webhookRequest struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	URL         string   `json:"url"`
	Events      []string `json:"events"`
}

func validateWebhookURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL format")
	}

	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("URL must have a hostname")
	}

	// Block localhost variants
	if host == "localhost" || host == "0.0.0.0" || host == "[::1]" {
		return fmt.Errorf("URL cannot point to localhost")
	}

	// Resolve and check IPs
	ips, err := net.LookupHost(host)
	if err != nil {
		// If DNS fails, check if host is already an IP
		ip := net.ParseIP(host)
		if ip == nil {
			return fmt.Errorf("cannot resolve hostname")
		}
		ips = []string{host}
	}

	for _, ipStr := range ips {
		ip := net.ParseIP(ipStr)
		if ip == nil {
			continue
		}
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
			return fmt.Errorf("URL cannot point to a private or internal address")
		}
		// Block cloud metadata endpoints
		if ip.Equal(net.ParseIP("169.254.169.254")) {
			return fmt.Errorf("URL cannot point to a metadata endpoint")
		}
	}

	return nil
}

func validateWebhookRequest(req *webhookRequest) error {
	req.Name = strings.TrimSpace(req.Name)
	req.URL = strings.TrimSpace(req.URL)
	req.Description = strings.TrimSpace(req.Description)

	if req.Name == "" {
		return fmt.Errorf("name is required")
	}
	if len(req.Name) > 100 {
		return fmt.Errorf("name must be 100 characters or less")
	}
	if req.URL == "" {
		return fmt.Errorf("URL is required")
	}
	if !strings.HasPrefix(req.URL, "https://") && !strings.HasPrefix(req.URL, "http://") {
		return fmt.Errorf("URL must start with https:// or http://")
	}
	if err := validateWebhookURL(req.URL); err != nil {
		return err
	}
	if len(req.Events) == 0 {
		return fmt.Errorf("at least one event type is required")
	}
	for _, e := range req.Events {
		if !models.ValidWebhookEventType(models.WebhookEventType(e)) {
			return fmt.Errorf("invalid event type: %s", e)
		}
	}
	return nil
}

// CreateWebhook handles POST /api/admin/webhooks
func (h *WebhooksHandler) CreateWebhook(w http.ResponseWriter, r *http.Request) {
	var req webhookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := validateWebhookRequest(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	user, ok := middleware.GetUserFromContext(r.Context())
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	evts := make([]models.WebhookEventType, len(req.Events))
	for i, e := range req.Events {
		evts[i] = models.WebhookEventType(e)
	}

	// Auto-generate signing secret
	rawSecret := "whsec_" + generateRandomToken()
	storedSecret := rawSecret
	if h.dispatcher != nil {
		if encKey := h.dispatcher.EncryptionKey(); encKey != nil {
			encrypted, err := webhooks.EncryptSecret(rawSecret, encKey)
			if err != nil {
				respondWithError(w, http.StatusInternalServerError, "Failed to secure webhook secret")
				return
			}
			storedSecret = encrypted
		}
	}

	now := time.Now()
	hook := models.Webhook{
		Name:          req.Name,
		Description:   req.Description,
		URL:           req.URL,
		Secret:        storedSecret,
		SecretPreview: rawSecret[len(rawSecret)-8:],
		Events:        evts,
		IsActive:      true,
		CreatedBy:     user.ID,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if err := validation.Validate(&hook); err != nil {
		respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	result, err := h.db.Webhooks().InsertOne(r.Context(), hook)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to create webhook")
		return
	}
	id, ok := result.InsertedID.(primitive.ObjectID)
	if !ok {
		respondWithError(w, http.StatusInternalServerError, "Internal error")
		return
	}
	hook.ID = id

	h.syslog.HighWithUser(r.Context(), fmt.Sprintf("Webhook created: %s → %s", req.Name, req.URL), user.ID)

	respondWithJSON(w, http.StatusCreated, map[string]interface{}{"webhook": hook, "secret": rawSecret})
}

// UpdateWebhook handles PUT /api/admin/webhooks/{webhookId}
func (h *WebhooksHandler) UpdateWebhook(w http.ResponseWriter, r *http.Request) {
	whID, err := primitive.ObjectIDFromHex(mux.Vars(r)["webhookId"])
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid webhook ID")
		return
	}

	var req webhookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := validateWebhookRequest(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	evts := make([]models.WebhookEventType, len(req.Events))
	for i, e := range req.Events {
		evts[i] = models.WebhookEventType(e)
	}

	result, err := h.db.Webhooks().UpdateByID(r.Context(), whID, bson.M{
		"$set": bson.M{
			"name":        req.Name,
			"description": req.Description,
			"url":         req.URL,
			"events":      evts,
			"updatedAt":   time.Now(),
		},
	})
	if err != nil || result.MatchedCount == 0 {
		respondWithError(w, http.StatusNotFound, "Webhook not found")
		return
	}

	if user, ok := middleware.GetUserFromContext(r.Context()); ok {
		h.syslog.HighWithUser(r.Context(), fmt.Sprintf("Webhook updated: %s", req.Name), user.ID)
	}

	// Return updated webhook
	var hook models.Webhook
	if err := h.db.Webhooks().FindOne(r.Context(), bson.M{"_id": whID}).Decode(&hook); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Webhook updated but failed to read back")
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]interface{}{"webhook": hook})
}

// DeleteWebhook handles DELETE /api/admin/webhooks/{webhookId}
func (h *WebhooksHandler) DeleteWebhook(w http.ResponseWriter, r *http.Request) {
	whID, err := primitive.ObjectIDFromHex(mux.Vars(r)["webhookId"])
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid webhook ID")
		return
	}

	result, err := h.db.Webhooks().UpdateByID(r.Context(), whID, bson.M{
		"$set": bson.M{"isActive": false},
	})
	if err != nil || result.MatchedCount == 0 {
		respondWithError(w, http.StatusNotFound, "Webhook not found")
		return
	}

	if user, ok := middleware.GetUserFromContext(r.Context()); ok {
		h.syslog.HighWithUser(r.Context(), fmt.Sprintf("Webhook deleted: %s", whID.Hex()), user.ID)
	}

	respondWithJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// RegenerateSecret handles POST /api/admin/webhooks/{webhookId}/regenerate-secret
func (h *WebhooksHandler) RegenerateSecret(w http.ResponseWriter, r *http.Request) {
	whID, err := primitive.ObjectIDFromHex(mux.Vars(r)["webhookId"])
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid webhook ID")
		return
	}

	rawSecret := "whsec_" + generateRandomToken()
	preview := rawSecret[len(rawSecret)-8:]
	storedSecret := rawSecret
	if h.dispatcher != nil {
		if encKey := h.dispatcher.EncryptionKey(); encKey != nil {
			encrypted, err := webhooks.EncryptSecret(rawSecret, encKey)
			if err != nil {
				respondWithError(w, http.StatusInternalServerError, "Failed to secure webhook secret")
				return
			}
			storedSecret = encrypted
		}
	}

	result, err := h.db.Webhooks().UpdateByID(r.Context(), whID, bson.M{
		"$set": bson.M{
			"secret":        storedSecret,
			"secretPreview": preview,
			"updatedAt":     time.Now(),
		},
	})
	if err != nil || result.MatchedCount == 0 {
		respondWithError(w, http.StatusNotFound, "Webhook not found")
		return
	}

	if user, ok := middleware.GetUserFromContext(r.Context()); ok {
		h.syslog.HighWithUser(r.Context(), fmt.Sprintf("Webhook secret regenerated: %s", whID.Hex()), user.ID)
	}

	respondWithJSON(w, http.StatusOK, map[string]interface{}{"secret": rawSecret, "secretPreview": preview})
}

// TestWebhook handles POST /api/admin/webhooks/{webhookId}/test
func (h *WebhooksHandler) TestWebhook(w http.ResponseWriter, r *http.Request) {
	whID, err := primitive.ObjectIDFromHex(mux.Vars(r)["webhookId"])
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid webhook ID")
		return
	}

	var hook models.Webhook
	if err := h.db.Webhooks().FindOne(r.Context(), bson.M{"_id": whID, "isActive": true}).Decode(&hook); err != nil {
		respondWithError(w, http.StatusNotFound, "Webhook not found")
		return
	}

	delivery := h.dispatcher.DeliverTest(r.Context(), hook)

	respondWithJSON(w, http.StatusOK, map[string]interface{}{"delivery": delivery})
}

// ListEventTypes handles GET /api/admin/webhooks/event-types
func (h *WebhooksHandler) ListEventTypes(w http.ResponseWriter, r *http.Request) {
	type eventInfo struct {
		Type     string `json:"type"`
		Category string `json:"category"`
		Desc     string `json:"description"`
	}

	meta := map[models.WebhookEventType][2]string{
		// Billing [category, description]
		models.WebhookEventSubscriptionActivated: {"Billing", "A subscription is activated after successful checkout"},
		models.WebhookEventSubscriptionCanceled:  {"Billing", "A subscription is canceled by the user, admin, or Stripe"},
		models.WebhookEventPaymentReceived:       {"Billing", "A recurring subscription payment succeeds"},
		models.WebhookEventPaymentFailed:         {"Billing", "A subscription payment fails (tenant moves to past-due)"},
		// Team Lifecycle
		models.WebhookEventMemberInvited:        {"Team Lifecycle", "A member is invited to a tenant"},
		models.WebhookEventMemberJoined:         {"Team Lifecycle", "A user accepts an invitation and joins a tenant"},
		models.WebhookEventMemberRemoved:        {"Team Lifecycle", "A member is removed from a tenant by an admin"},
		models.WebhookEventMemberRoleChanged:    {"Team Lifecycle", "A member's role within a tenant is changed"},
		models.WebhookEventOwnershipTransferred: {"Team Lifecycle", "Tenant ownership is transferred to another member"},
		// User Lifecycle
		models.WebhookEventUserRegistered:  {"User Lifecycle", "A new user account is created"},
		models.WebhookEventUserVerified:    {"User Lifecycle", "A user verifies their email address"},
		models.WebhookEventUserDeactivated: {"User Lifecycle", "An admin deactivates a user account"},
		// Credits & Plans
		models.WebhookEventCreditsPurchased:  {"Credits & Plans", "A credit bundle is purchased"},
		models.WebhookEventPlanChanged:       {"Credits & Plans", "A tenant's plan changes (upgrade, downgrade, or expiry)"},
		models.WebhookEventTenantCreated:     {"Credits & Plans", "A new tenant (team/organization) is created"},
		models.WebhookEventTenantDeactivated: {"Credits & Plans", "An admin deactivates a tenant"},
		// Audit & Security
		models.WebhookEventUserDeleted:   {"Audit & Security", "An admin permanently deletes a user account"},
		models.WebhookEventTenantDeleted: {"Audit & Security", "A tenant is permanently deleted"},
		models.WebhookEventAPIKeyCreated: {"Audit & Security", "A new API key is created"},
		models.WebhookEventAPIKeyRevoked: {"Audit & Security", "An API key is revoked"},
	}

	types := make([]eventInfo, len(models.AllWebhookEventTypes))
	for i, t := range models.AllWebhookEventTypes {
		m := meta[t]
		types[i] = eventInfo{Type: string(t), Category: m[0], Desc: m[1]}
	}
	respondWithJSON(w, http.StatusOK, map[string]interface{}{"eventTypes": types})
}
