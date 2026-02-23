package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	"lastsaas/internal/db"
	"lastsaas/internal/events"
	"lastsaas/internal/models"
	stripeservice "lastsaas/internal/stripe"
	"lastsaas/internal/syslog"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	mongoDB "go.mongodb.org/mongo-driver/mongo"

	stripe "github.com/stripe/stripe-go/v82"
)

type WebhookHandler struct {
	stripe    *stripeservice.Service
	db        *db.MongoDB
	events    events.Emitter
	syslog    *syslog.Logger
	getConfig func(string) string
}

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
	body, err := io.ReadAll(io.LimitReader(r.Body, 65536))
	if err != nil {
		http.Error(w, "read error", http.StatusBadRequest)
		return
	}

	event, err := h.stripe.ConstructEvent(body, r.Header.Get("Stripe-Signature"))
	if err != nil {
		log.Printf("Webhook signature verification failed: %v", err)
		http.Error(w, "invalid signature", http.StatusBadRequest)
		return
	}

	log.Printf("Webhook: received event %s type=%s", event.ID, event.Type)

	ctx := r.Context()

	// Idempotency check
	_, err = h.db.WebhookEvents().InsertOne(ctx, bson.M{
		"eventId":   event.ID,
		"type":      string(event.Type),
		"createdAt": time.Now(),
	})
	if err != nil {
		// Duplicate — already processed
		log.Printf("Webhook: duplicate event %s, skipping", event.ID)
		w.WriteHeader(http.StatusOK)
		return
	}

	// Multi-instance filtering: if this instance has an instance ID configured,
	// skip events that were created by a different instance. Checkout sessions and
	// subscriptions carry an "instance" key in their metadata; invoices are filtered
	// by subscription lookup (each instance has its own database).
	if instanceID := h.stripe.InstanceID(); instanceID != "" {
		if inst, found := extractInstanceFromEvent(event); found && inst != instanceID {
			log.Printf("Webhook: skipping event %s (instance %q != %q)", event.ID, inst, instanceID)
			w.WriteHeader(http.StatusOK)
			return
		}
	}

	log.Printf("Webhook: processing event %s type=%s", event.ID, event.Type)

	switch event.Type {
	case "checkout.session.completed":
		h.handleCheckoutCompleted(ctx, event)
	case "invoice.paid":
		h.handleInvoicePaid(ctx, event)
	case "invoice.payment_failed":
		h.handleInvoicePaymentFailed(ctx, event)
	case "customer.subscription.updated":
		h.handleSubscriptionUpdated(ctx, event)
	case "customer.subscription.deleted":
		h.handleSubscriptionDeleted(ctx, event)
	default:
		log.Printf("Webhook: unhandled event type %s", event.Type)
	}

	w.WriteHeader(http.StatusOK)
}

