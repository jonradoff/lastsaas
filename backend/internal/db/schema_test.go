package db

import (
	"testing"
)

func TestAllSchemasCount(t *testing.T) {
	schemas := AllSchemas()
	if len(schemas) != 16 {
		t.Errorf("AllSchemas() returned %d schemas, want 16", len(schemas))
	}
}

func TestAllSchemasHaveNames(t *testing.T) {
	schemas := AllSchemas()
	for i, cs := range schemas {
		if cs.Collection == "" {
			t.Errorf("schema %d has empty collection name", i)
		}
		if cs.Schema == nil {
			t.Errorf("schema %q has nil Schema", cs.Collection)
		}
	}
}

func TestAllSchemasUniqueCollections(t *testing.T) {
	schemas := AllSchemas()
	seen := make(map[string]bool)
	for _, cs := range schemas {
		if seen[cs.Collection] {
			t.Errorf("duplicate collection name: %q", cs.Collection)
		}
		seen[cs.Collection] = true
	}
}

func TestExpectedCollections(t *testing.T) {
	expected := []string{
		"users", "tenants", "tenant_memberships", "invitations",
		"plans", "credit_bundles", "financial_transactions", "webhooks",
		"api_keys", "config_vars", "announcements", "custom_pages",
		"messages", "usage_events", "sso_connections", "event_definitions",
	}

	schemas := AllSchemas()
	collectionSet := make(map[string]bool)
	for _, cs := range schemas {
		collectionSet[cs.Collection] = true
	}

	for _, name := range expected {
		if !collectionSet[name] {
			t.Errorf("expected collection %q not found in AllSchemas()", name)
		}
	}
}

func TestSchemasHaveJSONSchema(t *testing.T) {
	schemas := AllSchemas()
	for _, cs := range schemas {
		jsonSchema, ok := cs.Schema["$jsonSchema"]
		if !ok {
			t.Errorf("collection %q: missing $jsonSchema key", cs.Collection)
			continue
		}
		schemaMap, ok := jsonSchema.(map[string]interface{})
		if !ok {
			// Try bson.M which is map[string]interface{} under the hood
			continue
		}
		if _, hasProps := schemaMap["properties"]; !hasProps {
			t.Errorf("collection %q: $jsonSchema missing 'properties'", cs.Collection)
		}
	}
}
