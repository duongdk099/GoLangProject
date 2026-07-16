package credits

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	"barterswap/internal/database"
)

func openIntegrationDatabase(t *testing.T) *sql.DB {
	t.Helper()
	if os.Getenv("RUN_POSTGRES_INTEGRATION") != "1" {
		t.Skip("set RUN_POSTGRES_INTEGRATION=1 to run the PostgreSQL integration test")
	}
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Fatal("TEST_DATABASE_URL is required for the PostgreSQL integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	db, err := database.Open(ctx, databaseURL)
	if err != nil {
		t.Fatalf("database.Open() error = %v", err)
	}
	if err := database.Migrate(ctx, db); err != nil {
		t.Fatalf("database.Migrate() error = %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// createTestUser inserts a bare user row directly, without depending on the
// users package: internal/users imports internal/credits (to record the
// welcome credit through Record), so this package cannot import users back
// without an import cycle. users.Create's own welcome-credit behavior is
// covered by internal/users/store_postgres_integration_test.go instead; this
// test only needs a valid user id to satisfy credit_transactions' foreign key.
func createTestUser(t *testing.T, db *sql.DB, pseudo string) int {
	t.Helper()
	var id int
	if err := db.QueryRowContext(context.Background(), `
		INSERT INTO users (pseudo) VALUES ($1) RETURNING id
	`, pseudo).Scan(&id); err != nil {
		t.Fatalf("create test user: %v", err)
	}
	return id
}

func TestJournalIntegration(t *testing.T) {
	db := openIntegrationDatabase(t)
	ctx := context.Background()

	userID := createTestUser(t, db, "Credits Integration User")
	t.Cleanup(func() {
		db.ExecContext(context.Background(), `DELETE FROM users WHERE id = $1`, userID)
	})

	// Record the welcome credit directly; this mirrors what users.Create does
	// through the same Record function, without importing the users package.
	if err := Record(ctx, db, Entry{UserID: userID, Amount: 10, Type: TypeEarn}); err != nil {
		t.Fatalf("Record(welcome) error = %v", err)
	}
	balance, err := Balance(ctx, db, userID)
	if err != nil || balance != 10 {
		t.Fatalf("Balance() = %d, err = %v, want 10", balance, err)
	}

	// A spend tied to an (unconstrained) exchange id lowers the balance.
	if err := Record(ctx, db, Entry{UserID: userID, ExchangeID: 777, Amount: 3, Type: TypeSpend}); err != nil {
		t.Fatalf("Record(spend) error = %v", err)
	}
	if balance, _ := Balance(ctx, db, userID); balance != 7 {
		t.Fatalf("Balance() after spend = %d, want 7", balance)
	}

	// The same movement for the same exchange and type is rejected as a conflict.
	if err := Record(ctx, db, Entry{UserID: userID, ExchangeID: 777, Amount: 3, Type: TypeSpend}); err == nil {
		t.Fatal("Record(duplicate spend) expected a conflict error")
	}
}

// TestJournalOnClosedDatabase drives the database-error return branches of
// Record and Balance by using a connection pool that has been closed.
func TestJournalOnClosedDatabase(t *testing.T) {
	closed := cloneClosed(t)

	if _, err := Balance(context.Background(), closed, 1); err == nil {
		t.Fatal("Balance() on closed database expected an error")
	}
	if err := Record(context.Background(), closed, Entry{UserID: 1, Amount: 1, Type: TypeEarn}); err == nil {
		t.Fatal("Record() on closed database expected an error")
	}
}

// cloneClosed opens a pool to the integration database and closes it, so
// calls on it fail with a driver error without disturbing a shared open pool.
func cloneClosed(t *testing.T) *sql.DB {
	t.Helper()
	if os.Getenv("RUN_POSTGRES_INTEGRATION") != "1" {
		t.Skip("set RUN_POSTGRES_INTEGRATION=1 to run the PostgreSQL integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	closed, err := database.Open(ctx, os.Getenv("TEST_DATABASE_URL"))
	if err != nil {
		t.Fatalf("open pool: %v", err)
	}
	closed.Close()
	return closed
}
