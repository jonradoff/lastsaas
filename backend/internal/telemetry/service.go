package telemetry

import (
	"context"
	"log/slog"
	"math"
	"sort"
	"sync"
	"time"

	"lastsaas/internal/db"
	"lastsaas/internal/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/sync/singleflight"
)

const (
	trackBufferSize    = 200
	trackFlushInterval = 100 * time.Millisecond
	trackMaxBackoff    = 30 * time.Second
	kpiCacheTTL        = 5 * time.Minute
)

// Service provides telemetry tracking and querying for product analytics.
// Apps using MCPLens as a library can call Track/TrackBatch directly
// without going through the HTTP API.
type Service struct {
	db *db.MongoDB

	// Async write buffer
	trackCh chan models.TelemetryEvent
	stopCh  chan struct{}
	stopped chan struct{}

	// Optional observer called for each tracked event (e.g., DataDog forwarding).
	// Must be non-blocking.
	onTrack func(models.TelemetryEvent)

	// KPI cache (singleflight prevents thundering herd on expiry)
	kpiMu       sync.Mutex
	kpiCache    *KPIData
	kpiCachedAt time.Time
	kpiGroup    singleflight.Group
}

// New creates a new telemetry service with async write buffering.
func New(database *db.MongoDB) *Service {
	s := &Service{
		db:      database,
		trackCh: make(chan models.TelemetryEvent, trackBufferSize),
		stopCh:  make(chan struct{}),
		stopped: make(chan struct{}),
	}
	go s.flushLoop()
	return s
}

// Stop gracefully drains the track buffer and shuts down the flush loop.
func (s *Service) Stop() {
	close(s.stopCh)
	<-s.stopped
}

