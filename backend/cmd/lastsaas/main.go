package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"

	"lastsaas/internal/auth"
	"lastsaas/internal/config"
	"lastsaas/internal/configstore"
	"lastsaas/internal/db"
	"lastsaas/internal/models"
	"lastsaas/internal/validation"
	"lastsaas/internal/version"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"golang.org/x/term"
)

func main() {
	config.LoadEnvFile()
	version.Load()
	parseGlobalFlags()

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "setup":
		cmdSetup()
	case "start":
		cmdStart()
	case "stop":
		cmdStop()
	case "restart":
		cmdRestart()
	case "change-password":
		cmdChangePassword()
	case "send-message":
		cmdSendMessage()
	case "transfer-root-owner":
		cmdTransferRootOwner()
	case "config":
		cmdConfig()
	case "version":
		cmdVersion()
	case "status":
		cmdStatus()
	case "logs":
		cmdLogs()
	case "users":
		cmdUsers()
	case "tenants":
		cmdTenants()
	case "health":
		cmdHealth()
	case "stats":
		cmdStats()
	case "doctor":
		cmdDoctor()
	case "financial":
		cmdFinancial()
	case "db":
		cmdDB()
	case "mcp":
		cmdMCP()
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`mcplens - MCPLens system administration tool

Usage:
  mcplens <command> [flags] [--json]

Commands:
  setup                Initialize the system (create root tenant + owner account)
  start                Start backend and/or frontend servers
  stop                 Stop running servers
  restart              Restart servers (stop + start)
  status               Check system and database status
  version              Show software and database version

  logs                 View and tail system logs
  users                Manage users (list, get, suspend, activate, revoke-sessions)
  tenants              Manage tenants (list, get)
  health               Show system health and node status
  stats                Dashboard summary statistics
  financial            Financial data and reporting (summary, transactions, metrics)
  doctor               Run comprehensive system diagnostics
  db                   Database statistics (stats)
  mcp                  Start MCP server (stdio) for AI assistant integration

  change-password      Change a user's password
  send-message         Send a message to a user
  transfer-root-owner  Transfer root tenant ownership
  config               Manage configuration variables (list, get, set, reset)
  help                 Show this help message

Global flags:
  --json               Output in JSON format (works with most commands)

Examples:
  mcplens setup
  mcplens start --backend
  mcplens stop

  mcplens logs --severity critical,high --tail 100
  mcplens logs --follow --category security
  mcplens logs --from 24h
  mcplens users list --json
  mcplens users get --email admin@example.com
  mcplens users suspend --email bad@example.com
  mcplens tenants list
  mcplens tenants get root

  mcplens health
  mcplens stats --json
  mcplens financial summary
  mcplens financial transactions --type subscription --limit 50
  mcplens financial metrics --days 30
  mcplens doctor
  mcplens db stats

  mcplens config list
  mcplens config get log.min_level
  mcplens config set log.min_level high
  mcplens config reset log.min_level
  mcplens change-password --email admin@example.com

  MCPLENS_URL=http://localhost:3000 MCPLENS_API_KEY=lsk_xxx mcplens mcp`)
}

// connectDB loads config and connects to MongoDB, printing helpful guidance on failure.
func connectDB() (*db.MongoDB, *config.Config, func()) {
	env := config.GetEnv()
	cfg, err := config.Load(env)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n\n", err)
		printConfigHelp(env)
		os.Exit(1)
	}

	database, err := db.NewMongoDB(cfg.Database.URI, cfg.Database.Name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error connecting to MongoDB: %v\n\n", err)
		printMongoHelp(env)
		os.Exit(1)
	}

	cleanup := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		database.Close(ctx)
	}

	return database, cfg, cleanup
}

func printConfigHelp(env string) {
	fmt.Fprintf(os.Stderr, `Configuration file not found or invalid.

Make sure you have a config file at: config/%s.yaml
You can copy from the example:
  cp config/%s.example.yaml config/%s.yaml

Then edit config/%s.yaml with your settings.
`, env, env, env, env)
}

