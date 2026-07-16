package users

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"barterswap/internal/database"
	"barterswap/pkg/httpapi"
)

func TestPostgresStoreIntegration(t *testing.T) {
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
	defer db.Close()
	if err := database.Migrate(ctx, db); err != nil {
		t.Fatalf("database.Migrate() error = %v", err)
	}

	store := NewPostgresStore(db)
	user, err := store.Create(ctx, CreateUserParams{
		Pseudo: "Integration Test User",
		Bio:    "Temporary",
		Ville:  "Paris",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	defer func() {
		if _, err := db.ExecContext(context.Background(), `DELETE FROM users WHERE id = $1`, user.ID); err != nil {
			t.Errorf("delete temporary user: %v", err)
		}
	}()
	if user.CreditBalance != WelcomeCredits {
		t.Fatalf("Create() balance = %d, want %d", user.CreditBalance, WelcomeCredits)
	}

	got, err := store.GetByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if got.Pseudo != user.Pseudo || got.CreditBalance != WelcomeCredits {
		t.Fatalf("GetByID() = %+v", got)
	}

	updated, err := store.Update(ctx, user.ID, UpdateUserParams{
		Pseudo: "Updated Integration User",
		Bio:    "Updated",
		Ville:  "Lyon",
	})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if updated.Pseudo != "Updated Integration User" {
		t.Fatalf("Update() = %+v", updated)
	}

	wantSkills := []Skill{
		{Nom: "Go", Niveau: "expert"},
		{Nom: "Cuisine", Niveau: "débutant"},
	}
	if err := store.ReplaceSkills(ctx, user.ID, wantSkills); err != nil {
		t.Fatalf("ReplaceSkills() error = %v", err)
	}
	skills, err := store.ListSkills(ctx, user.ID)
	if err != nil {
		t.Fatalf("ListSkills() error = %v", err)
	}
	if len(skills) != len(wantSkills) {
		t.Fatalf("ListSkills() = %+v", skills)
	}

	exists, err := store.Exists(ctx, user.ID)
	if err != nil || !exists {
		t.Fatalf("Exists() = %v, %v", exists, err)
	}
	hasSkill, err := store.HasSkill(ctx, user.ID, "go")
	if err != nil || !hasSkill {
		t.Fatalf("HasSkill() = %v, %v", hasSkill, err)
	}

	if _, err := store.GetByID(ctx, 2147483647); !errors.Is(err, httpapi.ErrNotFound) {
		t.Fatalf("GetByID(missing) error = %v, want not found", err)
	}
	if _, err := store.Update(ctx, 2147483647, UpdateUserParams{Pseudo: "Missing"}); !errors.Is(err, httpapi.ErrNotFound) {
		t.Fatalf("Update(missing) error = %v, want not found", err)
	}
	if err := store.ReplaceSkills(ctx, 2147483647, nil); !errors.Is(err, httpapi.ErrNotFound) {
		t.Fatalf("ReplaceSkills(missing) error = %v, want not found", err)
	}
}
