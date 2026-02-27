package stripe

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"lastsaas/internal/models"
	"lastsaas/internal/testutil"

	gostripe "github.com/stripe/stripe-go/v82"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func setupMockStripe(t *testing.T, handler http.HandlerFunc) (*httptest.Server, func()) {
	t.Helper()
	mock := httptest.NewServer(handler)

	// Configure stripe to use mock backend
	backend := gostripe.GetBackendWithConfig(gostripe.APIBackend, &gostripe.BackendConfig{
		URL: gostripe.String(mock.URL),
	})
	gostripe.SetBackend(gostripe.APIBackend, backend)

	return mock, func() {
		mock.Close()
	}
}

func TestNewService(t *testing.T) {
	svc := New("sk_test_123", "pk_test_123", "whsec_123", nil, "https://myapp.example.com")
	if svc == nil {
		t.Fatal("expected non-nil service")
	}
	if svc.PublishableKey != "pk_test_123" {
		t.Errorf("expected pk_test_123, got %q", svc.PublishableKey)
	}
	if svc.secretKey != "sk_test_123" {
		t.Errorf("expected sk_test_123, got %q", svc.secretKey)
	}
	if svc.webhookSecret != "whsec_123" {
		t.Errorf("expected whsec_123, got %q", svc.webhookSecret)
	}
	if svc.frontendURL != "https://myapp.example.com" {
		t.Errorf("expected https://myapp.example.com, got %q", svc.frontendURL)
	}
}

func TestInstanceIDFromURL(t *testing.T) {
	tests := []struct {
		name        string
		frontendURL string
		expected    string
	}{
		{"standard URL", "https://myapp.example.com", "myapp.example.com"},
		{"with port", "http://localhost:4280", "localhost"},
		{"with path", "https://app.example.com/admin", "app.example.com"},
		{"empty URL", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := New("sk", "pk", "whsec", nil, tt.frontendURL)
			if svc.InstanceID() != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, svc.InstanceID())
			}
		})
	}
}

func TestConstructEventInvalidSignature(t *testing.T) {
	svc := New("sk_test", "pk_test", "whsec_test_secret", nil, "http://localhost")
	payload := []byte(`{"type":"checkout.session.completed"}`)

	_, err := svc.ConstructEvent(payload, "invalid-signature")
	if err == nil {
		t.Fatal("expected error for invalid webhook signature")
	}
}

func TestNextInvoiceNumber(t *testing.T) {
	database, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()
	testutil.CleanupCollections(t, database)

	svc := New("sk_test", "pk_test", "whsec_test", database, "http://localhost")

	inv1, err := svc.NextInvoiceNumber(t.Context())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	// Verify format: INV-XXXXXX (6-digit zero-padded)
	if len(inv1) != 10 || inv1[:4] != "INV-" {
		t.Errorf("unexpected format: got %q", inv1)
	}

	inv2, err := svc.NextInvoiceNumber(t.Context())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(inv2) != 10 || inv2[:4] != "INV-" {
		t.Errorf("unexpected format: got %q", inv2)
	}
	// Sequential calls must produce incrementing values
	if inv2 <= inv1 {
		t.Errorf("expected inv2 > inv1, got inv1=%q inv2=%q", inv1, inv2)
	}
}

func TestGetOrCreateCustomerExisting(t *testing.T) {
	database, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()
	testutil.CleanupCollections(t, database)

	svc := New("sk_test", "pk_test", "whsec_test", database, "http://localhost")

	tenant := &models.Tenant{
		ID:               primitive.NewObjectID(),
		StripeCustomerID: "cus_existing123",
	}

	custID, err := svc.GetOrCreateCustomer(t.Context(), tenant, "user@example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if custID != "cus_existing123" {
		t.Errorf("expected existing customer ID, got %q", custID)
	}
}

