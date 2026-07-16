// Package testutil provides small, dependency-free helpers shared by the
// HTTP tests of every feature package. It intentionally knows nothing about
// any BarterSwap domain type.
package testutil

import (
	"net/http"
	"net/http/httptest"
	"strings"
)

// PerformRequest builds and executes an httptest request against handler,
// optionally attaching a JSON body and the X-User-ID authentication header.
func PerformRequest(handler http.Handler, method, target, body, userID string) *httptest.ResponseRecorder {
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
