package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"lastsaas/internal/configstore"
	"lastsaas/internal/db"
	"lastsaas/internal/models"
	"lastsaas/internal/syslog"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const maxAssetSize = 5 << 20  // 5 MB for logo/favicon
const maxMediaSize = 10 << 20 // 10 MB for media uploads

type BrandingHandler struct {
	db            *db.MongoDB
	store         *configstore.Store
	syslog        *syslog.Logger
	authProviders map[string]bool
}

func NewBrandingHandler(database *db.MongoDB, store *configstore.Store, sysLogger *syslog.Logger) *BrandingHandler {
	return &BrandingHandler{db: database, store: store, syslog: sysLogger}
}

func (h *BrandingHandler) SetAuthProviders(providers map[string]bool) { h.authProviders = providers }

// ---------- Public endpoints ----------

// GetBranding returns the full branding config for the frontend (public, no auth).
func (h *BrandingHandler) GetBranding(w http.ResponseWriter, r *http.Request) {
	var cfg models.BrandingConfig
	err := h.db.BrandingConfig().FindOne(r.Context(), bson.M{}).Decode(&cfg)
	if err == mongo.ErrNoDocuments {
		// Return defaults
		cfg = defaultBrandingConfig(h.store.Get("app.name"))
	} else if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to load branding config")
		return
	}

	analyticsSnippet := h.store.Get("analytics.head_snippet")

	// Build logo/favicon URLs if assets exist
	logoURL := ""
	faviconURL := ""
	var logoAsset models.BrandingAsset
	if err := h.db.BrandingAssets().FindOne(r.Context(), bson.M{"key": "logo"}).Decode(&logoAsset); err == nil {
		logoURL = "/api/branding/asset/logo"
	}
	var favAsset models.BrandingAsset
	if err := h.db.BrandingAssets().FindOne(r.Context(), bson.M{"key": "favicon"}).Decode(&favAsset); err == nil {
		faviconURL = "/api/branding/asset/favicon"
	}

	// Build auth providers from static config + runtime config store
	authProviders := map[string]bool{"password": true}
	if h.authProviders != nil {
		for k, v := range h.authProviders {
			authProviders[k] = v
		}
	}
	authProviders["magicLink"] = h.store.Get("auth.magic_link.enabled") == "true"
	authProviders["passkeys"] = h.store.Get("auth.passkeys.enabled") == "true"

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"appName":          cfg.AppName,
		"tagline":          cfg.Tagline,
		"logoMode":         cfg.LogoMode,
		"logoUrl":          logoURL,
		"faviconUrl":       faviconURL,
		"primaryColor":     cfg.PrimaryColor,
		"accentColor":      cfg.AccentColor,
		"backgroundColor":  cfg.BackgroundColor,
		"surfaceColor":     cfg.SurfaceColor,
		"textColor":        cfg.TextColor,
		"fontFamily":       cfg.FontFamily,
		"headingFont":      cfg.HeadingFont,
		"landingEnabled":   cfg.LandingEnabled,
		"landingTitle":     cfg.LandingTitle,
		"landingMeta":      cfg.LandingMeta,
		"landingHtml":      cfg.LandingHTML,
		"dashboardHtml":    cfg.DashboardHTML,
		"loginHeading":     cfg.LoginHeading,
		"loginSubtext":     cfg.LoginSubtext,
		"signupHeading":    cfg.SignupHeading,
		"signupSubtext":    cfg.SignupSubtext,
		"customCss":        cfg.CustomCSS,
		"headHtml":         cfg.HeadHTML,
		"ogImageUrl":       cfg.OgImageURL,
		"navItems":         cfg.NavItems,
		"analyticsSnippet": analyticsSnippet,
		"authProviders":    authProviders,
	})
}

// ServeAsset serves a branding asset (logo, favicon) by key.
func (h *BrandingHandler) ServeAsset(w http.ResponseWriter, r *http.Request) {
	key := mux.Vars(r)["key"]
	if key != "logo" && key != "favicon" {
		http.NotFound(w, r)
		return
	}

	var asset models.BrandingAsset
	err := h.db.BrandingAssets().FindOne(r.Context(), bson.M{"key": key}).Decode(&asset)
	if err == mongo.ErrNoDocuments {
		http.NotFound(w, r)
		return
	} else if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", asset.ContentType)
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Write(asset.Data)
}

// ServeMedia serves a media library file by ID.
func (h *BrandingHandler) ServeMedia(w http.ResponseWriter, r *http.Request) {
	key := mux.Vars(r)["id"]

	var asset models.BrandingAsset
	err := h.db.BrandingAssets().FindOne(r.Context(), bson.M{"key": key}).Decode(&asset)
	if err == mongo.ErrNoDocuments {
		http.NotFound(w, r)
		return
	} else if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", asset.ContentType)
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Write(asset.Data)
}

