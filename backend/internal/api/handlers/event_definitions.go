package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"lastsaas/internal/db"
	"lastsaas/internal/models"
	"lastsaas/internal/syslog"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var validDefName = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

// EventDefinitionsHandler manages event definition CRUD and Sankey data.
type EventDefinitionsHandler struct {
	db     *db.MongoDB
	syslog *syslog.Logger
}

// NewEventDefinitionsHandler creates a new EventDefinitionsHandler.
func NewEventDefinitionsHandler(database *db.MongoDB, sysLogger *syslog.Logger) *EventDefinitionsHandler {
	return &EventDefinitionsHandler{
		db:     database,
		syslog: sysLogger,
	}
}

type eventDefRequest struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	ParentID    *string `json:"parentId"`
}

// ListEventDefinitions returns all event definitions with telemetry counts.
func (h *EventDefinitionsHandler) ListEventDefinitions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	cursor, err := h.db.EventDefinitions().Find(ctx, bson.M{}, options.Find().SetSort(bson.D{{Key: "name", Value: 1}}))
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to list event definitions")
		return
	}
	defer cursor.Close(ctx)

	var defs []models.EventDefinition
	if err := cursor.All(ctx, &defs); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to decode event definitions")
		return
	}
	if defs == nil {
		defs = []models.EventDefinition{}
	}

	// Gather telemetry counts for each definition in the time range.
	start, end := parsePMTimeRange(r)
	names := make([]string, len(defs))
	for i, d := range defs {
		names[i] = d.Name
	}

	counts := map[string]int64{}
	if len(names) > 0 {
		pipeline := mongo.Pipeline{
			{{Key: "$match", Value: bson.M{
				"eventName": bson.M{"$in": names},
				"createdAt": bson.M{"$gte": start, "$lte": end},
			}}},
			{{Key: "$group", Value: bson.M{
				"_id":   "$eventName",
				"count": bson.M{"$sum": 1},
			}}},
		}
		cur, err := h.db.TelemetryEvents().Aggregate(ctx, pipeline)
		if err == nil {
			defer cur.Close(ctx)
			for cur.Next(ctx) {
				var result struct {
					Name  string `bson:"_id"`
					Count int64  `bson:"count"`
				}
				if cur.Decode(&result) == nil {
					counts[result.Name] = result.Count
				}
			}
		}
	}

	type defResponse struct {
		models.EventDefinition `json:",inline"`
		Count                  int64 `json:"count"`
	}
	resp := make([]defResponse, len(defs))
	for i, d := range defs {
		resp[i] = defResponse{EventDefinition: d, Count: counts[d.Name]}
	}

	respondWithJSON(w, http.StatusOK, map[string]interface{}{"definitions": resp})
}

// CreateEventDefinition creates a new event definition.
func (h *EventDefinitionsHandler) CreateEventDefinition(w http.ResponseWriter, r *http.Request) {
	var req eventDefRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	req.Description = strings.TrimSpace(req.Description)

	if req.Name == "" || len(req.Name) > 128 || !validDefName.MatchString(req.Name) {
		respondWithError(w, http.StatusBadRequest, "Invalid name: use alphanumeric, dots, underscores, hyphens (1-128 chars)")
		return
	}
	if len(req.Description) > 256 {
		respondWithError(w, http.StatusBadRequest, "Description must be 256 characters or less")
		return
	}

	ctx := r.Context()

	// Check uniqueness.
	count, _ := h.db.EventDefinitions().CountDocuments(ctx, bson.M{"name": req.Name})
	if count > 0 {
		respondWithError(w, http.StatusConflict, "An event definition with this name already exists")
		return
	}

	now := time.Now()
	def := models.EventDefinition{
		ID:          primitive.NewObjectID(),
		Name:        req.Name,
		Description: req.Description,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if req.ParentID != nil && *req.ParentID != "" {
		parentID, err := primitive.ObjectIDFromHex(*req.ParentID)
		if err != nil {
			respondWithError(w, http.StatusBadRequest, "Invalid parent ID")
			return
		}
		// Verify parent exists.
		count, _ := h.db.EventDefinitions().CountDocuments(ctx, bson.M{"_id": parentID})
		if count == 0 {
			respondWithError(w, http.StatusBadRequest, "Parent event definition not found")
			return
		}
		def.ParentID = &parentID
	}

	if _, err := h.db.EventDefinitions().InsertOne(ctx, def); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to create event definition")
		return
	}

	h.syslog.Log(ctx, models.LogMedium, fmt.Sprintf("Event definition created: %s", def.Name))
	respondWithJSON(w, http.StatusCreated, def)
}