func TestGetOrCreateCustomerNew(t *testing.T) {
	database, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()
	testutil.CleanupCollections(t, database)

	mock, mockCleanup := setupMockStripe(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":     "cus_new123",
			"object": "customer",
			"email":  "user@example.com",
		})
	})
	defer mockCleanup()
	_ = mock

	owner := testutil.CreateTestUser(t, database, "owner@example.com", "password123", "Owner")
	tenant := testutil.CreateTestTenant(t, database, "New Tenant", owner.ID, false)

	svc := New("sk_test", "pk_test", "whsec_test", database, "http://localhost:4280")

	custID, err := svc.GetOrCreateCustomer(t.Context(), tenant, "user@example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if custID != "cus_new123" {
		t.Errorf("expected cus_new123, got %q", custID)
	}
}

func TestCreateCheckoutSessionMissingIDs(t *testing.T) {
	svc := New("sk_test", "pk_test", "whsec_test", nil, "http://localhost")

	_, err := svc.CreateCheckoutSession(t.Context(), CheckoutRequest{
		CustomerID: "cus_123",
		TenantID:   "tenant_123",
		UserID:     "user_123",
		// Neither PlanID nor BundleID set
	})
	if err == nil {
		t.Fatal("expected error for missing planId/bundleId")
	}
}

func TestCreateCheckoutSessionSubscription(t *testing.T) {
	database, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()
	testutil.CleanupCollections(t, database)

	mock, mockCleanup := setupMockStripe(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/v1/products":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id":     "prod_test123",
				"object": "product",
			})
		case r.URL.Path == "/v1/prices":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id":     "price_test123",
				"object": "price",
			})
		case r.URL.Path == "/v1/checkout/sessions":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id":     "cs_test123",
				"object": "checkout.session",
				"url":    "https://checkout.stripe.com/pay/cs_test123",
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
	defer mockCleanup()
	_ = mock

	planID := primitive.NewObjectID()
	svc := New("sk_test", "pk_test", "whsec_test", database, "http://localhost:4280")

	url, err := svc.CreateCheckoutSession(t.Context(), CheckoutRequest{
		CustomerID:      "cus_123",
		PlanID:          &planID,
		PlanName:        "Pro Plan",
		AmountCents:     1999,
		BillingInterval: "month",
		TenantID:        "tenant_123",
		UserID:          "user_123",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if url == "" {
		t.Error("expected non-empty checkout URL")
	}
}

func TestCreateCheckoutSessionOneTimePayment(t *testing.T) {
	database, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()
	testutil.CleanupCollections(t, database)

	mock, mockCleanup := setupMockStripe(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/v1/products":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id":     "prod_bundle",
				"object": "product",
			})
		case r.URL.Path == "/v1/prices":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id":     "price_bundle",
				"object": "price",
			})
		case r.URL.Path == "/v1/checkout/sessions":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id":     "cs_bundle",
				"object": "checkout.session",
				"url":    "https://checkout.stripe.com/pay/cs_bundle",
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
	defer mockCleanup()
	_ = mock

	bundleID := primitive.NewObjectID()
	svc := New("sk_test", "pk_test", "whsec_test", database, "http://localhost:4280")

	url, err := svc.CreateCheckoutSession(t.Context(), CheckoutRequest{
		CustomerID:  "cus_123",
		BundleID:    &bundleID,
		BundleName:  "Credit Bundle",
		AmountCents: 999,
		TenantID:    "tenant_123",
		UserID:      "user_123",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if url == "" {
		t.Error("expected non-empty checkout URL")
	}
}

func TestCreateCheckoutSessionWithCustomLineItems(t *testing.T) {
	database, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()
	testutil.CleanupCollections(t, database)

	mock, mockCleanup := setupMockStripe(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/v1/checkout/sessions" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id":     "cs_custom",
				"object": "checkout.session",
				"url":    "https://checkout.stripe.com/pay/cs_custom",
			})
		}
	})
	defer mockCleanup()
	_ = mock

	planID := primitive.NewObjectID()
	svc := New("sk_test", "pk_test", "whsec_test", database, "http://localhost:4280")

	url, err := svc.CreateCheckoutSession(t.Context(), CheckoutRequest{
		CustomerID:      "cus_123",
		PlanID:          &planID,
		PlanName:        "Enterprise",
		BillingInterval: "year",
		TenantID:        "tenant_123",
		UserID:          "user_123",
		SeatQuantity:    10,
		TrialDays:       14,
		AutomaticTax:    true,
		CustomLineItems: []CheckoutLineItem{
			{PriceID: "price_base", Quantity: 1},
			{PriceID: "price_seat", Quantity: 10},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if url == "" {
		t.Error("expected non-empty checkout URL")
	}
}

func TestCreateBillingPortalSession(t *testing.T) {
	mock, mockCleanup := setupMockStripe(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":     "bps_test",
			"object": "billing_portal.session",
			"url":    "https://billing.stripe.com/session/bps_test",
		})
	})
	defer mockCleanup()
	_ = mock

	svc := New("sk_test", "pk_test", "whsec_test", nil, "http://localhost:4280")

	url, err := svc.CreateBillingPortalSession(t.Context(), "cus_123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if url == "" {
		t.Error("expected non-empty portal URL")
	}
}

