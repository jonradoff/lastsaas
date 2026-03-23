package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"lastsaas/internal/db"
	"lastsaas/internal/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func cmdUsers() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, `Usage: mcplens users <subcommand>

Subcommands:
  list                          List all users
  get --email <email>           Show user details
  suspend --email <email>       Suspend a user account
  activate --email <email>      Reactivate a suspended account
  revoke-sessions --email <email>  Revoke all sessions for a user`)
		os.Exit(1)
	}

	switch os.Args[2] {
	case "list":
		cmdUsersList()
	case "get":
		cmdUsersGet()
	case "suspend":
		cmdUsersSetActive(false)
	case "activate":
		cmdUsersSetActive(true)
	case "revoke-sessions":
		cmdUsersRevokeSessions()
	default:
		fmt.Fprintf(os.Stderr, "Unknown users subcommand: %s\n", os.Args[2])
		os.Exit(1)
	}
}

func cmdUsersList() {
	fs := flag.NewFlagSet("users list", flag.ExitOnError)
	limit := fs.Int("limit", 50, "Max users to show")
	inactive := fs.Bool("inactive", false, "Show only inactive/suspended users")
	fs.Parse(os.Args[3:])

	database, _, cleanup := connectDB()
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	filter := bson.M{}
	if *inactive {
		filter["isActive"] = false
	}

	opts := options.Find().
		SetSort(bson.D{{Key: "createdAt", Value: -1}}).
		SetLimit(int64(*limit))

	cursor, err := database.Users().Find(ctx, filter, opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to query users: %v\n", err)
		os.Exit(1)
	}
	defer cursor.Close(ctx)

	var users []models.User
	if err := cursor.All(ctx, &users); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to read users: %v\n", err)
		os.Exit(1)
	}

	if jsonOutput {
		type userRow struct {
			ID          string   `json:"id"`
			Email       string   `json:"email"`
			DisplayName string   `json:"displayName"`
			IsActive    bool     `json:"isActive"`
			MFA         bool     `json:"mfa"`
			AuthMethods []string `json:"authMethods"`
			CreatedAt   string   `json:"createdAt"`
			LastLogin   string   `json:"lastLogin,omitempty"`
		}
		rows := make([]userRow, 0, len(users))
		for _, u := range users {
			methods := make([]string, len(u.AuthMethods))
			for i, m := range u.AuthMethods {
				methods[i] = string(m)
			}
			r := userRow{
				ID:          u.ID.Hex(),
				Email:       u.Email,
				DisplayName: u.DisplayName,
				IsActive:    u.IsActive,
				MFA:         u.TOTPEnabled,
				AuthMethods: methods,
				CreatedAt:   u.CreatedAt.Format(time.RFC3339),
			}
			if u.LastLoginAt != nil {
				r.LastLogin = u.LastLoginAt.Format(time.RFC3339)
			}
			rows = append(rows, r)
		}
		printJSON(rows)
		return
	}

	if len(users) == 0 {
		fmt.Println("No users found.")
		return
	}

	fmt.Printf("%-36s %-30s %-20s %-8s %-5s %s\n",
		bold("ID"), bold("EMAIL"), bold("NAME"), bold("STATUS"), bold("MFA"), bold("CREATED"))
	fmt.Printf("%-36s %-30s %-20s %-8s %-5s %s\n",
		"----", "-----", "----", "------", "---", "-------")

	for _, u := range users {
		status := clr(cGreen, "active")
		if !u.IsActive {
			status = clr(cRed, "suspended")
		}
		mfa := ""
		if u.TOTPEnabled {
			mfa = clr(cGreen, "yes")
		}
		fmt.Printf("%-36s %-30s %-20s %-8s %-5s %s\n",
			u.ID.Hex(),
			truncate(u.Email, 30),
			truncate(u.DisplayName, 20),
			status,
			mfa,
			timeAgo(u.CreatedAt),
		)
	}
	fmt.Printf("\n%d users shown\n", len(users))
}

func cmdUsersGet() {
	fs := flag.NewFlagSet("users get", flag.ExitOnError)
	email := fs.String("email", "", "User email address (required)")
	fs.Parse(os.Args[3:])

	if *email == "" {
		fmt.Fprintln(os.Stderr, "Usage: mcplens users get --email <email>")
		os.Exit(1)
	}

	database, _, cleanup := connectDB()
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	user, memberships := lookupUserWithMemberships(ctx, database, *email)

	// Resolve tenant names
	tenantIDs := make([]primitive.ObjectID, 0, len(memberships))
	for _, m := range memberships {
		tenantIDs = append(tenantIDs, m.TenantID)
	}
	tenantNames := resolveTenantNames(ctx, database, tenantIDs)

	if jsonOutput {
		type membershipInfo struct {
			TenantID   string `json:"tenantId"`
			TenantName string `json:"tenantName"`
			Role       string `json:"role"`
		}
		type detail struct {
			ID          string           `json:"id"`
			Email       string           `json:"email"`
			DisplayName string           `json:"displayName"`
			IsActive    bool             `json:"isActive"`
			MFA         bool             `json:"mfa"`
			AuthMethods []string         `json:"authMethods"`
			Verified    bool             `json:"emailVerified"`
			Memberships []membershipInfo `json:"memberships"`
			CreatedAt   string           `json:"createdAt"`
			LastLogin   string           `json:"lastLogin,omitempty"`
		}
		methods := make([]string, len(user.AuthMethods))
		for i, m := range user.AuthMethods {
			methods[i] = string(m)
		}
		d := detail{
			ID:          user.ID.Hex(),
			Email:       user.Email,
			DisplayName: user.DisplayName,
			IsActive:    user.IsActive,
			MFA:         user.TOTPEnabled,
			AuthMethods: methods,
			Verified:    user.EmailVerified,
			CreatedAt:   user.CreatedAt.Format(time.RFC3339),
		}
		if user.LastLoginAt != nil {
			d.LastLogin = user.LastLoginAt.Format(time.RFC3339)
		}
		for _, m := range memberships {
			d.Memberships = append(d.Memberships, membershipInfo{
				TenantID:   m.TenantID.Hex(),
				TenantName: tenantNames[m.TenantID],
				Role:       string(m.Role),
			})
		}
		printJSON(d)
		return
	}

	fmt.Printf("%s %s\n", bold("User:"), user.DisplayName)
	fmt.Printf("  ID:         %s\n", user.ID.Hex())
	fmt.Printf("  Email:      %s\n", user.Email)
	fmt.Printf("  Verified:   %v\n", user.EmailVerified)
	status := clr(cGreen, "active")
	if !user.IsActive {
		status = clr(cRed, "suspended")
	}
	fmt.Printf("  Status:     %s\n", status)

	methods := make([]string, len(user.AuthMethods))
	for i, m := range user.AuthMethods {
		methods[i] = string(m)
	}
	fmt.Printf("  Auth:       %s\n", strings.Join(methods, ", "))
	if user.TOTPEnabled {
		fmt.Printf("  MFA:        %s\n", clr(cGreen, "enabled"))
	}
	fmt.Printf("  Created:    %s (%s)\n", user.CreatedAt.Format(time.RFC3339), timeAgo(user.CreatedAt))
	if user.LastLoginAt != nil {
		fmt.Printf("  Last login: %s (%s)\n", user.LastLoginAt.Format(time.RFC3339), timeAgo(*user.LastLoginAt))
	}

	if len(memberships) > 0 {
		fmt.Printf("\n  %s\n", bold("Memberships:"))
		for _, m := range memberships {
			name := tenantNames[m.TenantID]
			if name == "" {
				name = m.TenantID.Hex()
			}
			fmt.Printf("    - %s (%s)\n", name, m.Role)
		}
	}
}

func cmdUsersSetActive(active bool) {
	verb := "suspend"
	if active {
		verb = "activate"
	}

	fs := flag.NewFlagSet("users "+verb, flag.ExitOnError)
	email := fs.String("email", "", "User email address (required)")
	fs.Parse(os.Args[3:])

	if *email == "" {
		fmt.Fprintf(os.Stderr, "Usage: mcplens users %s --email <email>\n", verb)
		os.Exit(1)
	}

	database, _, cleanup := connectDB()
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	emailNorm := strings.TrimSpace(strings.ToLower(*email))
	var user models.User
	if err := database.Users().FindOne(ctx, bson.M{"email": emailNorm}).Decode(&user); err != nil {
		fmt.Fprintf(os.Stderr, "User not found: %s\n", emailNorm)
		os.Exit(1)
	}

	if user.IsActive == active {
		if active {
			fmt.Printf("User %s is already active.\n", emailNorm)
		} else {
			fmt.Printf("User %s is already suspended.\n", emailNorm)
		}
		return
	}

	_, err := database.Users().UpdateOne(ctx,
		bson.M{"_id": user.ID},
		bson.M{"$set": bson.M{"isActive": active, "updatedAt": time.Now()}},
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to update user: %v\n", err)
		os.Exit(1)
	}

	if active {
		fmt.Printf("User %s (%s) has been reactivated.\n", user.DisplayName, user.Email)
	} else {
		// Also revoke sessions when suspending
		database.RefreshTokens().DeleteMany(ctx, bson.M{"userId": user.ID})
		fmt.Printf("User %s (%s) has been suspended and all sessions revoked.\n", user.DisplayName, user.Email)
	}
}

func cmdUsersRevokeSessions() {
	fs := flag.NewFlagSet("users revoke-sessions", flag.ExitOnError)
	email := fs.String("email", "", "User email address (required)")
	fs.Parse(os.Args[3:])

	if *email == "" {
		fmt.Fprintln(os.Stderr, "Usage: mcplens users revoke-sessions --email <email>")
		os.Exit(1)
	}

	database, _, cleanup := connectDB()
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	emailNorm := strings.TrimSpace(strings.ToLower(*email))
	var user models.User
	if err := database.Users().FindOne(ctx, bson.M{"email": emailNorm}).Decode(&user); err != nil {
		fmt.Fprintf(os.Stderr, "User not found: %s\n", emailNorm)
		os.Exit(1)
	}

	result, err := database.RefreshTokens().DeleteMany(ctx, bson.M{"userId": user.ID})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to revoke sessions: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Revoked %d session(s) for %s (%s).\n", result.DeletedCount, user.DisplayName, user.Email)
}

// lookupUserWithMemberships finds a user by email and their memberships.
func lookupUserWithMemberships(ctx context.Context, database *db.MongoDB, email string) (models.User, []models.TenantMembership) {
	emailNorm := strings.TrimSpace(strings.ToLower(email))
	var user models.User
	if err := database.Users().FindOne(ctx, bson.M{"email": emailNorm}).Decode(&user); err != nil {
		fmt.Fprintf(os.Stderr, "User not found: %s\n", emailNorm)
		os.Exit(1)
	}

	cursor, err := database.TenantMemberships().Find(ctx, bson.M{"userId": user.ID})
	if err != nil {
		return user, nil
	}
	defer cursor.Close(ctx)

	var memberships []models.TenantMembership
	cursor.All(ctx, &memberships)
	return user, memberships
}

// resolveTenantNames batch-resolves tenant IDs to names.
func resolveTenantNames(ctx context.Context, database *db.MongoDB, ids []primitive.ObjectID) map[primitive.ObjectID]string {
	names := make(map[primitive.ObjectID]string)
	if len(ids) == 0 {
		return names
	}
	cursor, err := database.Tenants().Find(ctx, bson.M{"_id": bson.M{"$in": ids}})
	if err != nil {
		return names
	}
	defer cursor.Close(ctx)

	var tenants []models.Tenant
	cursor.All(ctx, &tenants)
	for _, t := range tenants {
		names[t.ID] = t.Name
	}
	return names
}