// UpdateEventDefinition updates an existing event definition.
func (h *EventDefinitionsHandler) UpdateEventDefinition(w http.ResponseWriter, r *http.Request) {
	defID, err := primitive.ObjectIDFromHex(mux.Vars(r)["defId"])
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid definition ID")
		return
	}

	var req eventDefRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	req.Description = strings.TrimSpace(req.Description)

	if req.Name == "" || len(req.Name) > 128 || !validDefName.MatchString(req.Name) {
		respondWithError(w, http.StatusBadRequest, "Invalid name: use alphanumeric, dots, underscores, hyphens (1-128 chars)")
		return
	}
	if len(req.Description) > 256 {
		respondWithError(w, http.StatusBadRequest, "Description must be 256 characters or less")
		return
	}

	ctx := r.Context()

	// Verify definition exists.
	var existing models.EventDefinition
	if err := h.db.EventDefinitions().FindOne(ctx, bson.M{"_id": defID}).Decode(&existing); err != nil {
		respondWithError(w, http.StatusNotFound, "Event definition not found")
		return
	}

	// Check uniqueness if name changed.
	if req.Name != existing.Name {
		count, _ := h.db.EventDefinitions().CountDocuments(ctx, bson.M{"name": req.Name, "_id": bson.M{"$ne": defID}})
		if count > 0 {
			respondWithError(w, http.StatusConflict, "An event definition with this name already exists")
			return
		}
	}

	update := bson.M{
		"$set": bson.M{
			"name":        req.Name,
			"description": req.Description,
			"updatedAt":   time.Now(),
		},
	}

	if req.ParentID != nil && *req.ParentID != "" {
		parentID, err := primitive.ObjectIDFromHex(*req.ParentID)
		if err != nil {
			respondWithError(w, http.StatusBadRequest, "Invalid parent ID")
			return
		}
		if parentID == defID {
			respondWithError(w, http.StatusBadRequest, "An event cannot be its own parent")
			return
		}
		// Verify parent exists.
		count, _ := h.db.EventDefinitions().CountDocuments(ctx, bson.M{"_id": parentID})
		if count == 0 {
			respondWithError(w, http.StatusBadRequest, "Parent event definition not found")
			return
		}
		// Circular dependency check.
		if h.wouldCreateCycle(ctx, defID, parentID) {
			respondWithError(w, http.StatusBadRequest, "Circular dependency detected")
			return
		}
		update["$set"].(bson.M)["parentId"] = parentID
	} else {
		// Remove parent.
		update["$unset"] = bson.M{"parentId": ""}
	}

	if _, err := h.db.EventDefinitions().UpdateOne(ctx, bson.M{"_id": defID}, update); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to update event definition")
		return
	}

	h.syslog.Log(ctx, models.LogMedium, fmt.Sprintf("Event definition updated: %s", req.Name))

	// Return updated document.
	var updated models.EventDefinition
	h.db.EventDefinitions().FindOne(ctx, bson.M{"_id": defID}).Decode(&updated)
	respondWithJSON(w, http.StatusOK, updated)
}

// DeleteEventDefinition deletes an event definition and orphans any children.
func (h *EventDefinitionsHandler) DeleteEventDefinition(w http.ResponseWriter, r *http.Request) {
	defID, err := primitive.ObjectIDFromHex(mux.Vars(r)["defId"])
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid definition ID")
		return
	}

	ctx := r.Context()

	var existing models.EventDefinition
	if err := h.db.EventDefinitions().FindOne(ctx, bson.M{"_id": defID}).Decode(&existing); err != nil {
		respondWithError(w, http.StatusNotFound, "Event definition not found")
		return
	}

	// Orphan children by removing their parentId.
	h.db.EventDefinitions().UpdateMany(ctx, bson.M{"parentId": defID}, bson.M{
		"$unset": bson.M{"parentId": ""},
		"$set":   bson.M{"updatedAt": time.Now()},
	})

	if _, err := h.db.EventDefinitions().DeleteOne(ctx, bson.M{"_id": defID}); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to delete event definition")
		return
	}

	h.syslog.Log(ctx, models.LogMedium, fmt.Sprintf("Event definition deleted: %s", existing.Name))
	respondWithJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// GetSankeyData computes the Sankey visualization data from event definitions and telemetry counts.
