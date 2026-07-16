package users

import (
	"context"
	"os"
	"testing"
	"time"

	"barterswap/internal/database"
)

// TestPostgresStoreConstraintErrors drives the in-transaction insert-error
// branches of Create and ReplaceSkills against a live database by violating the
// schema CHECK constraints directly (the store performs no validation of its
// own; the service does). This reaches error returns that a closed pool cannot,
// because the surrounding transaction opens successfully and only the inner
// statement fails.
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

	// A blank pseudo violates the users CHECK constraint, so the INSERT inside
	// the create transaction fails.
	if _, err := store.Create(ctx, CreateUserParams{Pseudo: "   "}); err == nil {
		t.Fatal("Create() with a blank pseudo expected an insert error")
	}

	// Seed a valid user so ReplaceSkills locks it successfully and only the
	// skill INSERT fails.
	user, err := store.Create(ctx, CreateUserParams{Pseudo: "Constraint Test User"})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	defer func() {
		if _, err := db.ExecContext(context.Background(), `DELETE FROM users WHERE id = $1`, user.ID); err != nil {
			t.Errorf("delete temporary user: %v", err)
		}
	}()

	// An invalid niveau violates the skills CHECK constraint, failing the INSERT
	// inside the replace-skills transaction.
	if err := store.ReplaceSkills(ctx, user.ID, []Skill{{Nom: "Go", Niveau: "grandmaster"}}); err == nil {
		t.Fatal("ReplaceSkills() with an invalid niveau expected an insert error")
	}
}
