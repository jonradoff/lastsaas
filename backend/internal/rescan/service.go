// Package rescan provides automated scheduled rescanning of tracked stores
// and email alerts when scores drop significantly.
package rescan

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"lastsaas/internal/db"
	"lastsaas/internal/email"
	"lastsaas/internal/models"
	"lastsaas/internal/scanner"
	"lastsaas/internal/syslog"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	lockName       = "rescan_leader"
	leaseTTL       = 2 * time.Minute
	renewalTick    = 30 * time.Second
	cycleTick      = 5 * time.Minute
	batchSize      = 10
	scanTimeout    = 90 * time.Second
	scoreDropThreshold = 5
)

// rescanJob is a single store that needs rescanning.
type rescanJob struct {
	TrackedStoreID primitive.ObjectID
	TenantID       primitive.ObjectID
	Domain         string
	CurrentScore   int
	FrequencyHours int
}

const digestInterval = 7 * 24 * time.Hour // weekly

// Service manages automated rescans and score change alerts.
type Service struct {
	db       *db.MongoDB
	scanner  *scanner.Service
	email    *email.ResendService
	syslog   *syslog.Logger
	holderID string
	stop     chan struct{}
}

// New creates a rescan service. emailSvc may be nil (alerts will be skipped).
func New(database *db.MongoDB, scannerSvc *scanner.Service, emailSvc *email.ResendService, logger *syslog.Logger) *Service {
	hostname, _ := os.Hostname()
	holderID := hostname + "-rescan-" + time.Now().Format("20060102150405")

	return &Service{
		db:       database,
		scanner:  scannerSvc,
		email:    emailSvc,
		syslog:   logger,
		holderID: holderID,
		stop:     make(chan struct{}),
	}
}

// Start begins the background rescan loop.
func (s *Service) Start() {
	go s.run()
	slog.Info("Rescan service started", "holder", s.holderID)
}

// Stop gracefully shuts down the rescan loop and releases the leader lock.
func (s *Service) Stop() {
	close(s.stop)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s.releaseLock(ctx)
}

func (s *Service) run() {
	// Try leadership immediately
	if s.tryAcquireOrRenew() {
		s.processRescanCycle(context.Background())
	}

	renewTicker := time.NewTicker(renewalTick)
	cycleTicker := time.NewTicker(cycleTick)
	defer renewTicker.Stop()
	defer cycleTicker.Stop()

	// Check for weekly digests once per cycle too
	digestTicker := time.NewTicker(1 * time.Hour) // check hourly
	defer digestTicker.Stop()

	for {
		select {
		case <-renewTicker.C:
			s.tryAcquireOrRenew()
		case <-cycleTicker.C:
			if s.isLeader() {
				s.processRescanCycle(context.Background())
			}
		case <-digestTicker.C:
			if s.isLeader() {
				s.processWeeklyDigests(context.Background())
			}
		case <-s.stop:
			return
		}
	}
}

// --- Leader lock (same pattern as internal/metrics) ---

func (s *Service) tryAcquireOrRenew() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	now := time.Now().UTC()
	newExpiry := now.Add(leaseTTL)

	filter := bson.M{
		"_id": lockName,
		"$or": bson.A{
			bson.M{"holderId": s.holderID},
			bson.M{"expiresAt": bson.M{"$lte": now}},
		},
	}
	update := bson.M{
		"$set": bson.M{
			"holderId":  s.holderID,
			"expiresAt": newExpiry,
			"updatedAt": now,
		},
		"$setOnInsert": bson.M{
			"_id":       lockName,
			"createdAt": now,
		},
	}

	result := s.db.LeaderLocks().FindOneAndUpdate(ctx, filter, update,
		options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After),
	)

	if result.Err() != nil {
		if result.Err() == mongo.ErrNoDocuments || mongo.IsDuplicateKeyError(result.Err()) {
			return false
		}
		slog.Error("Rescan leader lock error", "error", result.Err())
		return false
	}

	var doc struct {
		HolderID string `bson:"holderId"`
	}
	if err := result.Decode(&doc); err != nil {
		return false
	}
	return doc.HolderID == s.holderID
}

func (s *Service) isLeader() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var doc struct {
		HolderID  string    `bson:"holderId"`
		ExpiresAt time.Time `bson:"expiresAt"`
	}
	err := s.db.LeaderLocks().FindOne(ctx, bson.M{"_id": lockName}).Decode(&doc)
	if err != nil {
		return false
	}
	return doc.HolderID == s.holderID && doc.ExpiresAt.After(time.Now().UTC())
}

func (s *Service) releaseLock(ctx context.Context) {
	if s.db == nil {
		return
	}
	_, _ = s.db.LeaderLocks().DeleteOne(ctx, bson.M{
		"_id":      lockName,
		"holderId": s.holderID,
	})
}

// --- Rescan cycle ---

