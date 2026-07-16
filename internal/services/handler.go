package services

import (
	"context"
	"net/http"
	"strconv"

	"barterswap/pkg/httpapi"
)

type UseCasesInterface interface {
	Create(ctx context.Context, providerID int, request CreateRequest) (Service, error)
	Get(ctx context.Context, serviceID int) (Service, error)
	Update(ctx context.Context, actorID, serviceID int, request UpdateRequest) (Service, error)
	Delete(ctx context.Context, actorID, serviceID int) error
	List(ctx context.Context, filter Filter) ([]Service, error)
}

type Handler struct {
	services UseCasesInterface
}

func NewHandler(services UseCasesInterface) *Handler {
	return &Handler{services: services}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/services", h.list)
	mux.HandleFunc("POST /api/services", h.create)
	mux.HandleFunc("GET /api/services/{id}", h.get)
	mux.HandleFunc("PUT /api/services/{id}", h.update)
	mux.HandleFunc("DELETE /api/services/{id}", h.delete)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	actorID, ok := httpapi.RequireAuthenticatedUser(w, r)
	if !ok {
		return
	}
	var request CreateRequest
	if err := httpapi.DecodeJSON(w, r, &request); err != nil {
		httpapi.WriteBadRequest(w, err.Error())
		return
	}
	service, err := h.services.Create(r.Context(), actorID, request)
	if err != nil {
		httpapi.WriteAPIError(w, err)
		return
	}
	w.Header().Set("Location", "/api/services/"+strconv.Itoa(service.ID))
	httpapi.WriteJSON(w, http.StatusCreated, service)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	serviceID, ok := httpapi.PathID(w, r)
	if !ok {
		return
	}
	service, err := h.services.Get(r.Context(), serviceID)
	if err != nil {
		httpapi.WriteAPIError(w, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, service)
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	actorID, ok := httpapi.RequireAuthenticatedUser(w, r)
	if !ok {
		return
	}
	serviceID, ok := httpapi.PathID(w, r)
	if !ok {
		return
	}
	var request UpdateRequest
	if err := httpapi.DecodeJSON(w, r, &request); err != nil {
		httpapi.WriteBadRequest(w, err.Error())
		return
	}
	service, err := h.services.Update(r.Context(), actorID, serviceID, request)
	if err != nil {
		httpapi.WriteAPIError(w, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, service)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	actorID, ok := httpapi.RequireAuthenticatedUser(w, r)
	if !ok {
		return
	}
	serviceID, ok := httpapi.PathID(w, r)
	if !ok {
		return
	}
	if err := h.services.Delete(r.Context(), actorID, serviceID); err != nil {
		httpapi.WriteAPIError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	filter := Filter{
		Categorie: query.Get("categorie"),
		Ville:     query.Get("ville"),
		Search:    query.Get("search"),
	}
	services, err := h.services.List(r.Context(), filter)
	if err != nil {
		httpapi.WriteAPIError(w, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, services)
}