// GetPublicPage returns a published custom page by slug.
func (h *BrandingHandler) GetPublicPage(w http.ResponseWriter, r *http.Request) {
	slug := mux.Vars(r)["slug"]

	var page models.CustomPage
	err := h.db.CustomPages().FindOne(r.Context(), bson.M{"slug": slug, "isPublished": true}).Decode(&page)
	if err == mongo.ErrNoDocuments {
		http.NotFound(w, r)
		return
	} else if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to load page")
		return
	}

	respondWithJSON(w, http.StatusOK, page)
}

// ListPublicPages returns published custom pages (slug + title only).
func (h *BrandingHandler) ListPublicPages(w http.ResponseWriter, r *http.Request) {
	opts := options.Find().SetSort(bson.D{{Key: "sortOrder", Value: 1}}).SetProjection(bson.M{
		"slug":      1,
		"title":     1,
		"sortOrder": 1,
	})
	cursor, err := h.db.CustomPages().Find(r.Context(), bson.M{"isPublished": true}, opts)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to list pages")
		return
	}
	var pages []models.CustomPage
	if err := cursor.All(r.Context(), &pages); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to decode pages")
		return
	}
	if pages == nil {
		pages = []models.CustomPage{}
	}
	respondWithJSON(w, http.StatusOK, map[string]interface{}{"pages": pages})
}

// ---------- Admin endpoints ----------

// UpdateBranding updates the branding config.
func (h *BrandingHandler) UpdateBranding(w http.ResponseWriter, r *http.Request) {
	var req models.BrandingConfig
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	now := time.Now()
	update := bson.M{
		"$set": bson.M{
			"appName":         req.AppName,
			"tagline":         req.Tagline,
			"logoMode":        req.LogoMode,
			"primaryColor":    req.PrimaryColor,
			"accentColor":     req.AccentColor,
			"backgroundColor": req.BackgroundColor,
			"surfaceColor":    req.SurfaceColor,
			"textColor":       req.TextColor,
			"fontFamily":      req.FontFamily,
			"headingFont":     req.HeadingFont,
			"landingEnabled":  req.LandingEnabled,
			"landingTitle":    req.LandingTitle,
			"landingMeta":     req.LandingMeta,
			"landingHtml":     req.LandingHTML,
			"dashboardHtml":   req.DashboardHTML,
			"loginHeading":    req.LoginHeading,
			"loginSubtext":    req.LoginSubtext,
			"signupHeading":   req.SignupHeading,
			"signupSubtext":   req.SignupSubtext,
			"customCss":       req.CustomCSS,
			"headHtml":        req.HeadHTML,
			"ogImageUrl":      req.OgImageURL,
			"navItems":        req.NavItems,
			"updatedAt":       now,
		},
		"$setOnInsert": bson.M{
			"createdAt": now,
		},
	}

	opts := options.Update().SetUpsert(true)
	_, err := h.db.BrandingConfig().UpdateOne(r.Context(), bson.M{}, update, opts)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to update branding config")
		return
	}

	h.syslog.Critical(r.Context(), "Branding configuration updated")
	respondWithJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// UploadAsset handles logo/favicon uploads (multipart).
func (h *BrandingHandler) UploadAsset(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(maxAssetSize); err != nil {
		respondWithError(w, http.StatusBadRequest, "File too large (max 5MB)")
		return
	}

	key := r.FormValue("key")
	if key != "logo" && key != "favicon" {
		respondWithError(w, http.StatusBadRequest, "Key must be 'logo' or 'favicon'")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Missing file upload")
		return
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, maxAssetSize+1))
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to read file")
		return
	}
	if int64(len(data)) > maxAssetSize {
		respondWithError(w, http.StatusBadRequest, "File too large (max 5MB)")
		return
	}

	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = http.DetectContentType(data)
	}

	// Validate image types
	if !strings.HasPrefix(contentType, "image/") {
		respondWithError(w, http.StatusBadRequest, "Only image files are allowed for logo/favicon")
		return
	}

	now := time.Now()
	asset := models.BrandingAsset{
		Key:         key,
		Filename:    header.Filename,
		ContentType: contentType,
		Data:        data,
		Size:        int64(len(data)),
		CreatedAt:   now,
	}

	opts := options.Update().SetUpsert(true)
	_, err = h.db.BrandingAssets().UpdateOne(r.Context(), bson.M{"key": key}, bson.M{"$set": asset}, opts)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to save asset")
		return
	}

	h.syslog.Critical(r.Context(), fmt.Sprintf("Branding asset uploaded: %s (%s)", key, header.Filename))
	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"key":         key,
		"filename":    header.Filename,
		"contentType": contentType,
		"size":        len(data),
		"url":         fmt.Sprintf("/api/branding/asset/%s", key),
	})
}