func printMongoHelp(env string) {
	fmt.Fprintf(os.Stderr, `Could not connect to MongoDB. Check your database.uri in config/%s.yaml.

--- Using MongoDB Atlas (recommended for getting started) ---

1. Go to https://www.mongodb.com/atlas and create a free account
2. Create a free M0 cluster (the free tier is sufficient)
3. Set up database access:
   - Go to "Database Access" in the left sidebar
   - Click "Add New Database User"
   - Choose password authentication and create a username/password
4. Set up network access:
   - Go to "Network Access" in the left sidebar
   - Click "Add IP Address"
   - Add your current IP or use 0.0.0.0/0 for development
5. Get your connection string:
   - Go to "Database" > click "Connect" on your cluster
   - Choose "Drivers" > select "Go"
   - Copy the connection string (looks like: mongodb+srv://user:pass@cluster.xxxxx.mongodb.net/)
6. Paste the connection string in config/%s.yaml under database.uri

--- Using Local MongoDB ---

Install MongoDB Community Edition and set database.uri to:
  mongodb://localhost:27017
`, env, env)
}

// --- setup command ---

func cmdSetup() {
	database, cfg, cleanup := connectDB()
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Check if already initialized
	var sys models.SystemConfig
	err := database.SystemConfig().FindOne(ctx, bson.M{}).Decode(&sys)
	if err == nil && sys.Initialized {
		fmt.Println("System is already initialized.")
		fmt.Println()
		fmt.Printf("  Database: %s\n", cfg.Database.Name)

		// Look up root tenant owner
		var rootTenant models.Tenant
		if err := database.Tenants().FindOne(ctx, bson.M{"isRoot": true}).Decode(&rootTenant); err == nil {
			var membership models.TenantMembership
			if err := database.TenantMemberships().FindOne(ctx, bson.M{"tenantId": rootTenant.ID, "role": "owner"}).Decode(&membership); err == nil {
				var owner models.User
				if err := database.Users().FindOne(ctx, bson.M{"_id": membership.UserID}).Decode(&owner); err == nil {
					fmt.Printf("  Owner:    %s (%s)\n", owner.DisplayName, owner.Email)
				}
			}
		}

		fmt.Println()
		fmt.Println("If you need to change a user's password, use: mcplens change-password --email <email>")
		os.Exit(0)
	}

	fmt.Println("=== MCPLens Initial Setup ===")
	fmt.Println()

	reader := bufio.NewReader(os.Stdin)

	orgName := prompt(reader, "Organization name")
	displayName := prompt(reader, "Your name")
	email := prompt(reader, "Email address")
	email = strings.TrimSpace(strings.ToLower(email))

	if orgName == "" || displayName == "" || email == "" {
		fmt.Fprintln(os.Stderr, "All fields are required.")
		os.Exit(1)
	}

	passwordService := auth.NewPasswordService()

	password := promptPassword("Password")
	confirm := promptPassword("Confirm password")

	if password != confirm {
		fmt.Fprintln(os.Stderr, "Passwords do not match.")
		os.Exit(1)
	}

	if err := passwordService.ValidatePasswordStrength(password); err != nil {
		fmt.Fprintf(os.Stderr, "Password too weak: %v\n", err)
		fmt.Fprintln(os.Stderr, "Requirements: 10+ characters, uppercase, lowercase, number, special character")
		os.Exit(1)
	}

	passwordHash, err := passwordService.HashPassword(password)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to hash password: %v\n", err)
		os.Exit(1)
	}

	now := time.Now()

	// Create root tenant
	tenant := models.Tenant{
		ID:        primitive.NewObjectID(),
		Name:      strings.TrimSpace(orgName),
		Slug:      "root",
		IsRoot:    true,
		IsActive:  true,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := validation.Validate(&tenant); err != nil {
		fmt.Fprintf(os.Stderr, "Tenant validation failed: %v\n", err)
		os.Exit(1)
	}
	if _, err := database.Tenants().InsertOne(ctx, tenant); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create root tenant: %v\n", err)
		os.Exit(1)
	}

	// Create owner user
	user := models.User{
		ID:            primitive.NewObjectID(),
		Email:         email,
		DisplayName:   strings.TrimSpace(displayName),
		PasswordHash:  passwordHash,
		AuthMethods:   []models.AuthMethod{models.AuthMethodPassword},
		EmailVerified: true,
		IsActive:      true,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := validation.Validate(&user); err != nil {
		database.Tenants().DeleteOne(ctx, bson.M{"_id": tenant.ID})
		fmt.Fprintf(os.Stderr, "User validation failed: %v\n", err)
		os.Exit(1)
	}
	if _, err := database.Users().InsertOne(ctx, user); err != nil {
		database.Tenants().DeleteOne(ctx, bson.M{"_id": tenant.ID})
		fmt.Fprintf(os.Stderr, "Failed to create user: %v\n", err)
		os.Exit(1)
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
		database.Users().DeleteOne(ctx, bson.M{"_id": user.ID})
		database.Tenants().DeleteOne(ctx, bson.M{"_id": tenant.ID})
		fmt.Fprintf(os.Stderr, "Membership validation failed: %v\n", err)
		os.Exit(1)
	}
	if _, err := database.TenantMemberships().InsertOne(ctx, membership); err != nil {
		database.Users().DeleteOne(ctx, bson.M{"_id": user.ID})
		database.Tenants().DeleteOne(ctx, bson.M{"_id": tenant.ID})
		fmt.Fprintf(os.Stderr, "Failed to create membership: %v\n", err)
		os.Exit(1)
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
		database.TenantMemberships().DeleteOne(ctx, bson.M{"_id": membership.ID})
		database.Users().DeleteOne(ctx, bson.M{"_id": user.ID})
		database.Tenants().DeleteOne(ctx, bson.M{"_id": tenant.ID})
		fmt.Fprintf(os.Stderr, "Failed to mark system as initialized: %v\n", err)
		os.Exit(1)
	}

	// Send welcome message to the new owner
	welcomeMsg := models.Message{
		ID:        primitive.NewObjectID(),
		UserID:    user.ID,
		Subject:   "Welcome to MCPLens v" + version.Current,
		Body:      "Your system has been initialized. Welcome to MCPLens!",
		IsSystem:  true,
		Read:      false,
		CreatedAt: now,
	}
	database.Messages().InsertOne(ctx, welcomeMsg)

	fmt.Println()
	fmt.Println("System initialized successfully!")
	fmt.Println()
	fmt.Printf("  Organization: %s\n", tenant.Name)
	fmt.Printf("  Owner:        %s (%s)\n", user.DisplayName, user.Email)
	fmt.Printf("  Database:     %s\n", cfg.Database.Name)
	fmt.Println()
	fmt.Println("You can now start the server and log in.")
}

