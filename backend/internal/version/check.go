package version

import (
	"context"
	"log/slog"
	"time"

	"lastsaas/internal/db"
	"lastsaas/internal/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// CheckAndMigrate compares the VERSION file to the DB version.
// If they differ, it runs migrations and sends a welcome message
// to the root tenant owner.
func CheckAndMigrate(database *db.MongoDB) {
	if Current == "" || Current == "unknown" {
		slog.Warn("VERSION file not found, skipping version check")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var sys models.SystemConfig
	err := database.SystemConfig().FindOne(ctx, bson.M{}).Decode(&sys)
	if err != nil {
		// System not initialized yet — nothing to check
		return
	}

	if sys.Version == Current {
		slog.Info("Version up to date", "version", Current)
		return
	}

	oldVersion := sys.Version
	slog.Info("Version changed", "from", oldVersion, "to", Current)

	// Run migrations (placeholder for future use)
	runMigrations(database, oldVersion, Current)

	// Send welcome message to root tenant owner
	sendUpgradeMessage(ctx, database, Current)

	// Update DB version
	database.SystemConfig().UpdateOne(ctx,
		bson.M{"_id": sys.ID},
		bson.M{"$set": bson.M{"version": Current}},
	)
	slog.Info("Database version updated", "version", Current)
}

func runMigrations(database *db.MongoDB, from, to string) {
	// Placeholder: future migrations will be dispatched here
	// based on version comparison.
	slog.Info("Migrations: none registered", "from", from, "to", to)
}

func sendUpgradeMessage(ctx context.Context, database *db.MongoDB, newVersion string) {
	var rootTenant models.Tenant
	err := database.Tenants().FindOne(ctx, bson.M{"isRoot": true}).Decode(&rootTenant)
	if err != nil {
		slog.Warn("Could not find root tenant for upgrade message", "error", err)
		return
	}

	var membership models.TenantMembership
	err = database.TenantMemberships().FindOne(ctx, bson.M{
		"tenantId": rootTenant.ID,
		"role":     "owner",
	}).Decode(&membership)
	if err != nil {
		slog.Warn("Could not find root tenant owner for upgrade message", "error", err)
		return
	}

	msg := models.Message{
		ID:        primitive.NewObjectID(),
		UserID:    membership.UserID,
		Subject:   "Welcome to MCPLens v" + newVersion,
		Body:      "Your system has been upgraded to version " + newVersion + ". Thank you for using MCPLens!",
		IsSystem:  true,
		Read:      false,
		CreatedAt: time.Now(),
	}

	if _, err := database.Messages().InsertOne(ctx, msg); err != nil {
		slog.Warn("Failed to send upgrade message", "error", err)
	}
}
