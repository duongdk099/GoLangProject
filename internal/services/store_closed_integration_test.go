package services

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"barterswap/internal/database"
	"barterswap/internal/users"
	"barterswap/pkg/httpapi"
)

// TestPostgresStoreFilterBranches exercises the Ville and Search filter
// branches of List and the not-found path of Update, which the main
// lifecycle test does not reach.
func TestPostgresStoreFilterBranches(t *testing.T) {
	db := openIntegrationDatabase(t)
	ctx := context.Background()

	userStore := users.NewPostgresStore(db)
	provider, err := userStore.Create(ctx, users.CreateUserParams{Pseudo: "Service Filter Provider"})
	if err != nil {
		t.Fatalf("create provider: %v", err)
	}
	t.Cleanup(func() {
		db.ExecContext(context.Background(), `DELETE FROM users WHERE id = $1`, provider.ID)
	})

	store := NewPostgresStore(db)
	service, err := store.Create(ctx, CreateParams{
		ProviderID: provider.ID, Titre: "Cours de Go filtre unique", Categorie: "Informatique",
		DureeMinutes: 60, Credits: 2, Ville: "Marseille-Filtre",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	t.Cleanup(func() {
		db.ExecContext(context.Background(), `DELETE FROM services WHERE id = $1`, service.ID)
	})

	byVille, err := store.List(ctx, Filter{Ville: "marseille-filtre"})
	if err != nil || len(byVille) != 1 {
		t.Fatalf("List(ville) = %+v, err = %v", byVille, err)
	}
	bySearch, err := store.List(ctx, Filter{Search: "filtre unique"})
	if err != nil || len(bySearch) != 1 {
		t.Fatalf("List(search) = %+v, err = %v", bySearch, err)
	}

	// Updating a service that does not exist returns not-found (rows == 0).
	if _, err := store.Update(ctx, 999_000_101, UpdateParams{
		Titre: "X", Categorie: "Informatique", DureeMinutes: 30, Credits: 1,
	}); !errors.Is(err, httpapi.ErrNotFound) {
		t.Fatalf("Update(missing) error = %v, want not found", err)
	}
}

// TestPostgresStoreOnClosedDatabase drives the database-error return branches of
// every store entry point using a closed connection pool.
func TestPostgresStoreOnClosedDatabase(t *testing.T) {
	_ = openIntegrationDatabase(t) // gate on the integration flag
	ctx := context.Background()

	openCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	closedDB, err := database.Open(openCtx, os.Getenv("TEST_DATABASE_URL"))
	if err != nil {
		t.Fatalf("open second pool: %v", err)
	}
	closedDB.Close()
	store := NewPostgresStore(closedDB)

	if _, err := store.Create(ctx, CreateParams{ProviderID: 1, Titre: "X", Categorie: "Informatique", DureeMinutes: 1, Credits: 1}); err == nil {
		t.Fatal("Create() on closed database expected an error")
	}
	if _, err := store.GetByID(ctx, 1); err == nil {
		t.Fatal("GetByID() on closed database expected an error")
	}
	if _, err := store.Update(ctx, 1, UpdateParams{Titre: "X", Categorie: "Informatique", DureeMinutes: 1, Credits: 1}); err == nil {
		t.Fatal("Update() on closed database expected an error")
	}
	if err := store.Delete(ctx, 1); err == nil {
		t.Fatal("Delete() on closed database expected an error")
	}
	if _, err := store.List(ctx, Filter{}); err == nil {
		t.Fatal("List() on closed database expected an error")
	}
}
