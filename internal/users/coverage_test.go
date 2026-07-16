package users

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"barterswap/internal/testutil"
	"barterswap/pkg/httpapi"
)

var errBoom = errors.New("boom")

// errStore embeds the in-memory store and lets a test inject an error into any
// single method, so the service's dependency-error branches can be driven
// without a database.
type errStore struct {
	*memoryStore
	existsErr        error
	getByIDErr       error
	updateErr        error
	listSkillsErr    error
	replaceSkillsErr error
}

func (s *errStore) GetByID(ctx context.Context, userID int) (User, error) {
	if s.getByIDErr != nil {
		return User{}, s.getByIDErr
	}
	return s.memoryStore.GetByID(ctx, userID)
}

func (s *errStore) Update(ctx context.Context, userID int, params UpdateUserParams) (User, error) {
	if s.updateErr != nil {
		return User{}, s.updateErr
	}
	return s.memoryStore.Update(ctx, userID, params)
}

func (s *errStore) ListSkills(ctx context.Context, userID int) ([]Skill, error) {
	if s.listSkillsErr != nil {
		return nil, s.listSkillsErr
	}
	return s.memoryStore.ListSkills(ctx, userID)
}

func (s *errStore) ReplaceSkills(ctx context.Context, userID int, skills []Skill) error {
	if s.replaceSkillsErr != nil {
		return s.replaceSkillsErr
	}
	return s.memoryStore.ReplaceSkills(ctx, userID, skills)
}

func (s *errStore) Exists(ctx context.Context, userID int) (bool, error) {
	if s.existsErr != nil {
		return false, s.existsErr
	}
	return s.memoryStore.Exists(ctx, userID)
}

func TestServiceGetPropagatesListSkillsError(t *testing.T) {
	backing := newMemoryStore()
	user := seedUser(backing, "Alice")
	service := NewService(&errStore{memoryStore: backing, listSkillsErr: errBoom})

	if _, err := service.Get(context.Background(), user.ID); !errors.Is(err, errBoom) {
		t.Fatalf("Get() error = %v, want boom", err)
	}
}

func TestServiceUpdateValidatesProfile(t *testing.T) {
	backing := newMemoryStore()
	user := seedUser(backing, "Alice")
	service := NewService(backing)

	// The owner is authorized, so validation runs and rejects the blank pseudo.
	if _, err := service.Update(context.Background(), user.ID, user.ID, UpdateUserRequest{Pseudo: "   "}); !errors.Is(err, httpapi.ErrValidation) {
		t.Fatalf("Update(blank pseudo) error = %v, want validation", err)
	}
}

func TestServiceUpdatePropagatesStoreErrors(t *testing.T) {
	backing := newMemoryStore()
	user := seedUser(backing, "Alice")

	updateErr := NewService(&errStore{memoryStore: backing, updateErr: errBoom})
	if _, err := updateErr.Update(context.Background(), user.ID, user.ID, UpdateUserRequest{Pseudo: "Alice"}); !errors.Is(err, errBoom) {
		t.Fatalf("Update(store error) = %v, want boom", err)
	}

	listErr := NewService(&errStore{memoryStore: backing, listSkillsErr: errBoom})
	if _, err := listErr.Update(context.Background(), user.ID, user.ID, UpdateUserRequest{Pseudo: "Alice"}); !errors.Is(err, errBoom) {
		t.Fatalf("Update(list skills error) = %v, want boom", err)
	}
}

