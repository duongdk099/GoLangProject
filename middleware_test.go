package main

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRecoveryMiddleware(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := Recovery(logger)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("test panic")
	}))

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/panic", nil))
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusInternalServerError)
	}
}

func TestChainOrderAndAuthenticationContext(t *testing.T) {
	handler := Authentication(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, ok := AuthenticatedUserID(r.Context())
		if !ok || userID != 42 {
			t.Fatalf("AuthenticatedUserID() = %d, %v", userID, ok)
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("X-User-ID", "42")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d", response.Code)
	}
}

func TestStatusRecorder(t *testing.T) {
	underlying := httptest.NewRecorder()
	recorder := &statusRecorder{ResponseWriter: underlying}
	if recorder.Unwrap() != underlying {
		t.Fatal("Unwrap() did not return the underlying writer")
	}
	recorder.WriteHeader(http.StatusCreated)
	recorder.WriteHeader(http.StatusNoContent)
	if _, err := recorder.Write([]byte("ok")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if recorder.status != http.StatusCreated || recorder.bytes != 2 {
		t.Fatalf("recorder = %+v", recorder)
	}

	implicit := &statusRecorder{ResponseWriter: httptest.NewRecorder()}
	if _, err := implicit.Write([]byte("body")); err != nil {
		t.Fatalf("implicit Write() error = %v", err)
	}
	if implicit.status != http.StatusOK {
		t.Fatalf("implicit status = %d", implicit.status)
	}
}
