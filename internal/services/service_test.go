package services

import (
	"context"
	"errors"
	"strings"
	"testing"

	"barterswap/pkg/httpapi"
)

type memoryStore struct {
	nextID   int
	services map[int]Service
}

func newMemoryStore() *memoryStore {
	return &memoryStore{nextID: 1, services: make(map[int]Service)}
}

func (s *memoryStore) Create(_ context.Context, params CreateParams) (Service, error) {
	service := Service{
		ID:           s.nextID,
		ProviderID:   params.ProviderID,
		Titre:        params.Titre,
		Description:  params.Description,
		Categorie:    params.Categorie,
		DureeMinutes: params.DureeMinutes,
		Credits:      params.Credits,
		Ville:        params.Ville,
		Actif:        true,
		CreatedAt:    "2026-01-01T00:00:00Z",
	}
	s.nextID++
	s.services[service.ID] = service
	return service, nil
}

func (s *memoryStore) GetByID(_ context.Context, serviceID int) (Service, error) {
	service, exists := s.services[serviceID]
	if !exists {
		return Service{}, httpapi.ErrNotFound
	}
	return service, nil
}

func (s *memoryStore) Update(_ context.Context, serviceID int, params UpdateParams) (Service, error) {
	service, exists := s.services[serviceID]
	if !exists {
		return Service{}, httpapi.ErrNotFound
	}
	service.Titre = params.Titre
	service.Description = params.Description
	service.Categorie = params.Categorie
	service.DureeMinutes = params.DureeMinutes
	service.Credits = params.Credits
	service.Ville = params.Ville
	service.Actif = params.Actif
	s.services[serviceID] = service
	return service, nil
}

func (s *memoryStore) Delete(_ context.Context, serviceID int) error {
	if _, exists := s.services[serviceID]; !exists {
		return httpapi.ErrNotFound
	}
	delete(s.services, serviceID)
	return nil
}

func (s *memoryStore) List(_ context.Context, filter Filter) ([]Service, error) {
	results := make([]Service, 0)
	for _, service := range s.services {
		if filter.Categorie != "" && service.Categorie != filter.Categorie {
			continue
		}
		if filter.Ville != "" && service.Ville != filter.Ville {
			continue
		}
		if filter.Search != "" &&
			!strings.Contains(strings.ToLower(service.Titre), strings.ToLower(filter.Search)) &&
			!strings.Contains(strings.ToLower(service.Description), strings.ToLower(filter.Search)) {
			continue
		}
		results = append(results, service)
	}
	return results, nil
}

type stubSkillChecker struct {
	skills map[int]map[string]bool
}

func newStubSkillChecker() *stubSkillChecker {
	return &stubSkillChecker{skills: make(map[int]map[string]bool)}
}

func (s *stubSkillChecker) grant(userID int, skill string) {
	if s.skills[userID] == nil {
		s.skills[userID] = make(map[string]bool)
	}
	s.skills[userID][skill] = true
}

func (s *stubSkillChecker) UserHasSkill(_ context.Context, userID int, name string) (bool, error) {
	return s.skills[userID][name], nil
}

