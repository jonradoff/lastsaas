package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"strconv"
	"time"

	"lastsaas/internal/db"
	"lastsaas/internal/events"
	"lastsaas/internal/middleware"
	"lastsaas/internal/models"
	stripeservice "lastsaas/internal/stripe"
	"lastsaas/internal/syslog"

	"github.com/gorilla/mux"
	"github.com/jung-kurt/gofpdf"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type BillingHandler struct {
	stripe *stripeservice.Service
	db     *db.MongoDB
	events events.Emitter
	syslog *syslog.Logger
}

func NewBillingHandler(stripeSvc *stripeservice.Service, database *db.MongoDB, emitter events.Emitter, sysLogger *syslog.Logger) *BillingHandler {
	return &BillingHandler{
		stripe: stripeSvc,
		db:     database,
		events: emitter,
		syslog: sysLogger,
	}
}

// Checkout creates a Stripe Checkout Session or directly assigns a plan if billing is waived.
func (h *BillingHandler) Checkout(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenant, ok := middleware.GetTenantFromContext(ctx)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "No tenant context")
		return
	}
	user, ok := middleware.GetUserFromContext(ctx)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	var req struct {
		PlanID               string `json:"planId"`
		BundleID             string `json:"bundleId"`
		BillingInterval      string `json:"billingInterval"`
		RemoveBillingWaiver  bool   `json:"removeBillingWaiver"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.PlanID != "" {
		planID, err := primitive.ObjectIDFromHex(req.PlanID)
		if err != nil {
			respondWithError(w, http.StatusBadRequest, "Invalid plan ID")
			return
		}

		var plan models.Plan
		if err := h.db.Plans().FindOne(ctx, bson.M{"_id": planID}).Decode(&plan); err != nil {
			respondWithError(w, http.StatusNotFound, "Plan not found")
			return
		}

		if req.BillingInterval == "" {
			req.BillingInterval = "year"
		}
		if req.BillingInterval != "month" && req.BillingInterval != "year" {
			respondWithError(w, http.StatusBadRequest, "billingInterval must be 'month' or 'year'")
			return
		}

		// If user is explicitly removing billing waiver, clear it now and proceed to Stripe
		if req.RemoveBillingWaiver && tenant.BillingWaived {
			h.db.Tenants().UpdateOne(ctx, bson.M{"_id": tenant.ID}, bson.M{
				"$set": bson.M{"billingWaived": false, "updatedAt": time.Now()},
			})
			tenant.BillingWaived = false
		}

		// Free plan or billing waived: assign directly
		if (plan.MonthlyPriceCents == 0 && plan.PerSeatPriceCents == 0) || tenant.BillingWaived {
			setFields := bson.M{
				"planId":              planID,
				"billingStatus":       models.BillingStatusActive,
				"billingInterval":     req.BillingInterval,
				"subscriptionCredits": plan.UsageCreditsPerMonth,
				"updatedAt":           time.Now(),
			}
			if plan.PricingModel == models.PricingModelPerSeat {
				memberCount, _ := h.db.TenantMemberships().CountDocuments(ctx, bson.M{"tenantId": tenant.ID})
				seats := int(memberCount)
				if seats < plan.MinSeats {
					seats = plan.MinSeats
				}
				if seats < 1 {
					seats = 1
				}
				setFields["seatQuantity"] = seats
			}
			h.db.Tenants().UpdateOne(ctx, bson.M{"_id": tenant.ID}, bson.M{
				"$set": setFields,
				"$inc": bson.M{"purchasedCredits": plan.BonusCredits},
			})
			respondWithJSON(w, http.StatusOK, map[string]interface{}{"waived": true})
			return
		}

		// Per-seat billing
		if plan.PricingModel == models.PricingModelPerSeat {
			memberCount, _ := h.db.TenantMemberships().CountDocuments(ctx, bson.M{"tenantId": tenant.ID})
			seats := int(memberCount)
			if seats < plan.MinSeats {
				seats = plan.MinSeats
			}
			if seats < 1 {
				seats = 1
			}
			if plan.MaxSeats > 0 && seats > plan.MaxSeats {
				respondWithError(w, http.StatusForbidden, fmt.Sprintf("Maximum seats (%d) exceeded", plan.MaxSeats))
				return
			}

			if h.stripe == nil {
				respondWithError(w, http.StatusServiceUnavailable, "Billing not configured")
				return
			}

			customerID, err := h.stripe.GetOrCreateCustomer(ctx, tenant, user.Email)
			if err != nil {
				log.Printf("Billing: failed to get/create customer: %v", err)
				respondWithError(w, http.StatusInternalServerError, "Failed to create billing session")
				return
			}

			checkoutReq := stripeservice.CheckoutRequest{
				CustomerID:      customerID,
				PlanID:          &planID,
				PlanName:        plan.Name,
				BillingInterval: req.BillingInterval,
				TenantID:        tenant.ID.Hex(),
				UserID:          user.ID.Hex(),
				SeatQuantity:    int64(seats),
			}

			if plan.MonthlyPriceCents > 0 && plan.IncludedSeats > 0 {
				// Base + per-seat: two line items
				baseAmount := plan.MonthlyPriceCents
				perSeatAmount := plan.PerSeatPriceCents
				if req.BillingInterval == "year" {
					baseAnnual := plan.MonthlyPriceCents * 12
					baseDiscount := int64(math.Round(float64(baseAnnual) * float64(plan.AnnualDiscountPct) / 100))
					baseAmount = baseAnnual - baseDiscount
					seatAnnual := plan.PerSeatPriceCents * 12
					seatDiscount := int64(math.Round(float64(seatAnnual) * float64(plan.AnnualDiscountPct) / 100))
					perSeatAmount = seatAnnual - seatDiscount
				}

				basePriceID, err := h.stripe.GetOrCreatePrice(ctx, "plan_base_"+req.BillingInterval, planID, plan.Name+" (Base)", baseAmount, req.BillingInterval)
				if err != nil {
					log.Printf("Billing: failed to create base price: %v", err)
					respondWithError(w, http.StatusInternalServerError, "Failed to create billing session")
					return
				}

				customItems := []stripeservice.CheckoutLineItem{
					{PriceID: basePriceID, Quantity: 1},
				}

				additionalSeats := int64(seats) - int64(plan.IncludedSeats)
				if additionalSeats > 0 {
					seatPriceID, err := h.stripe.GetOrCreatePrice(ctx, "plan_seat_"+req.BillingInterval, planID, plan.Name+" (Per Seat)", perSeatAmount, req.BillingInterval)
					if err != nil {
						log.Printf("Billing: failed to create seat price: %v", err)
						respondWithError(w, http.StatusInternalServerError, "Failed to create billing session")
						return
					}
					customItems = append(customItems, stripeservice.CheckoutLineItem{PriceID: seatPriceID, Quantity: additionalSeats})
				}

				checkoutReq.CustomLineItems = customItems
			} else {
				// Pure per-seat: single line item
				perSeatAmount := plan.PerSeatPriceCents
				if req.BillingInterval == "year" {
					annual := plan.PerSeatPriceCents * 12
					discount := int64(math.Round(float64(annual) * float64(plan.AnnualDiscountPct) / 100))
					perSeatAmount = annual - discount
				}
				checkoutReq.AmountCents = perSeatAmount
				checkoutReq.Quantity = int64(seats)
			}

			url, err := h.stripe.CreateCheckoutSession(ctx, checkoutReq)
			if err != nil {
				log.Printf("Billing: failed to create checkout session: %v", err)
				respondWithError(w, http.StatusInternalServerError, "Failed to create billing session")
				return
			}

			respondWithJSON(w, http.StatusOK, map[string]string{"checkoutUrl": url})
			return
		}

		// Calculate price (flat-rate plans)
		amountCents := plan.MonthlyPriceCents
		if req.BillingInterval == "year" {
			annual := plan.MonthlyPriceCents * 12
			discount := int64(math.Round(float64(annual) * float64(plan.AnnualDiscountPct) / 100))
			amountCents = annual - discount
		}

		if h.stripe == nil {
			respondWithError(w, http.StatusServiceUnavailable, "Billing not configured")
			return
		}

		customerID, err := h.stripe.GetOrCreateCustomer(ctx, tenant, user.Email)
		if err != nil {
			log.Printf("Billing: failed to get/create customer: %v", err)
			respondWithError(w, http.StatusInternalServerError, "Failed to create billing session")
			return
		}

		url, err := h.stripe.CreateCheckoutSession(ctx, stripeservice.CheckoutRequest{
			CustomerID:      customerID,
			PlanID:          &planID,
			PlanName:        plan.Name,
			AmountCents:     amountCents,
			BillingInterval: req.BillingInterval,
			TenantID:        tenant.ID.Hex(),
			UserID:          user.ID.Hex(),
		})
		if err != nil {
			log.Printf("Billing: failed to create checkout session: %v", err)
			respondWithError(w, http.StatusInternalServerError, "Failed to create billing session")
			return
		}

		respondWithJSON(w, http.StatusOK, map[string]string{"checkoutUrl": url})
		return
	}

	if req.BundleID != "" {
		bundleID, err := primitive.ObjectIDFromHex(req.BundleID)
		if err != nil {
			respondWithError(w, http.StatusBadRequest, "Invalid bundle ID")
			return
		}

		var bundle models.CreditBundle
		if err := h.db.CreditBundles().FindOne(ctx, bson.M{"_id": bundleID, "isActive": true}).Decode(&bundle); err != nil {
			respondWithError(w, http.StatusNotFound, "Bundle not found")
			return
		}

		if h.stripe == nil {
			respondWithError(w, http.StatusServiceUnavailable, "Billing not configured")
			return
		}

		customerID, err := h.stripe.GetOrCreateCustomer(ctx, tenant, user.Email)
		if err != nil {
			log.Printf("Billing: failed to get/create customer: %v", err)
			respondWithError(w, http.StatusInternalServerError, "Failed to create billing session")
			return
		}

		url, err := h.stripe.CreateCheckoutSession(ctx, stripeservice.CheckoutRequest{
			CustomerID:  customerID,
			BundleID:    &bundleID,
			BundleName:  bundle.Name,
			AmountCents: bundle.PriceCents,
			TenantID:    tenant.ID.Hex(),
			UserID:      user.ID.Hex(),
		})
		if err != nil {
			log.Printf("Billing: failed to create checkout session: %v", err)
			respondWithError(w, http.StatusInternalServerError, "Failed to create billing session")
			return
		}

		respondWithJSON(w, http.StatusOK, map[string]string{"checkoutUrl": url})
		return
	}

	respondWithError(w, http.StatusBadRequest, "Must specify planId or bundleId")
}

// Portal creates a Stripe Billing Portal session.
func (h *BillingHandler) Portal(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenant, ok := middleware.GetTenantFromContext(ctx)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "No tenant context")
		return
	}

	if tenant.StripeCustomerID == "" {
		respondWithError(w, http.StatusBadRequest, "No billing account")
		return
	}

	if h.stripe == nil {
		respondWithError(w, http.StatusServiceUnavailable, "Billing not configured")
		return
	}

	url, err := h.stripe.CreateBillingPortalSession(ctx, tenant.StripeCustomerID)
	if err != nil {
		log.Printf("Billing: failed to create portal session: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to create portal session")
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]string{"portalUrl": url})
}

// ListTransactions returns paginated transactions for the current tenant.
func (h *BillingHandler) ListTransactions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenant, ok := middleware.GetTenantFromContext(ctx)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "No tenant context")
		return
	}

	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	if page < 1 {
		page = 1
	}
	perPage, _ := strconv.Atoi(q.Get("perPage"))
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}

	filter := bson.M{"tenantId": tenant.ID}

	total, _ := h.db.FinancialTransactions().CountDocuments(ctx, filter)

	opts := options.Find().
		SetSort(bson.D{{Key: "createdAt", Value: -1}}).
		SetSkip(int64((page - 1) * perPage)).
		SetLimit(int64(perPage))

	cursor, err := h.db.FinancialTransactions().Find(ctx, filter, opts)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to fetch transactions")
		return
	}
	defer cursor.Close(ctx)

	var transactions []models.FinancialTransaction
	if err := cursor.All(ctx, &transactions); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to decode transactions")
		return
	}
	if transactions == nil {
		transactions = []models.FinancialTransaction{}
	}

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"transactions": transactions,
		"total":        total,
		"page":         page,
		"perPage":      perPage,
	})
}

// GetInvoice returns invoice data for a single transaction.
func (h *BillingHandler) GetInvoice(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenant, ok := middleware.GetTenantFromContext(ctx)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "No tenant context")
		return
	}

	txID, err := primitive.ObjectIDFromHex(mux.Vars(r)["id"])
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid transaction ID")
		return
	}

	var tx models.FinancialTransaction
	if err := h.db.FinancialTransactions().FindOne(ctx, bson.M{"_id": txID, "tenantId": tenant.ID}).Decode(&tx); err != nil {
		respondWithError(w, http.StatusNotFound, "Transaction not found")
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"transaction": tx,
		"tenant": map[string]interface{}{
			"name": tenant.Name,
		},
	})
}

// GetInvoicePDF generates and streams a PDF invoice.
func (h *BillingHandler) GetInvoicePDF(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenant, ok := middleware.GetTenantFromContext(ctx)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "No tenant context")
		return
	}

	txID, err := primitive.ObjectIDFromHex(mux.Vars(r)["id"])
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid transaction ID")
		return
	}

	var tx models.FinancialTransaction
	if err := h.db.FinancialTransactions().FindOne(ctx, bson.M{"_id": txID, "tenantId": tenant.ID}).Decode(&tx); err != nil {
		respondWithError(w, http.StatusNotFound, "Transaction not found")
		return
	}

	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()

	// Header
	pdf.SetFont("Helvetica", "B", 20)
	pdf.Cell(0, 12, "INVOICE")
	pdf.Ln(16)

	// Invoice details
	pdf.SetFont("Helvetica", "", 10)
	pdf.Cell(95, 6, fmt.Sprintf("Invoice #: %s", tx.InvoiceNumber))
	pdf.Cell(95, 6, fmt.Sprintf("Date: %s", tx.CreatedAt.Format("January 2, 2006")))
	pdf.Ln(8)

	// Bill to
	pdf.SetFont("Helvetica", "B", 10)
	pdf.Cell(0, 6, "Bill To:")
	pdf.Ln(6)
	pdf.SetFont("Helvetica", "", 10)
	pdf.Cell(0, 6, tenant.Name)
	pdf.Ln(12)

	// Table header
	pdf.SetFillColor(240, 240, 240)
	pdf.SetFont("Helvetica", "B", 10)
	pdf.CellFormat(100, 8, "Description", "1", 0, "", true, 0, "")
	pdf.CellFormat(45, 8, "Type", "1", 0, "", true, 0, "")
	pdf.CellFormat(45, 8, "Amount", "1", 0, "R", true, 0, "")
	pdf.Ln(-1)

	// Table row
	pdf.SetFont("Helvetica", "", 10)
	pdf.CellFormat(100, 8, tx.Description, "1", 0, "", false, 0, "")
	txType := string(tx.Type)
	if tx.Type == models.TransactionSubscription {
		txType = "Subscription"
	} else if tx.Type == models.TransactionCreditPurchase {
		txType = "Credit Purchase"
	}
	pdf.CellFormat(45, 8, txType, "1", 0, "", false, 0, "")
	pdf.CellFormat(45, 8, fmt.Sprintf("$%.2f", float64(tx.AmountCents)/100), "1", 0, "R", false, 0, "")
	pdf.Ln(12)

	// Total
	pdf.SetFont("Helvetica", "B", 12)
	pdf.Cell(145, 8, "Total:")
	pdf.Cell(45, 8, fmt.Sprintf("$%.2f", float64(tx.AmountCents)/100))
	pdf.Ln(8)

	pdf.SetFont("Helvetica", "", 9)
	pdf.SetTextColor(128, 128, 128)
	pdf.Cell(0, 6, fmt.Sprintf("Currency: %s", tx.Currency))

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to generate PDF")
		return
	}

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="invoice-%s.pdf"`, tx.InvoiceNumber))
	w.Header().Set("Content-Length", strconv.Itoa(buf.Len()))
	w.Write(buf.Bytes())
}

