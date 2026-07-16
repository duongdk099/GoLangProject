package credits

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	"barterswap/internal/database"
	"barterswap/internal/users"
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

func TestJournalIntegration(t *testing.T) {
	db := openIntegrationDatabase(t)
	ctx := context.Background()

	// A fresh user carries the 10 welcome credits.
	user, err := users.NewPostgresStore(db).Create(ctx, users.CreateUserParams{Pseudo: "Credits Integration User"})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	t.Cleanup(func() {
		db.ExecContext(context.Background(), `DELETE FROM users WHERE id = $1`, user.ID)
	})

	balance, err := Balance(ctx, db, user.ID)
	if err != nil || balance != users.WelcomeCredits {
		t.Fatalf("Balance() = %d, err = %v, want %d", balance, err, users.WelcomeCredits)
	}

	// A spend tied to an (unconstrained) exchange id lowers the balance.
	if err := Record(ctx, db, Entry{UserID: user.ID, ExchangeID: 777, Amount: 3, Type: TypeSpend}); err != nil {
		t.Fatalf("Record(spend) error = %v", err)
	}
	if balance, _ := Balance(ctx, db, user.ID); balance != users.WelcomeCredits-3 {
		t.Fatalf("Balance() after spend = %d, want %d", balance, users.WelcomeCredits-3)
	}

	// The same movement for the same exchange and type is rejected as a conflict.
	if err := Record(ctx, db, Entry{UserID: user.ID, ExchangeID: 777, Amount: 3, Type: TypeSpend}); err == nil {
		t.Fatal("Record(duplicate spend) expected a conflict error")
	}
}

// TestJournalOnClosedDatabase drives the database-error return branches of
// Record and Balance by using a connection pool that has been closed.
func TestJournalOnClosedDatabase(t *testing.T) {
	db := openIntegrationDatabase(t)
	closed := cloneClosed(t, db)

	if _, err := Balance(context.Background(), closed, 1); err == nil {
		t.Fatal("Balance() on closed database expected an error")
	}
	if err := Record(context.Background(), closed, Entry{UserID: 1, Amount: 1, Type: TypeEarn}); err == nil {
		t.Fatal("Record() on closed database expected an error")
	}
}

// cloneClosed opens a second pool to the same database and closes it, so calls
// on it fail with a driver error without disturbing the shared test database.
func cloneClosed(t *testing.T, _ *sql.DB) *sql.DB {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	closed, err := database.Open(ctx, os.Getenv("TEST_DATABASE_URL"))
	if err != nil {
		t.Fatalf("open second pool: %v", err)
	}
	closed.Close()
	return closed
}
