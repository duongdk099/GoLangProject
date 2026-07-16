package database

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

func TestEmbeddedSchemaContainsExpectedTables(t *testing.T) {
	for _, table := range []string{"users", "skills", "credit_transactions", "services", "reviews"} {
		if !strings.Contains(Schema, "CREATE TABLE IF NOT EXISTS "+table) {
			t.Fatalf("embedded schema does not define %s", table)
		}
	}
}

// TestOpenUnreachable covers Open's Ping-error branch without a database: an
// unreachable address with a short connect timeout makes PingContext fail.
func TestOpenUnreachable(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	db, err := Open(ctx, "postgres://bad:bad@127.0.0.1:1/none?sslmode=disable&connect_timeout=1")
	if err == nil {
		db.Close()
		t.Fatal("Open() with an unreachable address expected an error")
	}
}

// TestOpenAndMigrateIntegration credits Open's and Migrate's happy paths to this
// package by calling them directly against the migrated integration database.
func TestOpenAndMigrateIntegration(t *testing.T) {
	if os.Getenv("RUN_POSTGRES_INTEGRATION") != "1" {
		t.Skip("set RUN_POSTGRES_INTEGRATION=1 to run the PostgreSQL integration test")
	}
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Fatal("TEST_DATABASE_URL is required for the PostgreSQL integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	db, err := Open(ctx, databaseURL)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer db.Close()

	if err := Migrate(ctx, db); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}
}

// TestMigrateOnClosedDatabase covers Migrate's schema-apply error branch by
// running it against a connection pool that has been closed.
func TestMigrateOnClosedDatabase(t *testing.T) {
	if os.Getenv("RUN_POSTGRES_INTEGRATION") != "1" {
		t.Skip("set RUN_POSTGRES_INTEGRATION=1 to run the PostgreSQL integration test")
	}
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Fatal("TEST_DATABASE_URL is required for the PostgreSQL integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	db, err := Open(ctx, databaseURL)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	db.Close()

	if err := Migrate(ctx, db); err == nil {
		t.Fatal("Migrate() on a closed database expected an error")
	}
}
