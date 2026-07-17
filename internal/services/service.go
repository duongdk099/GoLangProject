package services

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"barterswap/pkg/httpapi"
)

type Store interface {
	Create(context.Context, CreateParams) (Service, error)
	GetByID(context.Context, int) (Service, error)
	Update(context.Context, int, UpdateParams) (Service, error)
	Delete(context.Context, int) error
	List(context.Context, Filter) ([]Service, error)
}

type SkillChecker interface {
	UserHasSkill(ctx context.Context, userID int, name string) (bool, error)
}

type UseCases struct {
	store  Store
	skills SkillChecker
}

func NewUseCases(store Store, skills SkillChecker) *UseCases {
	return &UseCases{store: store, skills: skills}
}

func (s *UseCases) Create(ctx context.Context, providerID int, request CreateRequest) (Service, error) {
	params := CreateParams{
		ProviderID:   providerID,
		Titre:        strings.TrimSpace(request.Titre),
		Description:  strings.TrimSpace(request.Description),
		Categorie:    strings.TrimSpace(request.Categorie),
		DureeMinutes: request.DureeMinutes,
		Credits:      request.Credits,
		Ville:        strings.TrimSpace(request.Ville),
	}
	if err := validateFields(params.Titre, params.Description, params.Categorie, params.Ville, params.DureeMinutes, params.Credits); err != nil {
		return Service{}, err
	}

	hasSkill, err := s.skills.UserHasSkill(ctx, providerID, params.Categorie)
	if err != nil {
		return Service{}, err
	}
	if !hasSkill {
		return Service{}, fmt.Errorf("%w: you can only publish a service for a skill you have", httpapi.ErrValidation)
	}

	return s.store.Create(ctx, params)
}

func (s *UseCases) Get(ctx context.Context, serviceID int) (Service, error) {
	if serviceID <= 0 {
		return Service{}, fmt.Errorf("%w: service ID must be positive", httpapi.ErrValidation)
	}
	return s.store.GetByID(ctx, serviceID)
}

func (s *UseCases) Update(ctx context.Context, actorID, serviceID int, request UpdateRequest) (Service, error) {
	existing, err := s.Get(ctx, serviceID)
	if err != nil {
		return Service{}, err
	}
	if existing.ProviderID != actorID {
		return Service{}, fmt.Errorf("%w: only the owner may update this service", httpapi.ErrForbidden)
	}

	params := UpdateParams{
		Titre:        strings.TrimSpace(request.Titre),
		Description:  strings.TrimSpace(request.Description),
		Categorie:    strings.TrimSpace(request.Categorie),
		DureeMinutes: request.DureeMinutes,
		Credits:      request.Credits,
		Ville:        strings.TrimSpace(request.Ville),
		Actif:        request.Actif,
	}
	if err := validateFields(params.Titre, params.Description, params.Categorie, params.Ville, params.DureeMinutes, params.Credits); err != nil {
		return Service{}, err
	}

	if params.Categorie != existing.Categorie {
		hasSkill, err := s.skills.UserHasSkill(ctx, actorID, params.Categorie)
		if err != nil {
			return Service{}, err
		}
		if !hasSkill {
			return Service{}, fmt.Errorf("%w: you can only publish a service for a skill you have", httpapi.ErrValidation)
		}
	}

	return s.store.Update(ctx, serviceID, params)
}

func (s *UseCases) Delete(ctx context.Context, actorID, serviceID int) error {
	existing, err := s.Get(ctx, serviceID)
	if err != nil {
		return err
	}
	if existing.ProviderID != actorID {
		return fmt.Errorf("%w: only the owner may delete this service", httpapi.ErrForbidden)
	}
	return s.store.Delete(ctx, serviceID)
}

func (s *UseCases) List(ctx context.Context, filter Filter) ([]Service, error) {
	filter.Categorie = strings.TrimSpace(filter.Categorie)
	filter.Ville = strings.TrimSpace(filter.Ville)
	filter.Search = strings.TrimSpace(filter.Search)
	if filter.Categorie != "" && !validCategory(filter.Categorie) {
		return nil, fmt.Errorf("%w: unknown categorie filter %q", httpapi.ErrValidation, filter.Categorie)
	}

	services, err := s.store.List(ctx, filter)
	if err != nil {
		return nil, err
	}
	if services == nil {
		services = []Service{}
	}
	return services, nil
}

func validateFields(titre, description, categorie, ville string, dureeMinutes, credits int) error {
	if titre == "" {
		return fmt.Errorf("%w: titre is required", httpapi.ErrValidation)
	}
	if utf8.RuneCountInString(titre) > 150 {
		return fmt.Errorf("%w: titre cannot exceed 150 characters", httpapi.ErrValidation)
	}
	if utf8.RuneCountInString(description) > 2000 {
		return fmt.Errorf("%w: description cannot exceed 2000 characters", httpapi.ErrValidation)
	}
	if !validCategory(categorie) {
		return fmt.Errorf("%w: categorie must be one of the allowed values", httpapi.ErrValidation)
	}
	if utf8.RuneCountInString(ville) > 100 {
		return fmt.Errorf("%w: ville cannot exceed 100 characters", httpapi.ErrValidation)
	}
	if dureeMinutes <= 0 {
		return fmt.Errorf("%w: duree_minutes must be positive", httpapi.ErrValidation)
	}
	if credits <= 0 {
		return fmt.Errorf("%w: credits must be positive", httpapi.ErrValidation)
	}
	return nil
}
