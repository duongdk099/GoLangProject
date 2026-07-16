package stats

import (
	"context"
	"errors"
	"testing"

	"barterswap/pkg/httpapi"
)

type stubStore struct {
	activeServices int
	balance        int
	earned         int
	spent          int
	average        float64
	reviewCount    int
}

func (s *stubStore) CountActiveServices(_ context.Context, _ int) (int, error) {
	return s.activeServices, nil
}

func (s *stubStore) CreditBalance(_ context.Context, _ int) (int, error) {
	return s.balance, nil
}

func (s *stubStore) CreditTotals(_ context.Context, _ int) (int, int, error) {
	return s.earned, s.spent, nil
}

func (s *stubStore) ReviewAggregate(_ context.Context, _ int) (float64, int, error) {
	return s.average, s.reviewCount, nil
}

type stubExchangeStatsProvider struct {
	completed int
}

func (s *stubExchangeStatsProvider) CountCompletedExchanges(_ context.Context, _ int) (int, error) {
	return s.completed, nil
}

type stubUserExistenceChecker struct {
	exists map[int]bool
}

func (s *stubUserExistenceChecker) UserExists(_ context.Context, userID int) (bool, error) {
	return s.exists[userID], nil
}

func TestUseCasesGet(t *testing.T) {
	store := &stubStore{activeServices: 2, balance: 15, earned: 20, spent: 5, average: 4.5, reviewCount: 2}
	exchanges := &stubExchangeStatsProvider{completed: 3}
	users := &stubUserExistenceChecker{exists: map[int]bool{1: true}}
	useCases := NewUseCases(store, exchanges, users)

	stats, err := useCases.Get(context.Background(), 1)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	want := UserStats{
		UserID: 1, ServicesActifs: 2, EchangesCompletes: 3, CreditBalance: 15,
		NoteMoyenne: 4.5, NbAvis: 2, TotalGagne: 20, TotalDepense: 5,
	}
	if stats != want {
		t.Fatalf("Get() = %+v, want %+v", stats, want)
	}

	if _, err := useCases.Get(context.Background(), 999); !errors.Is(err, httpapi.ErrNotFound) {
		t.Fatalf("Get(missing) error = %v, want not found", err)
	}
	if _, err := useCases.Get(context.Background(), 0); !errors.Is(err, httpapi.ErrValidation) {
		t.Fatalf("Get(invalid) error = %v, want validation", err)
	}
}

func TestUseCasesZeroData(t *testing.T) {
	store := &stubStore{}
	exchanges := &stubExchangeStatsProvider{}
	users := &stubUserExistenceChecker{exists: map[int]bool{1: true}}
	useCases := NewUseCases(store, exchanges, users)

	stats, err := useCases.Get(context.Background(), 1)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if stats != (UserStats{UserID: 1}) {
		t.Fatalf("Get() zero data = %+v", stats)
	}
}