// TestServiceGetAndUpdateNormalizeNilSkills covers the nil-to-empty skill slice
// normalization in Get and Update for a user that has no skills recorded.
func TestServiceGetAndUpdateNormalizeNilSkills(t *testing.T) {
	backing := newMemoryStore()
	user := seedUser(backing, "Alice")
	service := NewService(backing)

	got, err := service.Get(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Skills == nil || len(got.Skills) != 0 {
		t.Fatalf("Get() skills = %#v, want non-nil empty slice", got.Skills)
	}

	updated, err := service.Update(context.Background(), user.ID, user.ID, UpdateUserRequest{Pseudo: "Alice"})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if updated.Skills == nil || len(updated.Skills) != 0 {
		t.Fatalf("Update() skills = %#v, want non-nil empty slice", updated.Skills)
	}
}

func TestServiceSkillsPropagatesStoreErrors(t *testing.T) {
	backing := newMemoryStore()
	user := seedUser(backing, "Alice")

	existsErr := NewService(&errStore{memoryStore: backing, existsErr: errBoom})
	if _, err := existsErr.Skills(context.Background(), user.ID); !errors.Is(err, errBoom) {
		t.Fatalf("Skills(exists error) = %v, want boom", err)
	}

	listErr := NewService(&errStore{memoryStore: backing, listSkillsErr: errBoom})
	if _, err := listErr.Skills(context.Background(), user.ID); !errors.Is(err, errBoom) {
		t.Fatalf("Skills(list error) = %v, want boom", err)
	}
}

func TestServiceSkillsNormalizesNil(t *testing.T) {
	backing := newMemoryStore()
	user := seedUser(backing, "Alice")
	service := NewService(backing)

	skills, err := service.Skills(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("Skills() error = %v", err)
	}
	if skills == nil || len(skills) != 0 {
		t.Fatalf("Skills() = %#v, want non-nil empty slice", skills)
	}
}

func TestServiceSetSkillsValidation(t *testing.T) {
	backing := newMemoryStore()
	user := seedUser(backing, "Alice")
	service := NewService(backing)

	tooMany := make([]Skill, 51)
	for i := range tooMany {
		tooMany[i] = Skill{Nom: strings.Repeat("x", i+1), Niveau: "expert"}
	}
	if _, err := service.SetSkills(context.Background(), user.ID, user.ID, tooMany); !errors.Is(err, httpapi.ErrValidation) {
		t.Fatalf("SetSkills(>50) error = %v, want validation", err)
	}

	tooLong := []Skill{{Nom: strings.Repeat("x", 101), Niveau: "expert"}}
	if _, err := service.SetSkills(context.Background(), user.ID, user.ID, tooLong); !errors.Is(err, httpapi.ErrValidation) {
		t.Fatalf("SetSkills(long name) error = %v, want validation", err)
	}
}

func TestServiceSetSkillsPropagatesReplaceError(t *testing.T) {
	backing := newMemoryStore()
	user := seedUser(backing, "Alice")
	service := NewService(&errStore{memoryStore: backing, replaceSkillsErr: errBoom})

	if _, err := service.SetSkills(context.Background(), user.ID, user.ID, []Skill{{Nom: "Go", Niveau: "expert"}}); !errors.Is(err, errBoom) {
		t.Fatalf("SetSkills(replace error) = %v, want boom", err)
	}
}

func TestServiceUserExistsAndHasSkillDelegateErrors(t *testing.T) {
	backing := newMemoryStore()
	user := seedUser(backing, "Alice")
	service := NewService(&errStore{memoryStore: backing, existsErr: errBoom})

	if _, err := service.UserExists(context.Background(), user.ID); !errors.Is(err, errBoom) {
		t.Fatalf("UserExists(error) = %v, want boom", err)
	}
	// Non-positive id short-circuits before touching the store.
	if exists, err := service.UserExists(context.Background(), 0); err != nil || exists {
		t.Fatalf("UserExists(0) = %v, %v", exists, err)
	}
	if has, err := service.UserHasSkill(context.Background(), 0, "go"); err != nil || has {
		t.Fatalf("UserHasSkill(0) = %v, %v", has, err)
	}
	if has, err := service.UserHasSkill(context.Background(), user.ID, "  "); err != nil || has {
		t.Fatalf("UserHasSkill(empty) = %v, %v", has, err)
	}
}

// TestHandlerErrorPaths drives the handler branches not reached by the happy-path
// lifecycle test: invalid path ids, missing authentication, and malformed bodies.
func TestHandlerErrorPaths(t *testing.T) {
	tests := []struct {
		name   string
		method string
		target string
		body   string
		userID string
		status int
	}{
		{name: "get invalid path id", method: http.MethodGet, target: "/api/users/nope", status: 400},
		{name: "get skills invalid path id", method: http.MethodGet, target: "/api/users/nope/skills", status: 400},
		{name: "get skills missing user", method: http.MethodGet, target: "/api/users/999/skills", status: 404},
		{name: "update missing auth", method: http.MethodPut, target: "/api/users/1", body: `{"pseudo":"A"}`, status: 401},
		{name: "update invalid path id", method: http.MethodPut, target: "/api/users/nope", body: `{"pseudo":"A"}`, userID: "1", status: 400},
		{name: "update malformed body", method: http.MethodPut, target: "/api/users/1", body: `{"pseudo":`, userID: "1", status: 400},
		{name: "set skills missing auth", method: http.MethodPut, target: "/api/users/1/skills", body: `[]`, status: 401},
		{name: "set skills invalid path id", method: http.MethodPut, target: "/api/users/nope/skills", body: `[]`, userID: "1", status: 400},
		{name: "set skills malformed body", method: http.MethodPut, target: "/api/users/1/skills", body: `[`, userID: "1", status: 400},
	}

	store := newMemoryStore()
	seedUser(store, "Alice")
	handler := newTestApplication(store)

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := testutil.PerformRequest(handler, test.method, test.target, test.body, test.userID)
			if response.Code != test.status {
				t.Fatalf("status = %d, want %d; body = %s", response.Code, test.status, response.Body.String())
			}
		})
	}
}
