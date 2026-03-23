package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"lastsaas/internal/db"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// LeadsHandler handles email lead capture for the scan result page.
type LeadsHandler struct {
	db *db.MongoDB
}

// NewLeadsHandler creates a new LeadsHandler.
func NewLeadsHandler(database *db.MongoDB) *LeadsHandler {
	return &LeadsHandler{db: database}
}

type leadRequest struct {
	Email  string `json:"email"`
	Domain string `json:"domain"`
	Score  int    `json:"score"`
}

type leadDocument struct {
	ID        primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	Email     string             `json:"email" bson:"email"`
	Domain    string             `json:"domain" bson:"domain"`
	Score     int                `json:"score" bson:"score"`
	Token     string             `json:"token" bson:"token"`
	CreatedAt time.Time          `json:"createdAt" bson:"createdAt"`
}

// CaptureLead handles POST /api/leads.
// Accepts an email, domain, and score. Returns a token that unlocks fix instructions for the session.
func (h *LeadsHandler) CaptureLead(w http.ResponseWriter, r *http.Request) {
	var req leadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	req.Domain = strings.TrimSpace(strings.ToLower(req.Domain))

	if req.Email == "" || !strings.Contains(req.Email, "@") {
		respondWithError(w, http.StatusBadRequest, "Valid email is required")
		return
	}
	if req.Domain == "" {
		respondWithError(w, http.StatusBadRequest, "Domain is required")
		return
	}

	ctx := r.Context()

	// Check if this email+domain combo already exists; return existing token if so
	var existing leadDocument
	err := h.db.Leads().FindOne(ctx, bson.M{"email": req.Email, "domain": req.Domain}).Decode(&existing)
	if err == nil {
		respondWithJSON(w, http.StatusOK, map[string]string{"token": existing.Token})
		return
	}

	// Generate a random token
	tokenBytes := make([]byte, 16)
	if _, err := rand.Read(tokenBytes); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to generate token")
		return
	}
	token := hex.EncodeToString(tokenBytes)

	lead := leadDocument{
		Email:     req.Email,
		Domain:    req.Domain,
		Score:     req.Score,
		Token:     token,
		CreatedAt: time.Now(),
	}

	if _, err := h.db.Leads().InsertOne(ctx, lead); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to store lead")
		return
	}

	respondWithJSON(w, http.StatusCreated, map[string]string{"token": token})
}

// VerifyToken handles GET /api/leads/verify?token=...
// Returns 200 if the token is valid, 404 otherwise.
func (h *LeadsHandler) VerifyToken(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		respondWithError(w, http.StatusBadRequest, "Token is required")
		return
	}

	var lead leadDocument
	err := h.db.Leads().FindOne(r.Context(), bson.M{"token": token}).Decode(&lead)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "Invalid token")
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]interface{}{"valid": true, "email": lead.Email})
}