func (h *EventDefinitionsHandler) GetSankeyData(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Load all event definitions.
	cursor, err := h.db.EventDefinitions().Find(ctx, bson.M{})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to load event definitions")
		return
	}
	defer cursor.Close(ctx)

	var allDefs []models.EventDefinition
	if err := cursor.All(ctx, &allDefs); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to decode event definitions")
		return
	}

	// Check if any definition has a parent (i.e., dependencies exist).
	hasDeps := false
	defByID := map[primitive.ObjectID]models.EventDefinition{}
	childrenOf := map[primitive.ObjectID]bool{} // IDs that are parents
	for _, d := range allDefs {
		defByID[d.ID] = d
		if d.ParentID != nil {
			hasDeps = true
			childrenOf[*d.ParentID] = true
		}
	}

	if !hasDeps {
		respondWithJSON(w, http.StatusOK, map[string]interface{}{
			"nodes":           []interface{}{},
			"links":           []interface{}{},
			"hasDependencies": false,
		})
		return
	}

	// Collect definitions that participate in at least one dependency edge.
	participatingIDs := map[primitive.ObjectID]bool{}
	for _, d := range allDefs {
		if d.ParentID != nil {
			participatingIDs[d.ID] = true
			participatingIDs[*d.ParentID] = true
		}
	}

	// Build ordered node list (name + ID tracking for count lookup).
	type sankeyNode struct {
		Name  string `json:"name"`
		Count int64  `json:"count"`
	}
	var nodes []sankeyNode
	nodeIndex := map[primitive.ObjectID]int{}
	nodeDefIDs := []primitive.ObjectID{} // parallel to nodes for ID lookup
	for _, d := range allDefs {
		if participatingIDs[d.ID] {
			nodeIndex[d.ID] = len(nodes)
			nodes = append(nodes, sankeyNode{Name: d.Name})
			nodeDefIDs = append(nodeDefIDs, d.ID)
		}
	}

	// Get telemetry counts for participating event names in time range.
	start, end := parsePMTimeRange(r)
	participatingNames := make([]string, 0, len(nodes))
	for _, n := range nodes {
		participatingNames = append(participatingNames, n.Name)
	}

	counts := map[string]int64{}
	if len(participatingNames) > 0 {
		pipeline := mongo.Pipeline{
			{{Key: "$match", Value: bson.M{
				"eventName": bson.M{"$in": participatingNames},
				"createdAt": bson.M{"$gte": start, "$lte": end},
			}}},
			{{Key: "$group", Value: bson.M{
				"_id":   "$eventName",
				"count": bson.M{"$sum": 1},
			}}},
		}
		cur, err := h.db.TelemetryEvents().Aggregate(ctx, pipeline)
		if err == nil {
			defer cur.Close(ctx)
			for cur.Next(ctx) {
				var result struct {
					Name  string `bson:"_id"`
					Count int64  `bson:"count"`
				}
				if cur.Decode(&result) == nil {
					counts[result.Name] = result.Count
				}
			}
		}
	}

	// Populate node counts.
	for i := range nodes {
		nodes[i].Count = counts[nodes[i].Name]
	}

	// Build links: parent → child with value = child event count.
	type sankeyLink struct {
		Source int   `json:"source"`
		Target int   `json:"target"`
		Value  int64 `json:"value"`
	}
	var links []sankeyLink
	for _, d := range allDefs {
		if d.ParentID == nil {
			continue
		}
		srcIdx, srcOk := nodeIndex[*d.ParentID]
		tgtIdx, tgtOk := nodeIndex[d.ID]
		if !srcOk || !tgtOk {
			continue
		}
		value := counts[d.Name]
		if value == 0 {
			value = 1 // Minimum value so the link is visible in the Sankey.
		}
		links = append(links, sankeyLink{Source: srcIdx, Target: tgtIdx, Value: value})
	}

	if links == nil {
		links = []sankeyLink{}
	}

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"nodes":           nodes,
		"links":           links,
		"hasDependencies": true,
	})
}

// wouldCreateCycle checks if setting proposedParentID as the parent of defID would create a cycle.
func (h *EventDefinitionsHandler) wouldCreateCycle(ctx context.Context, defID, proposedParentID primitive.ObjectID) bool {
	visited := map[primitive.ObjectID]bool{defID: true}
	current := proposedParentID
	for {
		if visited[current] {
			return true
		}
		visited[current] = true
		var parent models.EventDefinition
		err := h.db.EventDefinitions().FindOne(ctx, bson.M{"_id": current}).Decode(&parent)
		if err != nil || parent.ParentID == nil {
			return false
		}
		current = *parent.ParentID
	}
}
