package reviews

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"barterswap/internal/services"
	"barterswap/internal/testutil"
	"barterswap/pkg/httpapi"
)

var errBoom = errors.New("boom")

type existsErrStore struct {
	*memoryStore
	err error
}

func (s existsErrStore) ExistsForAuthor(context.Context, int, int) (bool, error) {
	return false, s.err
}

type listTargetStore struct {
	*memoryStore
	list []Review
	err  error
}

func (s listTargetStore) ListByTarget(context.Context, int) ([]Review, error) {
	return s.list, s.err
}

type listServiceStore struct {
	*memoryStore
	list []Review
	err  error
}

func (s listServiceStore) ListByService(context.Context, int) ([]Review, error) {
	return s.list, s.err
}

func completedExchange() *stubExchangeLookup {
	exchanges := newStubExchangeLookup()
	exchanges.exchanges[1] = ExchangeSummary{ID: 1, ServiceID: 10, RequesterID: 2, OwnerID: 3, Status: StatusCompleted}
	return exchanges
}

func TestCreateRejectsTooLongComment(t *testing.T) {
	useCases := NewUseCases(newMemoryStore(), completedExchange(), &stubServiceExistenceChecker{services: map[int]services.Service{10: {ID: 10}}})
	if _, err := useCases.Create(context.Background(), 2, 1, CreateRequest{
		Note: 5, Commentaire: strings.Repeat("a", 1001),
	}); !errors.Is(err, httpapi.ErrValidation) {
		t.Fatalf("Create(too long comment) error = %v, want validation", err)
	}
}

func TestCreateByOwnerTargetsRequester(t *testing.T) {

	useCases := NewUseCases(newMemoryStore(), completedExchange(), &stubServiceExistenceChecker{services: map[int]services.Service{10: {ID: 10}}})
	review, err := useCases.Create(context.Background(), 3, 1, CreateRequest{Note: 4})
	if err != nil {
		t.Fatalf("Create(owner) error = %v", err)
	}
	if review.TargetID != 2 || review.AuthorID != 3 {
		t.Fatalf("Create(owner) = %+v, want target 2 author 3", review)
	}
}

func TestCreatePropagatesDuplicateCheckError(t *testing.T) {
	store := existsErrStore{memoryStore: newMemoryStore(), err: errBoom}
	useCases := NewUseCases(store, completedExchange(), &stubServiceExistenceChecker{services: map[int]services.Service{10: {ID: 10}}})
	if _, err := useCases.Create(context.Background(), 2, 1, CreateRequest{Note: 5}); !errors.Is(err, errBoom) {
		t.Fatalf("Create(duplicate check error) error = %v, want boom", err)
	}
}

func TestListForUserBranches(t *testing.T) {
	ctx := context.Background()
	svc := &stubServiceExistenceChecker{services: map[int]services.Service{10: {ID: 10}}}

	if _, err := NewUseCases(newMemoryStore(), completedExchange(), svc).ListForUser(ctx, 0); !errors.Is(err, httpapi.ErrValidation) {
		t.Fatalf("ListForUser(0) error = %v, want validation", err)
	}

	nilUseCases := NewUseCases(listTargetStore{memoryStore: newMemoryStore(), list: nil}, completedExchange(), svc)
	got, err := nilUseCases.ListForUser(ctx, 3)
	if err != nil {
		t.Fatalf("ListForUser() error = %v", err)
	}
	if got == nil || len(got) != 0 {
		t.Fatalf("ListForUser() = %#v, want non-nil empty slice", got)
	}

	errUseCases := NewUseCases(listTargetStore{memoryStore: newMemoryStore(), err: errBoom}, completedExchange(), svc)
	if _, err := errUseCases.ListForUser(ctx, 3); !errors.Is(err, errBoom) {
		t.Fatalf("ListForUser(store error) error = %v, want boom", err)
	}
}

func TestListForServiceBranches(t *testing.T) {
	ctx := context.Background()
	svc := &stubServiceExistenceChecker{services: map[int]services.Service{10: {ID: 10}}}

	nilUseCases := NewUseCases(listServiceStore{memoryStore: newMemoryStore(), list: nil}, completedExchange(), svc)
	got, err := nilUseCases.ListForService(ctx, 10)
	if err != nil {
		t.Fatalf("ListForService() error = %v", err)
	}
	if got == nil || len(got) != 0 {
		t.Fatalf("ListForService() = %#v, want non-nil empty slice", got)
	}

	errUseCases := NewUseCases(listServiceStore{memoryStore: newMemoryStore(), err: errBoom}, completedExchange(), svc)
	if _, err := errUseCases.ListForService(ctx, 10); !errors.Is(err, errBoom) {
		t.Fatalf("ListForService(store error) error = %v, want boom", err)
	}
}

type sqlStateError struct{ code string }

func (e sqlStateError) Error() string    { return "sqlstate " + e.code }
func (e sqlStateError) SQLState() string { return e.code }

func TestIsUniqueViolation(t *testing.T) {
	if isUniqueViolation(nil) {
		t.Fatal("isUniqueViolation(nil) = true, want false")
	}
	if isUniqueViolation(errBoom) {
		t.Fatal("isUniqueViolation(non-driver error) = true, want false")
	}
	if isUniqueViolation(sqlStateError{code: "23514"}) {
		t.Fatal("isUniqueViolation(other sqlstate) = true, want false")
	}
	if !isUniqueViolation(sqlStateError{code: "23505"}) {
		t.Fatal("isUniqueViolation(23505) = false, want true")
	}
}

func buildHandler(store Store, exchanges ExchangeLookup, svc ServiceExistenceChecker) http.Handler {
	useCases := NewUseCases(store, exchanges, svc)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return httpapi.NewApplicationHandler(logger, NewHandler(useCases))
}

func TestHTTPErrorBranches(t *testing.T) {
	svc := &stubServiceExistenceChecker{services: map[int]services.Service{10: {ID: 10}}}
	handler := buildHandler(newMemoryStore(), completedExchange(), svc)

	if r := testutil.PerformRequest(handler, http.MethodPost, "/api/exchanges/1/review", `{"note":`, "2"); r.Code != http.StatusBadRequest {
		t.Fatalf("create malformed body status = %d, want 400", r.Code)
	}
	if r := testutil.PerformRequest(handler, http.MethodPost, "/api/exchanges/abc/review", `{"note":5}`, "2"); r.Code != http.StatusBadRequest {
		t.Fatalf("create invalid path id status = %d, want 400", r.Code)
	}

	if r := testutil.PerformRequest(handler, http.MethodGet, "/api/users/abc/reviews", "", ""); r.Code != http.StatusBadRequest {
		t.Fatalf("listForUser invalid path id status = %d, want 400", r.Code)
	}
	if r := testutil.PerformRequest(handler, http.MethodGet, "/api/services/abc/reviews", "", ""); r.Code != http.StatusBadRequest {
		t.Fatalf("listForService invalid path id status = %d, want 400", r.Code)
	}

	errHandler := buildHandler(listTargetStore{memoryStore: newMemoryStore(), err: errBoom}, completedExchange(), svc)
	if r := testutil.PerformRequest(errHandler, http.MethodGet, "/api/users/3/reviews", "", ""); r.Code != http.StatusInternalServerError {
		t.Fatalf("listForUser store error status = %d, want 500", r.Code)
	}
}
