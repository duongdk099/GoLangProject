package exchanges

import (
	"context"
	"fmt"
	"strings"

	"barterswap/internal/reviews"
	"barterswap/internal/services"
	"barterswap/pkg/httpapi"
)

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

type ServiceLookup interface {
	Get(ctx context.Context, serviceID int) (services.Service, error)
}

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

func (u *UseCases) Accept(ctx context.Context, actorID, exchangeID int) (Exchange, error) {
	return u.store.Accept(ctx, exchangeID, actorID)
}

func (u *UseCases) Reject(ctx context.Context, actorID, exchangeID int) (Exchange, error) {
	return u.store.Reject(ctx, exchangeID, actorID)
}

func (u *UseCases) Complete(ctx context.Context, actorID, exchangeID int) (Exchange, error) {
	return u.store.Complete(ctx, exchangeID, actorID)
}

func (u *UseCases) Cancel(ctx context.Context, actorID, exchangeID int) (Exchange, error) {
	return u.store.Cancel(ctx, exchangeID, actorID)
}

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

func (u *UseCases) CountCompletedExchanges(ctx context.Context, userID int) (int, error) {
	return u.store.CountCompleted(ctx, userID)
}
