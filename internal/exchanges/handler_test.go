package exchanges

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"testing"

	"barterswap/internal/services"
	"barterswap/internal/testutil"
	"barterswap/pkg/httpapi"
)

// newTestApplication wires the exchange handler over a memory store with a
// service (id 1) owned by user 2 priced at 2 credits, and users 1, 2, and 3
// existing. The returned store lets a test seed credit balances.
func newTestApplication() (http.Handler, *memoryStore) {
	store := newMemoryStore()
	svc := stubServices{services: map[int]services.Service{
		1: {ID: 1, ProviderID: 2, Titre: "Cours de Go", Categorie: "Informatique", Credits: 2, Actif: true},
	}}
	usr := stubUsers{existing: map[int]bool{1: true, 2: true, 3: true}}
	useCases := NewUseCases(store, svc, usr)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return httpapi.NewApplicationHandler(logger, NewHandler(useCases)), store
}

func TestHTTPExchangeLifecycle(t *testing.T) {
	handler, store := newTestApplication()
	store.grant(1, 10)
	store.grant(3, 10) // a second candidate requester, funded so the 409 path is reached

	// Missing authentication.
	if got := testutil.PerformRequest(handler, http.MethodPost, "/api/exchanges", `{"service_id":1}`, ""); got.Code != http.StatusUnauthorized {
		t.Fatalf("POST without auth status = %d, want 401", got.Code)
	}

	created := testutil.PerformRequest(handler, http.MethodPost, "/api/exchanges", `{"service_id":1}`, "1")
	if created.Code != http.StatusCreated {
		t.Fatalf("POST /api/exchanges status = %d, body = %s", created.Code, created.Body.String())
	}
	var exchange Exchange
	if err := json.NewDecoder(created.Body).Decode(&exchange); err != nil {
		t.Fatalf("decode created exchange: %v", err)
	}
	if exchange.Status != StatusPending || exchange.OwnerID != 2 {
		t.Fatalf("created exchange = %+v", exchange)
	}

	// An outsider cannot read the exchange.
	if got := testutil.PerformRequest(handler, http.MethodGet, "/api/exchanges/1", "", "3"); got.Code != http.StatusForbidden {
		t.Fatalf("GET as outsider status = %d, want 403", got.Code)
	}
	if got := testutil.PerformRequest(handler, http.MethodGet, "/api/exchanges/1", "", "2"); got.Code != http.StatusOK {
		t.Fatalf("GET as owner status = %d, want 200", got.Code)
	}

	// The service is now reserved: a second request conflicts.
	if got := testutil.PerformRequest(handler, http.MethodPost, "/api/exchanges", `{"service_id":1}`, "3"); got.Code != http.StatusConflict {
		t.Fatalf("second POST status = %d, want 409; body = %s", got.Code, got.Body.String())
	}

	// Only the owner may accept.
	if got := testutil.PerformRequest(handler, http.MethodPut, "/api/exchanges/1/accept", "", "1"); got.Code != http.StatusForbidden {
		t.Fatalf("accept by requester status = %d, want 403", got.Code)
	}
	if got := testutil.PerformRequest(handler, http.MethodPut, "/api/exchanges/1/accept", "", "2"); got.Code != http.StatusOK {
		t.Fatalf("accept status = %d, body = %s", got.Code, got.Body.String())
	}
	if store.balances[1] != 8 {
		t.Fatalf("requester balance after accept = %d, want 8", store.balances[1])
	}

	// Completion is confirmed by the requester.
	if got := testutil.PerformRequest(handler, http.MethodPut, "/api/exchanges/1/complete", "", "1"); got.Code != http.StatusOK {
		t.Fatalf("complete status = %d, body = %s", got.Code, got.Body.String())
	}
	if store.balances[2] != 2 {
		t.Fatalf("owner balance after complete = %d, want 2", store.balances[2])
	}

	// Filter the list by status.
	list := testutil.PerformRequest(handler, http.MethodGet, "/api/exchanges?status=completed", "", "1")
	if list.Code != http.StatusOK {
		t.Fatalf("list status = %d", list.Code)
	}
	var exchanges []Exchange
	if err := json.NewDecoder(list.Body).Decode(&exchanges); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(exchanges) != 1 || exchanges[0].Status != StatusCompleted {
		t.Fatalf("list = %+v", exchanges)
	}
}