// CancelSubscription cancels the current tenant's subscription at period end.
func (h *BillingHandler) CancelSubscription(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenant, ok := middleware.GetTenantFromContext(ctx)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "No tenant context")
		return
	}

	if tenant.StripeSubscriptionID == "" {
		respondWithError(w, http.StatusBadRequest, "No active subscription")
		return
	}

	if h.stripe == nil {
		respondWithError(w, http.StatusServiceUnavailable, "Billing not configured")
		return
	}

	periodEnd, err := h.stripe.CancelSubscriptionAtPeriodEnd(ctx, tenant.StripeSubscriptionID)
	if err != nil {
		log.Printf("Billing: failed to cancel subscription: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to cancel subscription")
		return
	}

	now := time.Now()
	updates := bson.M{
		"billingStatus": models.BillingStatusCanceled,
		"canceledAt":    now,
		"updatedAt":     now,
	}
	if periodEnd != nil {
		updates["currentPeriodEnd"] = periodEnd
	}
	h.db.Tenants().UpdateOne(ctx, bson.M{"_id": tenant.ID}, bson.M{"$set": updates})

	h.syslog.High(ctx, fmt.Sprintf("Subscription canceled by user: tenant %s", tenant.ID.Hex()))

	h.events.Emit(events.Event{
		Type:      events.EventSubscriptionCanceled,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"tenantId":   tenant.ID.Hex(),
			"tenantName": tenant.Name,
			"reason":     "user_initiated",
		},
	})

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"message":          "Subscription will cancel at end of billing period",
		"currentPeriodEnd": periodEnd,
	})
}