func TestCancelSubscriptionAtPeriodEnd(t *testing.T) {
	mock, mockCleanup := setupMockStripe(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":                "sub_123",
			"object":            "subscription",
			"cancel_at_period_end": true,
			"cancel_at":         1735689600,
			"items": map[string]interface{}{
				"object": "list",
				"data": []map[string]interface{}{
					{
						"id":                 "si_123",
						"current_period_end": 1735689600,
					},
				},
			},
		})
	})
	defer mockCleanup()
	_ = mock

	svc := New("sk_test", "pk_test", "whsec_test", nil, "http://localhost")

	periodEnd, err := svc.CancelSubscriptionAtPeriodEnd(t.Context(), "sub_123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if periodEnd == nil {
		t.Fatal("expected non-nil period end")
	}
}

func TestCancelSubscriptionImmediately(t *testing.T) {
	mock, mockCleanup := setupMockStripe(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":     "sub_123",
			"object": "subscription",
			"status": "canceled",
		})
	})
	defer mockCleanup()
	_ = mock

	svc := New("sk_test", "pk_test", "whsec_test", nil, "http://localhost")

	err := svc.CancelSubscriptionImmediately(t.Context(), "sub_123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetCheckoutSession(t *testing.T) {
	mock, mockCleanup := setupMockStripe(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":     "cs_test123",
			"object": "checkout.session",
			"mode":   "subscription",
		})
	})
	defer mockCleanup()
	_ = mock

	svc := New("sk_test", "pk_test", "whsec_test", nil, "http://localhost")

	session, err := svc.GetCheckoutSession(t.Context(), "cs_test123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if session.ID != "cs_test123" {
		t.Errorf("expected cs_test123, got %q", session.ID)
	}
}

func TestGetOrCreatePriceExistingMapping(t *testing.T) {
	database, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()
	testutil.CleanupCollections(t, database)

	svc := New("sk_test", "pk_test", "whsec_test", database, "http://localhost:4280")

	// Insert a pre-existing mapping
	entityID := primitive.NewObjectID()
	database.StripeMappings().InsertOne(t.Context(), models.StripeMapping{
		EntityType:    "plan_month",
		EntityID:      entityID,
		StripePriceID: "price_existing",
	})

	priceID, err := svc.GetOrCreatePrice(t.Context(), "plan_month", entityID, "Pro Plan", 1999, "month", "usd")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if priceID != "price_existing" {
		t.Errorf("expected price_existing, got %q", priceID)
	}
}

func TestGetOrCreatePriceDefaultCurrency(t *testing.T) {
	database, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()
	testutil.CleanupCollections(t, database)

	mock, mockCleanup := setupMockStripe(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/v1/products":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id":     "prod_new",
				"object": "product",
			})
		case r.URL.Path == "/v1/prices":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id":     "price_new",
				"object": "price",
			})
		}
	})
	defer mockCleanup()
	_ = mock

	svc := New("sk_test", "pk_test", "whsec_test", database, "http://localhost:4280")

	// Empty currency should default to "usd"
	entityID := primitive.NewObjectID()
	priceID, err := svc.GetOrCreatePrice(t.Context(), "bundle", entityID, "Bundle", 500, "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if priceID != "price_new" {
		t.Errorf("expected price_new, got %q", priceID)
	}
}

