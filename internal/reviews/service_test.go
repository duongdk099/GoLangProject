package reviews

import (
	"context"
	"errors"
	"testing"

	"barterswap/internal/services"
	"barterswap/pkg/httpapi"
)

type memoryStore struct {
	nextID  int
	reviews map[int]Review
	service map[int]int // review ID -> service ID
}

func newMemoryStore() *memoryStore {
	return &memoryStore{nextID: 1, reviews: make(map[int]Review), service: make(map[int]int)}
}

func (s *memoryStore) Create(_ context.Context, params CreateParams) (Review, error) {
	review := Review{
		ID:          s.nextID,
		ExchangeID:  params.ExchangeID,
		AuthorID:    params.AuthorID,
		TargetID:    params.TargetID,
		Note:        params.Note,
		Commentaire: params.Commentaire,
		CreatedAt:   "2026-01-01T00:00:00Z",
	}
	s.service[review.ID] = params.ServiceID
	s.reviews[review.ID] = review
	s.nextID++
	return review, nil
}

func (s *memoryStore) ExistsForAuthor(_ context.Context, exchangeID, authorID int) (bool, error) {
	for _, review := range s.reviews {
		if review.ExchangeID == exchangeID && review.AuthorID == authorID {
			return true, nil
		}
	}
	return false, nil
}

func (s *memoryStore) ListByTarget(_ context.Context, targetID int) ([]Review, error) {
	results := make([]Review, 0)
	for _, review := range s.reviews {
		if review.TargetID == targetID {
			results = append(results, review)
		}
	}
	return results, nil
}

func (s *memoryStore) ListByService(_ context.Context, serviceID int) ([]Review, error) {
	results := make([]Review, 0)
	for id, review := range s.reviews {
		if s.service[id] == serviceID {
			results = append(results, review)
		}
	}
	return results, nil
}

type stubExchangeLookup struct {
	exchanges map[int]ExchangeSummary
}

func newStubExchangeLookup() *stubExchangeLookup {
	return &stubExchangeLookup{exchanges: make(map[int]ExchangeSummary)}
}

func (s *stubExchangeLookup) GetExchange(_ context.Context, exchangeID int) (ExchangeSummary, error) {
	exchange, exists := s.exchanges[exchangeID]
	if !exists {
		return ExchangeSummary{}, httpapi.ErrNotFound
	}
	return exchange, nil
}

type stubServiceExistenceChecker struct {
	services map[int]services.Service
}

func (s *stubServiceExistenceChecker) Get(_ context.Context, serviceID int) (services.Service, error) {
	service, exists := s.services[serviceID]
	if !exists {
		return services.Service{}, httpapi.ErrNotFound
	}
	return service, nil
}

func TestUseCasesCreate(t *testing.T) {
	exchanges := newStubExchangeLookup()
	exchanges.exchanges[1] = ExchangeSummary{ID: 1, ServiceID: 10, RequesterID: 2, OwnerID: 3, Status: StatusCompleted}
	exchanges.exchanges[2] = ExchangeSummary{ID: 2, ServiceID: 10, RequesterID: 2, OwnerID: 3, Status: "pending"}

	svc := &stubServiceExistenceChecker{services: map[int]services.Service{10: {ID: 10}}}
	store := newMemoryStore()
	useCases := NewUseCases(store, exchanges, svc)

	review, err := useCases.Create(context.Background(), 2, 1, CreateRequest{Note: 5, Commentaire: "Top"})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if review.TargetID != 3 || review.AuthorID != 2 {
		t.Fatalf("Create() = %+v", review)
	}

	if _, err := useCases.Create(context.Background(), 2, 1, CreateRequest{Note: 4}); !errors.Is(err, httpapi.ErrValidation) {
		t.Fatalf("Create(duplicate) error = %v, want validation", err)
	}

	if _, err := useCases.Create(context.Background(), 2, 2, CreateRequest{Note: 4}); !errors.Is(err, httpapi.ErrValidation) {
		t.Fatalf("Create(not completed) error = %v, want validation", err)
	}

	if _, err := useCases.Create(context.Background(), 99, 1, CreateRequest{Note: 4}); !errors.Is(err, httpapi.ErrForbidden) {
		t.Fatalf("Create(non participant) error = %v, want forbidden", err)
	}

	if _, err := useCases.Create(context.Background(), 2, 1, CreateRequest{Note: 6}); !errors.Is(err, httpapi.ErrValidation) {
		t.Fatalf("Create(invalid note) error = %v, want validation", err)
	}

	if _, err := useCases.Create(context.Background(), 2, 999, CreateRequest{Note: 4}); !errors.Is(err, httpapi.ErrNotFound) {
		t.Fatalf("Create(missing exchange) error = %v, want not found", err)
	}
}

func TestUseCasesListing(t *testing.T) {
	exchanges := newStubExchangeLookup()
	exchanges.exchanges[1] = ExchangeSummary{ID: 1, ServiceID: 10, RequesterID: 2, OwnerID: 3, Status: StatusCompleted}
	svc := &stubServiceExistenceChecker{services: map[int]services.Service{10: {ID: 10}}}
	store := newMemoryStore()
	useCases := NewUseCases(store, exchanges, svc)

	if _, err := useCases.Create(context.Background(), 2, 1, CreateRequest{Note: 5}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	forUser, err := useCases.ListForUser(context.Background(), 3)
	if err != nil || len(forUser) != 1 {
		t.Fatalf("ListForUser() = %+v, err = %v", forUser, err)
	}

	forService, err := useCases.ListForService(context.Background(), 10)
	if err != nil || len(forService) != 1 {
		t.Fatalf("ListForService() = %+v, err = %v", forService, err)
	}

	if _, err := useCases.ListForService(context.Background(), 999); !errors.Is(err, httpapi.ErrNotFound) {
		t.Fatalf("ListForService(missing) error = %v, want not found", err)
	}
}