// GetConfig returns the Stripe publishable key for frontend use.
func (h *BillingHandler) GetConfig(w http.ResponseWriter, r *http.Request) {
	pubKey := ""
	if h.stripe != nil {
		pubKey = h.stripe.PublishableKey
	}
	respondWithJSON(w, http.StatusOK, map[string]string{"publishableKey": pubKey})
}

// --- Admin endpoints ---

// AdminListTransactions returns all transactions with optional filters.
func (h *BillingHandler) AdminListTransactions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := r.URL.Query()

	page, _ := strconv.Atoi(q.Get("page"))
	if page < 1 {
		page = 1
	}
	perPage, _ := strconv.Atoi(q.Get("perPage"))
	if perPage < 1 || perPage > 100 {
		perPage = 50
	}

	filter := bson.M{}
	if tenantID := q.Get("tenantId"); tenantID != "" {
		if oid, err := primitive.ObjectIDFromHex(tenantID); err == nil {
			filter["tenantId"] = oid
		}
	}
	if search := q.Get("search"); search != "" {
		filter["$or"] = []bson.M{
			{"description": bson.M{"$regex": search, "$options": "i"}},
			{"invoiceNumber": bson.M{"$regex": search, "$options": "i"}},
			{"planName": bson.M{"$regex": search, "$options": "i"}},
			{"bundleName": bson.M{"$regex": search, "$options": "i"}},
		}
	}

	total, _ := h.db.FinancialTransactions().CountDocuments(ctx, filter)

	opts := options.Find().
		SetSort(bson.D{{Key: "createdAt", Value: -1}}).
		SetSkip(int64((page - 1) * perPage)).
		SetLimit(int64(perPage))

	cursor, err := h.db.FinancialTransactions().Find(ctx, filter, opts)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to fetch transactions")
		return
	}
	defer cursor.Close(ctx)

	var transactions []models.FinancialTransaction
	if err := cursor.All(ctx, &transactions); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to decode transactions")
		return
	}
	if transactions == nil {
		transactions = []models.FinancialTransaction{}
	}

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"transactions": transactions,
		"total":        total,
		"page":         page,
		"perPage":      perPage,
	})
}

