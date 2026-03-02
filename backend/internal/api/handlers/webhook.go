package handlers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"lastsaas/internal/db"
	"lastsaas/internal/events"
	"lastsaas/internal/models"
	stripeservice "lastsaas/internal/stripe"
	"lastsaas/internal/syslog"
	"lastsaas/internal/telemetry"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	mongoDB "go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	stripe "github.com/stripe/stripe-go/v82"
)

type WebhookHandler struct {
	stripe       *stripeservice.Service
	db           *db.MongoDB
	events       events.Emitter
	syslog       *syslog.Logger
	getConfig    func(string) string
	telemetrySvc *telemetry.Service
}

func (h *WebhookHandler) SetTelemetry(svc *telemetry.Service) { h.telemetrySvc = svc }

func NewWebhookHandler(stripeSvc *stripeservice.Service, database *db.MongoDB, emitter events.Emitter, sysLogger *syslog.Logger, getConfig func(string) string) *WebhookHandler {
	return &WebhookHandler{
		stripe:    stripeSvc,
		db:        database,
		events:    emitter,
		syslog:    sysLogger,
		getConfig: getConfig,
	}
}

func (h *WebhookHandler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 524288)) // 512KB — Stripe events with expanded objects can exceed 64KB
	if err != nil {
		http.Error(w, "read error", http.StatusBadRequest)
		return
	}

	event, err := h.stripe.ConstructEvent(body, r.Header.Get("Stripe-Signature"))
	if err != nil {
		slog.Error("Webhook signature verification failed", "error", err)
		http.Error(w, "invalid signature", http.StatusBadRequest)
		return
	}

	slog.Info("Webhook: received event", "eventId", event.ID, "type", event.Type)

	ctx := r.Context()

	// Atomic idempotency check: upsert with $setOnInsert so only the first
	// request creates the record. If the document already existed, it's a duplicate.
	idempResult := h.db.WebhookEvents().FindOneAndUpdate(ctx,
		bson.M{"eventId": event.ID},
		bson.M{"$setOnInsert": bson.M{
			"eventId":   event.ID,
			"type":      string(event.Type),
			"status":    "processing",
			"createdAt": time.Now(),
		}},
		options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.Before),
	)
	if idempResult.Err() == nil {
		// Document existed before our upsert — duplicate event
		slog.Warn("Webhook: duplicate event, skipping", "eventId", event.ID)
		w.WriteHeader(http.StatusOK)
		return
	}
	if idempResult.Err() != mongoDB.ErrNoDocuments {
		slog.Error("Webhook: idempotency check failed", "eventId", event.ID, "error", idempResult.Err())
		http.Error(w, "idempotency check failed", http.StatusInternalServerError)
		return
	}

	// Multi-instance filtering: if this instance has an instance ID configured,
	// skip events that were created by a different instance. Checkout sessions and
	// subscriptions carry an "instance" key in their metadata; invoices are filtered
	// by subscription lookup (each instance has its own database).
	if instanceID := h.stripe.InstanceID(); instanceID != "" {
		if inst, found := extractInstanceFromEvent(event); found && inst != instanceID {
			slog.Info("Webhook: skipping event from different instance", "eventId", event.ID, "eventInstance", inst, "ourInstance", instanceID)
			w.WriteHeader(http.StatusOK)
			return
		}
	}

	slog.Info("Webhook: processing event", "eventId", event.ID, "type", event.Type)

	var processingErr error
	switch event.Type {
	case "checkout.session.completed":
		processingErr = h.handleCheckoutCompleted(ctx, event)
	case "invoice.paid":
		processingErr = h.handleInvoicePaid(ctx, event)
	case "invoice.payment_failed":
		processingErr = h.handleInvoicePaymentFailed(ctx, event)
	case "customer.subscription.updated":
		processingErr = h.handleSubscriptionUpdated(ctx, event)
	case "customer.subscription.deleted":
		processingErr = h.handleSubscriptionDeleted(ctx, event)
	case "charge.refunded":
		processingErr = h.handleChargeRefunded(ctx, event)
	case "charge.dispute.created":
		processingErr = h.handleDisputeCreated(ctx, event)
	case "charge.dispute.closed":
		processingErr = h.handleDisputeClosed(ctx, event)
	default:
		slog.Warn("Webhook: unhandled event type", "type", event.Type)
	}

	// If processing failed, remove the idempotency record so Stripe can retry.
	if processingErr != nil {
		slog.Error("Webhook: processing failed, removing idempotency record for retry", "eventId", event.ID, "error", processingErr)
		h.db.WebhookEvents().DeleteOne(ctx, bson.M{"eventId": event.ID})
		http.Error(w, "processing failed", http.StatusInternalServerError)
		return
	}

	// Mark as completed
	h.db.WebhookEvents().UpdateOne(ctx, bson.M{"eventId": event.ID}, bson.M{
		"$set": bson.M{"status": "completed"},
	})

	w.WriteHeader(http.StatusOK)
}

