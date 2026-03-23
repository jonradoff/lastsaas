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

const collectionName = "scans"

// Store handles MongoDB persistence for scan results.
type Store struct {
	col *mongo.Collection
}

// NewStore creates a Store backed by the given MongoDB database.
func NewStore(database *mongo.Database) *Store {
	col := database.Collection(collectionName)

	// Ensure indexes (best-effort — ignore errors on startup).
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	_, _ = col.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "domain", Value: 1}, {Key: "createdAt", Value: -1}},
			Options: options.Index(),
		},
		{
			Keys:    bson.D{{Key: "tenantId", Value: 1}, {Key: "createdAt", Value: -1}},
			Options: options.Index().SetSparse(true),
		},
		{
			Keys:    bson.D{{Key: "createdAt", Value: -1}},
			Options: options.Index(),
		},
	})

	return &Store{col: col}
}

// SaveScan inserts a new scan document and returns its generated ID.
func (s *Store) SaveScan(ctx context.Context, domain string, result *ScanResult, tenantID *primitive.ObjectID) (primitive.ObjectID, error) {
	doc := StoredScan{
		ID:         primitive.NewObjectID(),
		Domain:     domain,
		CreatedAt:  time.Now().UTC(),
		TenantID:   tenantID,
		ScanResult: *result,
	}
	_, err := s.col.InsertOne(ctx, doc)
	if err != nil {
		return primitive.NilObjectID, fmt.Errorf("insert scan: %w", err)
	}
	return doc.ID, nil
}

// GetScan retrieves a scan by its ObjectID hex string.
func (s *Store) GetScan(ctx context.Context, scanID string) (*StoredScan, error) {
	oid, err := primitive.ObjectIDFromHex(scanID)
	if err != nil {
		return nil, fmt.Errorf("invalid scan ID: %w", err)
	}
	var scan StoredScan
	if err := s.col.FindOne(ctx, bson.M{"_id": oid}).Decode(&scan); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, fmt.Errorf("get scan: %w", err)
	}
	return &scan, nil
}

// GetLatestScan returns the most recent scan for a domain, or nil if none exists.
func (s *Store) GetLatestScan(ctx context.Context, domain string) (*StoredScan, error) {
	opts := options.FindOne().SetSort(bson.D{{Key: "createdAt", Value: -1}})
	var scan StoredScan
	if err := s.col.FindOne(ctx, bson.M{"domain": domain}, opts).Decode(&scan); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, fmt.Errorf("get latest scan: %w", err)
	}
	return &scan, nil
}

// ListScans returns a paginated list of scans for a tenant.
func (s *Store) ListScans(ctx context.Context, tenantID primitive.ObjectID, page, limit int) ([]StoredScan, int64, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if page <= 0 {
		page = 1
	}
	skip := int64((page - 1) * limit)

	filter := bson.M{"tenantId": tenantID}
	total, err := s.col.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, fmt.Errorf("count scans: %w", err)
	}

	opts := options.Find().
		SetSort(bson.D{{Key: "createdAt", Value: -1}}).
		SetSkip(skip).
		SetLimit(int64(limit))

	cursor, err := s.col.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, fmt.Errorf("list scans: %w", err)
	}
	defer cursor.Close(ctx)

	var scans []StoredScan
	if err := cursor.All(ctx, &scans); err != nil {
		return nil, 0, fmt.Errorf("decode scans: %w", err)
	}
	if scans == nil {
		scans = []StoredScan{}
	}
	return scans, total, nil
}
