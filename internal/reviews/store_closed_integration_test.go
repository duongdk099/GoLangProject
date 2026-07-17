package reviews

import (
	"context"
	"os"
	"testing"
	"time"

	"barterswap/internal/database"
)

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

	if _, err := store.Create(ctx, CreateParams{ExchangeID: 1, ServiceID: 1, AuthorID: 1, TargetID: 2, Note: 5}); err == nil {
		t.Fatal("Create() on closed database expected an error")
	}
	if _, err := store.ExistsForAuthor(ctx, 1, 1); err == nil {
		t.Fatal("ExistsForAuthor() on closed database expected an error")
	}
	if _, err := store.ListByTarget(ctx, 1); err == nil {
		t.Fatal("ListByTarget() on closed database expected an error")
	}
	if _, err := store.ListByService(ctx, 1); err == nil {
		t.Fatal("ListByService() on closed database expected an error")
	}
}
