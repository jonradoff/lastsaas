package main

import (
	"context"
	"fmt"
	"os"
	"sort"
	"time"

	"go.mongodb.org/mongo-driver/bson"
)

func cmdDB() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, `Usage: mcplens db <subcommand>

Subcommands:
  stats       Show collection document counts and sizes`)
		os.Exit(1)
	}

	switch os.Args[2] {
	case "stats":
		cmdDBStats()
	default:
		fmt.Fprintf(os.Stderr, "Unknown db subcommand: %s\n", os.Args[2])
		os.Exit(1)
	}
}

func cmdDBStats() {
	database, cfg, cleanup := connectDB()
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Get database stats
	var dbStats bson.M
	err := database.Database.RunCommand(ctx, bson.M{"dbStats": 1}).Decode(&dbStats)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get database stats: %v\n", err)
		os.Exit(1)
	}

	// List collections
	collections, err := database.Database.ListCollectionNames(ctx, bson.M{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to list collections: %v\n", err)
		os.Exit(1)
	}
	sort.Strings(collections)

	type collStats struct {
		Name      string `json:"name"`
		Documents int64  `json:"documents"`
		SizeBytes int64  `json:"sizeBytes"`
		IndexSize int64  `json:"indexSizeBytes"`
	}

	var stats []collStats
	for _, cName := range collections {
		var cs bson.M
		err := database.Database.RunCommand(ctx, bson.M{"collStats": cName}).Decode(&cs)
		if err != nil {
			continue
		}
		s := collStats{Name: cName}
		if v, ok := cs["count"]; ok {
			s.Documents = toInt64(v)
		}
		if v, ok := cs["size"]; ok {
			s.SizeBytes = toInt64(v)
		}
		if v, ok := cs["totalIndexSize"]; ok {
			s.IndexSize = toInt64(v)
		}
		stats = append(stats, s)
	}

	if jsonOutput {
		printJSON(map[string]interface{}{
			"database":    cfg.Database.Name,
			"collections": stats,
			"dataSize":    toInt64(dbStats["dataSize"]),
			"indexSize":   toInt64(dbStats["indexSize"]),
			"storageSize": toInt64(dbStats["storageSize"]),
		})
		return
	}

	fmt.Printf("%s — %s\n\n", bold("Database Stats"), cfg.Database.Name)

	if ds, ok := dbStats["dataSize"]; ok {
		fmt.Printf("  Total data:    %s\n", formatBytes(toInt64(ds)))
	}
	if is, ok := dbStats["indexSize"]; ok {
		fmt.Printf("  Total indexes: %s\n", formatBytes(toInt64(is)))
	}
	if ss, ok := dbStats["storageSize"]; ok {
		fmt.Printf("  Storage:       %s\n", formatBytes(toInt64(ss)))
	}

	fmt.Printf("\n  %-30s %10s %12s %12s\n", bold("COLLECTION"), bold("DOCS"), bold("DATA"), bold("INDEXES"))
	fmt.Printf("  %-30s %10s %12s %12s\n", "----------", "----", "----", "-------")

	for _, s := range stats {
		fmt.Printf("  %-30s %10d %12s %12s\n",
			s.Name,
			s.Documents,
			formatBytes(s.SizeBytes),
			formatBytes(s.IndexSize),
		)
	}
}

func toInt64(v interface{}) int64 {
	switch n := v.(type) {
	case int32:
		return int64(n)
	case int64:
		return n
	case float64:
		return int64(n)
	default:
		return 0
	}
}
