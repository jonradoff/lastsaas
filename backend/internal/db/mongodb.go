package db

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MongoDB struct {
	Client   *mongo.Client
	Database *mongo.Database
}

func NewMongoDB(uri, database string) (*MongoDB, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	clientOptions := options.Client().
		ApplyURI(uri).
		SetMaxPoolSize(100).
		SetMinPoolSize(5).
		SetMaxConnIdleTime(5 * time.Minute)

	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MongoDB: %w", err)
	}

	if err := client.Ping(ctx, nil); err != nil {
		return nil, fmt.Errorf("failed to ping MongoDB: %w", err)
	}

	db := &MongoDB{
		Client:   client,
		Database: client.Database(database),
	}

	db.ensureIndexes()
	if os.Getenv("LASTSAAS_ENV") != "test" {
		db.EnsureSchemaValidation()
	}

	return db, nil
}

func (m *MongoDB) ensureIndexes() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	indexes := []struct {
		collection string
		models     []mongo.IndexModel
	}{
		{
			"users",
			[]mongo.IndexModel{
				{Keys: bson.D{{Key: "email", Value: 1}}, Options: options.Index().SetUnique(true).SetSparse(true)},
				{Keys: bson.D{{Key: "googleId", Value: 1}}, Options: options.Index().SetSparse(true)},
				{Keys: bson.D{{Key: "githubId", Value: 1}}, Options: options.Index().SetSparse(true)},
				{Keys: bson.D{{Key: "microsoftId", Value: 1}}, Options: options.Index().SetSparse(true)},
				{Keys: bson.D{{Key: "displayName", Value: 1}}},
			},
		},
		{
			"tenants",
			[]mongo.IndexModel{
				{Keys: bson.D{{Key: "slug", Value: 1}}, Options: options.Index().SetUnique(true)},
				{Keys: bson.D{{Key: "isRoot", Value: 1}}},
				{Keys: bson.D{{Key: "name", Value: 1}}},
				{Keys: bson.D{{Key: "billingStatus", Value: 1}, {Key: "isActive", Value: 1}}},
				{Keys: bson.D{{Key: "planId", Value: 1}}},
				{Keys: bson.D{{Key: "trialUsedAt", Value: 1}}, Options: options.Index().SetSparse(true)},
			},
		},
		{
			"tenant_memberships",
			[]mongo.IndexModel{
				{Keys: bson.D{{Key: "userId", Value: 1}, {Key: "tenantId", Value: 1}}, Options: options.Index().SetUnique(true)},
				{Keys: bson.D{{Key: "tenantId", Value: 1}, {Key: "role", Value: 1}}},
				{Keys: bson.D{{Key: "userId", Value: 1}}},
			},
		},
		{
			"refresh_tokens",
			[]mongo.IndexModel{
				{Keys: bson.D{{Key: "userId", Value: 1}}},
				{Keys: bson.D{{Key: "expiresAt", Value: 1}}, Options: options.Index().SetExpireAfterSeconds(0)},
			},
		},
		{
			"verification_tokens",
			[]mongo.IndexModel{
				{Keys: bson.D{{Key: "userId", Value: 1}, {Key: "type", Value: 1}}},
				{Keys: bson.D{{Key: "token", Value: 1}}},
				{Keys: bson.D{{Key: "expiresAt", Value: 1}}, Options: options.Index().SetExpireAfterSeconds(0)},
			},
		},
		{
			"oauth_states",
			[]mongo.IndexModel{
				{Keys: bson.D{{Key: "expiresAt", Value: 1}}, Options: options.Index().SetExpireAfterSeconds(0)},
			},
		},
		{
			"revoked_tokens",
			[]mongo.IndexModel{
				{Keys: bson.D{{Key: "expiresAt", Value: 1}}, Options: options.Index().SetExpireAfterSeconds(0)},
				{Keys: bson.D{{Key: "tokenHash", Value: 1}}, Options: options.Index().SetUnique(true)},
			},
		},
		{
			"invitations",
			[]mongo.IndexModel{
				{Keys: bson.D{{Key: "tenantId", Value: 1}, {Key: "email", Value: 1}}, Options: options.Index().SetUnique(true)},
				{Keys: bson.D{{Key: "token", Value: 1}}},
				{Keys: bson.D{{Key: "expiresAt", Value: 1}}, Options: options.Index().SetExpireAfterSeconds(0)},
			},
		},
		{
			"audit_log",
			[]mongo.IndexModel{
				{Keys: bson.D{{Key: "createdAt", Value: 1}}, Options: options.Index().SetExpireAfterSeconds(90 * 24 * 3600)},
				{Keys: bson.D{{Key: "userId", Value: 1}, {Key: "createdAt", Value: -1}}},
				{Keys: bson.D{{Key: "tenantId", Value: 1}, {Key: "createdAt", Value: -1}}},
			},
		},
		{
			"messages",
			[]mongo.IndexModel{
				{Keys: bson.D{{Key: "userId", Value: 1}, {Key: "createdAt", Value: -1}}},
				{Keys: bson.D{{Key: "userId", Value: 1}, {Key: "read", Value: 1}}},
			},
		},
		{
			"system_logs",
			[]mongo.IndexModel{
				{Keys: bson.D{{Key: "createdAt", Value: 1}}, Options: options.Index().SetExpireAfterSeconds(180 * 24 * 3600)},
				{Keys: bson.D{{Key: "severity", Value: 1}, {Key: "createdAt", Value: -1}}},
				{Keys: bson.D{{Key: "category", Value: 1}, {Key: "createdAt", Value: -1}}},
				{Keys: bson.D{{Key: "message", Value: "text"}}},
				{Keys: bson.D{{Key: "userId", Value: 1}, {Key: "createdAt", Value: -1}}},
				{Keys: bson.D{{Key: "tenantId", Value: 1}, {Key: "createdAt", Value: -1}}},
			},
		},
		{
			"config_vars",
			[]mongo.IndexModel{
				{Keys: bson.D{{Key: "name", Value: 1}}, Options: options.Index().SetUnique(true)},
			},
		},
		{
			"plans",
			[]mongo.IndexModel{
				{Keys: bson.D{{Key: "name", Value: 1}}, Options: options.Index().SetUnique(true)},
				{Keys: bson.D{{Key: "isSystem", Value: 1}}},
			},
		},
		{
			"credit_bundles",
			[]mongo.IndexModel{
				{Keys: bson.D{{Key: "name", Value: 1}}, Options: options.Index().SetUnique(true)},
				{Keys: bson.D{{Key: "sortOrder", Value: 1}}},
			},
		},
		{
			"financial_transactions",
			[]mongo.IndexModel{
				{Keys: bson.D{{Key: "tenantId", Value: 1}, {Key: "createdAt", Value: -1}}},
				{Keys: bson.D{{Key: "userId", Value: 1}, {Key: "createdAt", Value: -1}}},
				{Keys: bson.D{{Key: "invoiceNumber", Value: 1}}, Options: options.Index().SetUnique(true)},
			},
		},
		{
			"stripe_mappings",
			[]mongo.IndexModel{
				{Keys: bson.D{{Key: "entityType", Value: 1}, {Key: "entityId", Value: 1}}, Options: options.Index().SetUnique(true)},
			},
		},
		{
			"daily_metrics",
			[]mongo.IndexModel{
				{Keys: bson.D{{Key: "date", Value: 1}}, Options: options.Index().SetUnique(true)},
				{Keys: bson.D{{Key: "createdAt", Value: 1}}, Options: options.Index().SetExpireAfterSeconds(400 * 24 * 3600)},
			},
		},
		{
			"leader_locks",
			[]mongo.IndexModel{
				{Keys: bson.D{{Key: "expiresAt", Value: 1}}, Options: options.Index().SetExpireAfterSeconds(0)},
			},
		},
		{
			"webhook_events",
			[]mongo.IndexModel{
				{Keys: bson.D{{Key: "eventId", Value: 1}}, Options: options.Index().SetUnique(true)},
				{Keys: bson.D{{Key: "createdAt", Value: 1}}, Options: options.Index().SetExpireAfterSeconds(30 * 24 * 3600)},
			},
		},
		{
			"system_nodes",
			[]mongo.IndexModel{
				{Keys: bson.D{{Key: "machineId", Value: 1}}, Options: options.Index().SetUnique(true)},
				{Keys: bson.D{{Key: "lastSeen", Value: 1}}},
				{Keys: bson.D{{Key: "startedAt", Value: 1}}},
			},
		},
		{
			"system_metrics",
			[]mongo.IndexModel{
				{Keys: bson.D{{Key: "timestamp", Value: 1}}, Options: options.Index().SetExpireAfterSeconds(30 * 24 * 3600)},
				{Keys: bson.D{{Key: "nodeId", Value: 1}, {Key: "timestamp", Value: -1}}},
			},
		},
		{
			"api_keys",
			[]mongo.IndexModel{
				{Keys: bson.D{{Key: "keyHash", Value: 1}}, Options: options.Index().SetUnique(true)},
				{Keys: bson.D{{Key: "createdBy", Value: 1}, {Key: "createdAt", Value: -1}}},
			},
		},
		{
			"webhooks",
			[]mongo.IndexModel{
				{Keys: bson.D{{Key: "createdBy", Value: 1}, {Key: "createdAt", Value: -1}}},
				{Keys: bson.D{{Key: "events", Value: 1}, {Key: "isActive", Value: 1}}},
			},
		},
		{
			"webhook_deliveries",
			[]mongo.IndexModel{
				{Keys: bson.D{{Key: "webhookId", Value: 1}, {Key: "createdAt", Value: -1}}},
				{Keys: bson.D{{Key: "createdAt", Value: 1}}, Options: options.Index().SetExpireAfterSeconds(30 * 24 * 3600)},
			},
		},
		{
			"branding_assets",
			[]mongo.IndexModel{
				{Keys: bson.D{{Key: "key", Value: 1}}, Options: options.Index().SetUnique(true)},
			},
		},
		{
			"custom_pages",
			[]mongo.IndexModel{
				{Keys: bson.D{{Key: "slug", Value: 1}}, Options: options.Index().SetUnique(true)},
				{Keys: bson.D{{Key: "isPublished", Value: 1}, {Key: "sortOrder", Value: 1}}},
			},
		},
		{
			"webauthn_credentials",
			[]mongo.IndexModel{
				{Keys: bson.D{{Key: "userId", Value: 1}}},
				{Keys: bson.D{{Key: "credentialId", Value: 1}}, Options: options.Index().SetUnique(true)},
			},
		},
		{
			"webauthn_sessions",
			[]mongo.IndexModel{
				{Keys: bson.D{{Key: "expiresAt", Value: 1}}, Options: options.Index().SetExpireAfterSeconds(0)},
			},
		},
		{
			"sso_connections",
			[]mongo.IndexModel{
				{Keys: bson.D{{Key: "tenantId", Value: 1}}, Options: options.Index().SetUnique(true)},
			},
		},
		{
			"auth_codes",
			[]mongo.IndexModel{
				{Keys: bson.D{{Key: "code", Value: 1}}, Options: options.Index().SetUnique(true)},
				{Keys: bson.D{{Key: "expiresAt", Value: 1}}, Options: options.Index().SetExpireAfterSeconds(0)},
			},
		},
		{
			"usage_events",
			[]mongo.IndexModel{
				{Keys: bson.D{{Key: "tenantId", Value: 1}, {Key: "createdAt", Value: -1}}},
				{Keys: bson.D{{Key: "tenantId", Value: 1}, {Key: "type", Value: 1}, {Key: "createdAt", Value: -1}}},
				{Keys: bson.D{{Key: "createdAt", Value: 1}}, Options: options.Index().SetExpireAfterSeconds(90 * 24 * 60 * 60)}, // TTL: 90 days
			},
		},
		{
			"telemetry_events",
			[]mongo.IndexModel{
				{Keys: bson.D{{Key: "eventName", Value: 1}, {Key: "createdAt", Value: -1}}},
				{Keys: bson.D{{Key: "category", Value: 1}, {Key: "createdAt", Value: -1}}},
				{Keys: bson.D{{Key: "userId", Value: 1}, {Key: "createdAt", Value: -1}}},
				{Keys: bson.D{{Key: "sessionId", Value: 1}, {Key: "createdAt", Value: -1}}},
				{Keys: bson.D{{Key: "createdAt", Value: 1}}, Options: options.Index().SetExpireAfterSeconds(365 * 24 * 3600)}, // TTL: 365 days
				{Keys: bson.D{{Key: "properties.page", Value: 1}, {Key: "createdAt", Value: -1}}, Options: options.Index().SetSparse(true)},
			},
		},
	}

	// Collections where unique index failure is a data integrity risk
	criticalCollections := map[string]bool{
		"users": true, "tenants": true, "financial_transactions": true,
		"api_keys": true, "config_vars": true, "stripe_mappings": true,
		"custom_pages": true, "branding_assets": true, "webauthn_credentials": true,
		"sso_connections": true, "auth_codes": true,
	}

	for _, idx := range indexes {
		coll := m.Database.Collection(idx.collection)
		_, err := coll.Indexes().CreateMany(ctx, idx.models)
		if err != nil {
			if criticalCollections[idx.collection] {
				slog.Error("FATAL: failed to create indexes on critical collection", "collection", idx.collection, "error", err)
				os.Exit(1)
			}
			slog.Warn("failed to create indexes", "collection", idx.collection, "error", err)
		}
	}
}