func (h *WebhookHandler) handleCheckoutCompleted(ctx context.Context, event stripe.Event) error {
	var session stripe.CheckoutSession
	if err := json.Unmarshal(event.Data.Raw, &session); err != nil {
		slog.Error("Webhook: failed to unmarshal checkout session", "error", err)
		return fmt.Errorf("unmarshal checkout session: %w", err)
	}

	tenantID, _ := primitive.ObjectIDFromHex(session.Metadata["tenantId"])
	userID, _ := primitive.ObjectIDFromHex(session.Metadata["userId"])
	if tenantID.IsZero() || userID.IsZero() {
		slog.Error("Webhook: missing tenantId or userId in session metadata")
		return fmt.Errorf("missing tenantId or userId in session metadata")
	}

	// Cross-reference: verify Stripe customer matches the tenant to prevent metadata manipulation
	if session.Customer != nil && session.Customer.ID != "" {
		var checkTenant models.Tenant
		if err := h.db.Tenants().FindOne(ctx, bson.M{"_id": tenantID}).Decode(&checkTenant); err != nil {
			slog.Error("Webhook: tenant not found for ID", "tenantId", tenantID.Hex())
			return fmt.Errorf("tenant not found for checkout: %w", err)
		}
		if checkTenant.StripeCustomerID != "" && checkTenant.StripeCustomerID != session.Customer.ID {
			h.syslog.Critical(ctx, fmt.Sprintf("SECURITY: Checkout customer mismatch — session customer %s != tenant %s customer %s",
				session.Customer.ID, tenantID.Hex(), checkTenant.StripeCustomerID))
			return fmt.Errorf("customer ID mismatch: session=%s tenant=%s", session.Customer.ID, checkTenant.StripeCustomerID)
		}
		// When tenant has no customer yet, verify no other tenant already uses this customer ID
		if checkTenant.StripeCustomerID == "" {
			var otherTenant models.Tenant
			if err := h.db.Tenants().FindOne(ctx, bson.M{
				"stripeCustomerId": session.Customer.ID,
				"_id":              bson.M{"$ne": tenantID},
			}).Decode(&otherTenant); err == nil {
				h.syslog.Critical(ctx, fmt.Sprintf("SECURITY: Customer %s already belongs to tenant %s, but checkout metadata claims tenant %s",
					session.Customer.ID, otherTenant.ID.Hex(), tenantID.Hex()))
				return fmt.Errorf("customer %s already belongs to another tenant", session.Customer.ID)
			}
		}
	}

	planIDStr := session.Metadata["planId"]
	bundleIDStr := session.Metadata["bundleId"]
	billingInterval := session.Metadata["billingInterval"]

	if planIDStr != "" {
		// Subscription checkout
		planID, _ := primitive.ObjectIDFromHex(planIDStr)
		var plan models.Plan
		if err := h.db.Plans().FindOne(ctx, bson.M{"_id": planID}).Decode(&plan); err != nil {
			slog.Error("Webhook: plan not found", "planId", planIDStr)
			return fmt.Errorf("plan not found: %s: %w", planIDStr, err)
		}

		subscriptionID := ""
		if session.Subscription != nil {
			subscriptionID = session.Subscription.ID
		}

		// Calculate period end from subscription items
		var periodEnd *time.Time
		if session.Subscription != nil && session.Subscription.Items != nil && len(session.Subscription.Items.Data) > 0 {
			t := time.Unix(session.Subscription.Items.Data[0].CurrentPeriodEnd, 0)
			periodEnd = &t
		}

		// Update tenant
		updates := bson.M{
			"planId":               planID,
			"billingStatus":        models.BillingStatusActive,
			"stripeSubscriptionId": subscriptionID,
			"billingInterval":      billingInterval,
			"canceledAt":           nil,
			"updatedAt":            time.Now(),
		}
		if periodEnd != nil {
			updates["currentPeriodEnd"] = periodEnd
		}
		// Mark trial as used so tenant (and user) can't get another free trial
		if session.Subscription != nil && session.Subscription.TrialEnd > 0 {
			now := time.Now()
			updates["trialUsedAt"] = &now
			// Also mark user-level trial usage to prevent multi-tenant trial abuse
			h.db.Users().UpdateOne(ctx, bson.M{"_id": userID}, bson.M{
				"$set": bson.M{"trialUsedAt": &now},
			})
		}
		// Store seat quantity for per-seat plans
		if seatQtyStr := session.Metadata["seatQuantity"]; seatQtyStr != "" {
			if seatQty, err := strconv.Atoi(seatQtyStr); err == nil && seatQty > 0 {
				updates["seatQuantity"] = seatQty
			}
		}
		// Set subscription credits from plan (combined into single update)
		updates["subscriptionCredits"] = plan.UsageCreditsPerMonth
		updateOp := bson.M{"$set": updates}
		if plan.BonusCredits > 0 {
			updateOp["$inc"] = bson.M{"purchasedCredits": plan.BonusCredits}
		}
		if _, err := h.db.Tenants().UpdateOne(ctx, bson.M{"_id": tenantID}, updateOp); err != nil {
			slog.Error("Webhook: failed to update tenant", "tenantId", tenantID.Hex(), "error", err)
			return fmt.Errorf("update tenant: %w", err)
		}

		// Record transaction with tax breakdown
		amountCents := int64(0)
		if session.AmountTotal > 0 {
			amountCents = session.AmountTotal
		}
		taxAmountCents := int64(0)
		subtotalCents := amountCents
		if session.TotalDetails != nil && session.TotalDetails.AmountTax > 0 {
			taxAmountCents = session.TotalDetails.AmountTax
			subtotalCents = amountCents - taxAmountCents
		}
		h.recordTransaction(ctx, tenantID, userID, models.TransactionSubscription, amountCents, subtotalCents, taxAmountCents, plan.Name, billingInterval, &planID, nil, subscriptionID, session.ID)

		h.syslog.High(ctx, fmt.Sprintf("Subscription activated: tenant %s, plan %s (%s), amount $%.2f",
			tenantID.Hex(), plan.Name, billingInterval, float64(amountCents)/100))

		h.events.Emit(events.Event{
			Type:      events.EventSubscriptionActivated,
			Timestamp: time.Now(),
			Data: map[string]interface{}{
				"tenantId":        tenantID.Hex(),
				"planId":          planID.Hex(),
				"planName":        plan.Name,
				"billingInterval": billingInterval,
				"amountCents":     amountCents,
			},
		})
		h.events.Emit(events.Event{
			Type:      events.EventPlanChanged,
			Timestamp: time.Now(),
			Data: map[string]interface{}{
				"tenantId": tenantID.Hex(),
				"planId":   planID.Hex(),
				"planName": plan.Name,
			},
		})

		if h.telemetrySvc != nil {
			h.telemetrySvc.Track(ctx, models.TelemetryEvent{
				EventName:  models.TelemetrySubscriptionActivated,
				Category:   models.TelemetryCategoryFunnel,
				UserID:     &userID,
				TenantID:   &tenantID,
				Properties: map[string]interface{}{"planName": plan.Name, "billingInterval": billingInterval},
			})
			h.telemetrySvc.Track(ctx, models.TelemetryEvent{
				EventName:  models.TelemetryPlanChanged,
				Category:   models.TelemetryCategoryFunnel,
				UserID:     &userID,
				TenantID:   &tenantID,
				Properties: map[string]interface{}{"planName": plan.Name},
			})
		}

	} else if bundleIDStr != "" {
		// One-time bundle purchase
		bundleID, _ := primitive.ObjectIDFromHex(bundleIDStr)
		var bundle models.CreditBundle
		if err := h.db.CreditBundles().FindOne(ctx, bson.M{"_id": bundleID}).Decode(&bundle); err != nil {
			slog.Error("Webhook: bundle not found", "bundleId", bundleIDStr)
			return fmt.Errorf("bundle not found: %s: %w", bundleIDStr, err)
		}

		// Add credits to tenant
		if _, err := h.db.Tenants().UpdateOne(ctx, bson.M{"_id": tenantID}, bson.M{
			"$inc": bson.M{"purchasedCredits": bundle.Credits},
			"$set": bson.M{"updatedAt": time.Now()},
		}); err != nil {
			slog.Error("Webhook: failed to add bundle credits to tenant", "tenantId", tenantID.Hex(), "error", err)
			return fmt.Errorf("add bundle credits: %w", err)
		}

		amountCents := int64(0)
		if session.AmountTotal > 0 {
			amountCents = session.AmountTotal
		}
		bundleTax := int64(0)
		bundleSubtotal := amountCents
		if session.TotalDetails != nil && session.TotalDetails.AmountTax > 0 {
			bundleTax = session.TotalDetails.AmountTax
			bundleSubtotal = amountCents - bundleTax
		}
		h.recordTransaction(ctx, tenantID, userID, models.TransactionCreditPurchase, amountCents, bundleSubtotal, bundleTax, bundle.Name, "", nil, &bundleID, "", session.ID)

		h.syslog.High(ctx, fmt.Sprintf("Credit bundle purchased: tenant %s, bundle %s (%d credits), amount $%.2f",
			tenantID.Hex(), bundle.Name, bundle.Credits, float64(amountCents)/100))

		h.events.Emit(events.Event{
			Type:      events.EventCreditsPurchased,
			Timestamp: time.Now(),
			Data: map[string]interface{}{
				"tenantId":    tenantID.Hex(),
				"bundleId":    bundleID.Hex(),
				"bundleName":  bundle.Name,
				"credits":     bundle.Credits,
				"amountCents": amountCents,
			},
		})
	}
	return nil
}