// --- change-password command ---

func cmdChangePassword() {
	fs := flag.NewFlagSet("change-password", flag.ExitOnError)
	emailFlag := fs.String("email", "", "User email address (required)")
	fs.Parse(os.Args[2:])

	if *emailFlag == "" {
		fmt.Fprintln(os.Stderr, "Usage: mcplens change-password --email <email>")
		os.Exit(1)
	}

	database, _, cleanup := connectDB()
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	email := strings.TrimSpace(strings.ToLower(*emailFlag))

	var user models.User
	err := database.Users().FindOne(ctx, bson.M{"email": email}).Decode(&user)
	if err != nil {
		fmt.Fprintf(os.Stderr, "User not found: %s\n", email)
		os.Exit(1)
	}

	fmt.Printf("Changing password for: %s (%s)\n", user.DisplayName, user.Email)

	passwordService := auth.NewPasswordService()

	password := promptPassword("New password")
	confirm := promptPassword("Confirm new password")

	if password != confirm {
		fmt.Fprintln(os.Stderr, "Passwords do not match.")
		os.Exit(1)
	}

	if err := passwordService.ValidatePasswordStrength(password); err != nil {
		fmt.Fprintf(os.Stderr, "Password too weak: %v\n", err)
		fmt.Fprintln(os.Stderr, "Requirements: 10+ characters, uppercase, lowercase, number, special character")
		os.Exit(1)
	}

	passwordHash, err := passwordService.HashPassword(password)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to hash password: %v\n", err)
		os.Exit(1)
	}

	_, err = database.Users().UpdateOne(ctx,
		bson.M{"_id": user.ID},
		bson.M{"$set": bson.M{
			"passwordHash": passwordHash,
			"updatedAt":    time.Now(),
		}},
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to update password: %v\n", err)
		os.Exit(1)
	}

	// Revoke all refresh tokens for this user
	database.RefreshTokens().DeleteMany(ctx, bson.M{"userId": user.ID})

	fmt.Println("Password updated successfully.")
}