func (m *MongoDB) Close(ctx context.Context) error {
	return m.Client.Disconnect(ctx)
}

func (m *MongoDB) Users() *mongo.Collection {
	return m.Database.Collection("users")
}

func (m *MongoDB) Tenants() *mongo.Collection {
	return m.Database.Collection("tenants")
}

func (m *MongoDB) TenantMemberships() *mongo.Collection {
	return m.Database.Collection("tenant_memberships")
}

func (m *MongoDB) RefreshTokens() *mongo.Collection {
	return m.Database.Collection("refresh_tokens")
}

func (m *MongoDB) VerificationTokens() *mongo.Collection {
	return m.Database.Collection("verification_tokens")
}

func (m *MongoDB) OAuthStates() *mongo.Collection {
	return m.Database.Collection("oauth_states")
}

func (m *MongoDB) RevokedTokens() *mongo.Collection {
	return m.Database.Collection("revoked_tokens")
}

func (m *MongoDB) SystemConfig() *mongo.Collection {
	return m.Database.Collection("system_config")
}

func (m *MongoDB) Invitations() *mongo.Collection {
	return m.Database.Collection("invitations")
}

func (m *MongoDB) AuditLog() *mongo.Collection {
	return m.Database.Collection("audit_log")
}

func (m *MongoDB) Messages() *mongo.Collection {
	return m.Database.Collection("messages")
}