// flushLoop batches buffered events and writes them periodically.
// Uses a timer instead of ticker so it only fires when data is buffered.
func (s *Service) flushLoop() {
	defer close(s.stopped)
	backoff := trackFlushInterval
	timer := time.NewTimer(backoff)
	timer.Stop() // start disarmed — only arm when buffer has data

	buf := make([]interface{}, 0, trackBufferSize)
	flush := func() bool {
		if len(buf) == 0 {
			return true
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		_, err := s.db.TelemetryEvents().InsertMany(ctx, buf)
		cancel()
		if err != nil {
			slog.Warn("telemetry: flush failed, will retry", "count", len(buf), "error", err)
			return false // retain buffer for next attempt
		}
		buf = buf[:0]
		return true
	}

	for {
		select {
		case ev := <-s.trackCh:
			wasEmpty := len(buf) == 0
			buf = append(buf, ev)
			if len(buf) >= trackBufferSize {
				if flush() {
					backoff = trackFlushInterval
				} else {
					backoff *= 2
					if backoff > trackMaxBackoff {
						backoff = trackMaxBackoff
					}
				}
			}
			// Arm timer when first event enters an empty buffer
			if wasEmpty && len(buf) > 0 {
				timer.Reset(backoff)
			}
		case <-timer.C:
			if flush() {
				backoff = trackFlushInterval
			} else {
				backoff *= 2
				if backoff > trackMaxBackoff {
					backoff = trackMaxBackoff
				}
			}
			// Re-arm if buffer still has data (retry after failed flush)
			if len(buf) > 0 {
				timer.Reset(backoff)
			}
		case <-s.stopCh:
			timer.Stop()
			// Drain remaining
			for {
				select {
				case ev := <-s.trackCh:
					buf = append(buf, ev)
				default:
					flush()
					return
				}
			}
		}
	}
}

// --- Tracking (Go SDK) ---

// SetOnTrack registers a callback invoked for each telemetry event.
// The callback must be non-blocking (e.g., enqueue to a buffered channel).
func (s *Service) SetOnTrack(fn func(models.TelemetryEvent)) {
	s.onTrack = fn
}

// Track records a single telemetry event asynchronously via the write buffer.
func (s *Service) Track(ctx context.Context, event models.TelemetryEvent) error {
	if event.ID.IsZero() {
		event.ID = primitive.NewObjectID()
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now()
	}
	if s.onTrack != nil {
		s.onTrack(event)
	}
	select {
	case s.trackCh <- event:
		return nil
	default:
		// Buffer full — drop event rather than blocking the caller
		slog.Warn("telemetry: buffer full, dropping event", "event", event.EventName)
		return nil
	}
}

// TrackBatch records multiple events.
func (s *Service) TrackBatch(ctx context.Context, events []models.TelemetryEvent) error {
	if len(events) == 0 {
		return nil
	}
	docs := make([]interface{}, len(events))
	now := time.Now()
	for i := range events {
		if events[i].ID.IsZero() {
			events[i].ID = primitive.NewObjectID()
		}
		if events[i].CreatedAt.IsZero() {
			events[i].CreatedAt = now
		}
		if s.onTrack != nil {
			s.onTrack(events[i])
		}
		docs[i] = events[i]
	}
	_, err := s.db.TelemetryEvents().InsertMany(ctx, docs)
	if err != nil {
		slog.Warn("telemetry: failed to track batch", "count", len(events), "error", err)
	}
	return err
}

// TrackPageView is a convenience method for page view events.
func (s *Service) TrackPageView(ctx context.Context, sessionID, page string, userID *primitive.ObjectID) error {
	return s.Track(ctx, models.TelemetryEvent{
		EventName:  models.TelemetryPageView,
		Category:   models.TelemetryCategoryFunnel,
		SessionID:  sessionID,
		UserID:     userID,
		Properties: map[string]interface{}{"page": page},
	})
}

// TrackCheckoutStarted is a convenience method for checkout initiation events.
func (s *Service) TrackCheckoutStarted(ctx context.Context, userID, tenantID primitive.ObjectID, planName string) error {
	return s.Track(ctx, models.TelemetryEvent{
		EventName:  models.TelemetryCheckoutStarted,
		Category:   models.TelemetryCategoryFunnel,
		UserID:     &userID,
		TenantID:   &tenantID,
		Properties: map[string]interface{}{"planName": planName},
	})
}

// TrackLogin is a convenience method for login events.
func (s *Service) TrackLogin(ctx context.Context, userID primitive.ObjectID) error {
	return s.Track(ctx, models.TelemetryEvent{
		EventName: models.TelemetryUserLogin,
		Category:  models.TelemetryCategoryEngagement,
		UserID:    &userID,
	})
}

// --- Query types ---

type FunnelData struct {
	UniqueVisitors   int64        `json:"uniqueVisitors"`
	Registrations    int64        `json:"registrations"`
	PlanPageViews    int64        `json:"planPageViews"`
	CheckoutsStarted int64       `json:"checkoutsStarted"`
	PaidConversions  int64        `json:"paidConversions"`
	Upgrades         int64        `json:"upgrades"`
	Steps            []FunnelStep `json:"steps"`
}

type FunnelStep struct {
	Name       string  `json:"name"`
	Count      int64   `json:"count"`
	Conversion float64 `json:"conversion"`
}

type CohortRow struct {
	CohortLabel string    `json:"cohortLabel"`
	CohortSize  int64     `json:"cohortSize"`
	Retention   []float64 `json:"retention"`
}

type EngagementData struct {
	DAU         []DailyPoint `json:"dau"`
	WAU         []DailyPoint `json:"wau"`
	MAU         []DailyPoint `json:"mau"`
	AvgSessions float64      `json:"avgSessions"`
	TopFeatures []FeatureUse `json:"topFeatures"`
	CreditTrend []DailyPoint `json:"creditTrend"`
}

type DailyPoint struct {
	Date  string `json:"date"`
	Value int64  `json:"value"`
}

type FeatureUse struct {
	Name  string `json:"name"`
	Count int64  `json:"count"`
}

type KPIData struct {
	MRR                 int64        `json:"mrr"`
	ARR                 int64        `json:"arr"`
	ARPU                int64        `json:"arpu"`
	LTV                 int64        `json:"ltv"`
	ChurnRate           float64      `json:"churnRate"`
	TrialConversionRate float64      `json:"trialConversionRate"`
	TimeToFirstPurchase float64      `json:"timeToFirstPurchase"`
	ActiveSubscribers   int64        `json:"activeSubscribers"`
	TotalRegistrations  int64        `json:"totalRegistrations"`
	PlanDistribution    []PlanShare  `json:"planDistribution"`
	MRRTrend            []DailyPoint `json:"mrrTrend"`
	SubscriberTrend     []DailyPoint `json:"subscriberTrend"`
}

type PlanShare struct {
	PlanName    string  `json:"planName"`
	Subscribers int64   `json:"subscribers"`
	Percentage  float64 `json:"percentage"`
	MRR         int64   `json:"mrr"`
}

type CustomEventData struct {
	EventName  string       `json:"eventName"`
	TotalCount int64        `json:"totalCount"`
	Trend      []DailyPoint `json:"trend"`
}

type EventTypeSummary struct {
	EventName string    `json:"eventName"`
	Category  string    `json:"category"`
	Count     int64     `json:"count"`
	LastSeen  time.Time `json:"lastSeen"`
}

// --- Query methods ---

// FunnelMetrics computes the conversion funnel for a date range.
func (s *Service) FunnelMetrics(ctx context.Context, start, end time.Time) (*FunnelData, error) {
	dateFilter := bson.M{"createdAt": bson.M{"$gte": start, "$lte": end}}

	// Unique visitors: distinct sessionIDs with page.view
	visitors, _ := s.countDistinct(ctx, "sessionId", bson.M{
		"eventName": models.TelemetryPageView,
		"createdAt": bson.M{"$gte": start, "$lte": end},
	})

	// Registrations: users created in period
	registrations, _ := s.db.Users().CountDocuments(ctx, bson.M{
		"createdAt": bson.M{"$gte": start, "$lte": end},
	})

	// Plan page views: page.view where properties.page = "/plan"
	planViews, _ := s.countDistinct(ctx, "sessionId", bson.M{
		"eventName":       models.TelemetryPageView,
		"properties.page": "/plan",
		"createdAt":       bson.M{"$gte": start, "$lte": end},
	})

	// Checkouts started
	checkouts, _ := s.db.TelemetryEvents().CountDocuments(ctx, bson.M{
		"eventName": models.TelemetryCheckoutStarted,
		"createdAt": bson.M{"$gte": start, "$lte": end},
	})

	// Paid conversions: subscription transactions in period
	conversions, _ := s.db.FinancialTransactions().CountDocuments(ctx, bson.M{
		"type":      models.TransactionSubscription,
		"createdAt": bson.M{"$gte": start, "$lte": end},
	})

	// Plan upgrades
	upgrades, _ := s.db.TelemetryEvents().CountDocuments(ctx, mergeBson(dateFilter, bson.M{
		"eventName": models.TelemetryPlanChanged,
	}))

	steps := buildFunnelSteps(visitors, registrations, planViews, checkouts, conversions, upgrades)

	return &FunnelData{
		UniqueVisitors:   visitors,
		Registrations:    registrations,
		PlanPageViews:    planViews,
		CheckoutsStarted: checkouts,
		PaidConversions:  conversions,
		Upgrades:         upgrades,
		Steps:            steps,
	}, nil
}

func buildFunnelSteps(visitors, registrations, planViews, checkouts, conversions, upgrades int64) []FunnelStep {
	steps := []FunnelStep{
		{Name: "Unique Visitors", Count: visitors},
		{Name: "Registrations", Count: registrations},
		{Name: "Plan Page Views", Count: planViews},
		{Name: "Checkouts Started", Count: checkouts},
		{Name: "Paid Conversions", Count: conversions},
		{Name: "Upgrades", Count: upgrades},
	}
	for i := range steps {
		if i == 0 {
			steps[i].Conversion = 100
		} else if steps[i-1].Count > 0 {
			steps[i].Conversion = math.Round(float64(steps[i].Count)/float64(steps[i-1].Count)*10000) / 100
		}
	}
	return steps
}

// RetentionCohorts computes cohort retention data using a single aggregation pipeline.
func (s *Service) RetentionCohorts(ctx context.Context, granularity string, periods int) ([]CohortRow, error) {
	if periods <= 0 {
		periods = 12
	}

	var intervalMs int64
	var labelFormat string
	switch granularity {
	case "monthly":
		intervalMs = 30 * 24 * 60 * 60 * 1000
		labelFormat = "2006-01"
	default:
		granularity = "weekly"
		intervalMs = 7 * 24 * 60 * 60 * 1000
		labelFormat = "2006-W02"
	}

	interval := time.Duration(intervalMs) * time.Millisecond
	now := time.Now()
	earliestCohortStart := now.Add(-time.Duration(periods) * interval)

	// Single aggregation: bucket users into cohorts, then use $facet to count
	// retention for each period within each cohort.
	pipeline := mongo.Pipeline{
		// Match active users created within the cohort window
		{{Key: "$match", Value: bson.M{
			"isActive":  true,
			"createdAt": bson.M{"$gte": earliestCohortStart, "$lt": now},
		}}},
		// Assign each user to a cohort index based on registration time
		{{Key: "$addFields", Value: bson.M{
			"cohortIdx": bson.M{
				"$floor": bson.M{
					"$divide": []interface{}{
						bson.M{"$subtract": []interface{}{"$createdAt", earliestCohortStart}},
						intervalMs,
					},
				},
			},
		}}},
		// Group by cohort, collect only lastLoginAt (createdAt unused in retention loop)
		{{Key: "$group", Value: bson.M{
			"_id":        "$cohortIdx",
			"cohortSize": bson.M{"$sum": 1},
			"users": bson.M{"$push": bson.M{
				"lastLoginAt": "$lastLoginAt",
			}},
		}}},
		{{Key: "$sort", Value: bson.D{{Key: "_id", Value: 1}}}},
	}

	cursor, err := s.db.Users().Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	type userInfo struct {
		LastLoginAt *time.Time `bson:"lastLoginAt"`
	}
	type cohortResult struct {
		CohortIdx  int        `bson:"_id"`
		CohortSize int64      `bson:"cohortSize"`
		Users      []userInfo `bson:"users"`
	}

	rows := make([]CohortRow, 0, periods)
	for cursor.Next(ctx) {
		var cr cohortResult
		if cursor.Decode(&cr) != nil {
			continue
		}
		if cr.CohortSize == 0 {
			continue
		}

		cohortStart := earliestCohortStart.Add(time.Duration(cr.CohortIdx) * interval)
		cohortEnd := cohortStart.Add(interval)
		maxPeriods := int(now.Sub(cohortEnd) / interval)

		retention := []float64{100} // P0 is always 100%
		for p := 1; p <= maxPeriods; p++ {
			periodStart := cohortEnd.Add(time.Duration(p-1) * interval)
			periodEnd := cohortEnd.Add(time.Duration(p) * interval)
			var active int64
			for _, u := range cr.Users {
				if u.LastLoginAt != nil && !u.LastLoginAt.Before(periodStart) && u.LastLoginAt.Before(periodEnd) {
					active++
				}
			}
			pct := math.Round(float64(active)/float64(cr.CohortSize)*10000) / 100
			retention = append(retention, pct)
		}

		rows = append(rows, CohortRow{
			CohortLabel: cohortStart.Format(labelFormat),
			CohortSize:  cr.CohortSize,
			Retention:   retention,
		})
	}

	return rows, nil
}

// EngagementMetrics computes engagement data for paying subscribers.
func (s *Service) EngagementMetrics(ctx context.Context, start, end time.Time) (*EngagementData, error) {
	// Get active tenant IDs (paying subscribers)
	activeTenantIDs, err := s.getActiveTenantIDs(ctx)
	if err != nil {
		return &EngagementData{}, nil
	}

	// Get user IDs who belong to active tenants
	activeUserIDs, err := s.getUserIDsForTenants(ctx, activeTenantIDs)
	if err != nil {
		return &EngagementData{}, nil
	}

	// DAU: for each day in range, count distinct users with login events
	dau := s.dailyActiveUsers(ctx, activeUserIDs, start, end)

	// WAU: rolling 7-day windows
	wau := s.weeklyActiveUsers(ctx, activeUserIDs, start, end)

	// MAU: rolling 30-day windows
	mau := s.monthlyActiveUsers(ctx, activeUserIDs, start, end)

	// Average sessions per user per week
	days := end.Sub(start).Hours() / 24
	weeks := days / 7
	if weeks < 1 {
		weeks = 1
	}
	totalLogins, _ := s.db.TelemetryEvents().CountDocuments(ctx, bson.M{
		"eventName": models.TelemetryUserLogin,
		"userId":    bson.M{"$in": activeUserIDs},
		"createdAt": bson.M{"$gte": start, "$lte": end},
	})
	avgSessions := 0.0
	if len(activeUserIDs) > 0 {
		avgSessions = math.Round(float64(totalLogins)/float64(len(activeUserIDs))/weeks*100) / 100
	}

	// Top features: custom events by count
	topFeatures := s.topCustomEvents(ctx, start, end, 10)

	// Credit consumption trend
	creditTrend := s.creditConsumptionTrend(ctx, start, end)

	return &EngagementData{
		DAU:         dau,
		WAU:         wau,
		MAU:         mau,
		AvgSessions: avgSessions,
		TopFeatures: topFeatures,
		CreditTrend: creditTrend,
	}, nil
}

// KPIs computes high-level product management KPIs.
// Results are cached in-process for 5 minutes to avoid repeated heavy aggregation.
func (s *Service) KPIs(ctx context.Context) (*KPIData, error) {
	s.kpiMu.Lock()
	if s.kpiCache != nil && time.Since(s.kpiCachedAt) < kpiCacheTTL {
		cached := s.kpiCache
		s.kpiMu.Unlock()
		return cached, nil
	}
	s.kpiMu.Unlock()

	// singleflight coalesces concurrent callers so only one computes
	v, err, _ := s.kpiGroup.Do("kpis", func() (interface{}, error) {
		data, err := s.computeKPIs(ctx)
		if err != nil {
			return nil, err
		}
		s.kpiMu.Lock()
		s.kpiCache = data
		s.kpiCachedAt = time.Now()
		s.kpiMu.Unlock()
		return data, nil
	})
	if err != nil {
		return nil, err
	}
	return v.(*KPIData), nil
}

func (s *Service) computeKPIs(ctx context.Context) (*KPIData, error) {
	now := time.Now()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	prevMonthStart := monthStart.AddDate(0, -1, 0)

	// Active subscribers
	activeSubscribers, _ := s.db.Tenants().CountDocuments(ctx, bson.M{
		"billingStatus": models.BillingStatusActive,
		"isActive":      true,
	})

	// Total registrations
	totalRegistrations, _ := s.db.Users().CountDocuments(ctx, bson.M{})

	// MRR: aggregate from active tenants with plans
	mrr := s.calculateMRR(ctx)
	arr := mrr * 12

	// ARPU
	arpu := int64(0)
	if activeSubscribers > 0 {
		arpu = mrr / activeSubscribers
	}

	// Churn: tenants canceled this month / active at start of month
	canceledThisMonth, _ := s.db.Tenants().CountDocuments(ctx, bson.M{
		"canceledAt": bson.M{"$gte": monthStart},
	})
	activeAtMonthStart, _ := s.db.Tenants().CountDocuments(ctx, bson.M{
		"billingStatus": bson.M{"$in": []string{string(models.BillingStatusActive), string(models.BillingStatusCanceled)}},
		"createdAt":     bson.M{"$lt": monthStart},
	})
	churnRate := 0.0
	if activeAtMonthStart > 0 {
		churnRate = math.Round(float64(canceledThisMonth)/float64(activeAtMonthStart)*10000) / 100
	}

	// Trial conversion
	totalTrials, _ := s.db.Tenants().CountDocuments(ctx, bson.M{
		"trialUsedAt": bson.M{"$ne": nil},
	})
	convertedTrials, _ := s.db.Tenants().CountDocuments(ctx, bson.M{
		"trialUsedAt":  bson.M{"$ne": nil},
		"billingStatus": models.BillingStatusActive,
	})
	trialConversion := 0.0
	if totalTrials > 0 {
		trialConversion = math.Round(float64(convertedTrials)/float64(totalTrials)*10000) / 100
	}

	// Time to first purchase (median)
	ttfp := s.medianTimeToFirstPurchase(ctx)

	// LTV: ARPU / monthly churn rate (simplified)
	ltv := int64(0)
	if churnRate > 0 {
		ltv = int64(float64(arpu) / (churnRate / 100))
	}

	// Plan distribution
	planDist := s.planDistribution(ctx)

	// MRR trend (last 30 days) — use daily_metrics if available
	mrrTrend := s.mrrTrend(ctx, now.AddDate(0, 0, -30), now)

	// Subscriber trend (last 30 days)
	subTrend := s.subscriberTrend(ctx, now.AddDate(0, 0, -30), now)

	_ = prevMonthStart // might use for growth rate later

	return &KPIData{
		MRR:                 mrr,
		ARR:                 arr,
		ARPU:                arpu,
		LTV:                 ltv,
		ChurnRate:           churnRate,
		TrialConversionRate: trialConversion,
		TimeToFirstPurchase: ttfp,
		ActiveSubscribers:   activeSubscribers,
		TotalRegistrations:  totalRegistrations,
		PlanDistribution:    planDist,
		MRRTrend:            mrrTrend,
		SubscriberTrend:     subTrend,
	}, nil
}

// CustomEventSummary returns trend data for a specific event name.
func (s *Service) CustomEventSummary(ctx context.Context, start, end time.Time, eventName string) (*CustomEventData, error) {
	filter := bson.M{
		"createdAt": bson.M{"$gte": start, "$lte": end},
	}
	if eventName != "" {
		filter["eventName"] = eventName
	}

	totalCount, _ := s.db.TelemetryEvents().CountDocuments(ctx, filter)

	// Daily trend (capped at 400 days to prevent unbounded results)
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: filter}},
		{{Key: "$group", Value: bson.M{
			"_id":   bson.M{"$dateToString": bson.M{"format": "%Y-%m-%d", "date": "$createdAt"}},
			"count": bson.M{"$sum": 1},
		}}},
		{{Key: "$sort", Value: bson.D{{Key: "_id", Value: 1}}}},
		{{Key: "$limit", Value: 400}},
	}

	cursor, err := s.db.TelemetryEvents().Aggregate(ctx, pipeline)
	if err != nil {
		return &CustomEventData{EventName: eventName, TotalCount: totalCount}, nil
	}
	defer cursor.Close(ctx)

	trend := []DailyPoint{}
	for cursor.Next(ctx) {
		var result struct {
			Date  string `bson:"_id"`
			Count int64  `bson:"count"`
		}
		if cursor.Decode(&result) == nil {
			trend = append(trend, DailyPoint{Date: result.Date, Value: result.Count})
		}
	}

	return &CustomEventData{
		EventName:  eventName,
		TotalCount: totalCount,
		Trend:      trend,
	}, nil
}