// --- send-message command ---

func cmdSendMessage() {
	fs := flag.NewFlagSet("send-message", flag.ExitOnError)
	emailFlag := fs.String("email", "", "Recipient email address (required)")
	messageFlag := fs.String("message", "", "Message body (required)")
	subjectFlag := fs.String("subject", "System Message", "Message subject")
	fs.Parse(os.Args[2:])

	if *emailFlag == "" || *messageFlag == "" {
		fmt.Fprintln(os.Stderr, "Usage: mcplens send-message --email <email> --message \"Your message here\"")
		fmt.Fprintln(os.Stderr, "  --subject \"Optional subject\" (default: \"System Message\")")
		os.Exit(1)
	}

	// Validate inputs
	subject := strings.TrimSpace(*subjectFlag)
	body := strings.TrimSpace(*messageFlag)

	if !utf8.ValidString(subject) || !utf8.ValidString(body) {
		fmt.Fprintln(os.Stderr, "Invalid characters in subject or message.")
		os.Exit(1)
	}
	if len(subject) > 200 {
		fmt.Fprintln(os.Stderr, "Subject must be 200 characters or less.")
		os.Exit(1)
	}
	if len(body) > 5000 {
		fmt.Fprintln(os.Stderr, "Message must be 5000 characters or less.")
		os.Exit(1)
	}
	if body == "" {
		fmt.Fprintln(os.Stderr, "Message body cannot be empty.")
		os.Exit(1)
	}

	database, _, cleanup := connectDB()
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	email := strings.TrimSpace(strings.ToLower(*emailFlag))

	var user models.User
	err := database.Users().FindOne(ctx, bson.M{"email": email}).Decode(&user)
	if err != nil {
		fmt.Fprintf(os.Stderr, "User not found: %s\n", email)
		os.Exit(1)
	}

	msg := models.Message{
		ID:        primitive.NewObjectID(),
		UserID:    user.ID,
		Subject:   subject,
		Body:      body,
		IsSystem:  true,
		Read:      false,
		CreatedAt: time.Now(),
	}

	if _, err := database.Messages().InsertOne(ctx, msg); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to send message: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Message sent to %s (%s).\n", user.DisplayName, user.Email)
}

// --- config command ---

func cmdConfig() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, `Usage: mcplens config <subcommand>

Subcommands:
  list              List all configuration variables
  get <name>        Show details of a configuration variable
  set <name> <val>  Update a configuration variable's value
  reset <name>      Reset a variable to its default value`)
		os.Exit(1)
	}

	switch os.Args[2] {
	case "list":
		cmdConfigList()
	case "get":
		if len(os.Args) < 4 {
			fmt.Fprintln(os.Stderr, "Usage: mcplens config get <name>")
			os.Exit(1)
		}
		cmdConfigGet(os.Args[3])
	case "set":
		if len(os.Args) < 5 {
			fmt.Fprintln(os.Stderr, "Usage: mcplens config set <name> <value>")
			os.Exit(1)
		}
		cmdConfigSet(os.Args[3], strings.Join(os.Args[4:], " "))
	case "reset":
		if len(os.Args) < 4 {
			fmt.Fprintln(os.Stderr, "Usage: mcplens config reset <name>")
			os.Exit(1)
		}
		cmdConfigReset(os.Args[3])
	default:
		fmt.Fprintf(os.Stderr, "Unknown config subcommand: %s\n", os.Args[2])
		os.Exit(1)
	}
}