func (h *WebhookHandler) handleInvoicePaid(ctx context.Context, event stripe.Event) error {
	var invoice stripe.Invoice
	if err := json.Unmarshal(event.Data.Raw, &invoice); err != nil {
		slog.Error("Webhook: failed to unmarshal invoice", "error", err)
		return fmt.Errorf("unmarshal invoice: %w", err)
	}

	// Skip the first invoice (handled by checkout.session.completed)
	billingReason := ""
	if invoice.BillingReason != "" {
		billingReason = string(invoice.BillingReason)
	}
	if billingReason == "subscription_create" {
		return nil
	}

	subscriptionID := ""
	if invoice.Parent != nil && invoice.Parent.SubscriptionDetails != nil && invoice.Parent.SubscriptionDetails.Subscription != nil {
		subscriptionID = invoice.Parent.SubscriptionDetails.Subscription.ID
	}
	if subscriptionID == "" {
		return nil
	}

	var tenant models.Tenant
	if err := h.db.Tenants().FindOne(ctx, bson.M{"stripeSubscriptionId": subscriptionID}).Decode(&tenant); err != nil {
		slog.Error("Webhook: tenant not found for subscription", "subscriptionId", subscriptionID)
		return fmt.Errorf("tenant not found for subscription %s: %w", subscriptionID, err)
	}

	// Ensure active status
	if _, err := h.db.Tenants().UpdateOne(ctx, bson.M{"_id": tenant.ID}, bson.M{
		"$set": bson.M{"billingStatus": models.BillingStatusActive, "updatedAt": time.Now()},
	}); err != nil {
		slog.Error("Webhook: failed to set billing status active for tenant", "tenantId", tenant.ID.Hex(), "error", err)
		return fmt.Errorf("set billing status: %w", err)
	}

	// Handle credit reset/accrue
	if tenant.PlanID != nil {
		var plan models.Plan
		if h.db.Plans().FindOne(ctx, bson.M{"_id": *tenant.PlanID}).Decode(&plan) == nil {
			if plan.CreditResetPolicy == models.CreditResetPolicyReset {
				if _, err := h.db.Tenants().UpdateOne(ctx, bson.M{"_id": tenant.ID}, bson.M{
					"$set": bson.M{"subscriptionCredits": plan.UsageCreditsPerMonth},
				}); err != nil {
					slog.Error("Webhook: failed to reset credits for tenant", "tenantId", tenant.ID.Hex(), "error", err)
				}
			} else {
				if _, err := h.db.Tenants().UpdateOne(ctx, bson.M{"_id": tenant.ID}, bson.M{
					"$inc": bson.M{"subscriptionCredits": plan.UsageCreditsPerMonth},
				}); err != nil {
					slog.Error("Webhook: failed to accrue credits for tenant", "tenantId", tenant.ID.Hex(), "error", err)
				}
			}
		}
	}

	// Record transaction with tax breakdown.
	// Note: TotalExcludingTax may be 0 for invoices created before tax was enabled — in that
	// case we correctly report 0 tax. We only compute tax when both values are positive.
	amountCents := invoice.AmountPaid
	invoiceTax := int64(0)
	invoiceSubtotal := amountCents
	if invoice.Total > 0 && invoice.TotalExcludingTax > 0 && invoice.Total > invoice.TotalExcludingTax {
		invoiceTax = invoice.Total - invoice.TotalExcludingTax
		invoiceSubtotal = invoice.TotalExcludingTax
	}
	// Find the owner of this tenant for the transaction record
	var membership models.TenantMembership
	if err := h.db.TenantMemberships().FindOne(ctx, bson.M{"tenantId": tenant.ID, "role": models.RoleOwner}).Decode(&membership); err != nil {
		slog.Warn("Webhook: owner membership not found for tenant", "tenantId", tenant.ID.Hex(), "error", err)
	}

	planName := ""
	if tenant.PlanID != nil {
		var plan models.Plan
		if h.db.Plans().FindOne(ctx, bson.M{"_id": *tenant.PlanID}).Decode(&plan) == nil {
			planName = plan.Name
		}
	}

	h.recordTransaction(ctx, tenant.ID, membership.UserID, models.TransactionSubscription, amountCents, invoiceSubtotal, invoiceTax, planName, tenant.BillingInterval, tenant.PlanID, nil, subscriptionID, "")

	h.syslog.High(ctx, fmt.Sprintf("Subscription payment received: tenant %s, amount $%.2f",
		tenant.ID.Hex(), float64(amountCents)/100))

	h.events.Emit(events.Event{
		Type:      events.EventPaymentReceived,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"tenantId":    tenant.ID.Hex(),
			"amountCents": amountCents,
			"currency":    "usd",
			"planName":    planName,
		},
	})
	return nil
}

