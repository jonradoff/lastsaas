package handlers

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"lastsaas/internal/db"
	"lastsaas/internal/events"
	"lastsaas/internal/middleware"
	"lastsaas/internal/models"
	"lastsaas/internal/syslog"
	"lastsaas/internal/validation"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/gorilla/mux"
)

const apiKeyPrefix = "lsk_"

type APIKeysHandler struct {
	db     *db.MongoDB
	events events.Emitter
	syslog *syslog.Logger
}

func NewAPIKeysHandler(database *db.MongoDB, emitter events.Emitter, sysLogger *syslog.Logger) *APIKeysHandler {
	return &APIKeysHandler{
		db:     database,
		events: emitter,
		syslog: sysLogger,
	}
}

// ListAPIKeys handles GET /api/admin/api-keys
func (h *APIKeysHandler) ListAPIKeys(w http.ResponseWriter, r *http.Request) {
	opts := options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}).SetLimit(100)
	cursor, err := h.db.APIKeys().Find(r.Context(), bson.M{"isActive": true}, opts)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to list API keys")
		return
	}
	defer cursor.Close(r.Context())

	var keys []models.APIKey
	if err := cursor.All(r.Context(), &keys); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to decode API keys")
		return
	}
	if keys == nil {
		keys = []models.APIKey{}
	}
	total, _ := h.db.APIKeys().CountDocuments(r.Context(), bson.M{"isActive": true})
	respondWithJSON(w, http.StatusOK, map[string]interface{}{"apiKeys": keys, "total": total})
}

// CreateAPIKey handles POST /api/admin/api-keys
func (h *APIKeysHandler) CreateAPIKey(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name      string `json:"name"`
		Authority string `json:"authority"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		respondWithError(w, http.StatusBadRequest, "Name is required")
		return
	}
	if len(req.Name) > 100 {
		respondWithError(w, http.StatusBadRequest, "Name must be 100 characters or less")
		return
	}

	authority := models.APIKeyAuthority(req.Authority)
	if !models.ValidAPIKeyAuthority(authority) {
		respondWithError(w, http.StatusBadRequest, "Authority must be 'admin' or 'user'")
		return
	}

	user, ok := middleware.GetUserFromContext(r.Context())
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	// Generate raw key: lsk_ + 32 bytes base64url
	rawKey := apiKeyPrefix + generateRandomToken()

	// Store SHA-256 hash
	hash := sha256.Sum256([]byte(rawKey))
	keyHash := base64.RawURLEncoding.EncodeToString(hash[:])

	// Preview: last 8 chars of the raw key
	keyPreview := rawKey[len(rawKey)-8:]

	now := time.Now()
	apiKey := models.APIKey{
		Name:      req.Name,
		KeyHash:   keyHash,
		KeyPreview: keyPreview,
		Authority: authority,
		CreatedBy: user.ID,
		CreatedAt: now,
		IsActive:  true,
	}

	if err := validation.Validate(&apiKey); err != nil {
		respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	result, err := h.db.APIKeys().InsertOne(r.Context(), apiKey)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to create API key")
		return
	}
	id, ok := result.InsertedID.(primitive.ObjectID)
	if !ok {
		respondWithError(w, http.StatusInternalServerError, "Internal error")
		return
	}
	apiKey.ID = id

	h.syslog.HighWithUser(r.Context(), fmt.Sprintf("API key created: %s (authority: %s)", req.Name, req.Authority), user.ID)

	h.events.Emit(events.Event{
		Type:      events.EventAPIKeyCreated,
		Timestamp: now,
		Data: map[string]interface{}{
			"keyId":     apiKey.ID.Hex(),
			"name":      req.Name,
			"authority": req.Authority,
			"createdBy": user.ID.Hex(),
		},
	})

	respondWithJSON(w, http.StatusCreated, map[string]interface{}{
		"apiKey": apiKey,
		"rawKey": rawKey,
	})
}

// DeleteAPIKey handles DELETE /api/admin/api-keys/{keyId}
func (h *APIKeysHandler) DeleteAPIKey(w http.ResponseWriter, r *http.Request) {
	keyID, err := primitive.ObjectIDFromHex(mux.Vars(r)["keyId"])
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid key ID")
		return
	}

	result, err := h.db.APIKeys().UpdateByID(r.Context(), keyID, bson.M{
		"$set": bson.M{"isActive": false},
	})
	if err != nil || result.MatchedCount == 0 {
		respondWithError(w, http.StatusNotFound, "API key not found")
		return
	}

	if user, ok := middleware.GetUserFromContext(r.Context()); ok {
		h.syslog.HighWithUser(r.Context(), fmt.Sprintf("API key deleted: %s", keyID.Hex()), user.ID)

		h.events.Emit(events.Event{
			Type:      events.EventAPIKeyRevoked,
			Timestamp: time.Now(),
			Data: map[string]interface{}{
				"keyId":     keyID.Hex(),
				"revokedBy": user.ID.Hex(),
			},
		})
	}

	respondWithJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