func TestHTTPReject(t *testing.T) {
	handler, store := newTestApplication()
	store.grant(1, 10)

	created := testutil.PerformRequest(handler, http.MethodPost, "/api/exchanges", `{"service_id":1}`, "1")
	var exchange Exchange
	if err := json.NewDecoder(created.Body).Decode(&exchange); err != nil {
		t.Fatalf("decode created exchange: %v", err)
	}

	// Only the owner may reject.
	if got := testutil.PerformRequest(handler, http.MethodPut, "/api/exchanges/1/reject", "", "1"); got.Code != http.StatusForbidden {
		t.Fatalf("reject by requester status = %d, want 403", got.Code)
	}

	rejected := testutil.PerformRequest(handler, http.MethodPut, "/api/exchanges/1/reject", "", "2")
	if rejected.Code != http.StatusOK {
		t.Fatalf("reject status = %d, body = %s", rejected.Code, rejected.Body.String())
	}
	var result Exchange
	if err := json.NewDecoder(rejected.Body).Decode(&result); err != nil {
		t.Fatalf("decode rejected exchange: %v", err)
	}
	if result.Status != StatusRejected {
		t.Fatalf("rejected exchange = %+v", result)
	}
	// No credit was blocked on a pending request, so nothing is refunded.
	if store.balances[1] != 10 {
		t.Fatalf("requester balance = %d, want 10 (nothing was blocked)", store.balances[1])
	}
}

func TestHTTPCancel(t *testing.T) {
	handler, store := newTestApplication()
	store.grant(1, 10)

	created := testutil.PerformRequest(handler, http.MethodPost, "/api/exchanges", `{"service_id":1}`, "1")
	var exchange Exchange
	if err := json.NewDecoder(created.Body).Decode(&exchange); err != nil {
		t.Fatalf("decode created exchange: %v", err)
	}
	if got := testutil.PerformRequest(handler, http.MethodPut, "/api/exchanges/1/accept", "", "2"); got.Code != http.StatusOK {
		t.Fatalf("accept status = %d", got.Code)
	}
	if store.balances[1] != 8 {
		t.Fatalf("requester balance after accept = %d, want 8", store.balances[1])
	}

	// An outsider may not cancel.
	if got := testutil.PerformRequest(handler, http.MethodPut, "/api/exchanges/1/cancel", "", "3"); got.Code != http.StatusForbidden {
		t.Fatalf("cancel by outsider status = %d, want 403", got.Code)
	}

	cancelled := testutil.PerformRequest(handler, http.MethodPut, "/api/exchanges/1/cancel", "", "1")
	if cancelled.Code != http.StatusOK {
		t.Fatalf("cancel status = %d, body = %s", cancelled.Code, cancelled.Body.String())
	}
	var result Exchange
	if err := json.NewDecoder(cancelled.Body).Decode(&result); err != nil {
		t.Fatalf("decode cancelled exchange: %v", err)
	}
	if result.Status != StatusCancelled {
		t.Fatalf("cancelled exchange = %+v", result)
	}
	// The blocked credits are refunded to the requester.
	if store.balances[1] != 10 {
		t.Fatalf("requester balance after refund = %d, want 10", store.balances[1])
	}

	// A rejected/cancelled exchange cannot be cancelled again.
	if got := testutil.PerformRequest(handler, http.MethodPut, "/api/exchanges/1/cancel", "", "1"); got.Code != http.StatusBadRequest {
		t.Fatalf("cancel again status = %d, want 400", got.Code)
	}
}

func TestHTTPExchangeErrors(t *testing.T) {
	handler, store := newTestApplication()
	store.grant(1, 1) // not enough for the 2-credit service

	tests := []struct {
		name   string
		method string
		target string
		body   string
		userID string
		status int
	}{
		{name: "insufficient credits", method: http.MethodPost, target: "/api/exchanges", body: `{"service_id":1}`, userID: "1", status: http.StatusBadRequest},
		{name: "own service", method: http.MethodPost, target: "/api/exchanges", body: `{"service_id":1}`, userID: "2", status: http.StatusBadRequest},
		{name: "unknown service", method: http.MethodPost, target: "/api/exchanges", body: `{"service_id":999}`, userID: "1", status: http.StatusNotFound},
		{name: "missing exchange", method: http.MethodGet, target: "/api/exchanges/999", body: "", userID: "1", status: http.StatusNotFound},
		{name: "list without auth", method: http.MethodGet, target: "/api/exchanges", body: "", userID: "", status: http.StatusUnauthorized},
		{name: "bad status filter", method: http.MethodGet, target: "/api/exchanges?status=weird", body: "", userID: "1", status: http.StatusBadRequest},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := testutil.PerformRequest(handler, test.method, test.target, test.body, test.userID)
			if got.Code != test.status {
				t.Fatalf("status = %d, want %d; body = %s", got.Code, test.status, got.Body.String())
			}
		})
	}
}
