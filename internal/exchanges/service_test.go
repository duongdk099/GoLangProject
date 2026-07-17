package exchanges

import (
	"context"
	"errors"
	"testing"

	"barterswap/internal/credits"
	"barterswap/internal/services"
	"barterswap/pkg/httpapi"
)

type memoryStore struct {
	nextID     int
	exchanges  map[int]stored
	balances   map[int]int
	ledger     []credits.Entry
	balanceErr error
}

type stored struct {
	Exchange
	cost int
}

func newMemoryStore() *memoryStore {
	return &memoryStore{nextID: 1, exchanges: make(map[int]stored), balances: make(map[int]int)}
}

func (s *memoryStore) grant(userID, amount int) { s.balances[userID] += amount }

func (s *memoryStore) record(entry credits.Entry) {
	s.ledger = append(s.ledger, entry)
}

func (s *memoryStore) Create(_ context.Context, params CreateParams) (Exchange, error) {
	for _, existing := range s.exchanges {
		if existing.ServiceID == params.ServiceID &&
			(existing.Status == StatusPending || existing.Status == StatusAccepted) {
			return Exchange{}, httpapi.ErrConflict
		}
	}
	exchange := Exchange{
		ID:          s.nextID,
		ServiceID:   params.ServiceID,
		RequesterID: params.RequesterID,
		OwnerID:     params.OwnerID,
		Status:      StatusPending,
		CreatedAt:   "2026-01-01T00:00:00Z",
		UpdatedAt:   "2026-01-01T00:00:00Z",
	}
	s.nextID++
	s.exchanges[exchange.ID] = stored{Exchange: exchange, cost: params.Cost}
	return exchange, nil
}

func (s *memoryStore) GetByID(_ context.Context, exchangeID int) (Exchange, error) {
	exchange, ok := s.exchanges[exchangeID]
	if !ok {
		return Exchange{}, httpapi.ErrNotFound
	}
	return exchange.Exchange, nil
}

func (s *memoryStore) List(_ context.Context, filter Filter) ([]Exchange, error) {
	results := make([]Exchange, 0)
	for _, exchange := range s.exchanges {
		if exchange.RequesterID != filter.UserID && exchange.OwnerID != filter.UserID {
			continue
		}
		if filter.Status != "" && exchange.Status != filter.Status {
			continue
		}
		results = append(results, exchange.Exchange)
	}
	return results, nil
}

func (s *memoryStore) Balance(_ context.Context, userID int) (int, error) {
	if s.balanceErr != nil {
		return 0, s.balanceErr
	}
	return s.balances[userID], nil
}

func (s *memoryStore) CountCompleted(_ context.Context, userID int) (int, error) {
	count := 0
	for _, exchange := range s.exchanges {
		if exchange.Status == StatusCompleted && (exchange.RequesterID == userID || exchange.OwnerID == userID) {
			count++
		}
	}
	return count, nil
}

func (s *memoryStore) Accept(_ context.Context, exchangeID, ownerID int) (Exchange, error) {
	exchange, ok := s.exchanges[exchangeID]
	if !ok {
		return Exchange{}, httpapi.ErrNotFound
	}
	if exchange.OwnerID != ownerID {
		return Exchange{}, httpapi.ErrForbidden
	}
	if exchange.Status != StatusPending {
		return Exchange{}, httpapi.ErrValidation
	}
	if s.balances[exchange.RequesterID] < exchange.cost {
		return Exchange{}, httpapi.ErrValidation
	}
	s.balances[exchange.RequesterID] -= exchange.cost
	s.record(credits.Entry{UserID: exchange.RequesterID, ExchangeID: exchange.ID, Amount: exchange.cost, Type: credits.TypeSpend})
	return s.setStatus(exchangeID, StatusAccepted), nil
}

func (s *memoryStore) Reject(_ context.Context, exchangeID, ownerID int) (Exchange, error) {
	exchange, ok := s.exchanges[exchangeID]
	if !ok {
		return Exchange{}, httpapi.ErrNotFound
	}
	if exchange.OwnerID != ownerID {
		return Exchange{}, httpapi.ErrForbidden
	}
	if exchange.Status != StatusPending {
		return Exchange{}, httpapi.ErrValidation
	}
	return s.setStatus(exchangeID, StatusRejected), nil
}

