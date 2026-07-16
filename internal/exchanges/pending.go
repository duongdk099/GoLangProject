// Package exchanges will own the exchange lifecycle and credit ledger
// (Person 3's scope: request/accept/reject/complete/cancel, credit
// blocking, reservation conflicts). It is not implemented yet.
//
// PendingIntegration is a temporary placeholder satisfying the
// reviews.ExchangeLookup and stats.ExchangeStatsProvider contracts that
// Person 2's reviews and statistics features depend on. It contains no
// business logic of its own: it exists only so the application compiles and
// runs before the real exchanges store lands.
//
// Replace the two lines in cmd/server/main.go that build
// exchanges.PendingIntegration{} with the real store once it exists; nothing
// else needs to change because both consumers depend on the interfaces, not
// on this type.
package exchanges

import (
	"context"

	"barterswap/internal/reviews"
	"barterswap/pkg/httpapi"
)

type PendingIntegration struct{}

func (PendingIntegration) GetExchange(context.Context, int) (reviews.ExchangeSummary, error) {
	return reviews.ExchangeSummary{}, httpapi.ErrNotFound
}

func (PendingIntegration) CountCompletedExchanges(context.Context, int) (int, error) {
	return 0, nil
}
