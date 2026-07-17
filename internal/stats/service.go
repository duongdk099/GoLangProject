package stats

import (
	"context"
	"fmt"

	"barterswap/pkg/httpapi"
)

type UserStats struct {
	UserID            int     `json:"user_id"`
	ServicesActifs    int     `json:"services_actifs"`
	EchangesCompletes int     `json:"echanges_completes"`
	CreditBalance     int     `json:"credit_balance"`
	NoteMoyenne       float64 `json:"note_moyenne"`
	NbAvis            int     `json:"nb_avis"`
	TotalGagne        int     `json:"total_gagne"`
	TotalDepense      int     `json:"total_depense"`
}

type Store interface {
	CountActiveServices(ctx context.Context, providerID int) (int, error)
	CreditBalance(ctx context.Context, userID int) (int, error)
	CreditTotals(ctx context.Context, userID int) (earned, spent int, err error)
	ReviewAggregate(ctx context.Context, targetID int) (average float64, count int, err error)
}

type ExchangeStatsProvider interface {
	CountCompletedExchanges(ctx context.Context, userID int) (int, error)
}

type UserExistenceChecker interface {
	UserExists(ctx context.Context, userID int) (bool, error)
}

type UseCases struct {
	store     Store
	exchanges ExchangeStatsProvider
	users     UserExistenceChecker
}

func NewUseCases(store Store, exchanges ExchangeStatsProvider, users UserExistenceChecker) *UseCases {
	return &UseCases{store: store, exchanges: exchanges, users: users}
}

func (s *UseCases) Get(ctx context.Context, userID int) (UserStats, error) {
	if userID <= 0 {
		return UserStats{}, fmt.Errorf("%w: user ID must be positive", httpapi.ErrValidation)
	}
	exists, err := s.users.UserExists(ctx, userID)
	if err != nil {
		return UserStats{}, err
	}
	if !exists {
		return UserStats{}, httpapi.ErrNotFound
	}

	stats := UserStats{UserID: userID}

	stats.ServicesActifs, err = s.store.CountActiveServices(ctx, userID)
	if err != nil {
		return UserStats{}, err
	}
	stats.CreditBalance, err = s.store.CreditBalance(ctx, userID)
	if err != nil {
		return UserStats{}, err
	}
	stats.TotalGagne, stats.TotalDepense, err = s.store.CreditTotals(ctx, userID)
	if err != nil {
		return UserStats{}, err
	}
	stats.NoteMoyenne, stats.NbAvis, err = s.store.ReviewAggregate(ctx, userID)
	if err != nil {
		return UserStats{}, err
	}
	stats.EchangesCompletes, err = s.exchanges.CountCompletedExchanges(ctx, userID)
	if err != nil {
		return UserStats{}, err
	}

	return stats, nil
}