func (s *memoryStore) Complete(_ context.Context, exchangeID, requesterID int) (Exchange, error) {
	exchange, ok := s.exchanges[exchangeID]
	if !ok {
		return Exchange{}, httpapi.ErrNotFound
	}
	if exchange.RequesterID != requesterID {
		return Exchange{}, httpapi.ErrForbidden
	}
	if exchange.Status != StatusAccepted {
		return Exchange{}, httpapi.ErrValidation
	}
	s.balances[exchange.OwnerID] += exchange.cost
	s.record(credits.Entry{UserID: exchange.OwnerID, ExchangeID: exchange.ID, Amount: exchange.cost, Type: credits.TypeEarn})
	return s.setStatus(exchangeID, StatusCompleted), nil
}

func (s *memoryStore) Cancel(_ context.Context, exchangeID, actorID int) (Exchange, error) {
	exchange, ok := s.exchanges[exchangeID]
	if !ok {
		return Exchange{}, httpapi.ErrNotFound
	}
	if actorID != exchange.RequesterID && actorID != exchange.OwnerID {
		return Exchange{}, httpapi.ErrForbidden
	}
	switch exchange.Status {
	case StatusPending:
	case StatusAccepted:
		s.balances[exchange.RequesterID] += exchange.cost
		s.record(credits.Entry{UserID: exchange.RequesterID, ExchangeID: exchange.ID, Amount: exchange.cost, Type: credits.TypeRefund})
	default:
		return Exchange{}, httpapi.ErrValidation
	}
	return s.setStatus(exchangeID, StatusCancelled), nil
}

func (s *memoryStore) setStatus(exchangeID int, status string) Exchange {
	exchange := s.exchanges[exchangeID]
	exchange.Status = status
	s.exchanges[exchangeID] = exchange
	return exchange.Exchange
}

func (s *memoryStore) countLedger(exchangeID int, entryType string) int {
	count := 0
	for _, entry := range s.ledger {
		if entry.ExchangeID == exchangeID && entry.Type == entryType {
			count++
		}
	}
	return count
}

type stubServices struct {
	services map[int]services.Service
	err      error
}

func (s stubServices) Get(_ context.Context, serviceID int) (services.Service, error) {
	if s.err != nil {
		return services.Service{}, s.err
	}
	service, ok := s.services[serviceID]
	if !ok {
		return services.Service{}, httpapi.ErrNotFound
	}
	return service, nil
}

type stubUsers struct {
	existing map[int]bool
	err      error
}

func (s stubUsers) UserExists(_ context.Context, userID int) (bool, error) {
	if s.err != nil {
		return false, s.err
	}
	return s.existing[userID], nil
}

func fixture(t *testing.T) (*UseCases, *memoryStore) {
	t.Helper()
	store := newMemoryStore()
	svc := stubServices{services: map[int]services.Service{
		1: {ID: 1, ProviderID: 2, Titre: "Cours de Go", Categorie: "Informatique", Credits: 2, Actif: true},
	}}
	usr := stubUsers{existing: map[int]bool{1: true, 2: true}}
	return NewUseCases(store, svc, usr), store
}

func TestCreateValid(t *testing.T) {
	useCases, store := fixture(t)
	store.grant(1, 10)

	exchange, err := useCases.Create(context.Background(), 1, CreateRequest{ServiceID: 1})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if exchange.RequesterID != 1 || exchange.OwnerID != 2 || exchange.Status != StatusPending {
		t.Fatalf("Create() = %+v", exchange)
	}
}

func TestCreateRejectsOwnService(t *testing.T) {
	useCases, store := fixture(t)
	store.grant(2, 10)

	if _, err := useCases.Create(context.Background(), 2, CreateRequest{ServiceID: 1}); !errors.Is(err, httpapi.ErrValidation) {
		t.Fatalf("Create(own service) error = %v, want validation", err)
	}
}

func TestCreateRejectsInsufficientCredits(t *testing.T) {
	useCases, store := fixture(t)
	store.grant(1, 1)

	if _, err := useCases.Create(context.Background(), 1, CreateRequest{ServiceID: 1}); !errors.Is(err, httpapi.ErrValidation) {
		t.Fatalf("Create(insufficient) error = %v, want validation", err)
	}
}

