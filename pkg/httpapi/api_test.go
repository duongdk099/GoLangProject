package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWriteJSON(t *testing.T) {
	response := httptest.NewRecorder()
	WriteJSON(response, http.StatusCreated, map[string]string{"status": "ok"})

	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusCreated)
	}
	if got := response.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Fatalf("Content-Type = %q", got)
	}
	var payload map[string]string
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if payload["status"] != "ok" {
		t.Fatalf("body = %+v", payload)
	}
}

func TestDecodeJSON(t *testing.T) {
	type payload struct {
		Name string `json:"name"`
	}

	t.Run("success", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"Alice"}`))
		var dst payload
		if err := DecodeJSON(httptest.NewRecorder(), request, &dst); err != nil {
			t.Fatalf("DecodeJSON() error = %v", err)
		}
		if dst.Name != "Alice" {
			t.Fatalf("decoded = %+v", dst)
		}
	})

	tests := []struct {
		name string
		body io.Reader
		want string
	}{
		{name: "malformed JSON syntax", body: strings.NewReader(`{"name": bad}`), want: "malformed JSON"},
		{name: "incomplete JSON", body: strings.NewReader(`{"name":`), want: "invalid JSON"},
		{name: "empty body", body: strings.NewReader(``), want: "must not be empty"},
		{name: "wrong type", body: strings.NewReader(`{"name":123}`), want: `request field "name" has an invalid value`},
		{name: "unknown field", body: strings.NewReader(`{"admin":true}`), want: "unknown field"},
		{name: "too large", body: strings.NewReader(`{"name":"` + strings.Repeat("a", maxRequestBodySize+10) + `"}`), want: "must not exceed"},
		{name: "trailing value", body: strings.NewReader(`{"name":"A"}{"name":"B"}`), want: "single JSON value"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/", test.body)
			var dst payload
			err := DecodeJSON(httptest.NewRecorder(), request, &dst)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("DecodeJSON() error = %v, want containing %q", err, test.want)
			}
		})
	}
}

func TestDecodeJSONInvalidValue(t *testing.T) {

	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`["not","an","object"]`))
	var dst struct {
		Name string `json:"name"`
	}
	err := DecodeJSON(httptest.NewRecorder(), request, &dst)
	if err == nil || !strings.Contains(err.Error(), "invalid value") {
		t.Fatalf("DecodeJSON() error = %v, want invalid value", err)
	}
}

func TestWriteAPIError(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{name: "validation", err: ErrValidation, wantStatus: http.StatusBadRequest, wantCode: "validation_error"},
		{name: "unauthorized", err: ErrUnauthorized, wantStatus: http.StatusUnauthorized, wantCode: "unauthorized"},
		{name: "forbidden", err: ErrForbidden, wantStatus: http.StatusForbidden, wantCode: "forbidden"},
		{name: "not found", err: ErrNotFound, wantStatus: http.StatusNotFound, wantCode: "not_found"},
		{name: "conflict", err: ErrConflict, wantStatus: http.StatusConflict, wantCode: "conflict"},
		{name: "internal", err: errors.New("boom"), wantStatus: http.StatusInternalServerError, wantCode: "internal_error"},
		{name: "nil", err: nil, wantStatus: http.StatusInternalServerError, wantCode: "internal_error"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			WriteAPIError(response, test.err)
			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", response.Code, test.wantStatus)
			}
			var envelope ErrorEnvelope
			if err := json.NewDecoder(response.Body).Decode(&envelope); err != nil {
				t.Fatalf("decode envelope: %v", err)
			}
			if envelope.Error.Code != test.wantCode {
				t.Fatalf("code = %q, want %q", envelope.Error.Code, test.wantCode)
			}
		})
	}
}

func TestWriteBadRequest(t *testing.T) {
	response := httptest.NewRecorder()
	WriteBadRequest(response, "something is wrong")
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
	var envelope ErrorEnvelope
	if err := json.NewDecoder(response.Body).Decode(&envelope); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	if envelope.Error.Code != "bad_request" || envelope.Error.Message != "something is wrong" {
		t.Fatalf("envelope = %+v", envelope.Error)
	}
}

func TestPathID(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /x/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, ok := PathID(w, r)
		if !ok {
			return
		}
		WriteJSON(w, http.StatusOK, map[string]int{"id": id})
	})

	t.Run("valid", func(t *testing.T) {
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/x/7", nil))
		if response.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
		}
		var body map[string]int
		if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if body["id"] != 7 {
			t.Fatalf("id = %d, want 7", body["id"])
		}
	})

	t.Run("invalid", func(t *testing.T) {
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/x/nope", nil))
		if response.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
		}
	})

	t.Run("non positive", func(t *testing.T) {
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/x/0", nil))
		if response.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
		}
	})

	t.Run("missing", func(t *testing.T) {

		response := httptest.NewRecorder()
		if _, ok := PathID(response, httptest.NewRequest(http.MethodGet, "/x", nil)); ok {
			t.Fatal("PathID() ok = true, want false for a missing id")
		}
		if response.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
		}
	})
}

func TestAuthentication(t *testing.T) {
	newHandler := func(seen *int) http.Handler {
		return Authentication(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if userID, ok := AuthenticatedUserID(r.Context()); ok {
				*seen = userID
			}
			w.WriteHeader(http.StatusNoContent)
		}))
	}

	t.Run("no header passes through unauthenticated", func(t *testing.T) {
		seen := -1
		response := httptest.NewRecorder()
		newHandler(&seen).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
		if response.Code != http.StatusNoContent {
			t.Fatalf("status = %d", response.Code)
		}
		if seen != -1 {
			t.Fatalf("AuthenticatedUserID leaked %d without a header", seen)
		}
	})

	t.Run("valid header sets the user", func(t *testing.T) {
		seen := -1
		request := httptest.NewRequest(http.MethodGet, "/", nil)
		request.Header.Set("X-User-ID", "42")
		response := httptest.NewRecorder()
		newHandler(&seen).ServeHTTP(response, request)
		if response.Code != http.StatusNoContent || seen != 42 {
			t.Fatalf("status = %d, seen = %d", response.Code, seen)
		}
	})

	for _, header := range []string{"abc", "0", "-1"} {
		t.Run("invalid header "+header, func(t *testing.T) {
			seen := -1
			request := httptest.NewRequest(http.MethodGet, "/", nil)
			request.Header.Set("X-User-ID", header)
			response := httptest.NewRecorder()
			newHandler(&seen).ServeHTTP(response, request)
			if response.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
			}
		})
	}
}

func TestAuthenticatedUserID(t *testing.T) {
	if _, ok := AuthenticatedUserID(context.Background()); ok {
		t.Fatal("AuthenticatedUserID() ok = true on an empty context")
	}
	ctx := context.WithValue(context.Background(), authenticatedUserKey{}, 7)
	if id, ok := AuthenticatedUserID(ctx); !ok || id != 7 {
		t.Fatalf("AuthenticatedUserID() = %d, %v", id, ok)
	}
}

func TestRequireAuthenticatedUser(t *testing.T) {
	t.Run("present", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodGet, "/", nil)
		request = request.WithContext(context.WithValue(request.Context(), authenticatedUserKey{}, 9))
		response := httptest.NewRecorder()
		id, ok := RequireAuthenticatedUser(response, request)
		if !ok || id != 9 {
			t.Fatalf("RequireAuthenticatedUser() = %d, %v", id, ok)
		}
	})

	t.Run("absent", func(t *testing.T) {
		response := httptest.NewRecorder()
		if _, ok := RequireAuthenticatedUser(response, httptest.NewRequest(http.MethodGet, "/", nil)); ok {
			t.Fatal("RequireAuthenticatedUser() ok = true without an authenticated user")
		}
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
		}
	})
}

type stubRegistrar struct{}

func (stubRegistrar) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/ping", func(w http.ResponseWriter, _ *http.Request) {
		WriteJSON(w, http.StatusOK, map[string]string{"pong": "true"})
	})
}

func TestNewRouter(t *testing.T) {
	router := NewRouter(stubRegistrar{})

	health := httptest.NewRecorder()
	router.ServeHTTP(health, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if health.Code != http.StatusOK {
		t.Fatalf("GET /healthz status = %d", health.Code)
	}

	ping := httptest.NewRecorder()
	router.ServeHTTP(ping, httptest.NewRequest(http.MethodGet, "/api/ping", nil))
	if ping.Code != http.StatusOK {
		t.Fatalf("GET /api/ping status = %d", ping.Code)
	}

	missing := httptest.NewRecorder()
	router.ServeHTTP(missing, httptest.NewRequest(http.MethodGet, "/does/not/exist", nil))
	if missing.Code != http.StatusNotFound {
		t.Fatalf("unknown route status = %d, want %d", missing.Code, http.StatusNotFound)
	}
}

func TestNewApplicationHandler(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := NewApplicationHandler(logger, stubRegistrar{})

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/ping", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("GET /api/ping status = %d", response.Code)
	}
}

func TestLoggingRecordsImplicitOK(t *testing.T) {

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := Logging(logger)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
}
