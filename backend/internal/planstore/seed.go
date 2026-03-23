package planstore

import (
	"context"
	"log/slog"
	"time"

	"lastsaas/internal/db"
	"lastsaas/internal/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// Seed ensures the system "Free" plan exists. Idempotent — will not overwrite.
func Seed(ctx context.Context, database *db.MongoDB) error {
	col := database.Plans()

	err := col.FindOne(ctx, bson.M{"isSystem": true}).Err()
	if err == mongo.ErrNoDocuments {
		now := time.Now()
		plan := models.Plan{
			Name:                 "Free",
			Description:          "Default free plan",
			PricingModel:         models.PricingModelFlat,
			MonthlyPriceCents:    0,
			AnnualDiscountPct:    0,
			UsageCreditsPerMonth: 0,
			CreditResetPolicy:    models.CreditResetPolicyReset,
			BonusCredits:         0,
			UserLimit:            0,
			Entitlements:         map[string]models.EntitlementValue{},
			IsSystem:             true,
			CreatedAt:            now,
			UpdatedAt:            now,
		}
		if _, insertErr := col.InsertOne(ctx, plan); insertErr != nil {
			return insertErr
		}
		slog.Info("Seeded system plan", "plan", "Free")
	} else if err != nil {
		return err
	}
	return nil
}
