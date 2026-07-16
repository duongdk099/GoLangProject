package users

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"barterswap/pkg/httpapi"
)

const WelcomeCredits = 10

type Store interface {
	Create(context.Context, CreateUserParams) (User, error)
	GetByID(context.Context, int) (User, error)
	Update(context.Context, int, UpdateUserParams) (User, error)
	ListSkills(context.Context, int) ([]Skill, error)
	ReplaceSkills(context.Context, int, []Skill) error
	Exists(context.Context, int) (bool, error)
	HasSkill(context.Context, int, string) (bool, error)
}

type Service struct {
	store Store
}

func NewService(store Store) *Service {
	return &Service{store: store}
}

func (s *Service) Create(ctx context.Context, request CreateUserRequest) (User, error) {
	params := CreateUserParams{
		Pseudo: strings.TrimSpace(request.Pseudo),
		Bio:    strings.TrimSpace(request.Bio),
		Ville:  strings.TrimSpace(request.Ville),
	}
	if err := validateProfile(params.Pseudo, params.Bio, params.Ville); err != nil {
		return User{}, err
	}

	return s.store.Create(ctx, params)
}

func (s *Service) Get(ctx context.Context, userID int) (User, error) {
	if userID <= 0 {
		return User{}, fmt.Errorf("%w: user ID must be positive", httpapi.ErrValidation)
	}

	user, err := s.store.GetByID(ctx, userID)
	if err != nil {
		return User{}, err
	}
	user.Skills, err = s.store.ListSkills(ctx, userID)
	if err != nil {
		return User{}, err
	}
	if user.Skills == nil {
		user.Skills = []Skill{}
	}
	return user, nil
}

func (s *Service) Update(ctx context.Context, actorID, userID int, request UpdateUserRequest) (User, error) {
	if actorID != userID {
		return User{}, fmt.Errorf("%w: users may only update their own profile", httpapi.ErrForbidden)
	}

	params := UpdateUserParams{
		Pseudo: strings.TrimSpace(request.Pseudo),
		Bio:    strings.TrimSpace(request.Bio),
		Ville:  strings.TrimSpace(request.Ville),
	}
	if err := validateProfile(params.Pseudo, params.Bio, params.Ville); err != nil {
		return User{}, err
	}

	user, err := s.store.Update(ctx, userID, params)
	if err != nil {
		return User{}, err
	}
	user.Skills, err = s.store.ListSkills(ctx, userID)
	if err != nil {
		return User{}, err
	}
	if user.Skills == nil {
		user.Skills = []Skill{}
	}
	return user, nil
}

func (s *Service) Skills(ctx context.Context, userID int) ([]Skill, error) {
	if userID <= 0 {
		return nil, fmt.Errorf("%w: user ID must be positive", httpapi.ErrValidation)
	}
	exists, err := s.store.Exists(ctx, userID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, httpapi.ErrNotFound
	}

	skills, err := s.store.ListSkills(ctx, userID)
	if err != nil {
		return nil, err
	}
	if skills == nil {
		skills = []Skill{}
	}
	return skills, nil
}

func (s *Service) SetSkills(ctx context.Context, actorID, userID int, skills []Skill) ([]Skill, error) {
	if actorID != userID {
		return nil, fmt.Errorf("%w: users may only update their own skills", httpapi.ErrForbidden)
	}
	if len(skills) > 50 {
		return nil, fmt.Errorf("%w: a user cannot define more than 50 skills", httpapi.ErrValidation)
	}

	normalized := make([]Skill, len(skills))
	seen := make(map[string]struct{}, len(skills))
	for i, skill := range skills {
		normalized[i] = Skill{
			Nom:    strings.TrimSpace(skill.Nom),
			Niveau: strings.ToLower(strings.TrimSpace(skill.Niveau)),
		}
		if normalized[i].Nom == "" {
			return nil, fmt.Errorf("%w: skill %d must have a name", httpapi.ErrValidation, i+1)
		}
		if utf8.RuneCountInString(normalized[i].Nom) > 100 {
			return nil, fmt.Errorf("%w: skill %d name is too long", httpapi.ErrValidation, i+1)
		}
		if !validSkillLevel(normalized[i].Niveau) {
			return nil, fmt.Errorf("%w: skill %d level must be débutant, intermédiaire, or expert", httpapi.ErrValidation, i+1)
		}

		key := strings.ToLower(normalized[i].Nom)
		if _, duplicate := seen[key]; duplicate {
			return nil, fmt.Errorf("%w: duplicate skill %q", httpapi.ErrValidation, normalized[i].Nom)
		}
		seen[key] = struct{}{}
	}

	if err := s.store.ReplaceSkills(ctx, userID, normalized); err != nil {
		return nil, err
	}
	return normalized, nil
}

func (s *Service) UserExists(ctx context.Context, userID int) (bool, error) {
	if userID <= 0 {
		return false, nil
	}
	return s.store.Exists(ctx, userID)
}

func (s *Service) UserHasSkill(ctx context.Context, userID int, name string) (bool, error) {
	name = strings.TrimSpace(name)
	if userID <= 0 || name == "" {
		return false, nil
	}
	return s.store.HasSkill(ctx, userID, name)
}

func validateProfile(pseudo, bio, ville string) error {
	if pseudo == "" {
		return fmt.Errorf("%w: pseudo is required", httpapi.ErrValidation)
	}
	if utf8.RuneCountInString(pseudo) > 50 {
		return fmt.Errorf("%w: pseudo cannot exceed 50 characters", httpapi.ErrValidation)
	}
	if utf8.RuneCountInString(bio) > 1000 {
		return fmt.Errorf("%w: bio cannot exceed 1000 characters", httpapi.ErrValidation)
	}
	if utf8.RuneCountInString(ville) > 100 {
		return fmt.Errorf("%w: ville cannot exceed 100 characters", httpapi.ErrValidation)
	}
	return nil
}

func validSkillLevel(level string) bool {
	switch level {
	case "débutant", "intermédiaire", "expert":
		return true
	default:
		return false
	}
}