// ListEventTypes returns all distinct event types with counts.
func (s *Service) ListEventTypes(ctx context.Context) ([]EventTypeSummary, error) {
	pipeline := mongo.Pipeline{
		{{Key: "$group", Value: bson.M{
			"_id":      bson.M{"eventName": "$eventName", "category": "$category"},
			"count":    bson.M{"$sum": 1},
			"lastSeen": bson.M{"$max": "$createdAt"},
		}}},
		{{Key: "$sort", Value: bson.D{{Key: "count", Value: -1}}}},
		{{Key: "$limit", Value: 500}},
	}

	cursor, err := s.db.TelemetryEvents().Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []EventTypeSummary
	for cursor.Next(ctx) {
		var result struct {
			ID struct {
				EventName string `bson:"eventName"`
				Category  string `bson:"category"`
			} `bson:"_id"`
			Count    int64     `bson:"count"`
			LastSeen time.Time `bson:"lastSeen"`
		}
		if cursor.Decode(&result) == nil {
			results = append(results, EventTypeSummary{
				EventName: result.ID.EventName,
				Category:  result.ID.Category,
				Count:     result.Count,
				LastSeen:  result.LastSeen,
			})
		}
	}
	if results == nil {
		results = []EventTypeSummary{}
	}
	return results, nil
}

