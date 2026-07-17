package testutil

import (
	"net/http"
	"net/http/httptest"
	"strings"
)

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