func cmdConfigList() {
	database, _, cleanup := connectDB()
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cursor, err := database.ConfigVars().Find(ctx, bson.M{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to query config vars: %v\n", err)
		os.Exit(1)
	}
	defer cursor.Close(ctx)

	var vars []models.ConfigVar
	if err := cursor.All(ctx, &vars); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to read config vars: %v\n", err)
		os.Exit(1)
	}

	if jsonOutput {
		printJSON(vars)
		return
	}

	if len(vars) == 0 {
		fmt.Println("No configuration variables found.")
		return
	}

	fmt.Printf("%-40s %-10s %-8s %s\n", "NAME", "TYPE", "SYSTEM", "VALUE")
	fmt.Printf("%-40s %-10s %-8s %s\n", "----", "----", "------", "-----")
	for _, v := range vars {
		value := v.Value
		if len(value) > 60 {
			value = value[:57] + "..."
		}
		// Replace newlines for table display
		value = strings.ReplaceAll(value, "\n", "\\n")
		sys := ""
		if v.IsSystem {
			sys = "yes"
		}
		fmt.Printf("%-40s %-10s %-8s %s\n", v.Name, v.Type, sys, value)
	}
}

func cmdConfigGet(name string) {
	database, _, cleanup := connectDB()
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var v models.ConfigVar
	err := database.ConfigVars().FindOne(ctx, bson.M{"name": name}).Decode(&v)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Config variable not found: %s\n", name)
		os.Exit(1)
	}

	if jsonOutput {
		printJSON(v)
		return
	}

	fmt.Printf("Name:        %s\n", v.Name)
	fmt.Printf("Description: %s\n", v.Description)
	fmt.Printf("Type:        %s\n", v.Type)
	fmt.Printf("System:      %v\n", v.IsSystem)
	if v.Options != "" {
		fmt.Printf("Options:     %s\n", v.Options)
	}
	fmt.Printf("Updated:     %s\n", v.UpdatedAt.Format(time.RFC3339))
	fmt.Printf("Value:\n%s\n", v.Value)
}

func cmdConfigSet(name, value string) {
	database, _, cleanup := connectDB()
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var v models.ConfigVar
	err := database.ConfigVars().FindOne(ctx, bson.M{"name": name}).Decode(&v)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Config variable not found: %s\n", name)
		os.Exit(1)
	}

	if err := configstore.ValidateValue(v.Type, value, v.Options); err != nil {
		fmt.Fprintf(os.Stderr, "Invalid value: %v\n", err)
		os.Exit(1)
	}

	_, err = database.ConfigVars().UpdateOne(ctx,
		bson.M{"name": name},
		bson.M{"$set": bson.M{"value": value, "updatedAt": time.Now()}},
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to update config variable: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Updated '%s' successfully.\n", name)
	fmt.Println("Note: The running server will pick up this change on next cache reload or restart.")
}

func cmdConfigReset(name string) {
	// Find the default value from SystemDefaults
	var defaultVal string
	found := false
	for _, def := range configstore.SystemDefaults {
		if def.Name == name {
			defaultVal = def.Value
			found = true
			break
		}
	}
	if !found {
		fmt.Fprintf(os.Stderr, "No system default found for: %s\n", name)
		fmt.Fprintln(os.Stderr, "Only system-defined variables can be reset to defaults.")
		os.Exit(1)
	}

	database, _, cleanup := connectDB()
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var v models.ConfigVar
	err := database.ConfigVars().FindOne(ctx, bson.M{"name": name}).Decode(&v)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Config variable not found in database: %s\n", name)
		os.Exit(1)
	}

	if v.Value == defaultVal {
		fmt.Printf("'%s' is already at its default value.\n", name)
		return
	}

	_, err = database.ConfigVars().UpdateOne(ctx,
		bson.M{"name": name},
		bson.M{"$set": bson.M{"value": defaultVal, "updatedAt": time.Now()}},
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to reset config variable: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Reset '%s' to default value.\n", name)
	fmt.Println("Note: The running server will pick up this change on next cache reload or restart.")
}