// --- Internal helpers ---

func (s *Service) countDistinct(ctx context.Context, field string, filter bson.M) (int64, error) {
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: filter}},
		{{Key: "$group", Value: bson.M{"_id": "$" + field}}},
		{{Key: "$count", Value: "total"}},
	}
	cursor, err := s.db.TelemetryEvents().Aggregate(ctx, pipeline)
	if err != nil {
		return 0, err
	}
	defer cursor.Close(ctx)

	if cursor.Next(ctx) {
		var result struct {
			Total int64 `bson:"total"`
		}
		if cursor.Decode(&result) == nil {
			return result.Total, nil
		}
	}
	return 0, nil
}

func (s *Service) getActiveTenantIDs(ctx context.Context) ([]primitive.ObjectID, error) {
	cursor, err := s.db.Tenants().Find(ctx, bson.M{
		"billingStatus": models.BillingStatusActive,
		"isActive":      true,
	}, options.Find().SetProjection(bson.M{"_id": 1}))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var ids []primitive.ObjectID
	for cursor.Next(ctx) {
		var t struct {
			ID primitive.ObjectID `bson:"_id"`
		}
		if cursor.Decode(&t) == nil {
			ids = append(ids, t.ID)
		}
	}
	return ids, nil
}

func (s *Service) getUserIDsForTenants(ctx context.Context, tenantIDs []primitive.ObjectID) ([]primitive.ObjectID, error) {
	if len(tenantIDs) == 0 {
		return nil, nil
	}
	cursor, err := s.db.TenantMemberships().Find(ctx, bson.M{
		"tenantId": bson.M{"$in": tenantIDs},
	}, options.Find().SetProjection(bson.M{"userId": 1}))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	seen := make(map[primitive.ObjectID]bool)
	var ids []primitive.ObjectID
	for cursor.Next(ctx) {
		var m struct {
			UserID primitive.ObjectID `bson:"userId"`
		}
		if cursor.Decode(&m) == nil && !seen[m.UserID] {
			seen[m.UserID] = true
			ids = append(ids, m.UserID)
		}
	}
	return ids, nil
}