func (h *WebhookHandler) handleCheckoutCompleted(ctx context.Context, event stripe.Event) {
	var session stripe.CheckoutSession
	if err := json.Unmarshal(event.Data.Raw, &session); err != nil {
		log.Printf("Webhook: failed to unmarshal checkout session: %v", err)
		return
	}

	tenantID, _ := primitive.ObjectIDFromHex(session.Metadata["tenantId"])
	userID, _ := primitive.ObjectIDFromHex(session.Metadata["userId"])
	if tenantID.IsZero() || userID.IsZero() {
		log.Printf("Webhook: missing tenantId or userId in session metadata")
		return
	}

	planIDStr := session.Metadata["planId"]
	bundleIDStr := session.Metadata["bundleId"]
	billingInterval := session.Metadata["billingInterval"]

	if planIDStr != "" {
		// Subscription checkout
		planID, _ := primitive.ObjectIDFromHex(planIDStr)
		var plan models.Plan
		if err := h.db.Plans().FindOne(ctx, bson.M{"_id": planID}).Decode(&plan); err != nil {
			log.Printf("Webhook: plan not found: %s", planIDStr)
			return
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
		// Store seat quantity for per-seat plans
		if seatQtyStr := session.Metadata["seatQuantity"]; seatQtyStr != "" {
			if seatQty, err := strconv.Atoi(seatQtyStr); err == nil && seatQty > 0 {
				updates["seatQuantity"] = seatQty
			}
		}
		h.db.Tenants().UpdateOne(ctx, bson.M{"_id": tenantID}, bson.M{"$set": updates})

		// Set subscription credits from plan
		h.db.Tenants().UpdateOne(ctx, bson.M{"_id": tenantID}, bson.M{
			"$set": bson.M{"subscriptionCredits": plan.UsageCreditsPerMonth},
			"$inc": bson.M{"purchasedCredits": plan.BonusCredits},
		})

		// Record transaction
		amountCents := int64(0)
		if session.AmountTotal > 0 {
			amountCents = session.AmountTotal
		}
		h.recordTransaction(ctx, tenantID, userID, models.TransactionSubscription, amountCents, plan.Name, billingInterval, &planID, nil, subscriptionID, session.ID)

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

	} else if bundleIDStr != "" {
		// One-time bundle purchase
		bundleID, _ := primitive.ObjectIDFromHex(bundleIDStr)
		var bundle models.CreditBundle
		if err := h.db.CreditBundles().FindOne(ctx, bson.M{"_id": bundleID}).Decode(&bundle); err != nil {
			log.Printf("Webhook: bundle not found: %s", bundleIDStr)
			return
		}

		// Add credits to tenant
		h.db.Tenants().UpdateOne(ctx, bson.M{"_id": tenantID}, bson.M{
			"$inc": bson.M{"purchasedCredits": bundle.Credits},
			"$set": bson.M{"updatedAt": time.Now()},
		})

		amountCents := int64(0)
		if session.AmountTotal > 0 {
			amountCents = session.AmountTotal
		}
		h.recordTransaction(ctx, tenantID, userID, models.TransactionCreditPurchase, amountCents, bundle.Name, "", nil, &bundleID, "", session.ID)

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
}

func (h *WebhookHandler) handleInvoicePaid(ctx context.Context, event stripe.Event) {
	var invoice stripe.Invoice
	if err := json.Unmarshal(event.Data.Raw, &invoice); err != nil {
		log.Printf("Webhook: failed to unmarshal invoice: %v", err)
		return
	}

	// Skip the first invoice (handled by checkout.session.completed)
	billingReason := ""
	if invoice.BillingReason != "" {
		billingReason = string(invoice.BillingReason)
	}
	if billingReason == "subscription_create" {
		return
	}

	subscriptionID := ""
	if invoice.Parent != nil && invoice.Parent.SubscriptionDetails != nil && invoice.Parent.SubscriptionDetails.Subscription != nil {
		subscriptionID = invoice.Parent.SubscriptionDetails.Subscription.ID
	}
	if subscriptionID == "" {
		return
	}

	var tenant models.Tenant
	if err := h.db.Tenants().FindOne(ctx, bson.M{"stripeSubscriptionId": subscriptionID}).Decode(&tenant); err != nil {
		log.Printf("Webhook: tenant not found for subscription %s", subscriptionID)
		return
	}

	// Ensure active status
	h.db.Tenants().UpdateOne(ctx, bson.M{"_id": tenant.ID}, bson.M{
		"$set": bson.M{"billingStatus": models.BillingStatusActive, "updatedAt": time.Now()},
	})

	// Handle credit reset/accrue
	if tenant.PlanID != nil {
		var plan models.Plan
		if h.db.Plans().FindOne(ctx, bson.M{"_id": *tenant.PlanID}).Decode(&plan) == nil {
			if plan.CreditResetPolicy == models.CreditResetPolicyReset {
				h.db.Tenants().UpdateOne(ctx, bson.M{"_id": tenant.ID}, bson.M{
					"$set": bson.M{"subscriptionCredits": plan.UsageCreditsPerMonth},
				})
			} else {
				h.db.Tenants().UpdateOne(ctx, bson.M{"_id": tenant.ID}, bson.M{
					"$inc": bson.M{"subscriptionCredits": plan.UsageCreditsPerMonth},
				})
			}
		}
	}

	// Record transaction
	amountCents := invoice.AmountPaid
	// Find the owner of this tenant for the transaction record
	var membership models.TenantMembership
	h.db.TenantMemberships().FindOne(ctx, bson.M{"tenantId": tenant.ID, "role": models.RoleOwner}).Decode(&membership)

	planName := ""
	if tenant.PlanID != nil {
		var plan models.Plan
		if h.db.Plans().FindOne(ctx, bson.M{"_id": *tenant.PlanID}).Decode(&plan) == nil {
			planName = plan.Name
		}
	}

	h.recordTransaction(ctx, tenant.ID, membership.UserID, models.TransactionSubscription, amountCents, planName, tenant.BillingInterval, tenant.PlanID, nil, subscriptionID, "")

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
}

func (h *WebhookHandler) handleInvoicePaymentFailed(ctx context.Context, event stripe.Event) {
	var invoice stripe.Invoice
	if err := json.Unmarshal(event.Data.Raw, &invoice); err != nil {
		log.Printf("Webhook: failed to unmarshal invoice: %v", err)
		return
	}

	subscriptionID := ""
	if invoice.Parent != nil && invoice.Parent.SubscriptionDetails != nil && invoice.Parent.SubscriptionDetails.Subscription != nil {
		subscriptionID = invoice.Parent.SubscriptionDetails.Subscription.ID
	}
	if subscriptionID == "" {
		return
	}

	var tenant models.Tenant
	if err := h.db.Tenants().FindOne(ctx, bson.M{"stripeSubscriptionId": subscriptionID}).Decode(&tenant); err != nil {
		log.Printf("Webhook: tenant not found for subscription %s", subscriptionID)
		return
	}

	// Set past_due
	h.db.Tenants().UpdateOne(ctx, bson.M{"_id": tenant.ID}, bson.M{
		"$set": bson.M{"billingStatus": models.BillingStatusPastDue, "updatedAt": time.Now()},
	})

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
}

func (h *WebhookHandler) handleSubscriptionUpdated(ctx context.Context, event stripe.Event) {
	var sub stripe.Subscription
	if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
		log.Printf("Webhook: failed to unmarshal subscription: %v", err)
		return
	}

	var tenant models.Tenant
	if err := h.db.Tenants().FindOne(ctx, bson.M{"stripeSubscriptionId": sub.ID}).Decode(&tenant); err != nil {
		return
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

	h.db.Tenants().UpdateOne(ctx, bson.M{"_id": tenant.ID}, bson.M{"$set": updates})
}

func (h *WebhookHandler) handleSubscriptionDeleted(ctx context.Context, event stripe.Event) {
	var sub stripe.Subscription
	if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
		log.Printf("Webhook: failed to unmarshal subscription: %v", err)
		return
	}

	var tenant models.Tenant
	if err := h.db.Tenants().FindOne(ctx, bson.M{"stripeSubscriptionId": sub.ID}).Decode(&tenant); err != nil {
		return
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

	h.db.Tenants().UpdateOne(ctx, bson.M{"_id": tenant.ID}, bson.M{"$set": updates})

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
}

func (h *WebhookHandler) recordTransaction(ctx context.Context, tenantID, userID primitive.ObjectID, txType models.TransactionType, amountCents int64, itemName, interval string, planID, bundleID *primitive.ObjectID, stripeSubID, stripeSessionID string) {
	invoiceNum, err := h.stripe.NextInvoiceNumber(ctx)
	if err != nil {
		log.Printf("Failed to generate invoice number: %v", err)
		invoiceNum = fmt.Sprintf("INV-ERR-%d", time.Now().Unix())
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
		log.Printf("Failed to record transaction: %v", err)
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

// Ensure unused imports are used
var _ = mongoDB.ErrNoDocuments