func TestCreateRejectsInactiveService(t *testing.T) {
	useCases, store := fixture(t)
	store.grant(1, 10)
	svc := useCases.services.(stubServices)
	svc.services[1] = services.Service{ID: 1, ProviderID: 2, Credits: 2, Actif: false}

	if _, err := useCases.Create(context.Background(), 1, CreateRequest{ServiceID: 1}); !errors.Is(err, httpapi.ErrValidation) {
		t.Fatalf("Create(inactive) error = %v, want validation", err)
	}
}

func TestCreateConflictWhenAlreadyReserved(t *testing.T) {
	useCases, store := fixture(t)
	store.grant(1, 10)
	store.grant(3, 10)
	store.existingUser(useCases, 3)

	if _, err := useCases.Create(context.Background(), 1, CreateRequest{ServiceID: 1}); err != nil {
		t.Fatalf("first Create() error = %v", err)
	}
	if _, err := useCases.Create(context.Background(), 3, CreateRequest{ServiceID: 1}); !errors.Is(err, httpapi.ErrConflict) {
		t.Fatalf("second Create() error = %v, want conflict", err)
	}
}

func (s *memoryStore) existingUser(useCases *UseCases, userID int) {
	useCases.users.(stubUsers).existing[userID] = true
}

func TestAcceptOnlyOwnerAndDebitsOnce(t *testing.T) {
	useCases, store := fixture(t)
	store.grant(1, 10)
	created, _ := useCases.Create(context.Background(), 1, CreateRequest{ServiceID: 1})

	if _, err := useCases.Accept(context.Background(), 1, created.ID); !errors.Is(err, httpapi.ErrForbidden) {
		t.Fatalf("Accept(non owner) error = %v, want forbidden", err)
	}

	accepted, err := useCases.Accept(context.Background(), 2, created.ID)
	if err != nil || accepted.Status != StatusAccepted {
		t.Fatalf("Accept() = %+v, err = %v", accepted, err)
	}
	if store.balances[1] != 8 {
		t.Fatalf("requester balance = %d, want 8", store.balances[1])
	}

	if _, err := useCases.Accept(context.Background(), 2, created.ID); !errors.Is(err, httpapi.ErrValidation) {
		t.Fatalf("Accept(again) error = %v, want validation", err)
	}
	if store.balances[1] != 8 {
		t.Fatalf("requester balance after repeat = %d, want 8", store.balances[1])
	}
	if got := store.countLedger(created.ID, credits.TypeSpend); got != 1 {
		t.Fatalf("spend entries = %d, want 1", got)
	}
}

func TestCompleteCreditsOwnerOnce(t *testing.T) {
	useCases, store := fixture(t)
	store.grant(1, 10)
	created, _ := useCases.Create(context.Background(), 1, CreateRequest{ServiceID: 1})

	if _, err := useCases.Complete(context.Background(), 1, created.ID); !errors.Is(err, httpapi.ErrValidation) {
		t.Fatalf("Complete(pending) error = %v, want validation", err)
	}

	if _, err := useCases.Accept(context.Background(), 2, created.ID); err != nil {
		t.Fatalf("Accept() error = %v", err)
	}

	if _, err := useCases.Complete(context.Background(), 2, created.ID); !errors.Is(err, httpapi.ErrForbidden) {
		t.Fatalf("Complete(owner) error = %v, want forbidden", err)
	}

	completed, err := useCases.Complete(context.Background(), 1, created.ID)
	if err != nil || completed.Status != StatusCompleted {
		t.Fatalf("Complete() = %+v, err = %v", completed, err)
	}
	if store.balances[2] != 2 {
		t.Fatalf("owner balance = %d, want 2", store.balances[2])
	}

	if _, err := useCases.Complete(context.Background(), 1, created.ID); !errors.Is(err, httpapi.ErrValidation) {
		t.Fatalf("Complete(again) error = %v, want validation", err)
	}
	if store.balances[2] != 2 {
		t.Fatalf("owner balance after repeat = %d, want 2", store.balances[2])
	}
	if got := store.countLedger(created.ID, credits.TypeEarn); got != 1 {
		t.Fatalf("earn entries = %d, want 1", got)
	}
}

