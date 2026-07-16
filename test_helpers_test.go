package main

import (
	"context"
	"strings"
)

type memoryUserStore struct {
	nextID int
	users  map[int]User
	skills map[int][]Skill
}

func newMemoryUserStore() *memoryUserStore {
	return &memoryUserStore{
		nextID: 1,
		users:  make(map[int]User),
		skills: make(map[int][]Skill),
	}
}

func (s *memoryUserStore) Create(_ context.Context, params CreateUserParams) (User, error) {
	user := User{
		ID:            s.nextID,
		Pseudo:        params.Pseudo,
		Bio:           params.Bio,
		Ville:         params.Ville,
		Skills:        []Skill{},
		CreditBalance: WelcomeCredits,
		CreatedAt:     "2026-01-01T00:00:00Z",
	}
	s.nextID++
	s.users[user.ID] = user
	return user, nil
}

func (s *memoryUserStore) GetByID(_ context.Context, userID int) (User, error) {
	user, exists := s.users[userID]
	if !exists {
		return User{}, ErrNotFound
	}
	return user, nil
}

func (s *memoryUserStore) Update(_ context.Context, userID int, params UpdateUserParams) (User, error) {
	user, exists := s.users[userID]
	if !exists {
		return User{}, ErrNotFound
	}
	user.Pseudo = params.Pseudo
	user.Bio = params.Bio
	user.Ville = params.Ville
	s.users[userID] = user
	return user, nil
}

func (s *memoryUserStore) ListSkills(_ context.Context, userID int) ([]Skill, error) {
	skills := s.skills[userID]
	return append([]Skill(nil), skills...), nil
}

func (s *memoryUserStore) ReplaceSkills(_ context.Context, userID int, skills []Skill) error {
	if _, exists := s.users[userID]; !exists {
		return ErrNotFound
	}
	s.skills[userID] = append([]Skill(nil), skills...)
	return nil
}

func (s *memoryUserStore) Exists(_ context.Context, userID int) (bool, error) {
	_, exists := s.users[userID]
	return exists, nil
}

func (s *memoryUserStore) HasSkill(_ context.Context, userID int, name string) (bool, error) {
	for _, skill := range s.skills[userID] {
		if strings.EqualFold(skill.Nom, name) {
			return true, nil
		}
	}
	return false, nil
}

func seedUser(store *memoryUserStore, pseudo string) User {
	user, _ := store.Create(context.Background(), CreateUserParams{Pseudo: pseudo})
	return user
}
