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
	"lastsaas/internal/syslog"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/gorilla/mux"
)

type BundlesHandler struct {
	db     *db.MongoDB
	syslog *syslog.Logger
}

func NewBundlesHandler(database *db.MongoDB, sysLogger *syslog.Logger) *BundlesHandler {
	return &BundlesHandler{
		db:     database,
		syslog: sysLogger,
	}
}

type bundleRequest struct {
	Name       string `json:"name"`
	Credits    int64  `json:"credits"`
	PriceCents int64  `json:"priceCents"`
	IsActive   bool   `json:"isActive"`
	SortOrder  int    `json:"sortOrder"`
}

func validateBundleRequest(req *bundleRequest) error {
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		return fmt.Errorf("name is required")
	}
	if req.Credits <= 0 {
		return fmt.Errorf("credits must be greater than 0")
	}
	if req.PriceCents <= 0 {
		return fmt.Errorf("price must be greater than 0")
	}
	return nil
}

// ListBundles returns all credit bundles for admin.
func (h *BundlesHandler) ListBundles(w http.ResponseWriter, r *http.Request) {
	opts := options.Find().SetSort(bson.D{{Key: "sortOrder", Value: 1}, {Key: "createdAt", Value: 1}}).SetLimit(100)
	cursor, err := h.db.CreditBundles().Find(r.Context(), bson.M{}, opts)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to list credit bundles")
		return
	}
	defer cursor.Close(r.Context())

	var bundles []models.CreditBundle
	if err := cursor.All(r.Context(), &bundles); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to decode credit bundles")
		return
	}
	if bundles == nil {
		bundles = []models.CreditBundle{}
	}
	total, _ := h.db.CreditBundles().CountDocuments(r.Context(), bson.M{})
	respondWithJSON(w, http.StatusOK, map[string]interface{}{"bundles": bundles, "total": total})
}

// CreateBundle creates a new credit bundle.
func (h *BundlesHandler) CreateBundle(w http.ResponseWriter, r *http.Request) {
	var req bundleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if err := validateBundleRequest(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Check name uniqueness
	count, _ := h.db.CreditBundles().CountDocuments(r.Context(), bson.M{"name": req.Name})
	if count > 0 {
		respondWithError(w, http.StatusConflict, "A credit bundle with this name already exists")
		return
	}

	now := time.Now()
	bundle := models.CreditBundle{
		Name:       req.Name,
		Credits:    req.Credits,
		PriceCents: req.PriceCents,
		IsActive:   req.IsActive,
		SortOrder:  req.SortOrder,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	result, err := h.db.CreditBundles().InsertOne(r.Context(), bundle)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to create credit bundle")
		return
	}
	id, ok := result.InsertedID.(primitive.ObjectID)
	if !ok {
		respondWithError(w, http.StatusInternalServerError, "Internal error")
		return
	}
	bundle.ID = id

	if user, ok := middleware.GetUserFromContext(r.Context()); ok {
		h.syslog.LogWithUser(r.Context(), models.LogMedium, fmt.Sprintf("Credit bundle created: %s", bundle.Name), user.ID)
	}

	respondWithJSON(w, http.StatusCreated, bundle)
}

// UpdateBundle updates an existing credit bundle.
func (h *BundlesHandler) UpdateBundle(w http.ResponseWriter, r *http.Request) {
	bundleID, err := primitive.ObjectIDFromHex(mux.Vars(r)["bundleId"])
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid bundle ID")
		return
	}

	var existing models.CreditBundle
	if err := h.db.CreditBundles().FindOne(r.Context(), bson.M{"_id": bundleID}).Decode(&existing); err != nil {
		if err == mongo.ErrNoDocuments {
			respondWithError(w, http.StatusNotFound, "Credit bundle not found")
			return
		}
		respondWithError(w, http.StatusInternalServerError, "Failed to get credit bundle")
		return
	}

	var req bundleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if err := validateBundleRequest(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Check name uniqueness if changed
	if req.Name != existing.Name {
		count, _ := h.db.CreditBundles().CountDocuments(r.Context(), bson.M{"name": req.Name, "_id": bson.M{"$ne": bundleID}})
		if count > 0 {
			respondWithError(w, http.StatusConflict, "A credit bundle with this name already exists")
			return
		}
	}

	update := bson.M{"$set": bson.M{
		"name":       req.Name,
		"credits":    req.Credits,
		"priceCents": req.PriceCents,
		"isActive":   req.IsActive,
		"sortOrder":  req.SortOrder,
		"updatedAt":  time.Now(),
	}}

	if _, err := h.db.CreditBundles().UpdateByID(r.Context(), bundleID, update); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to update credit bundle")
		return
	}

	if user, ok := middleware.GetUserFromContext(r.Context()); ok {
		h.syslog.LogWithUser(r.Context(), models.LogMedium, fmt.Sprintf("Credit bundle updated: %s", req.Name), user.ID)
	}

	var updated models.CreditBundle
	h.db.CreditBundles().FindOne(r.Context(), bson.M{"_id": bundleID}).Decode(&updated)
	respondWithJSON(w, http.StatusOK, updated)
}

// DeleteBundle deletes a credit bundle.
func (h *BundlesHandler) DeleteBundle(w http.ResponseWriter, r *http.Request) {
	bundleID, err := primitive.ObjectIDFromHex(mux.Vars(r)["bundleId"])
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid bundle ID")
		return
	}

	var bundle models.CreditBundle
	if err := h.db.CreditBundles().FindOne(r.Context(), bson.M{"_id": bundleID}).Decode(&bundle); err != nil {
		if err == mongo.ErrNoDocuments {
			respondWithError(w, http.StatusNotFound, "Credit bundle not found")
			return
		}
		respondWithError(w, http.StatusInternalServerError, "Failed to get credit bundle")
		return
	}

	if _, err := h.db.CreditBundles().DeleteOne(r.Context(), bson.M{"_id": bundleID}); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to delete credit bundle")
		return
	}

	if user, ok := middleware.GetUserFromContext(r.Context()); ok {
		h.syslog.LogWithUser(r.Context(), models.LogMedium, fmt.Sprintf("Credit bundle deleted: %s", bundle.Name), user.ID)
	}

	respondWithJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// ListBundlesPublic returns active credit bundles for authenticated users.
func (h *BundlesHandler) ListBundlesPublic(w http.ResponseWriter, r *http.Request) {
	opts := options.Find().SetSort(bson.D{{Key: "sortOrder", Value: 1}, {Key: "createdAt", Value: 1}})
	cursor, err := h.db.CreditBundles().Find(r.Context(), bson.M{"isActive": true}, opts)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to list credit bundles")
		return
	}
	defer cursor.Close(r.Context())

	var bundles []models.CreditBundle
	if err := cursor.All(r.Context(), &bundles); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to decode credit bundles")
		return
	}
	if bundles == nil {
		bundles = []models.CreditBundle{}
	}
	respondWithJSON(w, http.StatusOK, map[string]interface{}{"bundles": bundles})
}
