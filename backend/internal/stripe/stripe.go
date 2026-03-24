package stripe

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"lastsaas/internal/apicounter"
	"lastsaas/internal/db"
	"lastsaas/internal/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	stripe "github.com/stripe/stripe-go/v82"
	portalsession "github.com/stripe/stripe-go/v82/billingportal/session"
	checkoutsession "github.com/stripe/stripe-go/v82/checkout/session"
	"github.com/stripe/stripe-go/v82/customer"
	"github.com/stripe/stripe-go/v82/price"
	"github.com/stripe/stripe-go/v82/product"
	"github.com/stripe/stripe-go/v82/subscription"
	"github.com/stripe/stripe-go/v82/webhook"
)

type Service struct {
	secretKey      string
	PublishableKey string
	webhookSecret  string
	instanceID     string
	db             *db.MongoDB
	frontendURL    string
}

func New(secretKey, publishableKey, webhookSecret string, database *db.MongoDB, frontendURL string) *Service {
	stripe.Key = secretKey

	// Derive instance ID from the frontend URL hostname for multi-instance Stripe account sharing.
	instanceID := ""
	if u, err := url.Parse(frontendURL); err == nil && u.Hostname() != "" {
		instanceID = u.Hostname()
	}

	return &Service{
		secretKey:      secretKey,
		PublishableKey: publishableKey,
		webhookSecret:  webhookSecret,
		instanceID:     instanceID,
		db:             database,
		frontendURL:    frontendURL,
	}
}

// InstanceID returns the instance identifier (frontend hostname) for multi-instance Stripe account sharing.
func (s *Service) InstanceID() string { return s.instanceID }

// GetOrCreateCustomer finds or creates a Stripe customer for the given tenant.
func (s *Service) GetOrCreateCustomer(ctx context.Context, tenant *models.Tenant, userEmail string) (string, error) {
	if tenant.StripeCustomerID != "" {
		return tenant.StripeCustomerID, nil
	}

	custMeta := map[string]string{
		"tenantId": tenant.ID.Hex(),
	}
	if s.instanceID != "" {
		custMeta["instance"] = s.instanceID
	}
	params := &stripe.CustomerParams{
		Email:    stripe.String(userEmail),
		Name:     stripe.String(tenant.Name),
		Metadata: custMeta,
	}
	c, err := customer.New(params)
	apicounter.StripeAPICalls.Add(1)
	if err != nil {
		return "", fmt.Errorf("stripe customer create: %w", err)
	}

	_, err = s.db.Tenants().UpdateOne(ctx,
		bson.M{"_id": tenant.ID},
		bson.M{"$set": bson.M{"stripeCustomerId": c.ID, "updatedAt": time.Now()}},
	)
	if err != nil {
		return "", fmt.Errorf("save stripe customer id: %w", err)
	}

	return c.ID, nil
}

// GetOrCreatePrice finds an existing Stripe price mapping or creates a new Product + Price.
func (s *Service) GetOrCreatePrice(ctx context.Context, entityType string, entityID primitive.ObjectID, name string, amountCents int64, interval string, currency string) (string, error) {
	if currency == "" {
		currency = "usd"
	}
	// Check existing mapping
	var mapping models.StripeMapping
	err := s.db.StripeMappings().FindOne(ctx, bson.M{
		"entityType": entityType,
		"entityId":   entityID,
	}).Decode(&mapping)
	if err == nil {
		return mapping.StripePriceID, nil
	}
	if err != mongo.ErrNoDocuments {
		return "", fmt.Errorf("lookup stripe mapping: %w", err)
	}

	// Create product
	prod, err := product.New(&stripe.ProductParams{
		Name:    stripe.String(name),
		TaxCode: stripe.String("txcd_10103001"), // Software as a Service (SaaS) - business use
		Metadata: map[string]string{
			"entityType": entityType,
			"entityId":   entityID.Hex(),
		},
	})
	apicounter.StripeAPICalls.Add(1)
	if err != nil {
		return "", fmt.Errorf("stripe product create: %w", err)
	}

	// Create price
	priceParams := &stripe.PriceParams{
		Product:    stripe.String(prod.ID),
		Currency:   stripe.String(currency),
		UnitAmount: stripe.Int64(amountCents),
	}
	if interval != "" {
		priceParams.Recurring = &stripe.PriceRecurringParams{
			Interval: stripe.String(interval),
		}
	}
	p, err := price.New(priceParams)
	apicounter.StripeAPICalls.Add(1)
	if err != nil {
		return "", fmt.Errorf("stripe price create: %w", err)
	}

	// Save mapping
	if _, err := s.db.StripeMappings().InsertOne(ctx, models.StripeMapping{
		EntityType:      entityType,
		EntityID:        entityID,
		StripePriceID:   p.ID,
		StripeProductID: prod.ID,
		CreatedAt:       time.Now(),
	}); err != nil {
		// Duplicate key means another goroutine raced us — benign
		if !mongo.IsDuplicateKeyError(err) {
			return "", fmt.Errorf("stripe mapping insert: %w", err)
		}
	}

	return p.ID, nil
}

