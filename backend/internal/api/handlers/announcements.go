package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"lastsaas/internal/db"
	"lastsaas/internal/models"
	"lastsaas/internal/syslog"
	"lastsaas/internal/validation"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type AnnouncementsHandler struct {
	db     *db.MongoDB
	syslog *syslog.Logger
}

func NewAnnouncementsHandler(database *db.MongoDB, sysLogger *syslog.Logger) *AnnouncementsHandler {
	return &AnnouncementsHandler{db: database, syslog: sysLogger}
}

// ListPublic returns published announcements (for all authenticated users).
func (h *AnnouncementsHandler) ListPublic(w http.ResponseWriter, r *http.Request) {
	opts := options.Find().
		SetSort(bson.D{{Key: "publishedAt", Value: -1}}).
		SetLimit(20)
	cursor, err := h.db.Announcements().Find(r.Context(), bson.M{"isPublished": true}, opts)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to list announcements")
		return
	}
	defer cursor.Close(r.Context())

	var announcements []models.Announcement
	if err := cursor.All(r.Context(), &announcements); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to decode announcements")
		return
	}
	if announcements == nil {
		announcements = []models.Announcement{}
	}
	respondWithJSON(w, http.StatusOK, map[string]interface{}{"announcements": announcements})
}

// ListAll returns all announcements (admin only).
func (h *AnnouncementsHandler) ListAll(w http.ResponseWriter, r *http.Request) {
	opts := options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}).SetLimit(200)
	cursor, err := h.db.Announcements().Find(r.Context(), bson.M{}, opts)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to list announcements")
		return
	}
	defer cursor.Close(r.Context())

	var announcements []models.Announcement
	if err := cursor.All(r.Context(), &announcements); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to decode announcements")
		return
	}
	if announcements == nil {
		announcements = []models.Announcement{}
	}
	respondWithJSON(w, http.StatusOK, map[string]interface{}{"announcements": announcements})
}

// Create creates a new announcement.
func (h *AnnouncementsHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Title   string `json:"title"`
		Body    string `json:"body"`
		Publish bool   `json:"publish"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if req.Title == "" {
		respondWithError(w, http.StatusBadRequest, "Title is required")
		return
	}

	now := time.Now()
	ann := models.Announcement{
		Title:       req.Title,
		Body:        req.Body,
		IsPublished: req.Publish,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if req.Publish {
		ann.PublishedAt = &now
	}

	if err := validation.Validate(&ann); err != nil {
		respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	result, err := h.db.Announcements().InsertOne(r.Context(), ann)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to create announcement")
		return
	}
	id, ok := result.InsertedID.(primitive.ObjectID)
	if !ok {
		respondWithError(w, http.StatusInternalServerError, "Internal error")
		return
	}
	ann.ID = id
	respondWithJSON(w, http.StatusCreated, ann)
}

// Update updates an existing announcement.
func (h *AnnouncementsHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := primitive.ObjectIDFromHex(mux.Vars(r)["id"])
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid announcement ID")
		return
	}

	var req struct {
		Title   *string `json:"title"`
		Body    *string `json:"body"`
		Publish *bool   `json:"publish"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	update := bson.M{"updatedAt": time.Now()}
	if req.Title != nil {
		update["title"] = *req.Title
	}
	if req.Body != nil {
		update["body"] = *req.Body
	}
	if req.Publish != nil {
		update["isPublished"] = *req.Publish
		if *req.Publish {
			now := time.Now()
			update["publishedAt"] = now
		}
	}

	result, err := h.db.Announcements().UpdateOne(r.Context(), bson.M{"_id": id}, bson.M{"$set": update})
	if err != nil || result.MatchedCount == 0 {
		respondWithError(w, http.StatusNotFound, "Announcement not found")
		return
	}
	respondWithJSON(w, http.StatusOK, map[string]string{"message": "Updated"})
}

// Delete deletes an announcement.
func (h *AnnouncementsHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := primitive.ObjectIDFromHex(mux.Vars(r)["id"])
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid announcement ID")
		return
	}

	result, err := h.db.Announcements().DeleteOne(r.Context(), bson.M{"_id": id})
	if err != nil || result.DeletedCount == 0 {
		respondWithError(w, http.StatusNotFound, "Announcement not found")
		return
	}
	respondWithJSON(w, http.StatusOK, map[string]string{"message": "Deleted"})
}