// DeleteAsset removes a branding asset (logo or favicon).
func (h *BrandingHandler) DeleteAsset(w http.ResponseWriter, r *http.Request) {
	key := mux.Vars(r)["key"]
	if key != "logo" && key != "favicon" {
		respondWithError(w, http.StatusBadRequest, "Key must be 'logo' or 'favicon'")
		return
	}

	_, err := h.db.BrandingAssets().DeleteOne(r.Context(), bson.M{"key": key})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to delete asset")
		return
	}

	h.syslog.Critical(r.Context(), fmt.Sprintf("Branding asset deleted: %s", key))
	respondWithJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ---------- Media library ----------

// ListMedia lists all media library assets.
func (h *BrandingHandler) ListMedia(w http.ResponseWriter, r *http.Request) {
	// Exclude logo and favicon from media list
	filter := bson.M{"key": bson.M{"$nin": []string{"logo", "favicon"}}}
	opts := options.Find().
		SetSort(bson.D{{Key: "createdAt", Value: -1}}).
		SetProjection(bson.M{"data": 0}) // Don't return binary data in list

	cursor, err := h.db.BrandingAssets().Find(r.Context(), filter, opts)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to list media")
		return
	}
	var assets []models.BrandingAsset
	if err := cursor.All(r.Context(), &assets); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to decode media")
		return
	}
	if assets == nil {
		assets = []models.BrandingAsset{}
	}

	// Add URL to each asset
	type mediaItem struct {
		ID          string `json:"id"`
		Key         string `json:"key"`
		Filename    string `json:"filename"`
		ContentType string `json:"contentType"`
		Size        int64  `json:"size"`
		URL         string `json:"url"`
		CreatedAt   string `json:"createdAt"`
	}
	items := make([]mediaItem, len(assets))
	for i, a := range assets {
		items[i] = mediaItem{
			ID:          a.ID.Hex(),
			Key:         a.Key,
			Filename:    a.Filename,
			ContentType: a.ContentType,
			Size:        a.Size,
			URL:         fmt.Sprintf("/api/branding/media/%s", a.Key),
			CreatedAt:   a.CreatedAt.Format(time.RFC3339),
		}
	}
	respondWithJSON(w, http.StatusOK, map[string]interface{}{"media": items})
}

// UploadMedia handles media file uploads.
func (h *BrandingHandler) UploadMedia(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(maxMediaSize); err != nil {
		respondWithError(w, http.StatusBadRequest, "File too large (max 10MB)")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Missing file upload")
		return
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, maxMediaSize+1))
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to read file")
		return
	}
	if int64(len(data)) > maxMediaSize {
		respondWithError(w, http.StatusBadRequest, "File too large (max 10MB)")
		return
	}

	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = http.DetectContentType(data)
	}

	// Validate allowed types
	allowed := strings.HasPrefix(contentType, "image/") ||
		contentType == "application/pdf" ||
		contentType == "image/svg+xml"
	if !allowed {
		respondWithError(w, http.StatusBadRequest, "Only image and PDF files are allowed")
		return
	}

	// Generate unique key from ID
	id := primitive.NewObjectID()
	key := fmt.Sprintf("media_%s", id.Hex())

	now := time.Now()
	asset := models.BrandingAsset{
		ID:          id,
		Key:         key,
		Filename:    header.Filename,
		ContentType: contentType,
		Data:        data,
		Size:        int64(len(data)),
		CreatedAt:   now,
	}

	_, err = h.db.BrandingAssets().InsertOne(r.Context(), asset)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to save media")
		return
	}

	h.syslog.Low(r.Context(), fmt.Sprintf("Media uploaded: %s (%s, %d bytes)", header.Filename, contentType, len(data)))
	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"id":          id.Hex(),
		"key":         key,
		"filename":    header.Filename,
		"contentType": contentType,
		"size":        len(data),
		"url":         fmt.Sprintf("/api/branding/media/%s", key),
	})
}

