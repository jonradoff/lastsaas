package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"lastsaas/internal/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func cmdFinancial() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, `Usage: mcplens financial <subcommand>

Subcommands:
  summary                     Revenue summary and key metrics
  transactions                List recent financial transactions
  metrics                     Show daily business metrics`)
		os.Exit(1)
	}

	switch os.Args[2] {
	case "summary":
		cmdFinancialSummary()
	case "transactions":
		cmdFinancialTransactions()
	case "metrics":
		cmdFinancialMetrics()
	default:
		fmt.Fprintf(os.Stderr, "Unknown financial subcommand: %s\n", os.Args[2])
		os.Exit(1)
	}
}

func cmdFinancialSummary() {
	database, _, cleanup := connectDB()
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Total revenue (excluding refunds)
	revPipeline := bson.A{
		bson.M{"$match": bson.M{"type": bson.M{"$ne": "refund"}}},
		bson.M{"$group": bson.M{
			"_id":   nil,
			"total": bson.M{"$sum": "$amountCents"},
			"tax":   bson.M{"$sum": "$taxAmountCents"},
			"count": bson.M{"$sum": 1},
		}},
	}
	revCursor, _ := database.FinancialTransactions().Aggregate(ctx, revPipeline)
	var totalRevenue, totalTax, txCount int64
	if revCursor != nil {
		type r struct {
			Total int64 `bson:"total"`
			Tax   int64 `bson:"tax"`
			Count int64 `bson:"count"`
		}
		var res []r
		revCursor.All(ctx, &res)
		revCursor.Close(ctx)
		if len(res) > 0 {
			totalRevenue = res[0].Total
			totalTax = res[0].Tax
			txCount = res[0].Count
		}
	}

	// Refund total
	refundPipeline := bson.A{
		bson.M{"$match": bson.M{"type": "refund"}},
		bson.M{"$group": bson.M{"_id": nil, "total": bson.M{"$sum": "$amountCents"}, "count": bson.M{"$sum": 1}}},
	}
	refCursor, _ := database.FinancialTransactions().Aggregate(ctx, refundPipeline)
	var totalRefunds, refundCount int64
	if refCursor != nil {
		type r struct {
			Total int64 `bson:"total"`
			Count int64 `bson:"count"`
		}
		var res []r
		refCursor.All(ctx, &res)
		refCursor.Close(ctx)
		if len(res) > 0 {
			totalRefunds = res[0].Total
			refundCount = res[0].Count
		}
	}

	// Revenue by type
	typePipeline := bson.A{
		bson.M{"$group": bson.M{
			"_id":   "$type",
			"total": bson.M{"$sum": "$amountCents"},
			"count": bson.M{"$sum": 1},
		}},
	}
	typeCursor, _ := database.FinancialTransactions().Aggregate(ctx, typePipeline)
	typeBreakdown := map[string]struct{ total, count int64 }{}
	if typeCursor != nil {
		type r struct {
			Type  string `bson:"_id"`
			Total int64  `bson:"total"`
			Count int64  `bson:"count"`
		}
		var res []r
		typeCursor.All(ctx, &res)
		typeCursor.Close(ctx)
		for _, r := range res {
			typeBreakdown[r.Type] = struct{ total, count int64 }{r.Total, r.Count}
		}
	}

	// Active subscriptions and MRR
	activeSubs, _ := database.Tenants().CountDocuments(ctx, bson.M{"billingStatus": "active"})

	// Latest daily metric for ARR
	var latestMetric models.DailyMetric
	database.DailyMetrics().FindOne(ctx, bson.M{},
		options.FindOne().SetSort(bson.D{{Key: "date", Value: -1}})).Decode(&latestMetric)

	// Revenue last 30 days
	since30d := time.Now().Add(-30 * 24 * time.Hour)
	rev30Pipeline := bson.A{
		bson.M{"$match": bson.M{"type": bson.M{"$ne": "refund"}, "createdAt": bson.M{"$gte": since30d}}},
		bson.M{"$group": bson.M{"_id": nil, "total": bson.M{"$sum": "$amountCents"}}},
	}
	rev30Cursor, _ := database.FinancialTransactions().Aggregate(ctx, rev30Pipeline)
	var rev30d int64
	if rev30Cursor != nil {
		type r struct {
			Total int64 `bson:"total"`
		}
		var res []r
		rev30Cursor.All(ctx, &res)
		rev30Cursor.Close(ctx)
		if len(res) > 0 {
			rev30d = res[0].Total
		}
	}

	if jsonOutput {
		printJSON(map[string]interface{}{
			"totalRevenue":        totalRevenue,
			"totalTax":            totalTax,
			"netRevenue":          totalRevenue - totalTax,
			"totalRefunds":        totalRefunds,
			"transactionCount":    txCount,
			"refundCount":         refundCount,
			"activeSubscriptions": activeSubs,
			"arr":                 latestMetric.ARR,
			"revenue30d":          rev30d,
			"byType":              typeBreakdown,
		})
		return
	}

	fmt.Printf("%s\n\n", bold("Financial Summary"))

	fmt.Printf("  Total Revenue:       %s\n", bold(formatCents(totalRevenue, "usd")))
	if totalTax > 0 {
		fmt.Printf("    Tax collected:     %s\n", formatCents(totalTax, "usd"))
		fmt.Printf("    Net (excl. tax):   %s\n", formatCents(totalRevenue-totalTax, "usd"))
	}
	if totalRefunds > 0 {
		fmt.Printf("  Refunds:             %s (%d)\n", clr(cRed, formatCents(totalRefunds, "usd")), refundCount)
	}
	fmt.Printf("  Transactions:        %d\n", txCount)
	fmt.Printf("  Revenue (30d):       %s\n", bold(formatCents(rev30d, "usd")))
	fmt.Printf("  Active Subs:         %d\n", activeSubs)
	if latestMetric.ARR > 0 {
		fmt.Printf("  ARR:                 %s\n", bold(formatCents(latestMetric.ARR, "usd")))
	}

	if len(typeBreakdown) > 0 {
		fmt.Printf("\n  %s\n", bold("By Type:"))
		for _, t := range []string{"subscription", "credit_purchase", "refund"} {
			if b, ok := typeBreakdown[t]; ok {
				fmt.Printf("    %-20s %s (%d transactions)\n", t, formatCents(b.total, "usd"), b.count)
			}
		}
	}
}

func cmdFinancialTransactions() {
	fs := flag.NewFlagSet("financial transactions", flag.ExitOnError)
	limit := fs.Int("limit", 25, "Number of transactions to show")
	txType := fs.String("type", "", "Filter by type (subscription, credit_purchase, refund)")
	from := fs.String("from", "", "Start date (RFC3339 or relative: 7d, 30d)")
	to := fs.String("to", "", "End date (RFC3339)")
	tenantID := fs.String("tenant", "", "Filter by tenant ID")
	fs.Parse(os.Args[3:])

	database, _, cleanup := connectDB()
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	filter := bson.M{}
	if *txType != "" {
		filter["type"] = *txType
	}
	if *tenantID != "" {
		if oid, err := primitive.ObjectIDFromHex(*tenantID); err == nil {
			filter["tenantId"] = oid
		}
	}

	dateFilter := bson.M{}
	if *from != "" {
		if t := parseTimeArg(*from); !t.IsZero() {
			dateFilter["$gte"] = t
		}
	}
	if *to != "" {
		if t, err := time.Parse(time.RFC3339, *to); err == nil {
			dateFilter["$lte"] = t
		}
	}
	if len(dateFilter) > 0 {
		filter["createdAt"] = dateFilter
	}

	opts := options.Find().
		SetSort(bson.D{{Key: "createdAt", Value: -1}}).
		SetLimit(int64(*limit))

	cursor, err := database.FinancialTransactions().Find(ctx, filter, opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to query transactions: %v\n", err)
		os.Exit(1)
	}
	defer cursor.Close(ctx)

	var txns []models.FinancialTransaction
	if err := cursor.All(ctx, &txns); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to read transactions: %v\n", err)
		os.Exit(1)
	}

	if jsonOutput {
		type txRow struct {
			ID          string `json:"id"`
			Invoice     string `json:"invoiceNumber"`
			Type        string `json:"type"`
			Amount      int64  `json:"amountCents"`
			Tax         int64  `json:"taxAmountCents"`
			Currency    string `json:"currency"`
			Description string `json:"description"`
			TenantID    string `json:"tenantId"`
			PlanName    string `json:"planName,omitempty"`
			BundleName  string `json:"bundleName,omitempty"`
			CreatedAt   string `json:"createdAt"`
		}
		rows := make([]txRow, 0, len(txns))
		for _, t := range txns {
			rows = append(rows, txRow{
				ID:          t.ID.Hex(),
				Invoice:     t.InvoiceNumber,
				Type:        string(t.Type),
				Amount:      t.AmountCents,
				Tax:         t.TaxAmountCents,
				Currency:    t.Currency,
				Description: t.Description,
				TenantID:    t.TenantID.Hex(),
				PlanName:    t.PlanName,
				BundleName:  t.BundleName,
				CreatedAt:   t.CreatedAt.Format(time.RFC3339),
			})
		}
		printJSON(rows)
		return
	}

	if len(txns) == 0 {
		fmt.Println("No transactions found.")
		return
	}

	fmt.Printf("%-12s %-14s %-12s %-10s %-30s %s\n",
		bold("INVOICE"), bold("TYPE"), bold("AMOUNT"), bold("TAX"), bold("DESCRIPTION"), bold("DATE"))
	fmt.Printf("%-12s %-14s %-12s %-10s %-30s %s\n",
		"-------", "----", "------", "---", "-----------", "----")

	for _, t := range txns {
		typeClr := string(t.Type)
		if t.Type == "refund" {
			typeClr = clr(cRed, "refund")
		}
		tax := ""
		if t.TaxAmountCents > 0 {
			tax = formatCents(t.TaxAmountCents, t.Currency)
		}
		fmt.Printf("%-12s %-14s %-12s %-10s %-30s %s\n",
			t.InvoiceNumber,
			typeClr,
			formatCents(t.AmountCents, t.Currency),
			tax,
			truncate(t.Description, 30),
			t.CreatedAt.Local().Format("2006-01-02 15:04"),
		)
	}
	fmt.Printf("\n%d transactions shown\n", len(txns))
}

func cmdFinancialMetrics() {
	fs := flag.NewFlagSet("financial metrics", flag.ExitOnError)
	days := fs.Int("days", 30, "Number of days of metrics to show")
	fs.Parse(os.Args[3:])

	database, _, cleanup := connectDB()
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	opts := options.Find().
		SetSort(bson.D{{Key: "date", Value: -1}}).
		SetLimit(int64(*days))

	cursor, err := database.DailyMetrics().Find(ctx, bson.M{}, opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to query daily metrics: %v\n", err)
		os.Exit(1)
	}
	defer cursor.Close(ctx)

	var metrics []models.DailyMetric
	if err := cursor.All(ctx, &metrics); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to read metrics: %v\n", err)
		os.Exit(1)
	}

	if jsonOutput {
		printJSON(metrics)
		return
	}

	if len(metrics) == 0 {
		fmt.Println("No daily metrics available yet.")
		return
	}

	fmt.Printf("%-12s %12s %12s %8s %8s\n",
		bold("DATE"), bold("REVENUE"), bold("ARR"), bold("DAU"), bold("MAU"))
	fmt.Printf("%-12s %12s %12s %8s %8s\n",
		"----", "-------", "---", "---", "---")

	// Reverse to show oldest first
	for i := len(metrics) - 1; i >= 0; i-- {
		m := metrics[i]
		fmt.Printf("%-12s %12s %12s %8d %8d\n",
			m.Date,
			formatCents(m.Revenue, "usd"),
			formatCents(m.ARR, "usd"),
			m.DAU,
			m.MAU,
		)
	}
}
