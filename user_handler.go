package main

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
)

type UserUseCases interface {
	Create(context.Context, CreateUserRequest) (User, error)
	Get(context.Context, int) (User, error)
	Update(context.Context, int, int, UpdateUserRequest) (User, error)
	Skills(context.Context, int) ([]Skill, error)
	SetSkills(context.Context, int, int, []Skill) ([]Skill, error)
}

type UserHandler struct {
	users UserUseCases
}

func NewUserHandler(users UserUseCases) *UserHandler {
	return &UserHandler{users: users}
}

func (h *UserHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/users", h.create)
	mux.HandleFunc("GET /api/users/{id}", h.get)
	mux.HandleFunc("PUT /api/users/{id}", h.update)
	mux.HandleFunc("GET /api/users/{id}/skills", h.getSkills)
	mux.HandleFunc("PUT /api/users/{id}/skills", h.setSkills)
}

func (h *UserHandler) create(w http.ResponseWriter, r *http.Request) {
	var request CreateUserRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeBadRequest(w, err.Error())
		return
	}

	user, err := h.users.Create(r.Context(), request)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	w.Header().Set("Location", fmt.Sprintf("/api/users/%d", user.ID))
	writeJSON(w, http.StatusCreated, user)
}

func (h *UserHandler) get(w http.ResponseWriter, r *http.Request) {
	userID, ok := pathID(w, r)
	if !ok {
		return
	}
	user, err := h.users.Get(r.Context(), userID)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, user)
}

func (h *UserHandler) update(w http.ResponseWriter, r *http.Request) {
	actorID, ok := requireAuthenticatedUser(w, r)
	if !ok {
		return
	}
	userID, ok := pathID(w, r)
	if !ok {
		return
	}

	var request UpdateUserRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	user, err := h.users.Update(r.Context(), actorID, userID, request)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, user)
}

func (h *UserHandler) getSkills(w http.ResponseWriter, r *http.Request) {
	userID, ok := pathID(w, r)
	if !ok {
		return
	}
	skills, err := h.users.Skills(r.Context(), userID)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, skills)
}

func (h *UserHandler) setSkills(w http.ResponseWriter, r *http.Request) {
	actorID, ok := requireAuthenticatedUser(w, r)
	if !ok {
		return
	}
	userID, ok := pathID(w, r)
	if !ok {
		return
	}

	var skills []Skill
	if err := decodeJSON(w, r, &skills); err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	skills, err := h.users.SetSkills(r.Context(), actorID, userID, skills)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, skills)
}

func pathID(w http.ResponseWriter, r *http.Request) (int, bool) {
	userID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || userID <= 0 {
		writeBadRequest(w, "path ID must be a positive integer")
		return 0, false
	}
	return userID, true
}
