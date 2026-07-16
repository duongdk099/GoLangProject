package users

import (
	"context"
	"fmt"
	"net/http"

	"barterswap/pkg/httpapi"
)

type UseCases interface {
	Create(context.Context, CreateUserRequest) (User, error)
	Get(context.Context, int) (User, error)
	Update(context.Context, int, int, UpdateUserRequest) (User, error)
	Skills(context.Context, int) ([]Skill, error)
	SetSkills(context.Context, int, int, []Skill) ([]Skill, error)
}

type Handler struct {
	users UseCases
}

func NewHandler(users UseCases) *Handler {
	return &Handler{users: users}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/users", h.create)
	mux.HandleFunc("GET /api/users/{id}", h.get)
	mux.HandleFunc("PUT /api/users/{id}", h.update)
	mux.HandleFunc("GET /api/users/{id}/skills", h.getSkills)
	mux.HandleFunc("PUT /api/users/{id}/skills", h.setSkills)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var request CreateUserRequest
	if err := httpapi.DecodeJSON(w, r, &request); err != nil {
		httpapi.WriteBadRequest(w, err.Error())
		return
	}

	user, err := h.users.Create(r.Context(), request)
	if err != nil {
		httpapi.WriteAPIError(w, err)
		return
	}
	w.Header().Set("Location", fmt.Sprintf("/api/users/%d", user.ID))
	httpapi.WriteJSON(w, http.StatusCreated, user)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	userID, ok := httpapi.PathID(w, r)
	if !ok {
		return
	}
	user, err := h.users.Get(r.Context(), userID)
	if err != nil {
		httpapi.WriteAPIError(w, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, user)
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	actorID, ok := httpapi.RequireAuthenticatedUser(w, r)
	if !ok {
		return
	}
	userID, ok := httpapi.PathID(w, r)
	if !ok {
		return
	}

	var request UpdateUserRequest
	if err := httpapi.DecodeJSON(w, r, &request); err != nil {
		httpapi.WriteBadRequest(w, err.Error())
		return
	}
	user, err := h.users.Update(r.Context(), actorID, userID, request)
	if err != nil {
		httpapi.WriteAPIError(w, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, user)
}

func (h *Handler) getSkills(w http.ResponseWriter, r *http.Request) {
	userID, ok := httpapi.PathID(w, r)
	if !ok {
		return
	}
	skills, err := h.users.Skills(r.Context(), userID)
	if err != nil {
		httpapi.WriteAPIError(w, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, skills)
}

func (h *Handler) setSkills(w http.ResponseWriter, r *http.Request) {
	actorID, ok := httpapi.RequireAuthenticatedUser(w, r)
	if !ok {
		return
	}
	userID, ok := httpapi.PathID(w, r)
	if !ok {
		return
	}

	var skills []Skill
	if err := httpapi.DecodeJSON(w, r, &skills); err != nil {
		httpapi.WriteBadRequest(w, err.Error())
		return
	}
	skills, err := h.users.SetSkills(r.Context(), actorID, userID, skills)
	if err != nil {
		httpapi.WriteAPIError(w, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, skills)
}
