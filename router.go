package main

import (
	"log/slog"
	"net/http"
)

type RouteRegistrar interface {
	RegisterRoutes(*http.ServeMux)
}

func NewRouter(registrars ...RouteRegistrar) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	for _, registrar := range registrars {
		registrar.RegisterRoutes(mux)
	}
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		writeAPIError(w, ErrNotFound)
	})
	return mux
}

func NewApplicationHandler(logger *slog.Logger, registrars ...RouteRegistrar) http.Handler {
	return Chain(
		NewRouter(registrars...),
		Recovery(logger),
		Logging(logger),
		CORS,
		Authentication,
	)
}
