package reviews

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

func newTestApplication() (http.Handler, *stubExchangeLookup, *stubServiceExistenceChecker) {
	exchanges := newStubExchangeLookup()
	svc := &stubServiceExistenceChecker{services: make(map[int]services.Service)}
	useCases := NewUseCases(newMemoryStore(), exchanges, svc)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return httpapi.NewApplicationHandler(logger, NewHandler(useCases)), exchanges, svc
}

func TestHTTPLifecycle(t *testing.T) {
	handler, exchanges, svc := newTestApplication()
	svc.services[1] = services.Service{ID: 1, ProviderID: 3, Titre: "Taille de haies", Categorie: "Jardinage", Actif: true}
	exchanges.exchanges[1] = ExchangeSummary{ID: 1, ServiceID: 1, RequesterID: 2, OwnerID: 3, Status: StatusCompleted}

	created := testutil.PerformRequest(handler, http.MethodPost, "/api/exchanges/1/review", `{"note":5,"commentaire":"Excellent"}`, "2")
	if created.Code != http.StatusCreated {
		t.Fatalf("POST review status = %d, body = %s", created.Code, created.Body.String())
	}
	var review Review
	if err := json.NewDecoder(created.Body).Decode(&review); err != nil {
		t.Fatalf("decode review: %v", err)
	}
	if review.TargetID != 3 {
		t.Fatalf("review = %+v", review)
	}

	duplicate := testutil.PerformRequest(handler, http.MethodPost, "/api/exchanges/1/review", `{"note":4}`, "2")
	if duplicate.Code != http.StatusBadRequest {
		t.Fatalf("duplicate review status = %d", duplicate.Code)
	}

	forUser := testutil.PerformRequest(handler, http.MethodGet, "/api/users/3/reviews", "", "")
	if forUser.Code != http.StatusOK {
		t.Fatalf("GET user reviews status = %d", forUser.Code)
	}
	var userReviews []Review
	if err := json.NewDecoder(forUser.Body).Decode(&userReviews); err != nil || len(userReviews) != 1 {
		t.Fatalf("user reviews = %+v, err = %v", userReviews, err)
	}

	forService := testutil.PerformRequest(handler, http.MethodGet, "/api/services/1/reviews", "", "")
	if forService.Code != http.StatusOK {
		t.Fatalf("GET service reviews status = %d", forService.Code)
	}
}

func TestHTTPErrors(t *testing.T) {
	handler, exchanges, _ := newTestApplication()
	exchanges.exchanges[1] = ExchangeSummary{ID: 1, ServiceID: 1, RequesterID: 2, OwnerID: 3, Status: "pending"}

	tests := []struct {
		name   string
		target string
		body   string
		userID string
		status int
	}{
		{name: "missing auth", target: "/api/exchanges/1/review", body: `{"note":5}`, status: 401},
		{name: "invalid note", target: "/api/exchanges/1/review", body: `{"note":9}`, userID: "2", status: 400},
		{name: "not completed", target: "/api/exchanges/1/review", body: `{"note":5}`, userID: "2", status: 400},
		{name: "unknown exchange", target: "/api/exchanges/999/review", body: `{"note":5}`, userID: "2", status: 404},
		{name: "missing service reviews", target: "/api/services/999/reviews", status: 404},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			method := http.MethodPost
			if test.body == "" {
				method = http.MethodGet
			}
			response := testutil.PerformRequest(handler, method, test.target, test.body, test.userID)
			if response.Code != test.status {
				t.Fatalf("status = %d, want %d; body = %s", response.Code, test.status, response.Body.String())
			}
		})
	}
}