func TestUseCasesCreate(t *testing.T) {
	tests := []struct {
		name       string
		request    CreateRequest
		grantSkill bool
		wantError  bool
		wantErr    error
	}{
		{
			name:       "valid service",
			request:    CreateRequest{Titre: "Cours de Go", Categorie: "Informatique", DureeMinutes: 60, Credits: 2, Ville: "Paris"},
			grantSkill: true,
		},
		{
			name:      "empty title",
			request:   CreateRequest{Titre: "  ", Categorie: "Informatique", DureeMinutes: 60, Credits: 2},
			wantError: true,
			wantErr:   httpapi.ErrValidation,
		},
		{
			name:      "invalid category",
			request:   CreateRequest{Titre: "X", Categorie: "Espace", DureeMinutes: 60, Credits: 2},
			wantError: true,
			wantErr:   httpapi.ErrValidation,
		},
		{
			name:      "non positive duration",
			request:   CreateRequest{Titre: "X", Categorie: "Informatique", DureeMinutes: 0, Credits: 2},
			wantError: true,
			wantErr:   httpapi.ErrValidation,
		},
		{
			name:      "non positive credits",
			request:   CreateRequest{Titre: "X", Categorie: "Informatique", DureeMinutes: 30, Credits: 0},
			wantError: true,
			wantErr:   httpapi.ErrValidation,
		},
		{
			name:      "missing skill",
			request:   CreateRequest{Titre: "X", Categorie: "Informatique", DureeMinutes: 30, Credits: 1},
			wantError: true,
			wantErr:   httpapi.ErrValidation,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			skills := newStubSkillChecker()
			if test.grantSkill {
				skills.grant(1, "Informatique")
			}
			useCases := NewUseCases(newMemoryStore(), skills)

			service, err := useCases.Create(context.Background(), 1, test.request)
			if test.wantError {
				if !errors.Is(err, test.wantErr) {
					t.Fatalf("Create() error = %v, want %v", err, test.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Create() error = %v", err)
			}
			if service.ProviderID != 1 || !service.Actif {
				t.Fatalf("Create() = %+v", service)
			}
		})
	}
}

func TestUseCasesUpdateAndDelete(t *testing.T) {
	skills := newStubSkillChecker()
	skills.grant(1, "Informatique")
	skills.grant(1, "Cuisine")
	store := newMemoryStore()
	useCases := NewUseCases(store, skills)

	created, err := useCases.Create(context.Background(), 1, CreateRequest{
		Titre: "Cours de Go", Categorie: "Informatique", DureeMinutes: 60, Credits: 2,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if _, err := useCases.Update(context.Background(), 2, created.ID, UpdateRequest{
		Titre: "X", Categorie: "Informatique", DureeMinutes: 30, Credits: 1, Actif: true,
	}); !errors.Is(err, httpapi.ErrForbidden) {
		t.Fatalf("Update(wrong owner) error = %v, want forbidden", err)
	}

	updated, err := useCases.Update(context.Background(), 1, created.ID, UpdateRequest{
		Titre: "Cours de cuisine", Categorie: "Cuisine", DureeMinutes: 45, Credits: 3, Actif: false,
	})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if updated.Categorie != "Cuisine" || updated.Actif {
		t.Fatalf("Update() = %+v", updated)
	}

	if err := useCases.Delete(context.Background(), 2, created.ID); !errors.Is(err, httpapi.ErrForbidden) {
		t.Fatalf("Delete(wrong owner) error = %v, want forbidden", err)
	}
	if err := useCases.Delete(context.Background(), 1, created.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err := useCases.Get(context.Background(), created.ID); !errors.Is(err, httpapi.ErrNotFound) {
		t.Fatalf("Get(deleted) error = %v, want not found", err)
	}
}

func TestUseCasesList(t *testing.T) {
	skills := newStubSkillChecker()
	skills.grant(1, "Informatique")
	skills.grant(1, "Jardinage")
	store := newMemoryStore()
	useCases := NewUseCases(store, skills)

	_, _ = useCases.Create(context.Background(), 1, CreateRequest{
		Titre: "Cours de Go avancé", Categorie: "Informatique", DureeMinutes: 60, Credits: 2, Ville: "Paris",
	})
	_, _ = useCases.Create(context.Background(), 1, CreateRequest{
		Titre: "Taille de haies", Categorie: "Jardinage", DureeMinutes: 90, Credits: 3, Ville: "Lyon",
	})

	byCategory, err := useCases.List(context.Background(), Filter{Categorie: "Jardinage"})
	if err != nil || len(byCategory) != 1 {
		t.Fatalf("List(categorie) = %+v, err = %v", byCategory, err)
	}

	byVille, err := useCases.List(context.Background(), Filter{Ville: "Paris"})
	if err != nil || len(byVille) != 1 {
		t.Fatalf("List(ville) = %+v, err = %v", byVille, err)
	}

	bySearch, err := useCases.List(context.Background(), Filter{Search: "go"})
	if err != nil || len(bySearch) != 1 {
		t.Fatalf("List(search) = %+v, err = %v", bySearch, err)
	}

	if _, err := useCases.List(context.Background(), Filter{Categorie: "Espace"}); !errors.Is(err, httpapi.ErrValidation) {
		t.Fatalf("List(invalid categorie) error = %v, want validation", err)
	}
}
