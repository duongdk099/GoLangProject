package exchanges

import (
	"context"
	"net/http"

	"barterswap/pkg/httpapi"
)

type UseCasesInterface interface {
	Create(ctx context.Context, requesterID int, request CreateRequest) (Exchange, error)
	List(ctx context.Context, userID int, status string) ([]Exchange, error)
	Get(ctx context.Context, actorID, exchangeID int) (Exchange, error)
	Accept(ctx context.Context, actorID, exchangeID int) (Exchange, error)
	Reject(ctx context.Context, actorID, exchangeID int) (Exchange, error)
	Complete(ctx context.Context, actorID, exchangeID int) (Exchange, error)
	Cancel(ctx context.Context, actorID, exchangeID int) (Exchange, error)
}

type Handler struct {
	exchanges UseCasesInterface
}

func NewHandler(exchanges UseCasesInterface) *Handler {
	return &Handler{exchanges: exchanges}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/exchanges", h.create)
	mux.HandleFunc("GET /api/exchanges", h.list)
	mux.HandleFunc("GET /api/exchanges/{id}", h.get)
	mux.HandleFunc("PUT /api/exchanges/{id}/accept", h.accept)
	mux.HandleFunc("PUT /api/exchanges/{id}/reject", h.reject)
	mux.HandleFunc("PUT /api/exchanges/{id}/complete", h.complete)
	mux.HandleFunc("PUT /api/exchanges/{id}/cancel", h.cancel)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	requesterID, ok := httpapi.RequireAuthenticatedUser(w, r)
	if !ok {
		return
	}
	var request CreateRequest
	if err := httpapi.DecodeJSON(w, r, &request); err != nil {
		httpapi.WriteBadRequest(w, err.Error())
		return
	}
	exchange, err := h.exchanges.Create(r.Context(), requesterID, request)
	if err != nil {
		httpapi.WriteAPIError(w, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusCreated, exchange)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	userID, ok := httpapi.RequireAuthenticatedUser(w, r)
	if !ok {
		return
	}
	exchanges, err := h.exchanges.List(r.Context(), userID, r.URL.Query().Get("status"))
	if err != nil {
		httpapi.WriteAPIError(w, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, exchanges)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	actorID, exchangeID, ok := h.authAndID(w, r)
	if !ok {
		return
	}
	exchange, err := h.exchanges.Get(r.Context(), actorID, exchangeID)
	if err != nil {
		httpapi.WriteAPIError(w, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, exchange)
}

func (h *Handler) accept(w http.ResponseWriter, r *http.Request) {
	h.runTransition(w, r, h.exchanges.Accept)
}

func (h *Handler) reject(w http.ResponseWriter, r *http.Request) {
	h.runTransition(w, r, h.exchanges.Reject)
}

func (h *Handler) complete(w http.ResponseWriter, r *http.Request) {
	h.runTransition(w, r, h.exchanges.Complete)
}

func (h *Handler) cancel(w http.ResponseWriter, r *http.Request) {
	h.runTransition(w, r, h.exchanges.Cancel)
}

func (h *Handler) runTransition(w http.ResponseWriter, r *http.Request, action func(context.Context, int, int) (Exchange, error)) {
	actorID, exchangeID, ok := h.authAndID(w, r)
	if !ok {
		return
	}
	exchange, err := action(r.Context(), actorID, exchangeID)
	if err != nil {
		httpapi.WriteAPIError(w, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, exchange)
}

func (h *Handler) authAndID(w http.ResponseWriter, r *http.Request) (actorID, exchangeID int, ok bool) {
	actorID, ok = httpapi.RequireAuthenticatedUser(w, r)
	if !ok {
		return 0, 0, false
	}
	exchangeID, ok = httpapi.PathID(w, r)
	if !ok {
		return 0, 0, false
	}
	return actorID, exchangeID, true
}
