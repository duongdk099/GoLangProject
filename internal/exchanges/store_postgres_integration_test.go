package exchanges

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"testing"
	"time"

	"barterswap/internal/credits"
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

	if _, err := store.Create(ctx, CreateParams{
		ServiceID: service.ID, RequesterID: requester.ID, OwnerID: owner.ID, Cost: service.Credits,
	}); !errors.Is(err, httpapi.ErrConflict) {
		t.Fatalf("second Create() error = %v, want conflict", err)
	}

	if _, err := store.Accept(ctx, created.ID, requester.ID); !errors.Is(err, httpapi.ErrForbidden) {
		t.Fatalf("Accept(non owner) error = %v, want forbidden", err)
	}
	if _, err := store.Accept(ctx, created.ID, owner.ID); err != nil {
		t.Fatalf("Accept() error = %v", err)
	}
	if balance, _ := store.Balance(ctx, requester.ID); balance != users.WelcomeCredits-service.Credits {
		t.Fatalf("requester balance after accept = %d, want %d", balance, users.WelcomeCredits-service.Credits)
	}

	if _, err := store.Accept(ctx, created.ID, owner.ID); !errors.Is(err, httpapi.ErrValidation) {
		t.Fatalf("Accept(again) error = %v, want validation", err)
	}
	if balance, _ := store.Balance(ctx, requester.ID); balance != users.WelcomeCredits-service.Credits {
		t.Fatalf("requester balance after repeat accept = %d, want %d", balance, users.WelcomeCredits-service.Credits)
	}

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

