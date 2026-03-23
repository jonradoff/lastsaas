package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"lastsaas/internal/config"
	"lastsaas/internal/db"
	"lastsaas/internal/models"
	"lastsaas/internal/version"

	"go.mongodb.org/mongo-driver/bson"
)

func cmdDoctor() {
	fmt.Printf("%s\n\n", bold("System Diagnostics"))

	env := config.GetEnv()
	passes := 0
	warnings := 0
	failures := 0

	check := func(name string, ok bool, detail string) {
		if ok {
			passes++
			fmt.Printf("  %s  %s\n", clr(cGreen, "PASS"), name)
		} else {
			failures++
			fmt.Printf("  %s  %s — %s\n", clr(cRed, "FAIL"), name, detail)
		}
	}
	warn := func(name, detail string) {
		warnings++
		fmt.Printf("  %s  %s — %s\n", clr(cYellow, "WARN"), name, detail)
	}

	// 1. Config file
	cfg, err := config.Load(env)
	check("Config file ("+env+".yaml)", err == nil, fmt.Sprintf("%v", err))
	if err != nil {
		fmt.Printf("\n  Results: %d passed, %d warnings, %d failed\n", passes, warnings, failures)
		os.Exit(1)
	}

	// 2. MongoDB connection
	database, err := db.NewMongoDB(cfg.Database.URI, cfg.Database.Name)
	check("MongoDB connection", err == nil, fmt.Sprintf("%v", err))
	if err != nil {
		fmt.Printf("\n  Results: %d passed, %d warnings, %d failed\n", passes, warnings, failures)
		os.Exit(1)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		database.Close(ctx)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// 3. System initialized
	var sys models.SystemConfig
	sysErr := database.SystemConfig().FindOne(ctx, bson.M{}).Decode(&sys)
	check("System initialized", sysErr == nil && sys.Initialized, "Run 'mcplens setup' to initialize")

	// 4. Version match
	if sys.Initialized {
		if sys.Version != version.Current {
			warn("Version mismatch", fmt.Sprintf("binary v%s, database v%s — restart server to migrate", version.Current, sys.Version))
		} else {
			check("Version match (v"+version.Current+")", true, "")
		}
	}

	// 5. JWT secrets configured
	check("JWT access secret", cfg.JWT.AccessSecret != "", "jwt.access_secret not set in config")
	check("JWT refresh secret", cfg.JWT.RefreshSecret != "", "jwt.refresh_secret not set in config")

	// 6. Integration checks from config vars
	checkConfigIntegration(ctx, database, "Stripe", "stripe.secret_key", cfg, &passes, &warnings, &failures, check, warn)
	checkConfigIntegration(ctx, database, "Email (Resend)", "email.resend_api_key", cfg, &passes, &warnings, &failures, check, warn)

	// 7. OAuth providers
	if cfg.OAuth.GoogleClientID != "" {
		check("Google OAuth", cfg.OAuth.GoogleClientSecret != "", "google_client_id set but google_client_secret missing")
	} else {
		warn("Google OAuth", "not configured (optional)")
	}
	if cfg.OAuth.GitHubClientID != "" {
		check("GitHub OAuth", cfg.OAuth.GitHubClientSecret != "", "github_client_id set but github_client_secret missing")
	} else {
		warn("GitHub OAuth", "not configured (optional)")
	}

	// 8. Root tenant has an owner
	if sys.Initialized {
		var rootTenant models.Tenant
		if err := database.Tenants().FindOne(ctx, bson.M{"isRoot": true}).Decode(&rootTenant); err == nil {
			ownerCount, _ := database.TenantMemberships().CountDocuments(ctx, bson.M{
				"tenantId": rootTenant.ID,
				"role":     "owner",
			})
			check("Root tenant has owner", ownerCount > 0, "no owner found for root tenant")
		}
	}

	// 9. Nodes reporting
	nodeCount, _ := database.SystemNodes().CountDocuments(ctx, bson.M{
		"lastSeen": bson.M{"$gte": time.Now().Add(-2 * time.Minute)},
	})
	if nodeCount > 0 {
		check(fmt.Sprintf("Server nodes (%d active)", nodeCount), true, "")
	} else {
		warn("Server nodes", "no active nodes — is the server running?")
	}

	fmt.Printf("\n  Results: %s passed, %s warnings, %s failed\n",
		clr(cGreen, fmt.Sprintf("%d", passes)),
		clr(cYellow, fmt.Sprintf("%d", warnings)),
		clr(cRed, fmt.Sprintf("%d", failures)),
	)

	if failures > 0 {
		os.Exit(1)
	}
}

func checkConfigIntegration(ctx context.Context, database *db.MongoDB, name, cfgField string, cfg *config.Config, passes, warnings, failures *int, check func(string, bool, string), warn func(string, string)) {
	// Check if there's a config var for this
	switch cfgField {
	case "stripe.secret_key":
		if cfg.Stripe.SecretKey != "" {
			check(name, true, "")
		} else {
			warn(name, "not configured (billing disabled)")
		}
	case "email.resend_api_key":
		if cfg.Email.ResendAPIKey != "" {
			check(name, true, "")
		} else {
			warn(name, "not configured (emails disabled)")
		}
	}
}