func (h *WebhookHandler) handleInvoicePaymentFailed(ctx context.Context, event stripe.Event) error {
	var invoice stripe.Invoice
	if err := json.Unmarshal(event.Data.Raw, &invoice); err != nil {
		slog.Error("Webhook: failed to unmarshal invoice", "error", err)
		return fmt.Errorf("unmarshal invoice: %w", err)
	}

	subscriptionID := ""
	if invoice.Parent != nil && invoice.Parent.SubscriptionDetails != nil && invoice.Parent.SubscriptionDetails.Subscription != nil {
		subscriptionID = invoice.Parent.SubscriptionDetails.Subscription.ID
	}
	if subscriptionID == "" {
		return nil
	}

	var tenant models.Tenant
	if err := h.db.Tenants().FindOne(ctx, bson.M{"stripeSubscriptionId": subscriptionID}).Decode(&tenant); err != nil {
		slog.Error("Webhook: tenant not found for subscription", "subscriptionId", subscriptionID)
		return fmt.Errorf("tenant not found for subscription %s: %w", subscriptionID, err)
	}

	// Set past_due
	if _, err := h.db.Tenants().UpdateOne(ctx, bson.M{"_id": tenant.ID}, bson.M{
		"$set": bson.M{"billingStatus": models.BillingStatusPastDue, "updatedAt": time.Now()},
	}); err != nil {
		slog.Error("Webhook: failed to set past_due for tenant", "tenantId", tenant.ID.Hex(), "error", err)
		return fmt.Errorf("set past_due: %w", err)
	}

	// Send in-app message to all users in the tenant
	cursor, _ := h.db.TenantMemberships().Find(ctx, bson.M{"tenantId": tenant.ID})
	var memberships []models.TenantMembership
	if cursor != nil {
		cursor.All(ctx, &memberships)
		cursor.Close(ctx)
	}

	subject := h.getConfig("billing.failed_charge.message_subject")
	body := h.getConfig("billing.failed_charge.message_body")
	appName := h.getConfig("app.name")

	// Simple template substitution
	subject = templateReplace(subject, map[string]string{"AppName": appName})
	body = templateReplace(body, map[string]string{
		"AppName":    appName,
		"BillingURL": "/settings",
	})

	for _, m := range memberships {
		h.db.Messages().InsertOne(ctx, models.Message{
			UserID:    m.UserID,
			Subject:   subject,
			Body:      body,
			IsSystem:  true,
			Read:      false,
			CreatedAt: time.Now(),
		})
	}

	h.syslog.High(ctx, fmt.Sprintf("Payment failed: tenant %s (%s), subscription %s",
		tenant.ID.Hex(), tenant.Name, subscriptionID))

	h.events.Emit(events.Event{
		Type:      events.EventPaymentFailed,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"tenantId":   tenant.ID.Hex(),
			"tenantName": tenant.Name,
		},
	})
	return nil
}

