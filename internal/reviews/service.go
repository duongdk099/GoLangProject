package reviews

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"barterswap/internal/services"
	"barterswap/pkg/httpapi"
)

type Store interface {
	Create(context.Context, CreateParams) (Review, error)
	ExistsForAuthor(ctx context.Context, exchangeID, authorID int) (bool, error)
	ListByTarget(ctx context.Context, targetID int) ([]Review, error)
	ListByService(ctx context.Context, serviceID int) ([]Review, error)
}

type ExchangeLookup interface {
	GetExchange(ctx context.Context, exchangeID int) (ExchangeSummary, error)
}

type ServiceExistenceChecker interface {
	Get(ctx context.Context, serviceID int) (services.Service, error)
}

type UseCases struct {
	store     Store
	exchanges ExchangeLookup
	services  ServiceExistenceChecker
}

func NewUseCases(store Store, exchanges ExchangeLookup, services ServiceExistenceChecker) *UseCases {
	return &UseCases{store: store, exchanges: exchanges, services: services}
}

func (r *UseCases) Create(ctx context.Context, actorID, exchangeID int, request CreateRequest) (Review, error) {
	if request.Note < 1 || request.Note > 5 {
		return Review{}, fmt.Errorf("%w: note must be between 1 and 5", httpapi.ErrValidation)
	}
	commentaire := strings.TrimSpace(request.Commentaire)
	if utf8.RuneCountInString(commentaire) > 1000 {
		return Review{}, fmt.Errorf("%w: commentaire cannot exceed 1000 characters", httpapi.ErrValidation)
	}

	exchange, err := r.exchanges.GetExchange(ctx, exchangeID)
	if err != nil {
		return Review{}, err
	}
	if exchange.Status != StatusCompleted {
		return Review{}, fmt.Errorf("%w: only a completed exchange can be reviewed", httpapi.ErrValidation)
	}

	var targetID int
	switch actorID {
	case exchange.RequesterID:
		targetID = exchange.OwnerID
	case exchange.OwnerID:
		targetID = exchange.RequesterID
	default:
		return Review{}, fmt.Errorf("%w: only exchange participants may leave a review", httpapi.ErrForbidden)
	}

	duplicate, err := r.store.ExistsForAuthor(ctx, exchangeID, actorID)
	if err != nil {
		return Review{}, err
	}
	if duplicate {
		return Review{}, fmt.Errorf("%w: you already reviewed this exchange", httpapi.ErrValidation)
	}

	return r.store.Create(ctx, CreateParams{
		ExchangeID:  exchangeID,
		ServiceID:   exchange.ServiceID,
		AuthorID:    actorID,
		TargetID:    targetID,
		Note:        request.Note,
		Commentaire: commentaire,
	})
}

func (r *UseCases) ListForUser(ctx context.Context, userID int) ([]Review, error) {
	if userID <= 0 {
		return nil, fmt.Errorf("%w: user ID must be positive", httpapi.ErrValidation)
	}
	reviews, err := r.store.ListByTarget(ctx, userID)
	if err != nil {
		return nil, err
	}
	if reviews == nil {
		reviews = []Review{}
	}
	return reviews, nil
}

func (r *UseCases) ListForService(ctx context.Context, serviceID int) ([]Review, error) {
	if _, err := r.services.Get(ctx, serviceID); err != nil {
		return nil, err
	}
	reviews, err := r.store.ListByService(ctx, serviceID)
	if err != nil {
		return nil, err
	}
	if reviews == nil {
		reviews = []Review{}
	}
	return reviews, nil
}