func (s *Service) dailyActiveUsers(ctx context.Context, userIDs []primitive.ObjectID, start, end time.Time) []DailyPoint {
	if len(userIDs) == 0 {
		return nil
	}
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{
			"eventName": models.TelemetryUserLogin,
			"userId":    bson.M{"$in": userIDs},
			"createdAt": bson.M{"$gte": start, "$lte": end},
		}}},
		{{Key: "$group", Value: bson.M{
			"_id":   bson.M{"date": bson.M{"$dateToString": bson.M{"format": "%Y-%m-%d", "date": "$createdAt"}}, "userId": "$userId"},
		}}},
		{{Key: "$group", Value: bson.M{
			"_id":   "$_id.date",
			"count": bson.M{"$sum": 1},
		}}},
		{{Key: "$sort", Value: bson.D{{Key: "_id", Value: 1}}}},
	}
	return s.aggregateDailyPoints(ctx, pipeline)
}

func (s *Service) weeklyActiveUsers(ctx context.Context, userIDs []primitive.ObjectID, start, end time.Time) []DailyPoint {
	if len(userIDs) == 0 {
		return nil
	}
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{
			"eventName": models.TelemetryUserLogin,
			"userId":    bson.M{"$in": userIDs},
			"createdAt": bson.M{"$gte": start, "$lte": end},
		}}},
		{{Key: "$group", Value: bson.M{
			"_id": bson.M{
				"week":   bson.M{"$isoWeek": "$createdAt"},
				"year":   bson.M{"$isoWeekYear": "$createdAt"},
				"userId": "$userId",
			},
		}}},
		{{Key: "$group", Value: bson.M{
			"_id":   bson.M{"week": "$_id.week", "year": "$_id.year"},
			"count": bson.M{"$sum": 1},
		}}},
		{{Key: "$sort", Value: bson.D{{Key: "_id.year", Value: 1}, {Key: "_id.week", Value: 1}}}},
	}
	cursor, err := s.db.TelemetryEvents().Aggregate(ctx, pipeline)
	if err != nil {
		return nil
	}
	defer cursor.Close(ctx)

	points := []DailyPoint{}
	for cursor.Next(ctx) {
		var result struct {
			ID struct {
				Week int `bson:"week"`
				Year int `bson:"year"`
			} `bson:"_id"`
			Count int64 `bson:"count"`
		}
		if cursor.Decode(&result) == nil {
			label := time.Date(result.ID.Year, 1, 1, 0, 0, 0, 0, time.UTC).
				AddDate(0, 0, (result.ID.Week-1)*7).Format("2006-01-02")
			points = append(points, DailyPoint{Date: label, Value: result.Count})
		}
	}
	return points
}