// DeleteMedia deletes a media file.
func (h *BrandingHandler) DeleteMedia(w http.ResponseWriter, r *http.Request) {
	key := mux.Vars(r)["id"]

	// Prevent deleting logo/favicon through media endpoint
	if key == "logo" || key == "favicon" {
		respondWithError(w, http.StatusBadRequest, "Use the asset endpoint to manage logo/favicon")
		return
	}

	result, err := h.db.BrandingAssets().DeleteOne(r.Context(), bson.M{"key": key})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to delete media")
		return
	}
	if result.DeletedCount == 0 {
		respondWithError(w, http.StatusNotFound, "Media not found")
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ---------- Custom pages (admin) ----------

// AdminListPages lists all custom pages including unpublished.
func (h *BrandingHandler) AdminListPages(w http.ResponseWriter, r *http.Request) {
	opts := options.Find().SetSort(bson.D{{Key: "sortOrder", Value: 1}})
	cursor, err := h.db.CustomPages().Find(r.Context(), bson.M{}, opts)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to list pages")
		return
	}
	var pages []models.CustomPage
	if err := cursor.All(r.Context(), &pages); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to decode pages")
		return
	}
	if pages == nil {
		pages = []models.CustomPage{}
	}
	respondWithJSON(w, http.StatusOK, map[string]interface{}{"pages": pages})
}

// CreatePage creates a new custom page.
func (h *BrandingHandler) CreatePage(w http.ResponseWriter, r *http.Request) {
	var page models.CustomPage
	if err := json.NewDecoder(r.Body).Decode(&page); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	if page.Slug == "" || page.Title == "" {
		respondWithError(w, http.StatusBadRequest, "Slug and title are required")
		return
	}

	// Normalize slug
	page.Slug = strings.TrimPrefix(page.Slug, "/")
	page.Slug = strings.ToLower(strings.TrimSpace(page.Slug))

	now := time.Now()
	page.ID = primitive.NewObjectID()
	page.CreatedAt = now
	page.UpdatedAt = now

	_, err := h.db.CustomPages().InsertOne(r.Context(), page)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			respondWithError(w, http.StatusConflict, "A page with this slug already exists")
			return
		}
		respondWithError(w, http.StatusInternalServerError, "Failed to create page")
		return
	}

	h.syslog.Critical(r.Context(), fmt.Sprintf("Custom page created: %s (%s)", page.Title, page.Slug))
	respondWithJSON(w, http.StatusCreated, page)
}

// UpdatePage updates a custom page.
func (h *BrandingHandler) UpdatePage(w http.ResponseWriter, r *http.Request) {
	id, err := primitive.ObjectIDFromHex(mux.Vars(r)["id"])
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid page ID")
		return
	}

	var req models.CustomPage
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	if req.Slug != "" {
		req.Slug = strings.TrimPrefix(req.Slug, "/")
		req.Slug = strings.ToLower(strings.TrimSpace(req.Slug))
	}

	now := time.Now()
	update := bson.M{
		"$set": bson.M{
			"slug":            req.Slug,
			"title":           req.Title,
			"htmlBody":        req.HTMLBody,
			"metaDescription": req.MetaDescription,
			"ogImage":         req.OgImage,
			"isPublished":     req.IsPublished,
			"sortOrder":       req.SortOrder,
			"updatedAt":       now,
		},
	}

	result, err := h.db.CustomPages().UpdateByID(r.Context(), id, update)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			respondWithError(w, http.StatusConflict, "A page with this slug already exists")
			return
		}
		respondWithError(w, http.StatusInternalServerError, "Failed to update page")
		return
	}
	if result.MatchedCount == 0 {
		respondWithError(w, http.StatusNotFound, "Page not found")
		return
	}

	h.syslog.Critical(r.Context(), fmt.Sprintf("Custom page updated: %s", req.Title))
	respondWithJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// DeletePage deletes a custom page.
func (h *BrandingHandler) DeletePage(w http.ResponseWriter, r *http.Request) {
	id, err := primitive.ObjectIDFromHex(mux.Vars(r)["id"])
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid page ID")
		return
	}

	result, err := h.db.CustomPages().DeleteOne(r.Context(), bson.M{"_id": id})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to delete page")
		return
	}
	if result.DeletedCount == 0 {
		respondWithError(w, http.StatusNotFound, "Page not found")
		return
	}

	h.syslog.Critical(r.Context(), "Custom page deleted")
	respondWithJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ---------- Helpers ----------

func defaultBrandingConfig(appName string) models.BrandingConfig {
	if appName == "" {
		appName = "LastSaaS"
	}
	return models.BrandingConfig{
		AppName:  appName,
		LogoMode: "text",
		NavItems: []models.NavItem{
			{ID: "dashboard", Label: "Dashboard", Icon: "LayoutDashboard", Target: "/dashboard", IsBuiltIn: true, Visible: true, SortOrder: 0},
			{ID: "team", Label: "Team", Icon: "Users", Target: "/team", IsBuiltIn: true, Visible: true, SortOrder: 1},
			{ID: "plan", Label: "Plan", Icon: "CreditCard", Target: "/plan", IsBuiltIn: true, Visible: true, SortOrder: 2},
			{ID: "settings", Label: "Settings", Icon: "Settings", Target: "/settings", IsBuiltIn: true, Visible: true, SortOrder: 3},
		},
	}
}