// AdminGetMetrics returns time-series business metrics for dashboard charts.
func (h *BillingHandler) AdminGetMetrics(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := r.URL.Query()

	rangeParam := q.Get("range")
	if rangeParam == "" {
		rangeParam = "30d"
	}
	metric := q.Get("metric")
	if metric == "" {
		metric = "revenue"
	}

	var days int
	switch rangeParam {
	case "7d":
		days = 7
	case "30d":
		days = 30
	case "1y":
		days = 365
	default:
		days = 30
	}

	startDate := time.Now().UTC().AddDate(0, 0, -days).Format("2006-01-02")

	filter := bson.M{"date": bson.M{"$gte": startDate}}
	opts := options.Find().SetSort(bson.D{{Key: "date", Value: 1}})

	cursor, err := h.db.DailyMetrics().Find(ctx, filter, opts)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to fetch metrics")
		return
	}
	defer cursor.Close(ctx)

	var metrics []models.DailyMetric
	if err := cursor.All(ctx, &metrics); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to decode metrics")
		return
	}

	type point struct {
		Date  string `json:"date"`
		Value int64  `json:"value"`
	}
	var points []point
	for _, m := range metrics {
		var val int64
		switch metric {
		case "revenue":
			val = m.Revenue
		case "arr":
			val = m.ARR
		case "dau":
			val = m.DAU
		case "mau":
			val = m.MAU
		default:
			val = m.Revenue
		}
		points = append(points, point{Date: m.Date, Value: val})
	}
	if points == nil {
		points = []point{}
	}

	respondWithJSON(w, http.StatusOK, map[string]interface{}{"data": points})
}