func (s *Service) monthlyActiveUsers(ctx context.Context, userIDs []primitive.ObjectID, start, end time.Time) []DailyPoint {
	if len(userIDs) == 0 {
		return nil
	}
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{
			"eventName": models.TelemetryUserLogin,
			"userId":    bson.M{"$in": userIDs},
			"createdAt": bson.M{"$gte": start, "$lte": end},
		}}},
		{{Key: "$group", Value: bson.M{
			"_id": bson.M{
				"month":  bson.M{"$month": "$createdAt"},
				"year":   bson.M{"$year": "$createdAt"},
				"userId": "$userId",
			},
		}}},
		{{Key: "$group", Value: bson.M{
			"_id":   bson.M{"month": "$_id.month", "year": "$_id.year"},
			"count": bson.M{"$sum": 1},
		}}},
		{{Key: "$sort", Value: bson.D{{Key: "_id.year", Value: 1}, {Key: "_id.month", Value: 1}}}},
	}
	cursor, err := s.db.TelemetryEvents().Aggregate(ctx, pipeline)
	if err != nil {
		return nil
	}
	defer cursor.Close(ctx)

	points := []DailyPoint{}
	for cursor.Next(ctx) {
		var result struct {
			ID struct {
				Month int `bson:"month"`
				Year  int `bson:"year"`
			} `bson:"_id"`
			Count int64 `bson:"count"`
		}
		if cursor.Decode(&result) == nil {
			label := time.Date(result.ID.Year, time.Month(result.ID.Month), 1, 0, 0, 0, 0, time.UTC).Format("2006-01")
			points = append(points, DailyPoint{Date: label, Value: result.Count})
		}
	}
	return points
}

