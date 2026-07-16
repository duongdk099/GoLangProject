package services

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"barterswap/internal/testutil"
	"barterswap/pkg/httpapi"
)

var errBoom = errors.New("boom")

// errSkillChecker always fails the skill lookup, exercising the
// dependency-error return branches of Create and Update.
type errSkillChecker struct{ err error }

func (c errSkillChecker) UserHasSkill(context.Context, int, string) (bool, error) {
	return false, c.err
}

// listResultStore lets a test replace only List while inheriting every other
// method from the in-memory store.
type listResultStore struct {
	*memoryStore
	list []Service
	err  error
}

func (s listResultStore) List(context.Context, Filter) ([]Service, error) {
	return s.list, s.err
}

func TestGetRejectsNonPositiveID(t *testing.T) {
	useCases := NewUseCases(newMemoryStore(), newStubSkillChecker())
	if _, err := useCases.Get(context.Background(), 0); !errors.Is(err, httpapi.ErrValidation) {
		t.Fatalf("Get(0) error = %v, want validation", err)
	}
}

func TestCreatePropagatesSkillCheckError(t *testing.T) {
	useCases := NewUseCases(newMemoryStore(), errSkillChecker{err: errBoom})
	_, err := useCases.Create(context.Background(), 1, CreateRequest{
		Titre: "Cours de Go", Categorie: "Informatique", DureeMinutes: 60, Credits: 2,
	})
	if !errors.Is(err, errBoom) {
		t.Fatalf("Create(skill error) error = %v, want boom", err)
	}
}

func TestUpdateBranches(t *testing.T) {
	ctx := context.Background()

	// Delete and Update on a missing service surface the not-found from the
	// inner Get lookup they both perform first.
	if err := NewUseCases(newMemoryStore(), newStubSkillChecker()).Delete(ctx, 1, 42); !errors.Is(err, httpapi.ErrNotFound) {
		t.Fatalf("Delete(missing) error = %v, want not found", err)
	}
	if _, err := NewUseCases(newMemoryStore(), newStubSkillChecker()).Update(ctx, 1, 42, UpdateRequest{
		Titre: "T", Categorie: "Informatique", DureeMinutes: 30, Credits: 1,
	}); !errors.Is(err, httpapi.ErrNotFound) {
		t.Fatalf("Update(missing) error = %v, want not found", err)
	}

	// A validation failure inside Update is returned before any skill re-check.
	store := newMemoryStore()
	seeded, _ := store.Create(ctx, CreateParams{ProviderID: 1, Titre: "T", Categorie: "Informatique", DureeMinutes: 30, Credits: 1})
	useCases := NewUseCases(store, newStubSkillChecker())
	if _, err := useCases.Update(ctx, 1, seeded.ID, UpdateRequest{
		Titre: "", Categorie: "Informatique", DureeMinutes: 30, Credits: 1,
	}); !errors.Is(err, httpapi.ErrValidation) {
		t.Fatalf("Update(invalid fields) error = %v, want validation", err)
	}

	// Changing the category to a skill the owner lacks fails the re-check.
	if _, err := useCases.Update(ctx, 1, seeded.ID, UpdateRequest{
		Titre: "T", Categorie: "Cuisine", DureeMinutes: 30, Credits: 1,
	}); !errors.Is(err, httpapi.ErrValidation) {
		t.Fatalf("Update(missing skill) error = %v, want validation", err)
	}

	// The re-check propagates a dependency error from the skill checker.
	errStore := newMemoryStore()
	errSeeded, _ := errStore.Create(ctx, CreateParams{ProviderID: 1, Titre: "T", Categorie: "Informatique", DureeMinutes: 30, Credits: 1})
	errUseCases := NewUseCases(errStore, errSkillChecker{err: errBoom})
	if _, err := errUseCases.Update(ctx, 1, errSeeded.ID, UpdateRequest{
		Titre: "T", Categorie: "Cuisine", DureeMinutes: 30, Credits: 1,
	}); !errors.Is(err, errBoom) {
		t.Fatalf("Update(skill error) error = %v, want boom", err)
	}
}

func TestListNormalizesNilAndPropagatesError(t *testing.T) {
	ctx := context.Background()

	nilUseCases := NewUseCases(listResultStore{memoryStore: newMemoryStore(), list: nil}, newStubSkillChecker())
	got, err := nilUseCases.List(ctx, Filter{})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if got == nil || len(got) != 0 {
		t.Fatalf("List() = %#v, want non-nil empty slice", got)
	}

	errUseCases := NewUseCases(listResultStore{memoryStore: newMemoryStore(), err: errBoom}, newStubSkillChecker())
	if _, err := errUseCases.List(ctx, Filter{}); !errors.Is(err, errBoom) {
		t.Fatalf("List(store error) error = %v, want boom", err)
	}
}

func TestValidateFieldsMessages(t *testing.T) {
	longTitre := make([]rune, 151)
	for i := range longTitre {
		longTitre[i] = 'a'
	}
	longDescription := make([]rune, 2001)
	for i := range longDescription {
		longDescription[i] = 'a'
	}
	longVille := make([]rune, 101)
	for i := range longVille {
		longVille[i] = 'a'
	}

	tests := []struct {
		name    string
		request CreateRequest
	}{
		{name: "too long title", request: CreateRequest{Titre: string(longTitre), Categorie: "Informatique", DureeMinutes: 30, Credits: 1}},
		{name: "too long description", request: CreateRequest{Titre: "T", Description: string(longDescription), Categorie: "Informatique", DureeMinutes: 30, Credits: 1}},
		{name: "too long ville", request: CreateRequest{Titre: "T", Categorie: "Informatique", Ville: string(longVille), DureeMinutes: 30, Credits: 1}},
	}

	skills := newStubSkillChecker()
	skills.grant(1, "Informatique")
	useCases := NewUseCases(newMemoryStore(), skills)
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := useCases.Create(context.Background(), 1, test.request); !errors.Is(err, httpapi.ErrValidation) {
				t.Fatalf("Create(%s) error = %v, want validation", test.name, err)
			}
		})
	}
}

func TestHTTPErrorBranches(t *testing.T) {
	handler, skills := newTestApplication()
	skills.grant(1, "Informatique")

	tests := []struct {
		name   string
		method string
		target string
		body   string
		userID string
		status int
	}{
		{name: "create malformed body", method: http.MethodPost, target: "/api/services", body: `{"titre":`, userID: "1", status: http.StatusBadRequest},
		{name: "get invalid path id", method: http.MethodGet, target: "/api/services/abc", status: http.StatusBadRequest},
		{name: "update missing auth", method: http.MethodPut, target: "/api/services/1", body: `{"titre":"X","categorie":"Informatique","duree_minutes":10,"credits":1}`, status: http.StatusUnauthorized},
		{name: "update invalid path id", method: http.MethodPut, target: "/api/services/abc", body: `{"titre":"X","categorie":"Informatique","duree_minutes":10,"credits":1}`, userID: "1", status: http.StatusBadRequest},
		{name: "update malformed body", method: http.MethodPut, target: "/api/services/1", body: `{"titre":`, userID: "1", status: http.StatusBadRequest},
		{name: "delete missing auth", method: http.MethodDelete, target: "/api/services/1", status: http.StatusUnauthorized},
		{name: "delete invalid path id", method: http.MethodDelete, target: "/api/services/abc", userID: "1", status: http.StatusBadRequest},
		{name: "delete missing service", method: http.MethodDelete, target: "/api/services/999", userID: "1", status: http.StatusNotFound},
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
