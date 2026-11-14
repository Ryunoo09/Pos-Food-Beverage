package dashboard

import (
	"net/http"

	"github.com/ryunoo09/pos-food-beverage/backend/internal/auth"
	"github.com/ryunoo09/pos-food-beverage/backend/internal/httpx"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Summary(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFrom(r.Context())
	requestID := httpx.RequestIDFrom(r.Context())

	q := r.URL.Query()
	summary, err := h.svc.Summary(r.Context(), actor, q.Get("from"), q.Get("to"))
	if err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}

	httpx.WriteData(w, http.StatusOK, summary, requestID)
}