// --- transfer-root-owner command ---

func cmdTransferRootOwner() {
	fs := flag.NewFlagSet("transfer-root-owner", flag.ExitOnError)
	emailFlag := fs.String("email", "", "Email of the new root tenant owner (required)")
	fs.Parse(os.Args[2:])

	if *emailFlag == "" {
		fmt.Fprintln(os.Stderr, "Usage: mcplens transfer-root-owner --email <new-owner-email>")
		os.Exit(1)
	}

	database, _, cleanup := connectDB()
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Find root tenant
	var rootTenant models.Tenant
	if err := database.Tenants().FindOne(ctx, bson.M{"isRoot": true}).Decode(&rootTenant); err != nil {
		fmt.Fprintln(os.Stderr, "Root tenant not found. Is the system initialized?")
		os.Exit(1)
	}

	// Find new owner by email
	newEmail := strings.TrimSpace(strings.ToLower(*emailFlag))
	var newOwner models.User
	if err := database.Users().FindOne(ctx, bson.M{"email": newEmail}).Decode(&newOwner); err != nil {
		fmt.Fprintf(os.Stderr, "User not found: %s\n", newEmail)
		os.Exit(1)
	}

	// Verify new owner is a member of root tenant
	var newMembership models.TenantMembership
	if err := database.TenantMemberships().FindOne(ctx, bson.M{
		"userId":   newOwner.ID,
		"tenantId": rootTenant.ID,
	}).Decode(&newMembership); err != nil {
		fmt.Fprintf(os.Stderr, "User %s is not a member of the root tenant.\n", newEmail)
		fmt.Fprintln(os.Stderr, "They must be added to the root tenant before ownership can be transferred.")
		os.Exit(1)
	}

	if newMembership.Role == models.RoleOwner {
		fmt.Printf("User %s is already the owner of the root tenant.\n", newEmail)
		os.Exit(0)
	}

	// Find current owner
	var currentOwnerMembership models.TenantMembership
	if err := database.TenantMemberships().FindOne(ctx, bson.M{
		"tenantId": rootTenant.ID,
		"role":     "owner",
	}).Decode(&currentOwnerMembership); err != nil {
		fmt.Fprintln(os.Stderr, "Could not find current root tenant owner.")
		os.Exit(1)
	}

	var currentOwner models.User
	if err := database.Users().FindOne(ctx, bson.M{"_id": currentOwnerMembership.UserID}).Decode(&currentOwner); err != nil {
		fmt.Fprintln(os.Stderr, "Could not find current owner user record.")
		os.Exit(1)
	}

	// Interactive confirmation
	fmt.Println("=== Transfer Root Tenant Ownership ===")
	fmt.Println()
	fmt.Printf("  Current owner: %s (%s)\n", currentOwner.DisplayName, currentOwner.Email)
	fmt.Printf("  New owner:     %s (%s)\n", newOwner.DisplayName, newOwner.Email)
	fmt.Println()
	fmt.Println("The current owner will be demoted to admin.")
	fmt.Println("This is a critical security operation.")
	fmt.Println()
	fmt.Print("Type 'yes' to confirm: ")

	reader := bufio.NewReader(os.Stdin)
	answer, _ := reader.ReadString('\n')
	answer = strings.TrimSpace(answer)

	if answer != "yes" {
		fmt.Println("Aborted.")
		os.Exit(0)
	}

	now := time.Now()

	// Demote current owner to admin
	_, err := database.TenantMemberships().UpdateOne(ctx,
		bson.M{"_id": currentOwnerMembership.ID},
		bson.M{"$set": bson.M{"role": "admin", "updatedAt": now}},
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to demote current owner: %v\n", err)
		os.Exit(1)
	}

	// Promote new owner
	_, err = database.TenantMemberships().UpdateOne(ctx,
		bson.M{"_id": newMembership.ID},
		bson.M{"$set": bson.M{"role": "owner", "updatedAt": now}},
	)
	if err != nil {
		// Try to rollback
		database.TenantMemberships().UpdateOne(ctx,
			bson.M{"_id": currentOwnerMembership.ID},
			bson.M{"$set": bson.M{"role": "owner", "updatedAt": now}},
		)
		fmt.Fprintf(os.Stderr, "Failed to promote new owner: %v\n", err)
		os.Exit(1)
	}

	// Write system log entry directly
	logEntry := models.SystemLog{
		ID:        primitive.NewObjectID(),
		Severity:  models.LogCritical,
		Message:   fmt.Sprintf("Root tenant ownership transferred from %s (%s) to %s (%s) via CLI", currentOwner.DisplayName, currentOwner.Email, newOwner.DisplayName, newOwner.Email),
		CreatedAt: now,
	}
	database.SystemLogs().InsertOne(ctx, logEntry)

	fmt.Println()
	fmt.Printf("Root tenant ownership transferred to %s (%s).\n", newOwner.DisplayName, newOwner.Email)
	fmt.Printf("Previous owner %s has been demoted to admin.\n", currentOwner.Email)
}

