package stats

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"testing"

	"barterswap/internal/testutil"
	"barterswap/pkg/httpapi"
)

var errBoom = errors.New("boom")

type errStore struct {
	*stubStore
	activeErr  error
	balanceErr error
	totalsErr  error
	reviewErr  error
}

func (s errStore) CountActiveServices(ctx context.Context, id int) (int, error) {
	if s.activeErr != nil {
		return 0, s.activeErr
	}
	return s.stubStore.CountActiveServices(ctx, id)
}

func (s errStore) CreditBalance(ctx context.Context, id int) (int, error) {
	if s.balanceErr != nil {
		return 0, s.balanceErr
	}
	return s.stubStore.CreditBalance(ctx, id)
}

func (s errStore) CreditTotals(ctx context.Context, id int) (int, int, error) {
	if s.totalsErr != nil {
		return 0, 0, s.totalsErr
	}
	return s.stubStore.CreditTotals(ctx, id)
}

func (s errStore) ReviewAggregate(ctx context.Context, id int) (float64, int, error) {
	if s.reviewErr != nil {
		return 0, 0, s.reviewErr
	}
	return s.stubStore.ReviewAggregate(ctx, id)
}

type errUserChecker struct{ err error }

func (c errUserChecker) UserExists(context.Context, int) (bool, error) {
	return false, c.err
}

type errExchangeProvider struct{ err error }

func (p errExchangeProvider) CountCompletedExchanges(context.Context, int) (int, error) {
	return 0, p.err
}

func TestGetPropagatesDependencyErrors(t *testing.T) {
	ctx := context.Background()
	usersOK := &stubUserExistenceChecker{exists: map[int]bool{1: true}}
	exchangesOK := &stubExchangeStatsProvider{}

	tests := []struct {
		name      string
		useCases  *UseCases
		wantError error
	}{
		{
			name:      "user existence check error",
			useCases:  NewUseCases(&stubStore{}, exchangesOK, errUserChecker{err: errBoom}),
			wantError: errBoom,
		},
		{
			name:      "active services error",
			useCases:  NewUseCases(errStore{stubStore: &stubStore{}, activeErr: errBoom}, exchangesOK, usersOK),
			wantError: errBoom,
		},
		{
			name:      "credit balance error",
			useCases:  NewUseCases(errStore{stubStore: &stubStore{}, balanceErr: errBoom}, exchangesOK, usersOK),
			wantError: errBoom,
		},
		{
			name:      "credit totals error",
			useCases:  NewUseCases(errStore{stubStore: &stubStore{}, totalsErr: errBoom}, exchangesOK, usersOK),
			wantError: errBoom,
		},
		{
			name:      "review aggregate error",
			useCases:  NewUseCases(errStore{stubStore: &stubStore{}, reviewErr: errBoom}, exchangesOK, usersOK),
			wantError: errBoom,
		},
		{
			name:      "completed exchanges error",
			useCases:  NewUseCases(&stubStore{}, errExchangeProvider{err: errBoom}, usersOK),
			wantError: errBoom,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := test.useCases.Get(ctx, 1); !errors.Is(err, test.wantError) {
				t.Fatalf("Get() error = %v, want %v", err, test.wantError)
			}
		})
	}
}

func TestHTTPInvalidPathID(t *testing.T) {
	store := &stubStore{}
	users := &stubUserExistenceChecker{exists: map[int]bool{1: true}}
	useCases := NewUseCases(store, &stubExchangeStatsProvider{}, users)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := httpapi.NewApplicationHandler(logger, NewHandler(useCases))

	if r := testutil.PerformRequest(handler, http.MethodGet, "/api/users/abc/stats", "", ""); r.Code != http.StatusBadRequest {
		t.Fatalf("GET stats invalid path id status = %d, want 400", r.Code)
	}
}