func (s *Service) processRescanCycle(ctx context.Context) {
	jobs, err := s.getStoresDueForRescan(ctx)
	if err != nil {
		slog.Error("Failed to get stores due for rescan", "error", err)
		return
	}

	if len(jobs) == 0 {
		return
	}

	slog.Info("Rescan cycle: processing batch", "count", len(jobs))

	for _, job := range jobs {
		select {
		case <-s.stop:
			return
		default:
		}

		if err := s.rescanStore(ctx, job); err != nil {
			slog.Warn("Rescan failed, will retry next cycle",
				"domain", job.Domain, "tenantId", job.TenantID.Hex(), "error", err)
		}
	}
}

// getStoresDueForRescan finds tracked stores whose lastScannedAt is older than
// the tenant's rescan_frequency entitlement allows.
func (s *Service) getStoresDueForRescan(ctx context.Context) ([]rescanJob, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Aggregation: tracked_stores → tenants → plans → filter by frequency + lastScannedAt
	pipeline := mongo.Pipeline{
		// Join tracked_stores with tenants
		{{Key: "$lookup", Value: bson.M{
			"from":         "tenants",
			"localField":   "tenantId",
			"foreignField": "_id",
			"as":           "tenant",
		}}},
		{{Key: "$unwind", Value: "$tenant"}},

		// Only active billing
		{{Key: "$match", Value: bson.M{
			"tenant.billingStatus": models.BillingStatusActive,
			"tenant.planId":        bson.M{"$exists": true, "$ne": nil},
		}}},

		// Join with plans
		{{Key: "$lookup", Value: bson.M{
			"from":         "plans",
			"localField":   "tenant.planId",
			"foreignField": "_id",
			"as":           "plan",
		}}},
		{{Key: "$unwind", Value: "$plan"}},

		// Extract rescan_frequency entitlement
		{{Key: "$addFields", Value: bson.M{
			"rescanFreqHours": bson.M{
				"$ifNull": bson.A{
					"$plan.entitlements.rescan_frequency.numericValue",
					0,
				},
			},
		}}},

		// Only stores with rescan enabled
		{{Key: "$match", Value: bson.M{
			"rescanFreqHours": bson.M{"$gt": 0},
		}}},

		// Filter: lastScannedAt + frequency < now, OR lastScannedAt is null
		{{Key: "$match", Value: bson.M{
			"$or": bson.A{
				bson.M{"lastScannedAt": nil},
				bson.M{"lastScannedAt": bson.M{"$exists": false}},
				bson.M{"$expr": bson.M{
					"$lt": bson.A{
						bson.M{"$add": bson.A{
							"$lastScannedAt",
							bson.M{"$multiply": bson.A{"$rescanFreqHours", 3600000}}, // hours to ms
						}},
						time.Now().UTC(),
					},
				}},
			},
		}}},

		// Oldest first (natural staggering)
		{{Key: "$sort", Value: bson.D{{Key: "lastScannedAt", Value: 1}}}},

		// Limit batch
		{{Key: "$limit", Value: batchSize}},

		// Project the fields we need
		{{Key: "$project", Value: bson.M{
			"_id":              1,
			"tenantId":         1,
			"domain":           1,
			"currentScore":     1,
			"rescanFreqHours":  1,
		}}},
	}

	cursor, err := s.db.Database.Collection("tracked_stores").Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("rescan aggregation: %w", err)
	}
	defer cursor.Close(ctx)

	var results []struct {
		ID               primitive.ObjectID `bson:"_id"`
		TenantID         primitive.ObjectID `bson:"tenantId"`
		Domain           string             `bson:"domain"`
		CurrentScore     int                `bson:"currentScore"`
		RescanFreqHours  int                `bson:"rescanFreqHours"`
	}
	if err := cursor.All(ctx, &results); err != nil {
		return nil, fmt.Errorf("rescan cursor: %w", err)
	}

	jobs := make([]rescanJob, len(results))
	for i, r := range results {
		jobs[i] = rescanJob{
			TrackedStoreID: r.ID,
			TenantID:       r.TenantID,
			Domain:         r.Domain,
			CurrentScore:   r.CurrentScore,
			FrequencyHours: r.RescanFreqHours,
		}
	}
	return jobs, nil
}

// rescanStore runs a scan and updates the tracked store score.
func (s *Service) rescanStore(ctx context.Context, job rescanJob) error {
	ctx, cancel := context.WithTimeout(ctx, scanTimeout)
	defer cancel()

	slog.Info("Rescanning store", "domain", job.Domain, "tenantId", job.TenantID.Hex())

	result, err := s.scanner.ScanStore(ctx, job.Domain, &job.TenantID)
	if err != nil {
		s.syslog.Medium(ctx, fmt.Sprintf("Automated rescan failed for %s: %v", job.Domain, err))
		return err
	}

	scannedAt := time.Now().UTC()
	if err := s.scanner.TrackedStores().UpdateTrackedStoreScore(
		ctx, job.TenantID, job.Domain, result.CompositeScore, scannedAt,
	); err != nil {
		slog.Error("Failed to update score after rescan", "domain", job.Domain, "error", err)
		return err
	}

	slog.Info("Rescan complete", "domain", job.Domain, "score", result.CompositeScore, "previousScore", job.CurrentScore)

	// Check for significant score change and alert
	s.checkAndAlert(ctx, job, result.CompositeScore)
	return nil
}