type CheckoutLineItem struct {
	PriceID  string
	Quantity int64
}

type CheckoutRequest struct {
	CustomerID      string
	PlanID          *primitive.ObjectID
	PlanName        string
	BundleID        *primitive.ObjectID
	BundleName      string
	AmountCents     int64
	BillingInterval string              // "month" or "year"
	TenantID        string
	UserID          string
	Quantity        int64               // For per-seat plans; defaults to 1
	SeatQuantity    int64               // Stored in metadata for seat tracking
	CustomLineItems []CheckoutLineItem  // Override default single line item
	TrialDays       int                 // Free trial period in days (0 = no trial)
	Currency        string              // Currency code (e.g. "usd", "eur"); defaults to "usd"
	AutomaticTax    bool                // Enable Stripe Tax for automatic tax calculation
	ExtraMetadata   map[string]string   // Additional metadata for the checkout session
}

// CreateCheckoutSession creates a Stripe Checkout Session for a subscription or one-time payment.
func (s *Service) CreateCheckoutSession(ctx context.Context, req CheckoutRequest) (string, error) {
	metadata := map[string]string{
		"tenantId": req.TenantID,
		"userId":   req.UserID,
	}
	if s.instanceID != "" {
		metadata["instance"] = s.instanceID
	}

	var mode string
	if req.PlanID != nil {
		mode = "subscription"
		metadata["planId"] = req.PlanID.Hex()
		metadata["billingInterval"] = req.BillingInterval
	} else if req.BundleID != nil {
		mode = "payment"
		metadata["bundleId"] = req.BundleID.Hex()
	} else if req.AmountCents > 0 {
		mode = "payment" // one-time payment without a bundle (e.g., scan purchase)
	} else {
		return "", fmt.Errorf("must specify planId, bundleId, or amountCents")
	}

	for k, v := range req.ExtraMetadata {
		metadata[k] = v
	}

	if req.SeatQuantity > 0 {
		metadata["seatQuantity"] = fmt.Sprintf("%d", req.SeatQuantity)
	}

	// Build line items
	var lineItems []*stripe.CheckoutSessionLineItemParams
	if len(req.CustomLineItems) > 0 {
		for _, item := range req.CustomLineItems {
			if item.Quantity > 0 {
				lineItems = append(lineItems, &stripe.CheckoutSessionLineItemParams{
					Price:    stripe.String(item.PriceID),
					Quantity: stripe.Int64(item.Quantity),
				})
			}
		}
	} else {
		var priceID string
		var err error
		if req.PlanID != nil {
			entityType := "plan_" + req.BillingInterval
			priceID, err = s.GetOrCreatePrice(ctx, entityType, *req.PlanID, req.PlanName, req.AmountCents, req.BillingInterval, req.Currency)
		} else {
			priceID, err = s.GetOrCreatePrice(ctx, "bundle", *req.BundleID, req.BundleName, req.AmountCents, "", req.Currency)
		}
		if err != nil {
			return "", err
		}
		qty := int64(1)
		if req.Quantity > 0 {
			qty = req.Quantity
		}
		lineItems = []*stripe.CheckoutSessionLineItemParams{
			{
				Price:    stripe.String(priceID),
				Quantity: stripe.Int64(qty),
			},
		}
	}

	params := &stripe.CheckoutSessionParams{
		Customer:            stripe.String(req.CustomerID),
		Mode:                stripe.String(mode),
		SuccessURL:          stripe.String(s.frontendURL + "/billing/success?session_id={CHECKOUT_SESSION_ID}"),
		CancelURL:           stripe.String(s.frontendURL + "/billing/cancel"),
		AllowPromotionCodes: stripe.Bool(true),
		Metadata:            metadata,
		LineItems:           lineItems,
	}

	// Stripe Tax: automatic tax calculation + tax ID collection
	if req.AutomaticTax {
		params.AutomaticTax = &stripe.CheckoutSessionAutomaticTaxParams{
			Enabled: stripe.Bool(true),
		}
		params.TaxIDCollection = &stripe.CheckoutSessionTaxIDCollectionParams{
			Enabled: stripe.Bool(true),
		}
		params.CustomerUpdate = &stripe.CheckoutSessionCustomerUpdateParams{
			Address: stripe.String("auto"),
		}
	}

	// Subscription-specific settings (trial, instance metadata)
	if mode == "subscription" {
		subData := &stripe.CheckoutSessionSubscriptionDataParams{}
		if s.instanceID != "" {
			subData.Metadata = map[string]string{
				"instance": s.instanceID,
			}
		}
		if req.TrialDays > 0 {
			subData.TrialPeriodDays = stripe.Int64(int64(req.TrialDays))
		}
		if subData.Metadata != nil || req.TrialDays > 0 {
			params.SubscriptionData = subData
		}
	}

	session, err := checkoutsession.New(params)
	apicounter.StripeAPICalls.Add(1)
	if err != nil {
		return "", fmt.Errorf("stripe checkout create: %w", err)
	}

	return session.URL, nil
}

