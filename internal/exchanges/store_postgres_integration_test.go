package exchanges

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"testing"
	"time"

	"barterswap/internal/database"
	"barterswap/internal/services"
	"barterswap/internal/users"
	"barterswap/pkg/httpapi"
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
	requester, err := userStore.Create(ctx, users.CreateUserParams{Pseudo: "Exchange Integration Requester"})
	if err != nil {
		t.Fatalf("create requester: %v", err)
	}
	owner, err := userStore.Create(ctx, users.CreateUserParams{Pseudo: "Exchange Integration Owner"})
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}
	t.Cleanup(func() {
		db.ExecContext(context.Background(), `DELETE FROM users WHERE id IN ($1, $2)`, requester.ID, owner.ID)
	})

	serviceStore := services.NewPostgresStore(db)
	service, err := serviceStore.Create(ctx, services.CreateParams{
		ProviderID: owner.ID, Titre: "Cours", Categorie: "Informatique", DureeMinutes: 30, Credits: 2,
	})
	if err != nil {
		t.Fatalf("create service: %v", err)
	}

	store := NewPostgresStore(db)

	// Both users start with the 10 welcome credits.
	requesterBalance, err := store.Balance(ctx, requester.ID)
	if err != nil || requesterBalance != users.WelcomeCredits {
		t.Fatalf("initial requester balance = %d, err = %v", requesterBalance, err)
	}

	created, err := store.Create(ctx, CreateParams{
		ServiceID: service.ID, RequesterID: requester.ID, OwnerID: owner.ID, Cost: service.Credits,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.Status != StatusPending {
		t.Fatalf("created status = %q, want pending", created.Status)
	}

	// The service is reserved: a second request conflicts at the database level.
	if _, err := store.Create(ctx, CreateParams{
		ServiceID: service.ID, RequesterID: requester.ID, OwnerID: owner.ID, Cost: service.Credits,
	}); !errors.Is(err, httpapi.ErrConflict) {
		t.Fatalf("second Create() error = %v, want conflict", err)
	}

	// Only the owner may accept; a stranger is forbidden.
	if _, err := store.Accept(ctx, created.ID, requester.ID); !errors.Is(err, httpapi.ErrForbidden) {
		t.Fatalf("Accept(non owner) error = %v, want forbidden", err)
	}
	if _, err := store.Accept(ctx, created.ID, owner.ID); err != nil {
		t.Fatalf("Accept() error = %v", err)
	}
	if balance, _ := store.Balance(ctx, requester.ID); balance != users.WelcomeCredits-service.Credits {
		t.Fatalf("requester balance after accept = %d, want %d", balance, users.WelcomeCredits-service.Credits)
	}

	// Re-accepting is an invalid transition and must not debit again.
	if _, err := store.Accept(ctx, created.ID, owner.ID); !errors.Is(err, httpapi.ErrValidation) {
		t.Fatalf("Accept(again) error = %v, want validation", err)
	}
	if balance, _ := store.Balance(ctx, requester.ID); balance != users.WelcomeCredits-service.Credits {
		t.Fatalf("requester balance after repeat accept = %d, want %d", balance, users.WelcomeCredits-service.Credits)
	}

	// The requester confirms completion, releasing the credits to the owner.
	if _, err := store.Complete(ctx, created.ID, owner.ID); !errors.Is(err, httpapi.ErrForbidden) {
		t.Fatalf("Complete(owner) error = %v, want forbidden", err)
	}
	if _, err := store.Complete(ctx, created.ID, requester.ID); err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if balance, _ := store.Balance(ctx, owner.ID); balance != users.WelcomeCredits+service.Credits {
		t.Fatalf("owner balance after complete = %d, want %d", balance, users.WelcomeCredits+service.Credits)
	}

	if count, _ := store.CountCompleted(ctx, owner.ID); count != 1 {
		t.Fatalf("owner completed count = %d, want 1", count)
	}
	if count, _ := store.CountCompleted(ctx, requester.ID); count != 1 {
		t.Fatalf("requester completed count = %d, want 1", count)
	}

	// A fresh reservation on the same service can now proceed, and cancelling
	// after acceptance refunds the requester exactly once.
	second, err := store.Create(ctx, CreateParams{
		ServiceID: service.ID, RequesterID: requester.ID, OwnerID: owner.ID, Cost: service.Credits,
	})
	if err != nil {
		t.Fatalf("second Create() after completion error = %v", err)
	}
	if _, err := store.Accept(ctx, second.ID, owner.ID); err != nil {
		t.Fatalf("Accept(second) error = %v", err)
	}
	balanceBeforeCancel, _ := store.Balance(ctx, requester.ID)
	cancelled, err := store.Cancel(ctx, second.ID, requester.ID)
	if err != nil || cancelled.Status != StatusCancelled {
		t.Fatalf("Cancel() = %+v, err = %v", cancelled, err)
	}
	if balance, _ := store.Balance(ctx, requester.ID); balance != balanceBeforeCancel+service.Credits {
		t.Fatalf("requester balance after refund = %d, want %d", balance, balanceBeforeCancel+service.Credits)
	}
}
