package scanner

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const trackedStoresCollection = "tracked_stores"

// TrackedStoreStore handles MongoDB persistence for tracked stores.
type TrackedStoreStore struct {
	col    *mongo.Collection
	scans  *mongo.Collection // scans collection for history queries
}

// NewTrackedStoreStore creates a TrackedStoreStore backed by the given MongoDB database.
func NewTrackedStoreStore(database *mongo.Database) *TrackedStoreStore {
	col := database.Collection(trackedStoresCollection)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	_, _ = col.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "tenantId", Value: 1}, {Key: "addedAt", Value: -1}},
			Options: options.Index(),
		},
		{
			// Unique: one tenant can't track the same domain twice
			Keys:    bson.D{{Key: "tenantId", Value: 1}, {Key: "domain", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
	})

	return &TrackedStoreStore{
		col:   col,
		scans: database.Collection(collectionName),
	}
}

// AddTrackedStore inserts a new tracked store for the given tenant.
// Returns the created document. Does not enforce entitlement limits (caller must).
func (s *TrackedStoreStore) AddTrackedStore(ctx context.Context, tenantID primitive.ObjectID, domain string) (*TrackedStore, error) {
	domain = sanitiseDomain(domain)
	if domain == "" {
		return nil, fmt.Errorf("empty or invalid domain")
	}

	doc := TrackedStore{
		ID:       primitive.NewObjectID(),
		Domain:   domain,
		TenantID: tenantID,
		AddedAt:  time.Now().UTC(),
		Trend:    "stable",
	}

	// Seed with latest scan data if available
	opts := options.FindOne().SetSort(bson.D{{Key: "createdAt", Value: -1}})
	var latest StoredScan
	if err := s.scans.FindOne(ctx, bson.M{"domain": domain}, opts).Decode(&latest); err == nil {
		doc.CurrentScore = latest.CompositeScore
		now := latest.CreatedAt
		doc.LastScannedAt = &now
	}

	_, err := s.col.InsertOne(ctx, doc)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return nil, fmt.Errorf("store %q is already tracked", domain)
		}
		return nil, fmt.Errorf("insert tracked store: %w", err)
	}
	return &doc, nil
}

// RemoveTrackedStore deletes a tracked store by ID, scoped to the tenant.
func (s *TrackedStoreStore) RemoveTrackedStore(ctx context.Context, tenantID primitive.ObjectID, storeID primitive.ObjectID) error {
	res, err := s.col.DeleteOne(ctx, bson.M{"_id": storeID, "tenantId": tenantID})
	if err != nil {
		return fmt.Errorf("delete tracked store: %w", err)
	}
	if res.DeletedCount == 0 {
		return fmt.Errorf("tracked store not found")
	}
	return nil
}

// ListTrackedStores returns all tracked stores for a tenant.
func (s *TrackedStoreStore) ListTrackedStores(ctx context.Context, tenantID primitive.ObjectID) ([]TrackedStore, error) {
	opts := options.Find().SetSort(bson.D{{Key: "addedAt", Value: -1}})
	cursor, err := s.col.Find(ctx, bson.M{"tenantId": tenantID}, opts)
	if err != nil {
		return nil, fmt.Errorf("list tracked stores: %w", err)
	}
	defer cursor.Close(ctx)

	var stores []TrackedStore
	if err := cursor.All(ctx, &stores); err != nil {
		return nil, fmt.Errorf("decode tracked stores: %w", err)
	}
	if stores == nil {
		stores = []TrackedStore{}
	}
	return stores, nil
}

// GetTrackedStore retrieves a single tracked store by ID, scoped to the tenant.
func (s *TrackedStoreStore) GetTrackedStore(ctx context.Context, tenantID primitive.ObjectID, storeID primitive.ObjectID) (*TrackedStore, error) {
	var store TrackedStore
	err := s.col.FindOne(ctx, bson.M{"_id": storeID, "tenantId": tenantID}).Decode(&store)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get tracked store: %w", err)
	}
	return &store, nil
}

// CountTrackedStores returns the number of tracked stores for a tenant.
func (s *TrackedStoreStore) CountTrackedStores(ctx context.Context, tenantID primitive.ObjectID) (int64, error) {
	n, err := s.col.CountDocuments(ctx, bson.M{"tenantId": tenantID})
	if err != nil {
		return 0, fmt.Errorf("count tracked stores: %w", err)
	}
	return n, nil
}

// GetScanHistory returns the most recent scans for a domain (for history view).
// Scoped to the tenant for security.
func (s *TrackedStoreStore) GetScanHistory(ctx context.Context, tenantID primitive.ObjectID, domain string, limit int) ([]StoredScan, error) {
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	opts := options.Find().
		SetSort(bson.D{{Key: "createdAt", Value: -1}}).
		SetLimit(int64(limit))

	// Return scans that either belong to this tenant OR have no tenant (public scans)
	filter := bson.M{
		"domain": domain,
		"$or": []bson.M{
			{"tenantId": tenantID},
			{"tenantId": bson.M{"$exists": false}},
			{"tenantId": nil},
		},
	}

	cursor, err := s.scans.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("list scan history: %w", err)
	}
	defer cursor.Close(ctx)

	var scans []StoredScan
	if err := cursor.All(ctx, &scans); err != nil {
		return nil, fmt.Errorf("decode scan history: %w", err)
	}
	if scans == nil {
		scans = []StoredScan{}
	}
	return scans, nil
}

// UpdateTrackedStoreScore updates the score fields after a new scan for this domain.
func (s *TrackedStoreStore) UpdateTrackedStoreScore(ctx context.Context, tenantID primitive.ObjectID, domain string, newScore int, scannedAt time.Time) error {
	// Find the current tracked store to determine previous score
	var current TrackedStore
	err := s.col.FindOne(ctx, bson.M{"tenantId": tenantID, "domain": domain}).Decode(&current)
	if err == mongo.ErrNoDocuments {
		return nil // not tracked by this tenant, no-op
	}
	if err != nil {
		return fmt.Errorf("find tracked store for update: %w", err)
	}

	trend := "stable"
	if newScore > current.CurrentScore {
		trend = "up"
	} else if newScore < current.CurrentScore {
		trend = "down"
	}

	_, err = s.col.UpdateOne(ctx,
		bson.M{"tenantId": tenantID, "domain": domain},
		bson.M{"$set": bson.M{
			"previousScore": current.CurrentScore,
			"currentScore":  newScore,
			"trend":         trend,
			"lastScannedAt": scannedAt,
		}},
	)
	return err
}
