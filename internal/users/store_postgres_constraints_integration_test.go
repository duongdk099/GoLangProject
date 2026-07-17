package users

import (
	"context"
	"os"
	"testing"
	"time"

	"barterswap/internal/database"
)

func TestPostgresStoreConstraintErrors(t *testing.T) {
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

	if _, err := store.Create(ctx, CreateUserParams{Pseudo: "   "}); err == nil {
		t.Fatal("Create() with a blank pseudo expected an insert error")
	}

	user, err := store.Create(ctx, CreateUserParams{Pseudo: "Constraint Test User"})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	defer func() {
		if _, err := db.ExecContext(context.Background(), `DELETE FROM users WHERE id = $1`, user.ID); err != nil {
			t.Errorf("delete temporary user: %v", err)
		}
	}()

	if err := store.ReplaceSkills(ctx, user.ID, []Skill{{Nom: "Go", Niveau: "grandmaster"}}); err == nil {
		t.Fatal("ReplaceSkills() with an invalid niveau expected an insert error")
	}
}