func TestPostgresStoreReadsAndBranches(t *testing.T) {
	db := openIntegrationDatabase(t)
	ctx := context.Background()

	userStore := users.NewPostgresStore(db)
	requester, err := userStore.Create(ctx, users.CreateUserParams{Pseudo: "Exchange Reads Requester"})
	if err != nil {
		t.Fatalf("create requester: %v", err)
	}
	owner, err := userStore.Create(ctx, users.CreateUserParams{Pseudo: "Exchange Reads Owner"})
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}
	t.Cleanup(func() {
		db.ExecContext(context.Background(), `DELETE FROM users WHERE id IN ($1, $2)`, requester.ID, owner.ID)
	})

	service, err := services.NewPostgresStore(db).Create(ctx, services.CreateParams{
		ProviderID: owner.ID, Titre: "Cours", Categorie: "Informatique", DureeMinutes: 30, Credits: 2,
	})
	if err != nil {
		t.Fatalf("create service: %v", err)
	}
	store := NewPostgresStore(db)

	if _, err := store.GetByID(ctx, 999_000_001); !errors.Is(err, httpapi.ErrNotFound) {
		t.Fatalf("GetByID(missing) error = %v, want not found", err)
	}
	if _, err := store.Accept(ctx, 999_000_002, owner.ID); !errors.Is(err, httpapi.ErrNotFound) {
		t.Fatalf("Accept(missing) error = %v, want not found", err)
	}

	created, err := store.Create(ctx, CreateParams{
		ServiceID: service.ID, RequesterID: requester.ID, OwnerID: owner.ID, Cost: service.Credits,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if got, err := store.GetByID(ctx, created.ID); err != nil || got.ID != created.ID {
		t.Fatalf("GetByID() = %+v, err = %v", got, err)
	}
	if list, err := store.List(ctx, Filter{UserID: requester.ID}); err != nil || len(list) != 1 {
		t.Fatalf("List(all) = %+v, err = %v", list, err)
	}
	if list, err := store.List(ctx, Filter{UserID: owner.ID, Status: StatusPending}); err != nil || len(list) != 1 {
		t.Fatalf("List(status) = %+v, err = %v", list, err)
	}
	if list, err := store.List(ctx, Filter{UserID: requester.ID, Status: StatusCompleted}); err != nil || len(list) != 0 {
		t.Fatalf("List(no match) = %+v, err = %v", list, err)
	}

	if _, err := store.Reject(ctx, created.ID, requester.ID); !errors.Is(err, httpapi.ErrForbidden) {
		t.Fatalf("Reject(non owner) error = %v, want forbidden", err)
	}
	rejected, err := store.Reject(ctx, created.ID, owner.ID)
	if err != nil || rejected.Status != StatusRejected {
		t.Fatalf("Reject() = %+v, err = %v", rejected, err)
	}

	pending, err := store.Create(ctx, CreateParams{
		ServiceID: service.ID, RequesterID: requester.ID, OwnerID: owner.ID, Cost: service.Credits,
	})
	if err != nil {
		t.Fatalf("Create(pending) error = %v", err)
	}
	balanceBefore, _ := store.Balance(ctx, requester.ID)
	if _, err := store.Cancel(ctx, pending.ID, owner.ID); err != nil {
		t.Fatalf("Cancel(pending) error = %v", err)
	}
	if balance, _ := store.Balance(ctx, requester.ID); balance != balanceBefore {
		t.Fatalf("balance changed on pending cancel: %d != %d", balance, balanceBefore)
	}

	third, err := store.Create(ctx, CreateParams{
		ServiceID: service.ID, RequesterID: requester.ID, OwnerID: owner.ID, Cost: service.Credits,
	})
	if err != nil {
		t.Fatalf("Create(third) error = %v", err)
	}
	if _, err := store.Cancel(ctx, third.ID, 999_000_003); !errors.Is(err, httpapi.ErrForbidden) {
		t.Fatalf("Cancel(stranger) error = %v, want forbidden", err)
	}
	if _, err := store.Accept(ctx, third.ID, owner.ID); err != nil {
		t.Fatalf("Accept(third) error = %v", err)
	}
	if _, err := store.Complete(ctx, third.ID, requester.ID); err != nil {
		t.Fatalf("Complete(third) error = %v", err)
	}
	if _, err := store.Cancel(ctx, third.ID, requester.ID); !errors.Is(err, httpapi.ErrValidation) {
		t.Fatalf("Cancel(completed) error = %v, want validation", err)
	}
}

func TestPostgresAcceptRechecksBalance(t *testing.T) {
	db := openIntegrationDatabase(t)
	ctx := context.Background()

	userStore := users.NewPostgresStore(db)
	requester, err := userStore.Create(ctx, users.CreateUserParams{Pseudo: "Exchange Drain Requester"})
	if err != nil {
		t.Fatalf("create requester: %v", err)
	}
	owner, err := userStore.Create(ctx, users.CreateUserParams{Pseudo: "Exchange Drain Owner"})
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}
	t.Cleanup(func() {
		db.ExecContext(context.Background(), `DELETE FROM users WHERE id IN ($1, $2)`, requester.ID, owner.ID)
	})

	service, err := services.NewPostgresStore(db).Create(ctx, services.CreateParams{
		ProviderID: owner.ID, Titre: "Cours", Categorie: "Informatique", DureeMinutes: 30, Credits: 2,
	})
	if err != nil {
		t.Fatalf("create service: %v", err)
	}
	store := NewPostgresStore(db)

	created, err := store.Create(ctx, CreateParams{
		ServiceID: service.ID, RequesterID: requester.ID, OwnerID: owner.ID, Cost: service.Credits,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if err := credits.Record(ctx, db, credits.Entry{UserID: requester.ID, Amount: users.WelcomeCredits - 1, Type: credits.TypeSpend}); err != nil {
		t.Fatalf("drain requester: %v", err)
	}
	if _, err := store.Accept(ctx, created.ID, owner.ID); !errors.Is(err, httpapi.ErrValidation) {
		t.Fatalf("Accept(drained) error = %v, want validation", err)
	}
}

func TestPostgresStoreOnClosedDatabase(t *testing.T) {
	_ = openIntegrationDatabase(t)
	ctx := context.Background()

	openCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	closedDB, err := database.Open(openCtx, os.Getenv("TEST_DATABASE_URL"))
	if err != nil {
		t.Fatalf("open second pool: %v", err)
	}
	closedDB.Close()
	store := NewPostgresStore(closedDB)

	if _, err := store.Create(ctx, CreateParams{ServiceID: 1, RequesterID: 1, OwnerID: 2, Cost: 1}); err == nil {
		t.Fatal("Create() on closed database expected an error")
	}
	if _, err := store.GetByID(ctx, 1); err == nil {
		t.Fatal("GetByID() on closed database expected an error")
	}
	if _, err := store.List(ctx, Filter{UserID: 1}); err == nil {
		t.Fatal("List() on closed database expected an error")
	}
	if _, err := store.CountCompleted(ctx, 1); err == nil {
		t.Fatal("CountCompleted() on closed database expected an error")
	}
	if _, err := store.Balance(ctx, 1); err == nil {
		t.Fatal("Balance() on closed database expected an error")
	}
	if _, err := store.Accept(ctx, 1, 2); err == nil {
		t.Fatal("Accept() on closed database expected an error")
	}
	if _, err := store.Cancel(ctx, 1, 2); err == nil {
		t.Fatal("Cancel() on closed database expected an error")
	}
}
