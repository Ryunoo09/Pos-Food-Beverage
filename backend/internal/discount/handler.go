package discount

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

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFrom(r.Context())
	requestID := httpx.RequestIDFrom(r.Context())

	items, err := h.svc.List(r.Context(), actor, boolParam(r.URL.Query().Get("is_active")))
	if err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}
	httpx.WriteData(w, http.StatusOK, items, requestID)
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFrom(r.Context())
	requestID := httpx.RequestIDFrom(r.Context())

	item, err := h.svc.Discount(r.Context(), actor, r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}
	httpx.WriteData(w, http.StatusOK, item, requestID)
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFrom(r.Context())
	requestID := httpx.RequestIDFrom(r.Context())

	var in Input
	if err := httpx.DecodeJSON(w, r, &in); err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}

	item, err := h.svc.Create(r.Context(), actor, in)
	if err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}
	httpx.WriteData(w, http.StatusCreated, item, requestID)
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFrom(r.Context())
	requestID := httpx.RequestIDFrom(r.Context())

	var in Input
	if err := httpx.DecodeJSON(w, r, &in); err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}

	item, err := h.svc.Update(r.Context(), actor, r.PathValue("id"), in)
	if err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}
	httpx.WriteData(w, http.StatusOK, item, requestID)
}

func (h *Handler) SetActive(active bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, _ := auth.ActorFrom(r.Context())
		requestID := httpx.RequestIDFrom(r.Context())

		if err := h.svc.SetActive(r.Context(), actor, r.PathValue("id"), active); err != nil {
			httpx.WriteError(w, err, requestID)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func boolParam(raw string) *bool {
	switch raw {
	case "true", "1":
		b := true
		return &b
	case "false", "0":
		b := false
		return &b
	default:
		return nil
	}
}