func TestUpdateSubscriptionQuantity(t *testing.T) {
	requestCount := 0
	mock, mockCleanup := setupMockStripe(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		requestCount++
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":     "sub_seat",
			"object": "subscription",
			"status": "active",
			"items": map[string]interface{}{
				"object": "list",
				"data": []map[string]interface{}{
					{
						"id":     "si_item1",
						"object": "subscription_item",
					},
				},
			},
		})
	})
	defer mockCleanup()
	_ = mock

	svc := New("sk_test", "pk_test", "whsec_test", nil, "http://localhost")

	err := svc.UpdateSubscriptionQuantity(t.Context(), "sub_seat", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should have made 2 calls: Get + Update
	if requestCount < 2 {
		t.Errorf("expected at least 2 API calls, got %d", requestCount)
	}
}

func TestUpdateSubscriptionQuantityNoItems(t *testing.T) {
	mock, mockCleanup := setupMockStripe(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":     "sub_empty",
			"object": "subscription",
			"status": "active",
			"items": map[string]interface{}{
				"object": "list",
				"data":   []map[string]interface{}{},
			},
		})
	})
	defer mockCleanup()
	_ = mock

	svc := New("sk_test", "pk_test", "whsec_test", nil, "http://localhost")

	err := svc.UpdateSubscriptionQuantity(t.Context(), "sub_empty", 5)
	if err == nil {
		t.Fatal("expected error for subscription with no items")
	}
}

func TestCancelSubscriptionAtPeriodEndNoItems(t *testing.T) {
	mock, mockCleanup := setupMockStripe(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":        "sub_nope",
			"object":    "subscription",
			"cancel_at": 0,
			"items": map[string]interface{}{
				"object": "list",
				"data":   []map[string]interface{}{},
			},
		})
	})
	defer mockCleanup()
	_ = mock

	svc := New("sk_test", "pk_test", "whsec_test", nil, "http://localhost")

	periodEnd, err := svc.CancelSubscriptionAtPeriodEnd(t.Context(), "sub_nope")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if periodEnd != nil {
		t.Error("expected nil period end when no items and no cancel_at")
	}
}

func TestCreateCheckoutSessionWithQuantity(t *testing.T) {
	database, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()
	testutil.CleanupCollections(t, database)

	mock, mockCleanup := setupMockStripe(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/v1/products":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id":     "prod_seat",
				"object": "product",
			})
		case r.URL.Path == "/v1/prices":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id":     "price_seat",
				"object": "price",
			})
		case r.URL.Path == "/v1/checkout/sessions":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id":     "cs_seat",
				"object": "checkout.session",
				"url":    "https://checkout.stripe.com/pay/cs_seat",
			})
		}
	})
	defer mockCleanup()
	_ = mock

	planID := primitive.NewObjectID()
	svc := New("sk_test", "pk_test", "whsec_test", database, "http://localhost:4280")

	url, err := svc.CreateCheckoutSession(t.Context(), CheckoutRequest{
		CustomerID:      "cus_123",
		PlanID:          &planID,
		PlanName:        "Team Plan",
		AmountCents:     999,
		BillingInterval: "month",
		TenantID:        "tenant_123",
		UserID:          "user_123",
		Quantity:        5,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if url == "" {
		t.Error("expected non-empty checkout URL")
	}
}

func TestGetSubscription(t *testing.T) {
	mock, mockCleanup := setupMockStripe(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":     "sub_test123",
			"object": "subscription",
			"status": "active",
			"items": map[string]interface{}{
				"object": "list",
				"data":   []map[string]interface{}{},
			},
		})
	})
	defer mockCleanup()
	_ = mock

	svc := New("sk_test", "pk_test", "whsec_test", nil, "http://localhost")

	sub, err := svc.GetSubscription(t.Context(), "sub_test123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sub.ID != "sub_test123" {
		t.Errorf("expected sub_test123, got %q", sub.ID)
	}
}
