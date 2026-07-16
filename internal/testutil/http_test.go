package testutil

import (
	"io"
	"net/http"
	"testing"
)

func TestPerformRequest(t *testing.T) {
	var gotMethod, gotBody, gotUserID string
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotUserID = r.Header.Get("X-User-ID")
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
		w.WriteHeader(http.StatusTeapot)
	})

	response := PerformRequest(handler, http.MethodPost, "/anything", `{"a":1}`, "7")
	if response.Code != http.StatusTeapot {
		t.Fatalf("status = %d", response.Code)
	}
	if gotMethod != http.MethodPost || gotBody != `{"a":1}` || gotUserID != "7" {
		t.Fatalf("method=%q body=%q userID=%q", gotMethod, gotBody, gotUserID)
	}

	noAuth := PerformRequest(handler, http.MethodGet, "/anything", "", "")
	if noAuth.Code != http.StatusTeapot {
		t.Fatalf("status = %d", noAuth.Code)
	}
}
