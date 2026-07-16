package reviews

import (
	"context"
	"net/http"

	"barterswap/pkg/httpapi"
)

type UseCasesInterface interface {
	Create(ctx context.Context, actorID, exchangeID int, request CreateRequest) (Review, error)
	ListForUser(ctx context.Context, userID int) ([]Review, error)
	ListForService(ctx context.Context, serviceID int) ([]Review, error)
}

type Handler struct {
	reviews UseCasesInterface
}

func NewHandler(reviews UseCasesInterface) *Handler {
	return &Handler{reviews: reviews}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/exchanges/{id}/review", h.create)
	mux.HandleFunc("GET /api/users/{id}/reviews", h.listForUser)
	mux.HandleFunc("GET /api/services/{id}/reviews", h.listForService)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	actorID, ok := httpapi.RequireAuthenticatedUser(w, r)
	if !ok {
		return
	}
	exchangeID, ok := httpapi.PathID(w, r)
	if !ok {
		return
	}
	var request CreateRequest
	if err := httpapi.DecodeJSON(w, r, &request); err != nil {
		httpapi.WriteBadRequest(w, err.Error())
		return
	}
	review, err := h.reviews.Create(r.Context(), actorID, exchangeID, request)
	if err != nil {
		httpapi.WriteAPIError(w, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusCreated, review)
}

func (h *Handler) listForUser(w http.ResponseWriter, r *http.Request) {
	userID, ok := httpapi.PathID(w, r)
	if !ok {
		return
	}
	reviews, err := h.reviews.ListForUser(r.Context(), userID)
	if err != nil {
		httpapi.WriteAPIError(w, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, reviews)
}

func (h *Handler) listForService(w http.ResponseWriter, r *http.Request) {
	serviceID, ok := httpapi.PathID(w, r)
	if !ok {
		return
	}
	reviews, err := h.reviews.ListForService(r.Context(), serviceID)
	if err != nil {
		httpapi.WriteAPIError(w, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, reviews)
}
