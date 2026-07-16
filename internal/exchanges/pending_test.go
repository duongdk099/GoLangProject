package exchanges

import (
	"context"
	"errors"
	"testing"

	"barterswap/pkg/httpapi"
)

func TestPendingIntegration(t *testing.T) {
	var integration PendingIntegration

	if _, err := integration.GetExchange(context.Background(), 1); !errors.Is(err, httpapi.ErrNotFound) {
		t.Fatalf("GetExchange() error = %v, want not found", err)
	}

	count, err := integration.CountCompletedExchanges(context.Background(), 1)
	if err != nil || count != 0 {
		t.Fatalf("CountCompletedExchanges() = %d, err = %v", count, err)
	}
}