func TestCancelAcceptedRefundsOnce(t *testing.T) {
	useCases, store := fixture(t)
	store.grant(1, 10)
	created, _ := useCases.Create(context.Background(), 1, CreateRequest{ServiceID: 1})
	if _, err := useCases.Accept(context.Background(), 2, created.ID); err != nil {
		t.Fatalf("Accept() error = %v", err)
	}
	if store.balances[1] != 8 {
		t.Fatalf("requester balance after accept = %d, want 8", store.balances[1])
	}

	cancelled, err := useCases.Cancel(context.Background(), 1, created.ID)
	if err != nil || cancelled.Status != StatusCancelled {
		t.Fatalf("Cancel() = %+v, err = %v", cancelled, err)
	}
	if store.balances[1] != 10 {
		t.Fatalf("requester balance after refund = %d, want 10", store.balances[1])
	}
	if got := store.countLedger(created.ID, credits.TypeRefund); got != 1 {
		t.Fatalf("refund entries = %d, want 1", got)
	}
}

func TestRejectPendingDoesNotRefund(t *testing.T) {
	useCases, store := fixture(t)
	store.grant(1, 10)
	created, _ := useCases.Create(context.Background(), 1, CreateRequest{ServiceID: 1})

	if _, err := useCases.Reject(context.Background(), 1, created.ID); !errors.Is(err, httpapi.ErrForbidden) {
		t.Fatalf("Reject(non owner) error = %v, want forbidden", err)
	}

	rejected, err := useCases.Reject(context.Background(), 2, created.ID)
	if err != nil || rejected.Status != StatusRejected {
		t.Fatalf("Reject() = %+v, err = %v", rejected, err)
	}

	if store.balances[1] != 10 {
		t.Fatalf("requester balance = %d, want 10", store.balances[1])
	}
	if got := store.countLedger(created.ID, credits.TypeRefund); got != 0 {
		t.Fatalf("refund entries = %d, want 0", got)
	}
}

func TestGetVisibleOnlyToParticipants(t *testing.T) {
	useCases, store := fixture(t)
	store.grant(1, 10)
	created, _ := useCases.Create(context.Background(), 1, CreateRequest{ServiceID: 1})

	if _, err := useCases.Get(context.Background(), 3, created.ID); !errors.Is(err, httpapi.ErrForbidden) {
		t.Fatalf("Get(outsider) error = %v, want forbidden", err)
	}
	if _, err := useCases.Get(context.Background(), 1, created.ID); err != nil {
		t.Fatalf("Get(requester) error = %v", err)
	}
	if _, err := useCases.Get(context.Background(), 2, created.ID); err != nil {
		t.Fatalf("Get(owner) error = %v", err)
	}
}

func TestListFiltersByUserAndStatus(t *testing.T) {
	useCases, store := fixture(t)
	store.grant(1, 10)
	first, _ := useCases.Create(context.Background(), 1, CreateRequest{ServiceID: 1})
	if _, err := useCases.Accept(context.Background(), 2, first.ID); err != nil {
		t.Fatalf("Accept() error = %v", err)
	}

	all, err := useCases.List(context.Background(), 1, "")
	if err != nil || len(all) != 1 {
		t.Fatalf("List(user) = %+v, err = %v", all, err)
	}

	accepted, err := useCases.List(context.Background(), 1, StatusAccepted)
	if err != nil || len(accepted) != 1 {
		t.Fatalf("List(status=accepted) = %+v, err = %v", accepted, err)
	}

	pending, err := useCases.List(context.Background(), 1, StatusPending)
	if err != nil || len(pending) != 0 {
		t.Fatalf("List(status=pending) = %+v, err = %v", pending, err)
	}

	outsider, err := useCases.List(context.Background(), 3, "")
	if err != nil || len(outsider) != 0 {
		t.Fatalf("List(outsider) = %+v, err = %v", outsider, err)
	}

	if _, err := useCases.List(context.Background(), 1, "unknown"); !errors.Is(err, httpapi.ErrValidation) {
		t.Fatalf("List(bad status) error = %v, want validation", err)
	}
}