func (s *Service) topCustomEvents(ctx context.Context, start, end time.Time, limit int) []FeatureUse {
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{
			"category":  models.TelemetryCategoryCustom,
			"createdAt": bson.M{"$gte": start, "$lte": end},
		}}},
		{{Key: "$group", Value: bson.M{
			"_id":   "$eventName",
			"count": bson.M{"$sum": 1},
		}}},
		{{Key: "$sort", Value: bson.D{{Key: "count", Value: -1}}}},
		{{Key: "$limit", Value: limit}},
	}

	cursor, err := s.db.TelemetryEvents().Aggregate(ctx, pipeline)
	if err != nil {
		return nil
	}
	defer cursor.Close(ctx)

	features := []FeatureUse{}
	for cursor.Next(ctx) {
		var result struct {
			Name  string `bson:"_id"`
			Count int64  `bson:"count"`
		}
		if cursor.Decode(&result) == nil {
			features = append(features, FeatureUse{Name: result.Name, Count: result.Count})
		}
	}
	return features
}

func (s *Service) creditConsumptionTrend(ctx context.Context, start, end time.Time) []DailyPoint {
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{
			"createdAt": bson.M{"$gte": start, "$lte": end},
		}}},
		{{Key: "$group", Value: bson.M{
			"_id":   bson.M{"$dateToString": bson.M{"format": "%Y-%m-%d", "date": "$createdAt"}},
			"total": bson.M{"$sum": "$quantity"},
		}}},
		{{Key: "$sort", Value: bson.D{{Key: "_id", Value: 1}}}},
	}
	cursor, err := s.db.UsageEvents().Aggregate(ctx, pipeline)
	if err != nil {
		return nil
	}
	defer cursor.Close(ctx)

	points := []DailyPoint{}
	for cursor.Next(ctx) {
		var result struct {
			Date  string `bson:"_id"`
			Total int64  `bson:"total"`
		}
		if cursor.Decode(&result) == nil {
			points = append(points, DailyPoint{Date: result.Date, Value: result.Total})
		}
	}
	return points
}

func (s *Service) calculateMRR(ctx context.Context) int64 {
	// Pure aggregation pipeline: join tenants→plans, compute per-tenant MRR, sum.
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{
			"billingStatus": models.BillingStatusActive,
			"isActive":      true,
			"planId":        bson.M{"$ne": nil},
		}}},
		{{Key: "$lookup", Value: bson.M{
			"from":         "plans",
			"localField":   "planId",
			"foreignField": "_id",
			"as":           "plan",
		}}},
		{{Key: "$unwind", Value: "$plan"}},
		// Compute extra seats (clamped to 0)
		{{Key: "$addFields", Value: bson.M{
			"_extraSeats": bson.M{"$max": bson.A{
				bson.M{"$subtract": bson.A{
					bson.M{"$ifNull": bson.A{"$seatQuantity", 0}},
					bson.M{"$ifNull": bson.A{"$plan.includedSeats", 0}},
				}},
				0,
			}},
		}}},
		// Compute base monthly amount (flat vs per-seat)
		{{Key: "$addFields", Value: bson.M{
			"_monthly": bson.M{"$cond": bson.M{
				"if":   bson.M{"$eq": bson.A{"$plan.pricingModel", "per_seat"}},
				"then": bson.M{"$add": bson.A{"$plan.monthlyPriceCents", bson.M{"$multiply": bson.A{"$_extraSeats", "$plan.perSeatPriceCents"}}}},
				"else": "$plan.monthlyPriceCents",
			}},
		}}},
		// Apply annual discount if applicable
		{{Key: "$addFields", Value: bson.M{
			"_monthly": bson.M{"$cond": bson.M{
				"if": bson.M{"$and": bson.A{
					bson.M{"$eq": bson.A{"$billingInterval", "year"}},
					bson.M{"$gt": bson.A{bson.M{"$ifNull": bson.A{"$plan.annualDiscountPct", 0}}, 0}},
				}},
				"then": bson.M{"$divide": bson.A{
					bson.M{"$multiply": bson.A{
						bson.M{"$multiply": bson.A{"$_monthly", 12}},
						bson.M{"$subtract": bson.A{1, bson.M{"$divide": bson.A{"$plan.annualDiscountPct", 100}}}},
					}},
					12,
				}},
				"else": "$_monthly",
			}},
		}}},
		{{Key: "$group", Value: bson.M{
			"_id":      nil,
			"totalMRR": bson.M{"$sum": "$_monthly"},
		}}},
	}

	cursor, err := s.db.Tenants().Aggregate(ctx, pipeline)
	if err != nil {
		return 0
	}
	defer cursor.Close(ctx)

	if cursor.Next(ctx) {
		var result struct {
			TotalMRR int64 `bson:"totalMRR"`
		}
		if cursor.Decode(&result) == nil {
			return result.TotalMRR
		}
	}
	return 0
}

