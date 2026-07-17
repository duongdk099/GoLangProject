package users

import (
	"context"
	"os"
	"testing"
	"time"

	"barterswap/internal/database"
)

func TestPostgresStoreOnClosedDatabase(t *testing.T) {
	if os.Getenv("RUN_POSTGRES_INTEGRATION") != "1" {
		t.Skip("set RUN_POSTGRES_INTEGRATION=1 to run the PostgreSQL integration test")
	}
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Fatal("TEST_DATABASE_URL is required for the PostgreSQL integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	closedDB, err := database.Open(ctx, databaseURL)
	if err != nil {
		t.Fatalf("database.Open() error = %v", err)
	}
	closedDB.Close()
	store := NewPostgresStore(closedDB)

	background := context.Background()
	if _, err := store.Create(background, CreateUserParams{Pseudo: "Closed"}); err == nil {
		t.Fatal("Create() on closed database expected an error")
	}
	if _, err := store.GetByID(background, 1); err == nil {
		t.Fatal("GetByID() on closed database expected an error")
	}
	if _, err := store.Update(background, 1, UpdateUserParams{Pseudo: "Closed"}); err == nil {
		t.Fatal("Update() on closed database expected an error")
	}
	if _, err := store.ListSkills(background, 1); err == nil {
		t.Fatal("ListSkills() on closed database expected an error")
	}
	if err := store.ReplaceSkills(background, 1, []Skill{{Nom: "Go", Niveau: "expert"}}); err == nil {
		t.Fatal("ReplaceSkills() on closed database expected an error")
	}
	if _, err := store.Exists(background, 1); err == nil {
		t.Fatal("Exists() on closed database expected an error")
	}
	if _, err := store.HasSkill(background, 1, "go"); err == nil {
		t.Fatal("HasSkill() on closed database expected an error")
	}
}
