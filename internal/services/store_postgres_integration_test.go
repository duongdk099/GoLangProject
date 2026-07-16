package services

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"testing"
	"time"

	"barterswap/internal/database"
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
	provider, err := userStore.Create(ctx, users.CreateUserParams{Pseudo: "Service Integration Provider"})
	if err != nil {
		t.Fatalf("create provider: %v", err)
	}
	t.Cleanup(func() {
		db.ExecContext(context.Background(), `DELETE FROM users WHERE id = $1`, provider.ID)
	})

	store := NewPostgresStore(db)
	service, err := store.Create(ctx, CreateParams{
		ProviderID: provider.ID, Titre: "Cours de Go", Categorie: "Informatique",
		DureeMinutes: 60, Credits: 2, Ville: "Paris",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if !service.Actif {
		t.Fatalf("Create() = %+v, want actif", service)
	}

	got, err := store.GetByID(ctx, service.ID)
	if err != nil || got.Titre != service.Titre {
		t.Fatalf("GetByID() = %+v, err = %v", got, err)
	}

	updated, err := store.Update(ctx, service.ID, UpdateParams{
		Titre: "Cours de Go avancé", Categorie: "Informatique", DureeMinutes: 90, Credits: 3, Actif: false,
	})
	if err != nil || updated.Actif {
		t.Fatalf("Update() = %+v, err = %v", updated, err)
	}

	list, err := store.List(ctx, Filter{Categorie: "Informatique"})
	if err != nil || len(list) == 0 {
		t.Fatalf("List() = %+v, err = %v", list, err)
	}

	if err := store.Delete(ctx, service.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err := store.GetByID(ctx, service.ID); !errors.Is(err, httpapi.ErrNotFound) {
		t.Fatalf("GetByID(deleted) error = %v, want not found", err)
	}
	if err := store.Delete(ctx, service.ID); !errors.Is(err, httpapi.ErrNotFound) {
		t.Fatalf("Delete(missing) error = %v, want not found", err)
	}
}
