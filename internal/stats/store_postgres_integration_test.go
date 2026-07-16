package stats

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	"barterswap/internal/database"
	"barterswap/internal/services"
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

func TestPostgresStoreIntegration(t *testing.T) {
	db := openIntegrationDatabase(t)
	ctx := context.Background()

	userStore := users.NewPostgresStore(db)
	user, err := userStore.Create(ctx, users.CreateUserParams{Pseudo: "Stats Integration User"})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	t.Cleanup(func() {
		db.ExecContext(context.Background(), `DELETE FROM users WHERE id = $1`, user.ID)
	})

	serviceStore := services.NewPostgresStore(db)
	if _, err := serviceStore.Create(ctx, services.CreateParams{
		ProviderID: user.ID, Titre: "Cours", Categorie: "Informatique", DureeMinutes: 30, Credits: 1,
	}); err != nil {
		t.Fatalf("create service: %v", err)
	}

	store := NewPostgresStore(db)

	activeServices, err := store.CountActiveServices(ctx, user.ID)
	if err != nil || activeServices != 1 {
		t.Fatalf("CountActiveServices() = %d, err = %v", activeServices, err)
	}

	balance, err := store.CreditBalance(ctx, user.ID)
	if err != nil || balance != users.WelcomeCredits {
		t.Fatalf("CreditBalance() = %d, err = %v", balance, err)
	}

	earned, spent, err := store.CreditTotals(ctx, user.ID)
	if err != nil || earned != users.WelcomeCredits || spent != 0 {
		t.Fatalf("CreditTotals() = %d, %d, err = %v", earned, spent, err)
	}

	average, count, err := store.ReviewAggregate(ctx, user.ID)
	if err != nil || average != 0 || count != 0 {
		t.Fatalf("ReviewAggregate() = %v, %d, err = %v", average, count, err)
	}
}
