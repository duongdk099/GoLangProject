package stats

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

	if _, err := store.CountActiveServices(ctx, 1); err == nil {
		t.Fatal("CountActiveServices() on closed database expected an error")
	}
	if _, err := store.CreditBalance(ctx, 1); err == nil {
		t.Fatal("CreditBalance() on closed database expected an error")
	}
	if _, _, err := store.CreditTotals(ctx, 1); err == nil {
		t.Fatal("CreditTotals() on closed database expected an error")
	}
	if _, _, err := store.ReviewAggregate(ctx, 1); err == nil {
		t.Fatal("ReviewAggregate() on closed database expected an error")
	}
}
