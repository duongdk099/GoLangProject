package stats

import (
	"context"
	"net/http"

	"barterswap/pkg/httpapi"
)

type UseCasesInterface interface {
	Get(ctx context.Context, userID int) (UserStats, error)
}

type Handler struct {
	stats UseCasesInterface
}

func NewHandler(stats UseCasesInterface) *Handler {
	return &Handler{stats: stats}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/users/{id}/stats", h.get)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	userID, ok := httpapi.PathID(w, r)
	if !ok {
		return
	}
	stats, err := h.stats.Get(r.Context(), userID)
	if err != nil {
		httpapi.WriteAPIError(w, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, stats)
}