func (h *WebhookHandler) handleSubscriptionUpdated(ctx context.Context, event stripe.Event) error {
	var sub stripe.Subscription
	if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
		slog.Error("Webhook: failed to unmarshal subscription", "error", err)
		return fmt.Errorf("unmarshal subscription: %w", err)
	}

	var tenant models.Tenant
	if err := h.db.Tenants().FindOne(ctx, bson.M{"stripeSubscriptionId": sub.ID}).Decode(&tenant); err != nil {
		return nil // Tenant not found for this subscription — not an error, may belong to another instance
	}

	updates := bson.M{"updatedAt": time.Now()}

	// Update period end from items
	if sub.Items != nil && len(sub.Items.Data) > 0 {
		periodEnd := time.Unix(sub.Items.Data[0].CurrentPeriodEnd, 0)
		updates["currentPeriodEnd"] = periodEnd
	}

	if sub.CancelAtPeriodEnd {
		updates["billingStatus"] = models.BillingStatusCanceled
		now := time.Now()
		updates["canceledAt"] = now
		h.syslog.High(ctx, fmt.Sprintf("Subscription set to cancel at period end: tenant %s", tenant.ID.Hex()))

		h.events.Emit(events.Event{
			Type:      events.EventSubscriptionCanceled,
			Timestamp: time.Now(),
			Data: map[string]interface{}{
				"tenantId":   tenant.ID.Hex(),
				"tenantName": tenant.Name,
				"reason":     "cancel_at_period_end",
			},
		})
	}

	// Sync seat quantity for per-seat plans
	if tenant.PlanID != nil {
		var plan models.Plan
		if h.db.Plans().FindOne(ctx, bson.M{"_id": *tenant.PlanID}).Decode(&plan) == nil && plan.PricingModel == models.PricingModelPerSeat {
			if sub.Items != nil {
				maxQty := int64(0)
				for _, item := range sub.Items.Data {
					if item.Quantity > maxQty {
						maxQty = item.Quantity
					}
				}
				if maxQty > 0 {
					updates["seatQuantity"] = int(maxQty)
				}
			}
		}
	}

	if _, err := h.db.Tenants().UpdateOne(ctx, bson.M{"_id": tenant.ID}, bson.M{"$set": updates}); err != nil {
		slog.Error("Webhook: failed to update tenant on subscription update", "tenantId", tenant.ID.Hex(), "error", err)
		return fmt.Errorf("update tenant: %w", err)
	}
	return nil
}