// CreateBillingPortalSession creates a Stripe Billing Portal session for the given customer.
func (s *Service) CreateBillingPortalSession(ctx context.Context, customerID string) (string, error) {
	params := &stripe.BillingPortalSessionParams{
		Customer:  stripe.String(customerID),
		ReturnURL: stripe.String(s.frontendURL + "/settings"),
	}
	session, err := portalsession.New(params)
	apicounter.StripeAPICalls.Add(1)
	if err != nil {
		return "", fmt.Errorf("stripe portal create: %w", err)
	}
	return session.URL, nil
}

// CancelSubscriptionAtPeriodEnd marks a subscription to cancel at the end of the current period.
func (s *Service) CancelSubscriptionAtPeriodEnd(ctx context.Context, subscriptionID string) (*time.Time, error) {
	params := &stripe.SubscriptionParams{
		CancelAtPeriodEnd: stripe.Bool(true),
	}
	params.AddExpand("items")
	sub, err := subscription.Update(subscriptionID, params)
	apicounter.StripeAPICalls.Add(1)
	if err != nil {
		return nil, fmt.Errorf("stripe cancel subscription: %w", err)
	}
	// In v82, CurrentPeriodEnd is on SubscriptionItem, not Subscription
	if sub.Items != nil && len(sub.Items.Data) > 0 {
		periodEnd := time.Unix(sub.Items.Data[0].CurrentPeriodEnd, 0)
		return &periodEnd, nil
	}
	// Fallback: use CancelAt if set
	if sub.CancelAt > 0 {
		t := time.Unix(sub.CancelAt, 0)
		return &t, nil
	}
	return nil, nil
}

// CancelSubscriptionImmediately cancels a subscription immediately.
func (s *Service) CancelSubscriptionImmediately(ctx context.Context, subscriptionID string) error {
	_, err := subscription.Cancel(subscriptionID, nil)
	apicounter.StripeAPICalls.Add(1)
	if err != nil {
		return fmt.Errorf("stripe cancel subscription: %w", err)
	}
	return nil
}

// ConstructEvent verifies a webhook signature and returns the parsed event.
func (s *Service) ConstructEvent(payload []byte, sigHeader string) (stripe.Event, error) {
	return webhook.ConstructEventWithOptions(payload, sigHeader, s.webhookSecret, webhook.ConstructEventOptions{
		IgnoreAPIVersionMismatch: true,
	})
}

// NextInvoiceNumber atomically generates the next invoice number.
func (s *Service) NextInvoiceNumber(ctx context.Context) (string, error) {
	var result models.InvoiceCounter
	opts := options.FindOneAndUpdate().
		SetUpsert(true).
		SetReturnDocument(options.After)
	err := s.db.Counters().FindOneAndUpdate(ctx,
		bson.M{"_id": "invoice_number"},
		bson.M{"$inc": bson.M{"value": 1}},
		opts,
	).Decode(&result)
	if err != nil {
		return "", fmt.Errorf("generate invoice number: %w", err)
	}
	return fmt.Sprintf("INV-%06d", result.Value), nil
}

// GetCheckoutSession retrieves a checkout session by ID.
func (s *Service) GetCheckoutSession(ctx context.Context, sessionID string) (*stripe.CheckoutSession, error) {
	params := &stripe.CheckoutSessionParams{}
	params.AddExpand("subscription")
	s2, err := checkoutsession.Get(sessionID, params)
	apicounter.StripeAPICalls.Add(1)
	return s2, err
}

// UpdateSubscriptionQuantity updates the quantity on the first line item of a subscription.
// Used for per-seat billing adjustments.
func (s *Service) UpdateSubscriptionQuantity(ctx context.Context, subscriptionID string, quantity int64) error {
	params := &stripe.SubscriptionParams{}
	params.AddExpand("items")
	sub, err := subscription.Get(subscriptionID, params)
	apicounter.StripeAPICalls.Add(1)
	if err != nil {
		return fmt.Errorf("stripe get subscription: %w", err)
	}

	if sub.Items == nil || len(sub.Items.Data) == 0 {
		return fmt.Errorf("subscription has no items")
	}

	itemID := sub.Items.Data[0].ID
	updateParams := &stripe.SubscriptionParams{
		Items: []*stripe.SubscriptionItemsParams{
			{
				ID:       stripe.String(itemID),
				Quantity: stripe.Int64(quantity),
			},
		},
		ProrationBehavior: stripe.String("create_prorations"),
	}
	_, err = subscription.Update(subscriptionID, updateParams)
	apicounter.StripeAPICalls.Add(1)
	if err != nil {
		return fmt.Errorf("stripe update subscription quantity: %w", err)
	}
	return nil
}

// GetSubscription retrieves a subscription by ID.
func (s *Service) GetSubscription(ctx context.Context, subscriptionID string) (*stripe.Subscription, error) {
	params := &stripe.SubscriptionParams{}
	params.AddExpand("items")
	sub, err := subscription.Get(subscriptionID, params)
	apicounter.StripeAPICalls.Add(1)
	return sub, err
}