func (m *MongoDB) SystemLogs() *mongo.Collection {
	return m.Database.Collection("system_logs")
}

func (m *MongoDB) ConfigVars() *mongo.Collection {
	return m.Database.Collection("config_vars")
}

func (m *MongoDB) Plans() *mongo.Collection {
	return m.Database.Collection("plans")
}

func (m *MongoDB) CreditBundles() *mongo.Collection {
	return m.Database.Collection("credit_bundles")
}

func (m *MongoDB) SystemNodes() *mongo.Collection {
	return m.Database.Collection("system_nodes")
}

func (m *MongoDB) SystemMetrics() *mongo.Collection {
	return m.Database.Collection("system_metrics")
}

func (m *MongoDB) FinancialTransactions() *mongo.Collection {
	return m.Database.Collection("financial_transactions")
}

func (m *MongoDB) StripeMappings() *mongo.Collection {
	return m.Database.Collection("stripe_mappings")
}

func (m *MongoDB) Counters() *mongo.Collection {
	return m.Database.Collection("counters")
}

func (m *MongoDB) DailyMetrics() *mongo.Collection {
	return m.Database.Collection("daily_metrics")
}

func (m *MongoDB) WebhookEvents() *mongo.Collection {
	return m.Database.Collection("webhook_events")
}