func (h *WebhookHandler) handleSubscriptionDeleted(ctx context.Context, event stripe.Event) error {
	var sub stripe.Subscription
	if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
		slog.Error("Webhook: failed to unmarshal subscription", "error", err)
		return fmt.Errorf("unmarshal subscription: %w", err)
	}

	var tenant models.Tenant
	if err := h.db.Tenants().FindOne(ctx, bson.M{"stripeSubscriptionId": sub.ID}).Decode(&tenant); err != nil {
		return nil // Tenant not found for this subscription — not an error
	}

	// Find the Free (system) plan
	var freePlan models.Plan
	err := h.db.Plans().FindOne(ctx, bson.M{"isSystem": true}).Decode(&freePlan)

	updates := bson.M{
		"billingStatus":        models.BillingStatusNone,
		"stripeSubscriptionId": "",
		"billingInterval":      "",
		"currentPeriodEnd":     nil,
		"canceledAt":           nil,
		"subscriptionCredits":  int64(0),
		"seatQuantity":         0,
		"updatedAt":            time.Now(),
	}
	if err == nil {
		updates["planId"] = freePlan.ID
	}

	if _, err := h.db.Tenants().UpdateOne(ctx, bson.M{"_id": tenant.ID}, bson.M{"$set": updates}); err != nil {
		slog.Error("Webhook: failed to downgrade tenant", "tenantId", tenant.ID.Hex(), "error", err)
		return fmt.Errorf("downgrade tenant: %w", err)
	}

	h.syslog.High(ctx, fmt.Sprintf("Subscription ended: tenant %s (%s), downgraded to Free",
		tenant.ID.Hex(), tenant.Name))

	h.events.Emit(events.Event{
		Type:      events.EventSubscriptionCanceled,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"tenantId":   tenant.ID.Hex(),
			"tenantName": tenant.Name,
			"reason":     "subscription_ended",
		},
	})
	h.events.Emit(events.Event{
		Type:      events.EventPlanChanged,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"tenantId": tenant.ID.Hex(),
			"planName": "Free",
		},
	})
	return nil
}