func (s *Service) medianTimeToFirstPurchase(ctx context.Context) float64 {
	// Get first transaction per tenant
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{"type": models.TransactionSubscription}}},
		{{Key: "$sort", Value: bson.D{{Key: "createdAt", Value: 1}}}},
		{{Key: "$group", Value: bson.M{
			"_id":             "$userId",
			"firstPurchaseAt": bson.M{"$first": "$createdAt"},
		}}},
		{{Key: "$lookup", Value: bson.M{
			"from":         "users",
			"localField":   "_id",
			"foreignField": "_id",
			"as":           "user",
		}}},
		{{Key: "$unwind", Value: "$user"}},
		{{Key: "$project", Value: bson.M{
			"daysToPurchase": bson.M{
				"$divide": []interface{}{
					bson.M{"$subtract": []interface{}{"$firstPurchaseAt", "$user.createdAt"}},
					86400000, // ms to days
				},
			},
		}}},
	}

	cursor, err := s.db.FinancialTransactions().Aggregate(ctx, pipeline)
	if err != nil {
		return 0
	}
	defer cursor.Close(ctx)

	var days []float64
	for cursor.Next(ctx) {
		var result struct {
			Days float64 `bson:"daysToPurchase"`
		}
		if cursor.Decode(&result) == nil && result.Days >= 0 {
			days = append(days, result.Days)
		}
	}

	if len(days) == 0 {
		return 0
	}
	sort.Float64s(days)
	mid := len(days) / 2
	if len(days)%2 == 0 {
		return math.Round((days[mid-1]+days[mid])/2*10) / 10
	}
	return math.Round(days[mid]*10) / 10
}

func (s *Service) planDistribution(ctx context.Context) []PlanShare {
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{
			"billingStatus": models.BillingStatusActive,
			"isActive":      true,
			"planId":        bson.M{"$ne": nil},
		}}},
		{{Key: "$lookup", Value: bson.M{
			"from":         "plans",
			"localField":   "planId",
			"foreignField": "_id",
			"as":           "plan",
		}}},
		{{Key: "$unwind", Value: "$plan"}},
		{{Key: "$group", Value: bson.M{
			"_id":   bson.M{"planId": "$planId", "planName": "$plan.name"},
			"count": bson.M{"$sum": 1},
			"mrr":   bson.M{"$sum": "$plan.monthlyPriceCents"},
		}}},
		{{Key: "$sort", Value: bson.D{{Key: "count", Value: -1}}}},
	}

	cursor, err := s.db.Tenants().Aggregate(ctx, pipeline)
	if err != nil {
		return nil
	}
	defer cursor.Close(ctx)

	shares := []PlanShare{}
	var total int64
	for cursor.Next(ctx) {
		var result struct {
			ID struct {
				PlanName string `bson:"planName"`
			} `bson:"_id"`
			Count int64 `bson:"count"`
			MRR   int64 `bson:"mrr"`
		}
		if cursor.Decode(&result) == nil {
			shares = append(shares, PlanShare{
				PlanName:    result.ID.PlanName,
				Subscribers: result.Count,
				MRR:         result.MRR,
			})
			total += result.Count
		}
	}
	for i := range shares {
		if total > 0 {
			shares[i].Percentage = math.Round(float64(shares[i].Subscribers)/float64(total)*10000) / 100
		}
	}
	return shares
}

func (s *Service) mrrTrend(ctx context.Context, start, end time.Time) []DailyPoint {
	// Use daily_metrics ARR/12 as MRR proxy
	cursor, err := s.db.DailyMetrics().Find(ctx, bson.M{
		"date": bson.M{
			"$gte": start.Format("2006-01-02"),
			"$lte": end.Format("2006-01-02"),
		},
	}, options.Find().SetSort(bson.D{{Key: "date", Value: 1}}))
	if err != nil {
		return nil
	}
	defer cursor.Close(ctx)

	points := []DailyPoint{}
	for cursor.Next(ctx) {
		var m models.DailyMetric
		if cursor.Decode(&m) == nil {
			points = append(points, DailyPoint{Date: m.Date, Value: m.ARR / 12})
		}
	}
	return points
}

func (s *Service) subscriberTrend(ctx context.Context, start, end time.Time) []DailyPoint {
	// Count active tenants per day using transactions as proxy
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{
			"type":      models.TransactionSubscription,
			"createdAt": bson.M{"$gte": start, "$lte": end},
		}}},
		{{Key: "$group", Value: bson.M{
			"_id":   bson.M{"$dateToString": bson.M{"format": "%Y-%m-%d", "date": "$createdAt"}},
			"count": bson.M{"$sum": 1},
		}}},
		{{Key: "$sort", Value: bson.D{{Key: "_id", Value: 1}}}},
	}

	cursor, err := s.db.FinancialTransactions().Aggregate(ctx, pipeline)
	if err != nil {
		return nil
	}
	defer cursor.Close(ctx)

	points := []DailyPoint{}
	for cursor.Next(ctx) {
		var result struct {
			Date  string `bson:"_id"`
			Count int64  `bson:"count"`
		}
		if cursor.Decode(&result) == nil {
			points = append(points, DailyPoint{Date: result.Date, Value: result.Count})
		}
	}
	return points
}

func (s *Service) aggregateDailyPoints(ctx context.Context, pipeline mongo.Pipeline) []DailyPoint {
	cursor, err := s.db.TelemetryEvents().Aggregate(ctx, pipeline)
	if err != nil {
		return nil
	}
	defer cursor.Close(ctx)

	points := []DailyPoint{}
	for cursor.Next(ctx) {
		var result struct {
			Date  string `bson:"_id"`
			Count int64  `bson:"count"`
		}
		if cursor.Decode(&result) == nil {
			points = append(points, DailyPoint{Date: result.Date, Value: result.Count})
		}
	}
	return points
}

func mergeBson(a, b bson.M) bson.M {
	result := bson.M{}
	for k, v := range a {
		result[k] = v
	}
	for k, v := range b {
		result[k] = v
	}
	return result
}
