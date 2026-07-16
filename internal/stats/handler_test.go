package stats

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"testing"

	"barterswap/internal/testutil"
	"barterswap/pkg/httpapi"
)

func TestHTTP(t *testing.T) {
	store := &stubStore{activeServices: 1, balance: 8, earned: 10, spent: 2, average: 5, reviewCount: 1}
	users := &stubUserExistenceChecker{exists: map[int]bool{1: true}}
	useCases := NewUseCases(store, &stubExchangeStatsProvider{completed: 1}, users)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := httpapi.NewApplicationHandler(logger, NewHandler(useCases))

	ok := testutil.PerformRequest(handler, http.MethodGet, "/api/users/1/stats", "", "")
	if ok.Code != http.StatusOK {
		t.Fatalf("GET stats status = %d, body = %s", ok.Code, ok.Body.String())
	}
	var stats UserStats
	if err := json.NewDecoder(ok.Body).Decode(&stats); err != nil {
		t.Fatalf("decode stats: %v", err)
	}
	if stats.UserID != 1 || stats.CreditBalance != 8 || stats.EchangesCompletes != 1 {
		t.Fatalf("stats = %+v", stats)
	}

	missing := testutil.PerformRequest(handler, http.MethodGet, "/api/users/999/stats", "", "")
	if missing.Code != http.StatusNotFound {
		t.Fatalf("GET stats (missing user) status = %d", missing.Code)
	}
}
