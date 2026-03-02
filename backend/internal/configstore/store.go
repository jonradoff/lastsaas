package configstore

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"time"

	"lastsaas/internal/db"
	"lastsaas/internal/models"

	"go.mongodb.org/mongo-driver/bson"
)

// Store is a thread-safe in-memory cache of configuration variables backed by MongoDB.
type Store struct {
	db    *db.MongoDB
	mu    sync.RWMutex
	cache map[string]models.ConfigVar
}

// New creates a Store. Call Load() to populate from DB.
func New(database *db.MongoDB) *Store {
	return &Store{
		db:    database,
		cache: make(map[string]models.ConfigVar),
	}
}

// Load reads all config vars from DB into the cache.
func (s *Store) Load(ctx context.Context) error {
	cursor, err := s.db.ConfigVars().Find(ctx, bson.M{})
	if err != nil {
		return err
	}
	defer cursor.Close(ctx)

	var vars []models.ConfigVar
	if err := cursor.All(ctx, &vars); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.cache = make(map[string]models.ConfigVar, len(vars))
	for _, v := range vars {
		s.cache[v.Name] = v
	}
	return nil
}

// Get returns the value of a config variable by name.
// Returns "" if not found. This is the lightweight hot path — RLock only, no DB call.
func (s *Store) Get(name string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if v, ok := s.cache[name]; ok {
		return v.Value
	}
	return ""
}

// GetVar returns the full ConfigVar struct from the cache.
func (s *Store) GetVar(name string) (models.ConfigVar, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.cache[name]
	return v, ok
}

// GetAll returns all cached config variables sorted by name.
func (s *Store) GetAll() []models.ConfigVar {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]models.ConfigVar, 0, len(s.cache))
	for _, v := range s.cache {
		result = append(result, v)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

// Set updates a variable's value in DB and reloads it into the cache.
func (s *Store) Set(ctx context.Context, name, value string) error {
	now := time.Now()
	result, err := s.db.ConfigVars().UpdateOne(ctx,
		bson.M{"name": name},
		bson.M{"$set": bson.M{"value": value, "updatedAt": now}},
	)
	if err != nil {
		return err
	}
	if result.MatchedCount == 0 {
		return fmt.Errorf("config variable %q not found", name)
	}
	return s.Reload(ctx, name)
}

// Reload re-reads a single variable from DB into the cache.
func (s *Store) Reload(ctx context.Context, name string) error {
	var v models.ConfigVar
	err := s.db.ConfigVars().FindOne(ctx, bson.M{"name": name}).Decode(&v)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cache[v.Name] = v
	return nil
}

// StartAutoReload periodically reloads all config vars from the database.
// This ensures multi-machine deployments stay in sync when config is changed
// on one machine. The goroutine stops when the context is canceled.
func (s *Store) StartAutoReload(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := s.Load(ctx); err != nil {
					slog.Warn("configstore: auto-reload failed", "error", err)
				}
			}
		}
	}()
}
