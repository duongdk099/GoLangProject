package exchanges

import (
	"context"
	"fmt"
	"strings"

	"barterswap/internal/reviews"
	"barterswap/internal/services"
	"barterswap/pkg/httpapi"
)

// Store persists exchanges and performs the atomic status-and-credit
// transitions. Accept, Reject, Complete, and Cancel each run in a single SQL
// transaction that locks the exchange row, re-checks its state, moves credits
// when required, and updates the status, so a repeated call cannot transfer
// credits a second time and two callers cannot both win a transition.
type Store interface {
	Create(ctx context.Context, params CreateParams) (Exchange, error)
	GetByID(ctx context.Context, exchangeID int) (Exchange, error)
	List(ctx context.Context, filter Filter) ([]Exchange, error)
	Accept(ctx context.Context, exchangeID, ownerID int) (Exchange, error)
	Reject(ctx context.Context, exchangeID, ownerID int) (Exchange, error)
	Complete(ctx context.Context, exchangeID, requesterID int) (Exchange, error)
	Cancel(ctx context.Context, exchangeID, actorID int) (Exchange, error)
	CountCompleted(ctx context.Context, userID int) (int, error)
	Balance(ctx context.Context, userID int) (int, error)
}

// ServiceLookup is the slice of services.UseCases this feature needs to load a
// service's owner, price, and availability. It is declared here, on the
// consumer side, so this package does not depend on the service store; the
// services.Service type is imported for its fields only (one direction, no
// cycle).
type ServiceLookup interface {
	Get(ctx context.Context, serviceID int) (services.Service, error)
}

// RequesterChecker verifies the authenticated requester exists before an
// exchange is created. It is satisfied by *users.Service.
type RequesterChecker interface {
	UserExists(ctx context.Context, userID int) (bool, error)
}

type UseCases struct {
	store    Store
	services ServiceLookup
	users    RequesterChecker
}

func NewUseCases(store Store, svc ServiceLookup, users RequesterChecker) *UseCases {
	return &UseCases{store: store, services: svc, users: users}
}

// Create records a pending request for a service on behalf of the
// authenticated requester.
func (u *UseCases) Create(ctx context.Context, requesterID int, request CreateRequest) (Exchange, error) {
	if request.ServiceID <= 0 {
		return Exchange{}, fmt.Errorf("%w: service_id must be positive", httpapi.ErrValidation)
	}

	exists, err := u.users.UserExists(ctx, requesterID)
	if err != nil {
		return Exchange{}, err
	}
	if !exists {
		return Exchange{}, fmt.Errorf("%w: the authenticated user does not exist", httpapi.ErrValidation)
	}

	service, err := u.services.Get(ctx, request.ServiceID)
	if err != nil {
		return Exchange{}, err
	}
	if !service.Actif {
		return Exchange{}, fmt.Errorf("%w: this service is not available", httpapi.ErrValidation)
	}
	if service.ProviderID == requesterID {
		return Exchange{}, fmt.Errorf("%w: you cannot request your own service", httpapi.ErrValidation)
	}

	// Early affordability check for a clear 400. The acceptance transaction
	// re-checks the balance under a row lock, since credits can be spent
	// elsewhere between requesting and acceptance.
	balance, err := u.store.Balance(ctx, requesterID)
	if err != nil {
		return Exchange{}, err
	}
	if balance < service.Credits {
		return Exchange{}, fmt.Errorf("%w: insufficient credits to request this service", httpapi.ErrValidation)
	}

	return u.store.Create(ctx, CreateParams{
		ServiceID:   service.ID,
		RequesterID: requesterID,
		OwnerID:     service.ProviderID,
		Cost:        service.Credits,
	})
}

// List returns the exchanges the authenticated user takes part in, optionally
// filtered by status.
func (u *UseCases) List(ctx context.Context, userID int, status string) ([]Exchange, error) {
	status = strings.TrimSpace(status)
	if status != "" && !validStatus(status) {
		return nil, fmt.Errorf("%w: unknown status filter %q", httpapi.ErrValidation, status)
	}
	exchanges, err := u.store.List(ctx, Filter{UserID: userID, Status: status})
	if err != nil {
		return nil, err
	}
	if exchanges == nil {
		exchanges = []Exchange{}
	}
	return exchanges, nil
}

// Get returns one exchange, visible only to its participants.
func (u *UseCases) Get(ctx context.Context, actorID, exchangeID int) (Exchange, error) {
	exchange, err := u.store.GetByID(ctx, exchangeID)
	if err != nil {
		return Exchange{}, err
	}
	if actorID != exchange.RequesterID && actorID != exchange.OwnerID {
		return Exchange{}, fmt.Errorf("%w: only a participant may view this exchange", httpapi.ErrForbidden)
	}
	return exchange, nil
}

// Accept blocks the service price from the requester and moves a pending
// request to accepted. Only the service owner may accept.
func (u *UseCases) Accept(ctx context.Context, actorID, exchangeID int) (Exchange, error) {
	return u.store.Accept(ctx, exchangeID, actorID)
}

// Reject declines a pending request. No credit has been blocked yet, so
// nothing is refunded. Only the service owner may reject.
func (u *UseCases) Reject(ctx context.Context, actorID, exchangeID int) (Exchange, error) {
	return u.store.Reject(ctx, exchangeID, actorID)
}

// Complete finishes an accepted exchange and releases the blocked credits to
// the service owner. Completion is confirmed by the requester (the party who
// received the service), which is what releases their blocked credits.
func (u *UseCases) Complete(ctx context.Context, actorID, exchangeID int) (Exchange, error) {
	return u.store.Complete(ctx, exchangeID, actorID)
}

// Cancel cancels a pending or accepted exchange. When the exchange was already
// accepted the blocked credits are refunded to the requester; a still-pending
// exchange is cancelled without any refund. Either participant may cancel.
func (u *UseCases) Cancel(ctx context.Context, actorID, exchangeID int) (Exchange, error) {
	return u.store.Cancel(ctx, exchangeID, actorID)
}

// GetExchange satisfies reviews.ExchangeLookup so the reviews feature can
// validate that a review targets a completed exchange the author took part in.
func (u *UseCases) GetExchange(ctx context.Context, exchangeID int) (reviews.ExchangeSummary, error) {
	exchange, err := u.store.GetByID(ctx, exchangeID)
	if err != nil {
		return reviews.ExchangeSummary{}, err
	}
	return reviews.ExchangeSummary{
		ID:          exchange.ID,
		ServiceID:   exchange.ServiceID,
		RequesterID: exchange.RequesterID,
		OwnerID:     exchange.OwnerID,
		Status:      exchange.Status,
	}, nil
}

// CountCompletedExchanges satisfies stats.ExchangeStatsProvider.
func (u *UseCases) CountCompletedExchanges(ctx context.Context, userID int) (int, error) {
	return u.store.CountCompleted(ctx, userID)
}