// AdminCancelSubscription allows an admin to cancel a tenant's subscription.
func (h *BillingHandler) AdminCancelSubscription(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, err := primitive.ObjectIDFromHex(mux.Vars(r)["tenantId"])
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid tenant ID")
		return
	}

	var tenant models.Tenant
	if err := h.db.Tenants().FindOne(ctx, bson.M{"_id": tenantID}).Decode(&tenant); err != nil {
		respondWithError(w, http.StatusNotFound, "Tenant not found")
		return
	}

	if tenant.StripeSubscriptionID == "" {
		respondWithError(w, http.StatusBadRequest, "Tenant has no active subscription")
		return
	}

	var req struct {
		Immediate bool `json:"immediate"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	if h.stripe == nil {
		respondWithError(w, http.StatusServiceUnavailable, "Billing not configured")
		return
	}

	now := time.Now()
	if req.Immediate {
		if err := h.stripe.CancelSubscriptionImmediately(ctx, tenant.StripeSubscriptionID); err != nil {
			log.Printf("Admin: failed to cancel subscription immediately: %v", err)
			respondWithError(w, http.StatusInternalServerError, "Failed to cancel subscription")
			return
		}
		// subscription.deleted webhook will handle the rest
	} else {
		periodEnd, err := h.stripe.CancelSubscriptionAtPeriodEnd(ctx, tenant.StripeSubscriptionID)
		if err != nil {
			log.Printf("Admin: failed to cancel subscription: %v", err)
			respondWithError(w, http.StatusInternalServerError, "Failed to cancel subscription")
			return
		}
		updates := bson.M{
			"billingStatus": models.BillingStatusCanceled,
			"canceledAt":    now,
			"updatedAt":     now,
		}
		if periodEnd != nil {
			updates["currentPeriodEnd"] = periodEnd
		}
		h.db.Tenants().UpdateOne(ctx, bson.M{"_id": tenantID}, bson.M{"$set": updates})
	}

	h.syslog.High(ctx, fmt.Sprintf("Admin canceled subscription: tenant %s, immediate=%v", tenantID.Hex(), req.Immediate))
	respondWithJSON(w, http.StatusOK, map[string]string{"message": "Subscription canceled"})
}

// AdminUpdateSubscription allows an admin to modify subscription details.
func (h *BillingHandler) AdminUpdateSubscription(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, err := primitive.ObjectIDFromHex(mux.Vars(r)["tenantId"])
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid tenant ID")
		return
	}

	var req struct {
		CurrentPeriodEnd *time.Time `json:"currentPeriodEnd"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	updates := bson.M{"updatedAt": time.Now()}
	if req.CurrentPeriodEnd != nil {
		updates["currentPeriodEnd"] = req.CurrentPeriodEnd
	}

	result, err := h.db.Tenants().UpdateOne(ctx, bson.M{"_id": tenantID}, bson.M{"$set": updates})
	if err != nil || result.MatchedCount == 0 {
		respondWithError(w, http.StatusNotFound, "Tenant not found")
		return
	}

	h.syslog.High(ctx, fmt.Sprintf("Admin updated subscription: tenant %s", tenantID.Hex()))
	respondWithJSON(w, http.StatusOK, map[string]string{"message": "Subscription updated"})
}
