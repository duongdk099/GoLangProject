package main

import (
	"context"
	"net/http"
	"strconv"
	"strings"
)

type authenticatedUserKey struct{}

func Authentication(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := strings.TrimSpace(r.Header.Get("X-User-ID"))
		if header == "" {
			next.ServeHTTP(w, r)
			return
		}

		userID, err := strconv.Atoi(header)
		if err != nil || userID <= 0 {
			writeAPIError(w, ErrUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), authenticatedUserKey{}, userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func AuthenticatedUserID(ctx context.Context) (int, bool) {
	userID, ok := ctx.Value(authenticatedUserKey{}).(int)
	return userID, ok
}

func requireAuthenticatedUser(w http.ResponseWriter, r *http.Request) (int, bool) {
	userID, ok := AuthenticatedUserID(r.Context())
	if !ok {
		writeAPIError(w, ErrUnauthorized)
		return 0, false
	}
	return userID, true
}