// checkAndAlert sends an email if the score changed significantly (up or down).
func (s *Service) checkAndAlert(ctx context.Context, job rescanJob, newScore int) {
	delta := newScore - job.CurrentScore
	absDelta := delta
	if absDelta < 0 {
		absDelta = -absDelta
	}
	if absDelta < scoreDropThreshold {
		return
	}

	if s.email == nil {
		return
	}

	ownerEmail, ownerName, err := s.findTenantOwnerEmail(ctx, job.TenantID)
	if err != nil || ownerEmail == "" {
		slog.Warn("Could not find tenant owner for score alert", "tenantId", job.TenantID.Hex(), "error", err)
		return
	}

	if err := s.email.SendScoreChangeAlert(ownerEmail, ownerName, job.Domain, job.CurrentScore, newScore, delta); err != nil {
		slog.Error("Failed to send score change alert", "domain", job.Domain, "to", ownerEmail, "error", err)
	} else {
		slog.Info("Score change alert sent", "domain", job.Domain, "to", ownerEmail, "delta", delta)
		s.syslog.Medium(ctx, fmt.Sprintf("Score change alert: %s changed %+d points (%d → %d), notified %s",
			job.Domain, delta, job.CurrentScore, newScore, ownerEmail))
	}
}

// findTenantOwnerEmail looks up the owner's email for a tenant.
func (s *Service) findTenantOwnerEmail(ctx context.Context, tenantID primitive.ObjectID) (string, string, error) {
	var membership models.TenantMembership
	err := s.db.TenantMemberships().FindOne(ctx, bson.M{
		"tenantId": tenantID,
		"role":     models.RoleOwner,
	}).Decode(&membership)
	if err != nil {
		return "", "", err
	}

	var user models.User
	if err := s.db.Users().FindOne(ctx, bson.M{"_id": membership.UserID}).Decode(&user); err != nil {
		return "", "", err
	}
	return user.Email, user.DisplayName, nil
}

// processWeeklyDigests sends a weekly digest email to each tenant that has tracked stores.
// Uses a per-tenant timestamp in the "digest_sent" collection to avoid duplicates.
func (s *Service) processWeeklyDigests(ctx context.Context) {
	if s.email == nil {
		return
	}

	col := s.db.Database.Collection("digest_sent")
	now := time.Now().UTC()
	threshold := now.Add(-digestInterval)

	// Find all tenants with tracked stores
	pipeline := mongo.Pipeline{
		{{Key: "$group", Value: bson.M{
			"_id": "$tenantId",
		}}},
	}
	cursor, err := s.db.Database.Collection("tracked_stores").Aggregate(ctx, pipeline)
	if err != nil {
		slog.Error("Digest: failed to find tenants with tracked stores", "error", err)
		return
	}
	defer cursor.Close(ctx)

	var tenants []struct {
		ID primitive.ObjectID `bson:"_id"`
	}
	if err := cursor.All(ctx, &tenants); err != nil {
		slog.Error("Digest: failed to decode tenants", "error", err)
		return
	}

	for _, t := range tenants {
		// Check if digest was sent recently
		var lastDigest struct {
			SentAt time.Time `bson:"sentAt"`
		}
		err := col.FindOne(ctx, bson.M{"_id": t.ID}).Decode(&lastDigest)
		if err == nil && lastDigest.SentAt.After(threshold) {
			continue // already sent this week
		}

		s.sendDigestForTenant(ctx, t.ID, col, now)
	}
}

func (s *Service) sendDigestForTenant(ctx context.Context, tenantID primitive.ObjectID, col *mongo.Collection, now time.Time) {
	ownerEmail, ownerName, err := s.findTenantOwnerEmail(ctx, tenantID)
	if err != nil || ownerEmail == "" {
		return
	}

	stores, err := s.scanner.TrackedStores().ListTrackedStores(ctx, tenantID)
	if err != nil || len(stores) == 0 {
		return
	}

	digestStores := make([]email.DigestStore, len(stores))
	for i, store := range stores {
		delta := store.CurrentScore - store.PreviousScore
		digestStores[i] = email.DigestStore{
			Domain:       store.Domain,
			Label:        store.Label,
			CurrentScore: store.CurrentScore,
			Delta:        delta,
		}
	}

	if err := s.email.SendWeeklyDigest(ownerEmail, ownerName, digestStores); err != nil {
		slog.Error("Failed to send weekly digest", "tenantId", tenantID.Hex(), "error", err)
		return
	}

	// Mark digest as sent
	_, _ = col.UpdateOne(ctx,
		bson.M{"_id": tenantID},
		bson.M{"$set": bson.M{"sentAt": now}},
		options.Update().SetUpsert(true),
	)

	slog.Info("Weekly digest sent", "tenantId", tenantID.Hex(), "to", ownerEmail, "stores", len(stores))
}
