package main

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestUserServiceCreate(t *testing.T) {
	tests := []struct {
		name      string
		request   CreateUserRequest
		wantError bool
	}{
		{
			name:    "valid profile is trimmed",
			request: CreateUserRequest{Pseudo: "  Alice  ", Bio: "  Developer  ", Ville: "  Paris  "},
		},
		{
			name:      "empty pseudo",
			request:   CreateUserRequest{Pseudo: "   "},
			wantError: true,
		},
		{
			name:      "pseudo too long",
			request:   CreateUserRequest{Pseudo: strings.Repeat("a", 51)},
			wantError: true,
		},
		{
			name:      "bio too long",
			request:   CreateUserRequest{Pseudo: "Alice", Bio: strings.Repeat("a", 1001)},
			wantError: true,
		},
		{
			name:      "city too long",
			request:   CreateUserRequest{Pseudo: "Alice", Ville: strings.Repeat("a", 101)},
			wantError: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := NewUserService(newMemoryUserStore())
			user, err := service.Create(context.Background(), test.request)
			if test.wantError {
				if !errors.Is(err, ErrValidation) {
					t.Fatalf("Create() error = %v, want validation error", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Create() error = %v", err)
			}
			if user.Pseudo != "Alice" || user.Bio != "Developer" || user.Ville != "Paris" {
				t.Fatalf("Create() user was not normalized: %+v", user)
			}
			if user.CreditBalance != WelcomeCredits {
				t.Fatalf("Create() balance = %d, want %d", user.CreditBalance, WelcomeCredits)
			}
		})
	}
}

func TestUserServiceGetAndUpdate(t *testing.T) {
	store := newMemoryUserStore()
	user := seedUser(store, "Alice")
	store.skills[user.ID] = []Skill{{Nom: "Go", Niveau: "expert"}}
	service := NewUserService(store)

	got, err := service.Get(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if len(got.Skills) != 1 || got.Skills[0].Nom != "Go" {
		t.Fatalf("Get() skills = %+v", got.Skills)
	}

	_, err = service.Update(context.Background(), 99, user.ID, UpdateUserRequest{Pseudo: "Other"})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("Update() error = %v, want forbidden", err)
	}

	updated, err := service.Update(context.Background(), user.ID, user.ID, UpdateUserRequest{
		Pseudo: " Alice 2 ", Bio: " New bio ", Ville: " Lyon ",
	})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if updated.Pseudo != "Alice 2" || updated.Bio != "New bio" || updated.Ville != "Lyon" {
		t.Fatalf("Update() = %+v", updated)
	}

	if _, err := service.Get(context.Background(), 999); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get(missing) error = %v, want not found", err)
	}
	if _, err := service.Get(context.Background(), 0); !errors.Is(err, ErrValidation) {
		t.Fatalf("Get(invalid ID) error = %v, want validation", err)
	}
}

func TestUserServiceSetSkills(t *testing.T) {
	tests := []struct {
		name      string
		skills    []Skill
		want      []Skill
		wantError bool
	}{
		{
			name: "normalizes valid skills",
			skills: []Skill{
				{Nom: "  Go  ", Niveau: " EXPERT "},
				{Nom: "Cuisine", Niveau: "Débutant"},
			},
			want: []Skill{
				{Nom: "Go", Niveau: "expert"},
				{Nom: "Cuisine", Niveau: "débutant"},
			},
		},
		{name: "can clear skills", skills: []Skill{}, want: []Skill{}},
		{name: "empty name", skills: []Skill{{Niveau: "expert"}}, wantError: true},
		{name: "invalid level", skills: []Skill{{Nom: "Go", Niveau: "master"}}, wantError: true},
		{
			name: "duplicate names are case insensitive",
			skills: []Skill{
				{Nom: "Go", Niveau: "expert"},
				{Nom: "go", Niveau: "débutant"},
			},
			wantError: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := newMemoryUserStore()
			user := seedUser(store, "Alice")
			service := NewUserService(store)

			got, err := service.SetSkills(context.Background(), user.ID, user.ID, test.skills)
			if test.wantError {
				if !errors.Is(err, ErrValidation) {
					t.Fatalf("SetSkills() error = %v, want validation", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("SetSkills() error = %v", err)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("SetSkills() = %+v, want %+v", got, test.want)
			}
		})
	}
}

func TestUserServiceSkillAuthorizationAndLookups(t *testing.T) {
	store := newMemoryUserStore()
	user := seedUser(store, "Alice")
	service := NewUserService(store)

	if _, err := service.SetSkills(context.Background(), user.ID+1, user.ID, nil); !errors.Is(err, ErrForbidden) {
		t.Fatalf("SetSkills() error = %v, want forbidden", err)
	}
	if _, err := service.SetSkills(context.Background(), 999, 999, nil); !errors.Is(err, ErrNotFound) {
		t.Fatalf("SetSkills(missing) error = %v, want not found", err)
	}
	if _, err := service.Skills(context.Background(), 999); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Skills(missing) error = %v, want not found", err)
	}
	if _, err := service.Skills(context.Background(), 0); !errors.Is(err, ErrValidation) {
		t.Fatalf("Skills(invalid ID) error = %v, want validation", err)
	}

	_, err := service.SetSkills(context.Background(), user.ID, user.ID, []Skill{{Nom: "Go", Niveau: "expert"}})
	if err != nil {
		t.Fatalf("SetSkills() error = %v", err)
	}
	exists, err := service.UserExists(context.Background(), user.ID)
	if err != nil || !exists {
		t.Fatalf("UserExists() = %v, %v", exists, err)
	}
	hasSkill, err := service.UserHasSkill(context.Background(), user.ID, "go")
	if err != nil || !hasSkill {
		t.Fatalf("UserHasSkill() = %v, %v", hasSkill, err)
	}
	if exists, err := service.UserExists(context.Background(), -1); err != nil || exists {
		t.Fatalf("UserExists(invalid) = %v, %v", exists, err)
	}
	if has, err := service.UserHasSkill(context.Background(), user.ID, " "); err != nil || has {
		t.Fatalf("UserHasSkill(empty) = %v, %v", has, err)
	}
}
