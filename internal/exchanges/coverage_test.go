package exchanges

import (
	"context"
	"errors"
	"testing"

	"barterswap/internal/services"
	"barterswap/pkg/httpapi"
)

var errBoom = errors.New("boom")

type listOverrideStore struct {
	*memoryStore
	list []Exchange
	err  error
}

func (s listOverrideStore) List(context.Context, Filter) ([]Exchange, error) {
	return s.list, s.err
}

func TestGetExchangeSummary(t *testing.T) {
	useCases, store := fixture(t)
	store.grant(1, 10)
	created, _ := useCases.Create(context.Background(), 1, CreateRequest{ServiceID: 1})

	summary, err := useCases.GetExchange(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("GetExchange() error = %v", err)
	}
	if summary.ID != created.ID || summary.ServiceID != 1 || summary.RequesterID != 1 || summary.OwnerID != 2 || summary.Status != StatusPending {
		t.Fatalf("GetExchange() = %+v", summary)
	}

	if _, err := useCases.GetExchange(context.Background(), 999); !errors.Is(err, httpapi.ErrNotFound) {
		t.Fatalf("GetExchange(missing) error = %v, want not found", err)
	}
}

func TestCountCompletedExchangesDelegates(t *testing.T) {
	useCases, store := fixture(t)
	store.grant(1, 10)
	created, _ := useCases.Create(context.Background(), 1, CreateRequest{ServiceID: 1})
	if _, err := useCases.Accept(context.Background(), 2, created.ID); err != nil {
		t.Fatalf("Accept() error = %v", err)
	}
	if _, err := useCases.Complete(context.Background(), 1, created.ID); err != nil {
		t.Fatalf("Complete() error = %v", err)
	}

	for _, userID := range []int{1, 2} {
		count, err := useCases.CountCompletedExchanges(context.Background(), userID)
		if err != nil || count != 1 {
			t.Fatalf("CountCompletedExchanges(%d) = %d, err = %v", userID, count, err)
		}
	}
}

func TestCreateRejectsNonPositiveServiceID(t *testing.T) {
	useCases, _ := fixture(t)
	if _, err := useCases.Create(context.Background(), 1, CreateRequest{ServiceID: 0}); !errors.Is(err, httpapi.ErrValidation) {
		t.Fatalf("Create(service_id=0) error = %v, want validation", err)
	}
}

func TestCreateRejectsMissingRequester(t *testing.T) {
	store := newMemoryStore()
	store.grant(4, 10)
	svc := stubServices{services: map[int]services.Service{1: {ID: 1, ProviderID: 2, Credits: 2, Actif: true}}}
	useCases := NewUseCases(store, svc, stubUsers{existing: map[int]bool{}})

	if _, err := useCases.Create(context.Background(), 4, CreateRequest{ServiceID: 1}); !errors.Is(err, httpapi.ErrValidation) {
		t.Fatalf("Create(missing requester) error = %v, want validation", err)
	}
}

func TestCreatePropagatesUnknownService(t *testing.T) {
	useCases, store := fixture(t)
	store.grant(1, 10)
	if _, err := useCases.Create(context.Background(), 1, CreateRequest{ServiceID: 42}); !errors.Is(err, httpapi.ErrNotFound) {
		t.Fatalf("Create(unknown service) error = %v, want not found", err)
	}
}

func TestCreatePropagatesDependencyErrors(t *testing.T) {

	store := newMemoryStore()
	svc := stubServices{services: map[int]services.Service{1: {ID: 1, ProviderID: 2, Credits: 2, Actif: true}}}
	usersErr := NewUseCases(store, svc, stubUsers{existing: map[int]bool{1: true}, err: errBoom})
	if _, err := usersErr.Create(context.Background(), 1, CreateRequest{ServiceID: 1}); !errors.Is(err, errBoom) {
		t.Fatalf("Create(user check error) error = %v, want boom", err)
	}

	balanceStore := newMemoryStore()
	balanceStore.balanceErr = errBoom
	balanceErrUseCases := NewUseCases(balanceStore, svc, stubUsers{existing: map[int]bool{1: true}})
	if _, err := balanceErrUseCases.Create(context.Background(), 1, CreateRequest{ServiceID: 1}); !errors.Is(err, errBoom) {
		t.Fatalf("Create(balance error) error = %v, want boom", err)
	}
}

func TestListNormalizesNilAndPropagatesError(t *testing.T) {
	_, store := fixture(t)

	nilUseCases := NewUseCases(listOverrideStore{memoryStore: store, list: nil}, stubServices{}, stubUsers{})
	got, err := nilUseCases.List(context.Background(), 1, "")
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if got == nil || len(got) != 0 {
		t.Fatalf("List() = %#v, want non-nil empty slice", got)
	}

	errUseCases := NewUseCases(listOverrideStore{memoryStore: store, err: errBoom}, stubServices{}, stubUsers{})
	if _, err := errUseCases.List(context.Background(), 1, ""); !errors.Is(err, errBoom) {
		t.Fatalf("List(store error) error = %v, want boom", err)
	}
}
