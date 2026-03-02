package webhooks

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"lastsaas/internal/db"
	"lastsaas/internal/events"
	"lastsaas/internal/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// retryJob represents a pending webhook retry.
type retryJob struct {
	hook      models.Webhook
	eventType models.WebhookEventType
	event     events.Event
	retry     int
	fireAt    time.Time
}

// Dispatcher listens for events and fires matching webhooks.
type Dispatcher struct {
	db            *db.MongoDB
	client        *http.Client
	retryQ        chan retryJob
	stopCh        chan struct{}
	stopped       chan struct{}
	encryptionKey []byte        // AES-256 key for webhook secret encryption (nil = plaintext fallback)
	emitSem       chan struct{} // bounds concurrent Emit goroutines
}

const maxRetryWorkers = 5
const retryQueueSize = 500

func NewDispatcher(database *db.MongoDB, encryptionKey []byte) *Dispatcher {
	d := &Dispatcher{
		db: database,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		retryQ:        make(chan retryJob, retryQueueSize),
		stopCh:        make(chan struct{}),
		stopped:       make(chan struct{}),
		encryptionKey: encryptionKey,
		emitSem:       make(chan struct{}, 25), // max 25 concurrent Emit dispatches
	}
	go d.retryWorker()
	return d
}

// EncryptionKey returns the encryption key (nil if encryption is disabled).
func (d *Dispatcher) EncryptionKey() []byte {
	return d.encryptionKey
}

// Stop gracefully shuts down the retry worker.
func (d *Dispatcher) Stop() {
	close(d.stopCh)
	<-d.stopped
}

// retryWorker processes delayed retry jobs with bounded concurrency.
func (d *Dispatcher) retryWorker() {
	defer close(d.stopped)
	sem := make(chan struct{}, maxRetryWorkers)
	for {
		select {
		case <-d.stopCh:
			return
		case job := <-d.retryQ:
			// Wait until the scheduled fire time or shutdown
			delay := time.Until(job.fireAt)
			if delay > 0 {
				timer := time.NewTimer(delay)
				select {
				case <-timer.C:
				case <-d.stopCh:
					timer.Stop()
					return
				}
			}
			// Acquire semaphore slot
			select {
			case sem <- struct{}{}:
			case <-d.stopCh:
				return
			}
			go func(j retryJob) {
				defer func() { <-sem }()
				defer func() {
					if r := recover(); r != nil {
						slog.Error("webhooks: dispatch panic", "panic", r, "webhook", j.hook.Name)
					}
				}()
				d.deliverWithRetry(context.Background(), j.hook, j.eventType, j.event, j.retry)
			}(job)
		}
	}
}

// Emit implements events.Emitter — it checks for matching webhooks and fires them.
func (d *Dispatcher) Emit(event events.Event) {
	eventType := mapEventType(event.Type)
	if eventType == "" {
		return
	}

	// Acquire semaphore to bound concurrent dispatch goroutines
	select {
	case d.emitSem <- struct{}{}:
		go func() {
			defer func() { <-d.emitSem }()
			defer func() {
				if r := recover(); r != nil {
					slog.Error("webhooks: emit panic", "panic", r, "event_type", eventType)
				}
			}()
			d.dispatch(eventType, event)
		}()
	default:
		slog.Warn("webhooks: emit semaphore full, dropping dispatch", "event_type", eventType)
	}
}

// mapEventType converts from events.EventType to models.WebhookEventType.
func mapEventType(et events.EventType) models.WebhookEventType {
	switch et {
	// Tier 1: Billing
	case events.EventSubscriptionActivated:
		return models.WebhookEventSubscriptionActivated
	case events.EventSubscriptionCanceled:
		return models.WebhookEventSubscriptionCanceled
	case events.EventPaymentReceived:
		return models.WebhookEventPaymentReceived
	case events.EventPaymentFailed:
		return models.WebhookEventPaymentFailed
	// Tier 2: Team lifecycle
	case events.EventMemberInvited:
		return models.WebhookEventMemberInvited
	case events.EventMemberJoined:
		return models.WebhookEventMemberJoined
	case events.EventMemberRemoved:
		return models.WebhookEventMemberRemoved
	case events.EventMemberRoleChanged:
		return models.WebhookEventMemberRoleChanged
	case events.EventOwnershipTransferred:
		return models.WebhookEventOwnershipTransferred
	// Tier 3: User lifecycle
	case events.EventUserRegistered:
		return models.WebhookEventUserRegistered
	case events.EventUserVerified:
		return models.WebhookEventUserVerified
	case events.EventUserDeactivated:
		return models.WebhookEventUserDeactivated
	// Tier 4: Credits & billing details
	case events.EventCreditsPurchased:
		return models.WebhookEventCreditsPurchased
	case events.EventPlanChanged:
		return models.WebhookEventPlanChanged
	case events.EventTenantCreated:
		return models.WebhookEventTenantCreated
	case events.EventTenantDeactivated:
		return models.WebhookEventTenantDeactivated
	// Tier 5: Audit & security
	case events.EventUserDeleted:
		return models.WebhookEventUserDeleted
	case events.EventTenantDeleted:
		return models.WebhookEventTenantDeleted
	case events.EventAPIKeyCreated:
		return models.WebhookEventAPIKeyCreated
	case events.EventAPIKeyRevoked:
		return models.WebhookEventAPIKeyRevoked
	default:
		return ""
	}
}