func (h *WebhookHandler) handleChargeRefunded(ctx context.Context, event stripe.Event) error {
	var charge stripe.Charge
	if err := json.Unmarshal(event.Data.Raw, &charge); err != nil {
		slog.Error("Webhook: failed to unmarshal charge", "error", err)
		return fmt.Errorf("unmarshal charge: %w", err)
	}

	// Find tenant by Stripe customer ID
	if charge.Customer == nil {
		return nil
	}
	var tenant models.Tenant
	if err := h.db.Tenants().FindOne(ctx, bson.M{"stripeCustomerId": charge.Customer.ID}).Decode(&tenant); err != nil {
		slog.Warn("Webhook: tenant not found for customer on refund", "customerId", charge.Customer.ID)
		return nil // Not an error — may belong to another instance
	}

	refundedAmount := charge.AmountRefunded

	h.syslog.High(ctx, fmt.Sprintf("Refund received: tenant %s (%s), amount $%.2f",
		tenant.ID.Hex(), tenant.Name, float64(refundedAmount)/100))

	// Record refund transaction
	var ownerMembership models.TenantMembership
	ownerUserID := tenant.ID // fallback to tenant ID if owner lookup fails
	if err := h.db.TenantMemberships().FindOne(ctx, bson.M{"tenantId": tenant.ID, "role": models.RoleOwner}).Decode(&ownerMembership); err != nil {
		slog.Warn("Webhook: owner membership not found for tenant on refund", "tenantId", tenant.ID.Hex(), "error", err)
	} else {
		ownerUserID = ownerMembership.UserID
	}

	h.recordTransaction(ctx, tenant.ID, ownerUserID, models.TransactionRefund, -refundedAmount, -refundedAmount, 0, "Refund", "", nil, nil, "", charge.ID)

	h.events.Emit(events.Event{
		Type:      "billing.refund_received",
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"tenantId":    tenant.ID.Hex(),
			"tenantName":  tenant.Name,
			"amountCents": refundedAmount,
		},
	})

	return nil
}

func (h *WebhookHandler) handleDisputeCreated(ctx context.Context, event stripe.Event) error {
	var dispute stripe.Dispute
	if err := json.Unmarshal(event.Data.Raw, &dispute); err != nil {
		slog.Error("Webhook: failed to unmarshal dispute", "error", err)
		return fmt.Errorf("unmarshal dispute: %w", err)
	}

	// Find tenant by charge's customer
	customerID := ""
	if dispute.Charge != nil && dispute.Charge.Customer != nil {
		customerID = dispute.Charge.Customer.ID
	}
	if customerID == "" {
		return nil
	}

	var tenant models.Tenant
	if err := h.db.Tenants().FindOne(ctx, bson.M{"stripeCustomerId": customerID}).Decode(&tenant); err != nil {
		slog.Warn("Webhook: tenant not found for customer on dispute", "customerId", customerID)
		return nil
	}

	// Set billing status to past_due to restrict access during dispute
	if _, err := h.db.Tenants().UpdateOne(ctx, bson.M{"_id": tenant.ID}, bson.M{
		"$set": bson.M{"billingStatus": models.BillingStatusPastDue, "updatedAt": time.Now()},
	}); err != nil {
		slog.Error("Webhook: failed to set past_due for tenant on dispute", "tenantId", tenant.ID.Hex(), "error", err)
	}

	h.syslog.Critical(ctx, fmt.Sprintf("Payment dispute opened: tenant %s (%s), amount $%.2f, reason: %s",
		tenant.ID.Hex(), tenant.Name, float64(dispute.Amount)/100, dispute.Reason))

	h.events.Emit(events.Event{
		Type:      "billing.dispute_created",
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"tenantId":    tenant.ID.Hex(),
			"tenantName":  tenant.Name,
			"amountCents": dispute.Amount,
			"reason":      string(dispute.Reason),
		},
	})

	return nil
}

