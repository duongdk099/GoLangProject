package config

import (
	"strings"
	"testing"
)

func TestLoad(t *testing.T) {
	t.Setenv("PORT", "9090")
	t.Setenv("DATABASE_URL", "postgres://example")
	config := Load()
	if config.Address != ":9090" || config.DatabaseURL != "postgres://example" {
		t.Fatalf("Load() = %+v", config)
	}
	if config.ReadTimeout <= 0 || config.WriteTimeout <= 0 || config.IdleTimeout <= 0 {
		t.Fatalf("Load() has invalid timeouts: %+v", config)
	}
}

func TestLoadDefaults(t *testing.T) {
	t.Setenv("PORT", "")
	t.Setenv("DATABASE_URL", "")
	config := Load()
	if config.Address != ":8080" {
		t.Fatalf("default address = %q", config.Address)
	}
	if !strings.Contains(config.DatabaseURL, "barterswap") {
		t.Fatalf("default database URL = %q", config.DatabaseURL)
	}
}