// --- version command ---

func cmdVersion() {
	fmt.Printf("MCPLens v%s (binary)\n", version.Current)

	database, cfg, cleanup := connectDB()
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	fmt.Printf("Database:    %s\n", cfg.Database.Name)

	var sys models.SystemConfig
	err := database.SystemConfig().FindOne(ctx, bson.M{}).Decode(&sys)
	if err != nil || !sys.Initialized {
		fmt.Println("DB version:  (not initialized)")
		return
	}
	fmt.Printf("DB version:  %s\n", sys.Version)
	if sys.Version != version.Current {
		fmt.Println()
		fmt.Println("Note: Binary and database versions differ. Restart the server to trigger migration.")
	}
}

// --- status command ---

func cmdStatus() {
	env := config.GetEnv()
	fmt.Printf("Environment: %s\n", env)

	cfg, err := config.Load(env)
	if err != nil {
		fmt.Printf("Config:      ERROR - %v\n", err)
		return
	}
	fmt.Printf("Config:      OK (config/%s.yaml)\n", env)

	database, err := db.NewMongoDB(cfg.Database.URI, cfg.Database.Name)
	if err != nil {
		fmt.Printf("MongoDB:     ERROR - %v\n", err)
		return
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		database.Close(ctx)
	}()
	fmt.Printf("MongoDB:     Connected (%s)\n", cfg.Database.Name)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var sys models.SystemConfig
	err = database.SystemConfig().FindOne(ctx, bson.M{}).Decode(&sys)
	if err != nil || !sys.Initialized {
		fmt.Println("Initialized: No")
		fmt.Println()
		fmt.Println("Run 'mcplens setup' to initialize the system.")
		return
	}
	fmt.Printf("Initialized: Yes (v%s)\n", sys.Version)
	if sys.InitializedAt != nil {
		fmt.Printf("  Set up:    %s\n", sys.InitializedAt.Format(time.RFC3339))
	}

	userCount, _ := database.Users().CountDocuments(ctx, bson.M{})
	tenantCount, _ := database.Tenants().CountDocuments(ctx, bson.M{})
	fmt.Printf("Users:       %d\n", userCount)
	fmt.Printf("Tenants:     %d\n", tenantCount)
}

// --- helpers ---

func prompt(reader *bufio.Reader, label string) string {
	fmt.Printf("%s: ", label)
	text, _ := reader.ReadString('\n')
	return strings.TrimSpace(text)
}

func promptPassword(label string) string {
	fmt.Printf("%s: ", label)
	password, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading password: %v\n", err)
		os.Exit(1)
	}
	return string(password)
}