func (m *MongoDB) LeaderLocks() *mongo.Collection {
	return m.Database.Collection("leader_locks")
}

func (m *MongoDB) APIKeys() *mongo.Collection {
	return m.Database.Collection("api_keys")
}

func (m *MongoDB) Webhooks() *mongo.Collection {
	return m.Database.Collection("webhooks")
}

func (m *MongoDB) WebhookDeliveries() *mongo.Collection {
	return m.Database.Collection("webhook_deliveries")
}

func (m *MongoDB) BrandingConfig() *mongo.Collection {
	return m.Database.Collection("branding_config")
}

func (m *MongoDB) BrandingAssets() *mongo.Collection {
	return m.Database.Collection("branding_assets")
}

func (m *MongoDB) CustomPages() *mongo.Collection {
	return m.Database.Collection("custom_pages")
}

func (m *MongoDB) WebAuthnCredentials() *mongo.Collection {
	return m.Database.Collection("webauthn_credentials")
}

func (m *MongoDB) WebAuthnSessions() *mongo.Collection {
	return m.Database.Collection("webauthn_sessions")
}

func (m *MongoDB) SSOConnections() *mongo.Collection {
	return m.Database.Collection("sso_connections")
}

func (m *MongoDB) Announcements() *mongo.Collection {
	return m.Database.Collection("announcements")
}

func (m *MongoDB) UsageEvents() *mongo.Collection {
	return m.Database.Collection("usage_events")
}

func (m *MongoDB) AuthCodes() *mongo.Collection {
	return m.Database.Collection("auth_codes")
}

func (m *MongoDB) ImpersonationLogs() *mongo.Collection {
	return m.Database.Collection("impersonation_logs")
}

func (m *MongoDB) TelemetryEvents() *mongo.Collection {
	return m.Database.Collection("telemetry_events")
}
