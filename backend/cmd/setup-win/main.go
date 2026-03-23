package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"lastsaas/internal/auth"
	"lastsaas/internal/config"
	"lastsaas/internal/db"
	"lastsaas/internal/models"
	"lastsaas/internal/validation"
	"lastsaas/internal/version"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func main() {
	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "dev"
	}
	cfg, err := config.Load(env)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	database, err := db.NewMongoDB(cfg.Database.URI, cfg.Database.Name)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer database.Close(context.Background())

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Check if already initialized
	var sys models.SystemConfig
	findErr := database.SystemConfig().FindOne(ctx, bson.M{}).Decode(&sys)
	if findErr == nil && sys.Initialized {
		fmt.Println("System is already initialized.")
		os.Exit(0)
	}

	orgName := "MCPLens"
	displayName := "Admin"
	email := "admin@mcplens.dev"
	password := "McpLens2026!"

	passwordService := auth.NewPasswordService()
	passwordHash, err := passwordService.HashPassword(password)
	if err != nil {
		log.Fatalf("Failed to hash password: %v", err)
	}

	now := time.Now()

	// Create root tenant
	tenant := models.Tenant{
		ID:        primitive.NewObjectID(),
		Name:      orgName,
		Slug:      "root",
		IsRoot:    true,
		IsActive:  true,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := validation.Validate(&tenant); err != nil {
		log.Fatalf("Tenant validation failed: %v", err)
	}
	if _, err := database.Tenants().InsertOne(ctx, tenant); err != nil {
		log.Fatalf("Failed to create root tenant: %v", err)
	}

	// Create owner user
	user := models.User{
		ID:            primitive.NewObjectID(),
		Email:         email,
		DisplayName:   displayName,
		PasswordHash:  passwordHash,
		AuthMethods:   []models.AuthMethod{models.AuthMethodPassword},
		EmailVerified: true,
		IsActive:      true,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := validation.Validate(&user); err != nil {
		log.Fatalf("User validation failed: %v", err)
	}
	if _, err := database.Users().InsertOne(ctx, user); err != nil {
		log.Fatalf("Failed to create user: %v", err)
	}

	// Create owner membership
	membership := models.TenantMembership{
		ID:        primitive.NewObjectID(),
		UserID:    user.ID,
		TenantID:  tenant.ID,
		Role:      models.RoleOwner,
		JoinedAt:  now,
		UpdatedAt: now,
	}
	if err := validation.Validate(&membership); err != nil {
		log.Fatalf("Membership validation failed: %v", err)
	}
	if _, err := database.TenantMemberships().InsertOne(ctx, membership); err != nil {
		log.Fatalf("Failed to create membership: %v", err)
	}

	// Mark system as initialized
	sysConfig := models.SystemConfig{
		ID:            primitive.NewObjectID(),
		Initialized:   true,
		InitializedAt: &now,
		InitializedBy: &user.ID,
		Version:       version.Current,
	}
	if _, err := database.SystemConfig().InsertOne(ctx, sysConfig); err != nil {
		log.Fatalf("Failed to mark system as initialized: %v", err)
	}

	fmt.Println("=== MCPLens Setup Complete ===")
	fmt.Printf("  Organization: %s\n", orgName)
	fmt.Printf("  Admin email:  %s\n", email)
	fmt.Printf("  Admin password: %s\n", password)
	fmt.Println()
	fmt.Println("You can now log in at http://localhost:4280")
}
