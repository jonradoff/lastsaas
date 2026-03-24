package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"strings"
	"strconv"
	"time"

	"lastsaas/internal/configstore"
	"lastsaas/internal/db"
	"lastsaas/internal/events"
	"lastsaas/internal/middleware"
	"lastsaas/internal/models"
	stripeservice "lastsaas/internal/stripe"
	"lastsaas/internal/syslog"
	"lastsaas/internal/telemetry"

	"github.com/gorilla/mux"
	"github.com/jung-kurt/gofpdf"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type BillingHandler struct {
	stripe       *stripeservice.Service
	db           *db.MongoDB
	events       events.Emitter
	syslog       *syslog.Logger
	store        *configstore.Store
	telemetrySvc *telemetry.Service
}

func (h *BillingHandler) SetTelemetry(svc *telemetry.Service) { h.telemetrySvc = svc }

func NewBillingHandler(stripeSvc *stripeservice.Service, database *db.MongoDB, emitter events.Emitter, sysLogger *syslog.Logger, store *configstore.Store) *BillingHandler {
	return &BillingHandler{
		stripe: stripeSvc,
		db:     database,
		events: emitter,
		syslog: sysLogger,
		store:  store,
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

	// Read default currency from config
	currency := strings.ToLower(h.store.Get("billing.default_currency"))
	if currency == "" {
		currency = "usd"
	}

	// Stripe Tax: automatic tax calculation
	automaticTax := h.store.Get("billing.tax.enabled") == "true"

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

		if h.telemetrySvc != nil {
			h.telemetrySvc.TrackCheckoutStarted(ctx, user.ID, tenant.ID, plan.Name)
		}

		// Prevent trial abuse: skip trial if this tenant or user already used one
		trialDays := plan.TrialDays
		if tenant.TrialUsedAt != nil {
			trialDays = 0
		}
		if trialDays > 0 && user.TrialUsedAt != nil {
			trialDays = 0
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
				slog.Error("Billing: failed to get/create customer", "error", err)
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
				TrialDays:       trialDays,
				Currency:        currency,
				AutomaticTax:    automaticTax,
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

				basePriceID, err := h.stripe.GetOrCreatePrice(ctx, "plan_base_"+req.BillingInterval, planID, plan.Name+" (Base)", baseAmount, req.BillingInterval, currency)
				if err != nil {
					slog.Error("Billing: failed to create base price", "error", err)
					respondWithError(w, http.StatusInternalServerError, "Failed to create billing session")
					return
				}

				customItems := []stripeservice.CheckoutLineItem{
					{PriceID: basePriceID, Quantity: 1},
				}

				additionalSeats := int64(seats) - int64(plan.IncludedSeats)
				if additionalSeats > 0 {
					seatPriceID, err := h.stripe.GetOrCreatePrice(ctx, "plan_seat_"+req.BillingInterval, planID, plan.Name+" (Per Seat)", perSeatAmount, req.BillingInterval, currency)
					if err != nil {
						slog.Error("Billing: failed to create seat price", "error", err)
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
				slog.Error("Billing: failed to create checkout session", "error", err)
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
			slog.Error("Billing: failed to get/create customer", "error", err)
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
			TrialDays:       trialDays,
			Currency:        currency,
			AutomaticTax:    automaticTax,
		})
		if err != nil {
			slog.Error("Billing: failed to create checkout session", "error", err)
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
			slog.Error("Billing: failed to get/create customer", "error", err)
			respondWithError(w, http.StatusInternalServerError, "Failed to create billing session")
			return
		}

		url, err := h.stripe.CreateCheckoutSession(ctx, stripeservice.CheckoutRequest{
			CustomerID:   customerID,
			BundleID:     &bundleID,
			BundleName:   bundle.Name,
			AmountCents:  bundle.PriceCents,
			TenantID:     tenant.ID.Hex(),
			UserID:       user.ID.Hex(),
			Currency:     currency,
			AutomaticTax: automaticTax,
		})
		if err != nil {
			slog.Error("Billing: failed to create checkout session", "error", err)
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
		slog.Error("Billing: failed to create portal session", "error", err)
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

	// Branded title
	appName := h.store.Get("app.name")
	title := "INVOICE"
	if appName != "" {
		title = appName + " — INVOICE"
	}
	pdf.SetFont("Helvetica", "B", 20)
	pdf.Cell(0, 12, title)
	pdf.Ln(16)

	// Company info (from) and invoice details side by side
	companyName := h.store.Get("billing.company_name")
	companyAddress := h.store.Get("billing.company_address")
	companyTaxID := h.store.Get("billing.tax.id")

	if companyName != "" {
		pdf.SetFont("Helvetica", "B", 10)
		pdf.Cell(95, 6, companyName)
	} else {
		pdf.Cell(95, 6, "")
	}
	pdf.SetFont("Helvetica", "", 10)
	pdf.Cell(95, 6, fmt.Sprintf("Invoice #: %s", tx.InvoiceNumber))
	pdf.Ln(6)

	// Address lines (left) and date (right)
	addressLines := []string{}
	if companyAddress != "" {
		addressLines = strings.Split(strings.ReplaceAll(companyAddress, "\\n", "\n"), "\n")
	}
	if len(addressLines) > 0 {
		pdf.SetFont("Helvetica", "", 9)
		pdf.Cell(95, 5, strings.TrimSpace(addressLines[0]))
	} else {
		pdf.Cell(95, 5, "")
	}
	pdf.SetFont("Helvetica", "", 10)
	pdf.Cell(95, 5, fmt.Sprintf("Date: %s", tx.CreatedAt.Format("January 2, 2006")))
	pdf.Ln(5)

	// Remaining address lines
	if len(addressLines) > 1 {
		pdf.SetFont("Helvetica", "", 9)
		for _, line := range addressLines[1:] {
			pdf.Cell(95, 5, strings.TrimSpace(line))
			pdf.Ln(5)
		}
	}

	// Company tax/VAT ID
	if companyTaxID != "" {
		pdf.SetFont("Helvetica", "", 9)
		pdf.Cell(95, 5, companyTaxID)
		pdf.Ln(5)
	}
	pdf.Ln(4)

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
	// Show subtotal in line item if tax present, otherwise full amount
	lineAmount := tx.AmountCents
	if tx.TaxAmountCents > 0 && tx.SubtotalCents > 0 {
		lineAmount = tx.SubtotalCents
	}
	pdf.CellFormat(45, 8, fmt.Sprintf("$%.2f", float64(lineAmount)/100), "1", 0, "R", false, 0, "")
	pdf.Ln(12)

	// Totals section
	if tx.TaxAmountCents > 0 {
		// Subtotal
		pdf.SetFont("Helvetica", "", 10)
		pdf.Cell(145, 7, "Subtotal:")
		pdf.Cell(45, 7, fmt.Sprintf("$%.2f", float64(tx.SubtotalCents)/100))
		pdf.Ln(7)
		// Tax
		pdf.Cell(145, 7, "Tax:")
		pdf.Cell(45, 7, fmt.Sprintf("$%.2f", float64(tx.TaxAmountCents)/100))
		pdf.Ln(7)
	}
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
		slog.Error("Billing: failed to cancel subscription", "error", err)
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

	if h.telemetrySvc != nil {
		user, _ := middleware.GetUserFromContext(ctx)
		var userIDPtr *primitive.ObjectID
		if user != nil {
			userIDPtr = &user.ID
		}
		h.telemetrySvc.Track(ctx, models.TelemetryEvent{
			EventName: models.TelemetrySubscriptionCanceled,
			Category:  models.TelemetryCategoryFunnel,
			UserID:    userIDPtr,
			TenantID:  &tenant.ID,
		})
	}

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

// PurchaseScan creates a Stripe one-time checkout for a scan feature ($5 AI assessment or $10 simulation).
// Body: {"domain": "allbirds.com", "feature": "assess" | "simulate"}
// Requires auth + tenant.
func (h *BillingHandler) PurchaseScan(w http.ResponseWriter, r *http.Request) {
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
		Domain  string `json:"domain"`
		Feature string `json:"feature"` // "assess" or "simulate"
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if req.Domain == "" {
		respondWithError(w, http.StatusBadRequest, "domain is required")
		return
	}
	if req.Feature != "assess" && req.Feature != "simulate" {
		respondWithError(w, http.StatusBadRequest, "feature must be 'assess' or 'simulate'")
		return
	}

	if h.stripe == nil {
		respondWithError(w, http.StatusServiceUnavailable, "Billing not configured")
		return
	}

	var amountCents int64
	var productName string
	if req.Feature == "assess" {
		amountCents = 500 // $5
		productName = "AI Quality Assessment"
	} else {
		amountCents = 1000 // $10
		productName = "Simulated Buyer Agent"
	}

	currency := strings.ToLower(h.store.Get("billing.default_currency"))
	if currency == "" {
		currency = "usd"
	}

	customerID, err := h.stripe.GetOrCreateCustomer(ctx, tenant, user.Email)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to create billing session")
		return
	}

	url, err := h.stripe.CreateCheckoutSession(ctx, stripeservice.CheckoutRequest{
		CustomerID:  customerID,
		BundleName:  productName,
		AmountCents: amountCents,
		TenantID:    tenant.ID.Hex(),
		UserID:      user.ID.Hex(),
		Currency:    currency,
		ExtraMetadata: map[string]string{
			"scanPurchase": req.Feature,
			"scanDomain":   req.Domain,
		},
	})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to create checkout session")
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]string{"checkoutUrl": url})
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
		escaped := escapeRegexInput(search)
		filter["$or"] = []bson.M{
			{"description": bson.M{"$regex": escaped, "$options": "i"}},
			{"invoiceNumber": bson.M{"$regex": escaped, "$options": "i"}},
			{"planName": bson.M{"$regex": escaped, "$options": "i"}},
			{"bundleName": bson.M{"$regex": escaped, "$options": "i"}},
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
// For revenue and ARR, today's data point is computed live from source collections
// rather than relying on the hourly metrics snapshot, so new payments appear immediately.
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
	todayStr := time.Now().UTC().Format("2006-01-02")
	for _, m := range metrics {
		var val int64
		switch metric {
		case "revenue":
			val = m.Revenue
		case "arr":
			val = m.ARR
		case "dau":
			val = m.DAU
		case "wau":
			val = m.WAU
		case "mau":
			val = m.MAU
		default:
			val = m.Revenue
		}
		points = append(points, point{Date: m.Date, Value: val})
	}

	// For revenue and ARR, replace today's snapshot with a live computation
	// so new payments appear on the dashboard immediately.
	if metric == "revenue" || metric == "arr" {
		liveValue := h.computeLiveMetric(ctx, metric, todayStr)
		replaced := false
		for i, p := range points {
			if p.Date == todayStr {
				points[i].Value = liveValue
				replaced = true
				break
			}
		}
		if !replaced {
			points = append(points, point{Date: todayStr, Value: liveValue})
		}
	}

	if points == nil {
		points = []point{}
	}

	respondWithJSON(w, http.StatusOK, map[string]interface{}{"data": points})
}

// computeLiveMetric returns a real-time value for the given metric.
func (h *BillingHandler) computeLiveMetric(ctx context.Context, metric, dateStr string) int64 {
	switch metric {
	case "revenue":
		return h.computeLiveRevenue(ctx, dateStr)
	case "arr":
		return h.computeLiveARR(ctx)
	}
	return 0
}

// computeLiveRevenue sums today's financial transactions directly.
func (h *BillingHandler) computeLiveRevenue(ctx context.Context, dateStr string) int64 {
	dayStart, _ := time.Parse("2006-01-02", dateStr)
	dayEnd := dayStart.Add(24 * time.Hour)

	pipeline := bson.A{
		bson.M{"$match": bson.M{
			"createdAt": bson.M{"$gte": dayStart, "$lt": dayEnd},
		}},
		bson.M{"$group": bson.M{
			"_id":   nil,
			"total": bson.M{"$sum": "$amountCents"},
		}},
	}

	cursor, err := h.db.FinancialTransactions().Aggregate(ctx, pipeline)
	if err != nil {
		return 0
	}
	defer cursor.Close(ctx)

	var result []struct {
		Total int64 `bson:"total"`
	}
	if cursor.All(ctx, &result) == nil && len(result) > 0 {
		return result[0].Total
	}
	return 0
}

// computeLiveARR calculates ARR from active tenant subscriptions in real time.
func (h *BillingHandler) computeLiveARR(ctx context.Context) int64 {
	pipeline := bson.A{
		bson.M{"$match": bson.M{
			"billingStatus": models.BillingStatusActive,
			"planId":        bson.M{"$ne": nil},
		}},
		bson.M{"$lookup": bson.M{
			"from":         "plans",
			"localField":   "planId",
			"foreignField": "_id",
			"as":           "plan",
		}},
		bson.M{"$unwind": bson.M{"path": "$plan", "preserveNullAndEmptyArrays": false}},
		bson.M{"$group": bson.M{
			"_id":               nil,
			"totalMonthlyCents": bson.M{"$sum": "$plan.monthlyPriceCents"},
		}},
	}

	cursor, err := h.db.Tenants().Aggregate(ctx, pipeline)
	if err != nil {
		return 0
	}
	defer cursor.Close(ctx)

	var result []struct {
		TotalMonthlyCents int64 `bson:"totalMonthlyCents"`
	}
	if cursor.All(ctx, &result) == nil && len(result) > 0 {
		return result[0].TotalMonthlyCents * 12
	}
	return 0
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
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if h.stripe == nil {
		respondWithError(w, http.StatusServiceUnavailable, "Billing not configured")
		return
	}

	now := time.Now()
	if req.Immediate {
		if err := h.stripe.CancelSubscriptionImmediately(ctx, tenant.StripeSubscriptionID); err != nil {
			slog.Error("Admin: failed to cancel subscription immediately", "error", err)
			respondWithError(w, http.StatusInternalServerError, "Failed to cancel subscription")
			return
		}
		// subscription.deleted webhook will handle the rest
	} else {
		periodEnd, err := h.stripe.CancelSubscriptionAtPeriodEnd(ctx, tenant.StripeSubscriptionID)
		if err != nil {
			slog.Error("Admin: failed to cancel subscription", "error", err)
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