func (h *WebhookHandler) handleDisputeClosed(ctx context.Context, event stripe.Event) error {
	var dispute stripe.Dispute
	if err := json.Unmarshal(event.Data.Raw, &dispute); err != nil {
		slog.Error("Webhook: failed to unmarshal dispute", "error", err)
		return fmt.Errorf("unmarshal dispute: %w", err)
	}

	customerID := ""
	if dispute.Charge != nil && dispute.Charge.Customer != nil {
		customerID = dispute.Charge.Customer.ID
	}
	if customerID == "" {
		return nil
	}

	var tenant models.Tenant
	if err := h.db.Tenants().FindOne(ctx, bson.M{"stripeCustomerId": customerID}).Decode(&tenant); err != nil {
		return nil
	}

	won := dispute.Status == "won"
	if won {
		// Dispute resolved in our favor — restore active billing if subscription is still active
		if tenant.StripeSubscriptionID != "" {
			h.db.Tenants().UpdateOne(ctx, bson.M{"_id": tenant.ID}, bson.M{
				"$set": bson.M{"billingStatus": models.BillingStatusActive, "updatedAt": time.Now()},
			})
		}
		h.syslog.High(ctx, fmt.Sprintf("Payment dispute won: tenant %s (%s)", tenant.ID.Hex(), tenant.Name))
	} else {
		h.syslog.High(ctx, fmt.Sprintf("Payment dispute lost: tenant %s (%s), status: %s",
			tenant.ID.Hex(), tenant.Name, dispute.Status))
	}

	h.events.Emit(events.Event{
		Type:      "billing.dispute_closed",
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"tenantId":   tenant.ID.Hex(),
			"tenantName": tenant.Name,
			"status":     string(dispute.Status),
			"won":        won,
		},
	})

	return nil
}

func (h *WebhookHandler) recordTransaction(ctx context.Context, tenantID, userID primitive.ObjectID, txType models.TransactionType, amountCents, subtotalCents, taxAmountCents int64, itemName, interval string, planID, bundleID *primitive.ObjectID, stripeSubID, stripeSessionID string) {
	invoiceNum, err := h.stripe.NextInvoiceNumber(ctx)
	if err != nil {
		slog.Error("Failed to generate invoice number", "error", err)
		randBytes := make([]byte, 4)
		rand.Read(randBytes)
		invoiceNum = fmt.Sprintf("INV-ERR-%d-%s", time.Now().UnixNano(), hex.EncodeToString(randBytes))
	}

	desc := itemName
	if interval != "" {
		desc += " (" + interval + "ly)"
	}

	tx := models.FinancialTransaction{
		TenantID:             tenantID,
		UserID:               userID,
		Type:                 txType,
		AmountCents:          amountCents,
		SubtotalCents:        subtotalCents,
		TaxAmountCents:       taxAmountCents,
		Currency:             "usd",
		Description:          desc,
		InvoiceNumber:        invoiceNum,
		StripeSessionID:      stripeSessionID,
		StripeSubscriptionID: stripeSubID,
		BillingInterval:      interval,
		CreatedAt:            time.Now(),
	}
	if planID != nil {
		tx.PlanID = planID
		tx.PlanName = itemName
	}
	if bundleID != nil {
		tx.BundleID = bundleID
		tx.BundleName = itemName
	}

	if _, err := h.db.FinancialTransactions().InsertOne(ctx, tx); err != nil {
		slog.Error("Failed to record transaction", "error", err)
	}
}

// templateReplace does simple {{.Key}} replacement.
func templateReplace(tmpl string, vars map[string]string) string {
	for k, v := range vars {
		tmpl = replaceAll(tmpl, "{{."+k+"}}", v)
	}
	return tmpl
}

func replaceAll(s, old, new string) string {
	for {
		i := indexOf(s, old)
		if i < 0 {
			return s
		}
		s = s[:i] + new + s[i+len(old):]
	}
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// extractInstanceFromEvent extracts the "instance" metadata value from the event's
// top-level object. Works for checkout sessions, subscriptions, and any Stripe object
// that carries a metadata map. Returns ("", false) for objects without metadata (e.g. invoices).
func extractInstanceFromEvent(event stripe.Event) (string, bool) {
	var obj struct {
		Metadata map[string]string `json:"metadata"`
	}
	if err := json.Unmarshal(event.Data.Raw, &obj); err == nil && obj.Metadata != nil {
		if inst, ok := obj.Metadata["instance"]; ok {
			return inst, true
		}
	}
	return "", false
}

