package services

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"testing"

	"barterswap/internal/testutil"
	"barterswap/pkg/httpapi"
)

func newTestApplication() (http.Handler, *stubSkillChecker) {
	skills := newStubSkillChecker()
	useCases := NewUseCases(newMemoryStore(), skills)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return httpapi.NewApplicationHandler(logger, NewHandler(useCases)), skills
}

func TestHTTPLifecycle(t *testing.T) {
	handler, skills := newTestApplication()
	skills.grant(1, "Informatique")

	created := testutil.PerformRequest(handler, http.MethodPost, "/api/services", `{
		"titre":"Cours de Go","categorie":"Informatique","duree_minutes":60,"credits":2,"ville":"Paris"
	}`, "1")
	if created.Code != http.StatusCreated {
		t.Fatalf("POST /api/services status = %d, body = %s", created.Code, created.Body.String())
	}
	var service Service
	if err := json.NewDecoder(created.Body).Decode(&service); err != nil {
		t.Fatalf("decode created service: %v", err)
	}
	if service.ProviderID != 1 || !service.Actif {
		t.Fatalf("created service = %+v", service)
	}

	get := testutil.PerformRequest(handler, http.MethodGet, "/api/services/1", "", "")
	if get.Code != http.StatusOK {
		t.Fatalf("GET /api/services/1 status = %d", get.Code)
	}

	list := testutil.PerformRequest(handler, http.MethodGet, "/api/services?categorie=Informatique", "", "")
	if list.Code != http.StatusOK {
		t.Fatalf("GET /api/services status = %d", list.Code)
	}
	var services []Service
	if err := json.NewDecoder(list.Body).Decode(&services); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(services) != 1 {
		t.Fatalf("list = %+v", services)
	}

	update := testutil.PerformRequest(handler, http.MethodPut, "/api/services/1", `{
		"titre":"Cours de Go avancé","categorie":"Informatique","duree_minutes":90,"credits":3,"actif":false
	}`, "1")
	if update.Code != http.StatusOK {
		t.Fatalf("PUT /api/services/1 status = %d, body = %s", update.Code, update.Body.String())
	}

	forbiddenUpdate := testutil.PerformRequest(handler, http.MethodPut, "/api/services/1", `{
		"titre":"X","categorie":"Informatique","duree_minutes":10,"credits":1,"actif":true
	}`, "2")
	if forbiddenUpdate.Code != http.StatusForbidden {
		t.Fatalf("PUT /api/services/1 (wrong owner) status = %d", forbiddenUpdate.Code)
	}

	deleted := testutil.PerformRequest(handler, http.MethodDelete, "/api/services/1", "", "1")
	if deleted.Code != http.StatusNoContent {
		t.Fatalf("DELETE /api/services/1 status = %d, body = %s", deleted.Code, deleted.Body.String())
	}

	missing := testutil.PerformRequest(handler, http.MethodGet, "/api/services/1", "", "")
	if missing.Code != http.StatusNotFound {
		t.Fatalf("GET /api/services/1 (deleted) status = %d", missing.Code)
	}
}

func TestHTTPErrors(t *testing.T) {
	handler, _ := newTestApplication()

	tests := []struct {
		name   string
		method string
		target string
		body   string
		userID string
		status int
	}{
		{name: "missing auth", method: http.MethodPost, target: "/api/services", body: `{"titre":"X","categorie":"Informatique","duree_minutes":10,"credits":1}`, status: 401},
		{name: "missing skill", method: http.MethodPost, target: "/api/services", body: `{"titre":"X","categorie":"Informatique","duree_minutes":10,"credits":1}`, userID: "1", status: 400},
		{name: "empty title", method: http.MethodPost, target: "/api/services", body: `{"titre":"","categorie":"Informatique","duree_minutes":10,"credits":1}`, userID: "1", status: 400},
		{name: "invalid category", method: http.MethodPost, target: "/api/services", body: `{"titre":"X","categorie":"Espace","duree_minutes":10,"credits":1}`, userID: "1", status: 400},
		{name: "missing service", method: http.MethodGet, target: "/api/services/999", status: 404},
		{name: "invalid category filter", method: http.MethodGet, target: "/api/services?categorie=Espace", status: 400},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := testutil.PerformRequest(handler, test.method, test.target, test.body, test.userID)
			if response.Code != test.status {
				t.Fatalf("status = %d, want %d; body = %s", response.Code, test.status, response.Body.String())
			}
		})
	}
}