func (d *Dispatcher) dispatch(eventType models.WebhookEventType, event events.Event) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cursor, err := d.db.Webhooks().Find(ctx, bson.M{
		"events":   eventType,
		"isActive": true,
	})
	if err != nil {
		slog.Error("webhooks: failed to query webhooks", "event_type", eventType, "error", err)
		return
	}
	defer cursor.Close(ctx)

	var hooks []models.Webhook
	if err := cursor.All(ctx, &hooks); err != nil {
		slog.Error("webhooks: failed to decode webhooks", "error", err)
		return
	}

	for _, hook := range hooks {
		d.deliver(ctx, hook, eventType, event)
	}
}

const maxWebhookRetries = 3

// retryDelays defines exponential backoff delays for webhook retries.
var retryDelays = [maxWebhookRetries]time.Duration{1 * time.Minute, 5 * time.Minute, 30 * time.Minute}

// deliver sends a single webhook and records the delivery. Schedules retries on failure.
func (d *Dispatcher) deliver(ctx context.Context, hook models.Webhook, eventType models.WebhookEventType, event events.Event) {
	d.deliverWithRetry(ctx, hook, eventType, event, 0)
}

func (d *Dispatcher) deliverWithRetry(ctx context.Context, hook models.Webhook, eventType models.WebhookEventType, event events.Event, retryCount int) {
	payload := map[string]interface{}{
		"event":     string(eventType),
		"timestamp": event.Timestamp.UTC().Format(time.RFC3339),
		"data":      event.Data,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		slog.Error("webhooks: failed to marshal payload", "error", err)
		return
	}

	deliverCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(deliverCtx, "POST", hook.URL, bytes.NewReader(body))
	if err != nil {
		slog.Error("webhooks: failed to create request", "webhook", hook.Name, "error", err)
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Webhook-Event", string(eventType))

	// Sign with HMAC-SHA256
	if hook.Secret != "" {
		secret := d.resolveSecret(hook.Secret)
		if secret != "" {
			sig := computeSignature(body, secret)
			req.Header.Set("X-Webhook-Signature", sig)
		}
	}

	start := time.Now()
	resp, err := d.client.Do(req)
	duration := time.Since(start).Milliseconds()

	delivery := models.WebhookDelivery{
		ID:         primitive.NewObjectID(),
		WebhookID:  hook.ID,
		EventType:  eventType,
		Payload:    string(body),
		Duration:   duration,
		RetryCount: retryCount,
		MaxRetries: maxWebhookRetries,
		CreatedAt:  time.Now(),
	}

	if err != nil {
		delivery.Success = false
		delivery.ResponseCode = 0
		delivery.ResponseBody = err.Error()
	} else {
		defer resp.Body.Close()
		// Read limited response body for logging
		var respBuf bytes.Buffer
		respBuf.ReadFrom(io.LimitReader(resp.Body, 4096))
		delivery.ResponseCode = resp.StatusCode
		delivery.ResponseBody = respBuf.String()
		delivery.Success = resp.StatusCode >= 200 && resp.StatusCode < 300
	}

	if _, err := d.db.WebhookDeliveries().InsertOne(deliverCtx, delivery); err != nil {
		slog.Error("webhooks: failed to record delivery", "webhook", hook.Name, "error", err)
	}

	if !delivery.Success {
		slog.Warn("webhooks: delivery failed", "webhook", hook.Name, "status", delivery.ResponseCode, "retry", retryCount, "max_retries", maxWebhookRetries)
		if retryCount < maxWebhookRetries {
			delay := retryDelays[retryCount]
			select {
			case d.retryQ <- retryJob{
				hook:      hook,
				eventType: eventType,
				event:     event,
				retry:     retryCount + 1,
				fireAt:    time.Now().Add(delay),
			}:
			default:
				slog.Warn("webhooks: retry queue full, dropping retry", "webhook", hook.Name)
			}
		}
	}
}

// resolveSecret decrypts an encrypted secret if an encryption key is configured,
// otherwise returns the value as-is (plaintext fallback for migration).
func (d *Dispatcher) resolveSecret(stored string) string {
	if d.encryptionKey == nil || stored == "" {
		return stored
	}
	plaintext, err := DecryptSecret(stored, d.encryptionKey)
	if err != nil {
		// Fallback: may be a legacy plaintext secret not yet migrated
		if len(stored) > 0 && stored[0] != 0 {
			return stored
		}
		slog.Error("webhooks: failed to decrypt secret", "error", err)
		return ""
	}
	return plaintext
}

// computeSignature generates an HMAC-SHA256 hex digest.
func computeSignature(payload []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return fmt.Sprintf("sha256=%s", hex.EncodeToString(mac.Sum(nil)))
}

// DeliverTest sends a test event to a specific webhook and returns the delivery record.
func (d *Dispatcher) DeliverTest(ctx context.Context, hook models.Webhook) models.WebhookDelivery {
	testEvent := events.Event{
		Type:      events.EventTenantCreated,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"tenantId":   "test_" + primitive.NewObjectID().Hex()[:8],
			"tenantName": "Test Tenant",
			"userId":     "test_" + primitive.NewObjectID().Hex()[:8],
			"test":       true,
		},
	}

	payload := map[string]interface{}{
		"event":     string(models.WebhookEventTenantCreated),
		"timestamp": testEvent.Timestamp.UTC().Format(time.RFC3339),
		"data":      testEvent.Data,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		slog.Error("webhooks: failed to marshal test payload", "error", err)
		return models.WebhookDelivery{
			ID:           primitive.NewObjectID(),
			WebhookID:    hook.ID,
			EventType:    models.WebhookEventTenantCreated,
			ResponseCode: 0,
			ResponseBody: fmt.Sprintf("marshal error: %v", err),
			Success:      false,
			CreatedAt:    time.Now(),
		}
	}

	req, err := http.NewRequestWithContext(ctx, "POST", hook.URL, bytes.NewReader(body))
	if err != nil {
		return models.WebhookDelivery{
			ID:           primitive.NewObjectID(),
			WebhookID:    hook.ID,
			EventType:    models.WebhookEventTenantCreated,
			Payload:      string(body),
			ResponseCode: 0,
			ResponseBody: err.Error(),
			Success:      false,
			CreatedAt:    time.Now(),
		}
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Webhook-Event", string(models.WebhookEventTenantCreated))
	req.Header.Set("X-Webhook-Test", "true")

	if hook.Secret != "" {
		secret := d.resolveSecret(hook.Secret)
		if secret != "" {
			sig := computeSignature(body, secret)
			req.Header.Set("X-Webhook-Signature", sig)
		}
	}

	start := time.Now()
	resp, err := d.client.Do(req)
	duration := time.Since(start).Milliseconds()

	delivery := models.WebhookDelivery{
		ID:        primitive.NewObjectID(),
		WebhookID: hook.ID,
		EventType: models.WebhookEventTenantCreated,
		Payload:   string(body),
		Duration:  duration,
		CreatedAt: time.Now(),
	}

	if err != nil {
		delivery.Success = false
		delivery.ResponseCode = 0
		delivery.ResponseBody = err.Error()
	} else {
		defer resp.Body.Close()
		var respBuf bytes.Buffer
		respBuf.ReadFrom(io.LimitReader(resp.Body, 4096))
		delivery.ResponseCode = resp.StatusCode
		delivery.ResponseBody = respBuf.String()
		delivery.Success = resp.StatusCode >= 200 && resp.StatusCode < 300
	}

	if _, err := d.db.WebhookDeliveries().InsertOne(ctx, delivery); err != nil {
		slog.Error("webhooks: failed to record test delivery", "webhook", hook.Name, "error", err)
	}

	return delivery
}
