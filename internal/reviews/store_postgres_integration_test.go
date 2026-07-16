package reviews

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
	author, err := userStore.Create(ctx, users.CreateUserParams{Pseudo: "Review Integration Author"})
	if err != nil {
		t.Fatalf("create author: %v", err)
	}
	target, err := userStore.Create(ctx, users.CreateUserParams{Pseudo: "Review Integration Target"})
	if err != nil {
		t.Fatalf("create target: %v", err)
	}
	t.Cleanup(func() {
		db.ExecContext(context.Background(), `DELETE FROM users WHERE id IN ($1, $2)`, author.ID, target.ID)
	})

	serviceStore := services.NewPostgresStore(db)
	service, err := serviceStore.Create(ctx, services.CreateParams{
		ProviderID: target.ID, Titre: "Cours", Categorie: "Informatique", DureeMinutes: 30, Credits: 1,
	})
	if err != nil {
		t.Fatalf("create service: %v", err)
	}

	store := NewPostgresStore(db)
	review, err := store.Create(ctx, CreateParams{
		ExchangeID: 1, ServiceID: service.ID, AuthorID: author.ID, TargetID: target.ID, Note: 5, Commentaire: "Top",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if _, err := store.Create(ctx, CreateParams{
		ExchangeID: 1, ServiceID: service.ID, AuthorID: author.ID, TargetID: target.ID, Note: 4,
	}); !errors.Is(err, httpapi.ErrValidation) {
		t.Fatalf("Create(duplicate) error = %v, want validation", err)
	}

	exists, err := store.ExistsForAuthor(ctx, 1, author.ID)
	if err != nil || !exists {
		t.Fatalf("ExistsForAuthor() = %v, %v", exists, err)
	}

	byTarget, err := store.ListByTarget(ctx, target.ID)
	if err != nil || len(byTarget) != 1 || byTarget[0].ID != review.ID {
		t.Fatalf("ListByTarget() = %+v, err = %v", byTarget, err)
	}

	byService, err := store.ListByService(ctx, service.ID)
	if err != nil || len(byService) != 1 {
		t.Fatalf("ListByService() = %+v, err = %v", byService, err)
	}
}
