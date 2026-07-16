package main

import (
	"strings"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	t.Setenv("PORT", "9090")
	t.Setenv("DATABASE_URL", "postgres://example")
	config := LoadConfig()
	if config.Address != ":9090" || config.DatabaseURL != "postgres://example" {
		t.Fatalf("LoadConfig() = %+v", config)
	}
	if config.ReadTimeout <= 0 || config.WriteTimeout <= 0 || config.IdleTimeout <= 0 {
		t.Fatalf("LoadConfig() has invalid timeouts: %+v", config)
	}
}

func TestLoadConfigDefaults(t *testing.T) {
	t.Setenv("PORT", "")
	t.Setenv("DATABASE_URL", "")
	config := LoadConfig()
	if config.Address != ":8080" {
		t.Fatalf("default address = %q", config.Address)
	}
	if !strings.Contains(config.DatabaseURL, "barterswap") {
		t.Fatalf("default database URL = %q", config.DatabaseURL)
	}
}

func TestEmbeddedSchemaContainsPersonOneTables(t *testing.T) {
	for _, table := range []string{"users", "skills", "credit_transactions"} {
		if !strings.Contains(databaseSchema, "CREATE TABLE IF NOT EXISTS "+table) {
			t.Fatalf("embedded schema does not define %s", table)
		}
	}
}
