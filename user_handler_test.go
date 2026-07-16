package main

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestApplication(store *memoryUserStore) http.Handler {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewApplicationHandler(logger, NewUserHandler(NewUserService(store)))
}

func performRequest(handler http.Handler, method, target, body, userID string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	if userID != "" {
		request.Header.Set("X-User-ID", userID)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func TestUserHTTPLifecycle(t *testing.T) {
	store := newMemoryUserStore()
	handler := newTestApplication(store)

	created := performRequest(handler, http.MethodPost, "/api/users", `{
		"pseudo":"Alice","bio":"Go developer","ville":"Paris"
	}`, "")
	if created.Code != http.StatusCreated {
		t.Fatalf("POST /api/users status = %d, body = %s", created.Code, created.Body.String())
	}
	if created.Header().Get("Location") != "/api/users/1" {
		t.Fatalf("Location = %q", created.Header().Get("Location"))
	}
	var user User
	if err := json.NewDecoder(created.Body).Decode(&user); err != nil {
		t.Fatalf("decode created user: %v", err)
	}
	if user.ID != 1 || user.CreditBalance != WelcomeCredits {
		t.Fatalf("created user = %+v", user)
	}

	setSkills := performRequest(handler, http.MethodPut, "/api/users/1/skills", `[
		{"nom":"Go","niveau":"expert"},
		{"nom":"Cuisine","niveau":"débutant"}
	]`, "1")
	if setSkills.Code != http.StatusOK {
		t.Fatalf("PUT skills status = %d, body = %s", setSkills.Code, setSkills.Body.String())
	}

	profile := performRequest(handler, http.MethodGet, "/api/users/1", "", "")
	if profile.Code != http.StatusOK {
		t.Fatalf("GET profile status = %d, body = %s", profile.Code, profile.Body.String())
	}
	if err := json.NewDecoder(profile.Body).Decode(&user); err != nil {
		t.Fatalf("decode profile: %v", err)
	}
	if len(user.Skills) != 2 {
		t.Fatalf("profile skills = %+v", user.Skills)
	}

	updated := performRequest(handler, http.MethodPut, "/api/users/1", `{
		"pseudo":"Alice2","bio":"Updated","ville":"Lyon"
	}`, "1")
	if updated.Code != http.StatusOK {
		t.Fatalf("PUT profile status = %d, body = %s", updated.Code, updated.Body.String())
	}

	skills := performRequest(handler, http.MethodGet, "/api/users/1/skills", "", "")
	if skills.Code != http.StatusOK {
		t.Fatalf("GET skills status = %d, body = %s", skills.Code, skills.Body.String())
	}
}

func TestUserHTTPErrors(t *testing.T) {
	store := newMemoryUserStore()
	seedUser(store, "Alice")
	seedUser(store, "Bob")
	handler := newTestApplication(store)

	tests := []struct {
		name   string
		method string
		target string
		body   string
		userID string
		status int
		code   string
	}{
		{name: "empty pseudo", method: http.MethodPost, target: "/api/users", body: `{"pseudo":""}`, status: 400, code: "validation_error"},
		{name: "empty body", method: http.MethodPost, target: "/api/users", status: 400, code: "bad_request"},
		{name: "malformed JSON", method: http.MethodPost, target: "/api/users", body: `{"pseudo":`, status: 400, code: "bad_request"},
		{name: "unknown field", method: http.MethodPost, target: "/api/users", body: `{"pseudo":"A","admin":true}`, status: 400, code: "bad_request"},
		{name: "multiple values", method: http.MethodPost, target: "/api/users", body: `{"pseudo":"A"}{"pseudo":"B"}`, status: 400, code: "bad_request"},
		{name: "missing auth", method: http.MethodPut, target: "/api/users/1", body: `{"pseudo":"A"}`, status: 401, code: "unauthorized"},
		{name: "invalid auth", method: http.MethodPut, target: "/api/users/1", body: `{"pseudo":"A"}`, userID: "abc", status: 401, code: "unauthorized"},
		{name: "wrong owner", method: http.MethodPut, target: "/api/users/1", body: `{"pseudo":"A"}`, userID: "2", status: 403, code: "forbidden"},
		{name: "missing user", method: http.MethodGet, target: "/api/users/999", status: 404, code: "not_found"},
		{name: "invalid path ID", method: http.MethodGet, target: "/api/users/nope", status: 400, code: "bad_request"},
		{name: "invalid skill level", method: http.MethodPut, target: "/api/users/1/skills", body: `[{"nom":"Go","niveau":"master"}]`, userID: "1", status: 400, code: "validation_error"},
		{name: "missing route", method: http.MethodGet, target: "/missing", status: 404, code: "not_found"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := performRequest(handler, test.method, test.target, test.body, test.userID)
			if response.Code != test.status {
				t.Fatalf("status = %d, want %d; body = %s", response.Code, test.status, response.Body.String())
			}
			var envelope errorEnvelope
			if err := json.NewDecoder(response.Body).Decode(&envelope); err != nil {
				t.Fatalf("decode error response: %v; body = %s", err, response.Body.String())
			}
			if envelope.Error.Code != test.code {
				t.Fatalf("error code = %q, want %q", envelope.Error.Code, test.code)
			}
		})
	}
}

func TestHealthAndCORS(t *testing.T) {
	handler := newTestApplication(newMemoryUserStore())

	health := performRequest(handler, http.MethodGet, "/healthz", "", "")
	if health.Code != http.StatusOK {
		t.Fatalf("GET /healthz status = %d", health.Code)
	}

	options := performRequest(handler, http.MethodOptions, "/api/users", "", "")
	if options.Code != http.StatusNoContent {
		t.Fatalf("OPTIONS status = %d", options.Code)
	}
	if options.Header().Get("Access-Control-Allow-Headers") == "" {
		t.Fatal("OPTIONS response is missing CORS headers")
	}
}
