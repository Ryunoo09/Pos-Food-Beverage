package supplier

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

	q := r.URL.Query()
	items, err := h.svc.List(r.Context(), actor.BusinessID, q.Get("q"), boolParam(q.Get("is_active")))
	if err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}
	httpx.WriteData(w, http.StatusOK, items, requestID)
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFrom(r.Context())
	requestID := httpx.RequestIDFrom(r.Context())

	item, err := h.svc.Supplier(r.Context(), actor.BusinessID, r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}
	httpx.WriteData(w, http.StatusOK, item, requestID)
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFrom(r.Context())
	requestID := httpx.RequestIDFrom(r.Context())

	var in SupplierInput
	if err := httpx.DecodeJSON(w, r, &in); err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}

	item, err := h.svc.Create(r.Context(), actor.BusinessID, in)
	if err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}
	httpx.WriteData(w, http.StatusCreated, item, requestID)
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFrom(r.Context())
	requestID := httpx.RequestIDFrom(r.Context())

	var in SupplierInput
	if err := httpx.DecodeJSON(w, r, &in); err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}

	item, err := h.svc.Update(r.Context(), actor.BusinessID, r.PathValue("id"), in)
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

		if err := h.svc.SetActive(r.Context(), actor.BusinessID, r.PathValue("id"), active); err != nil {
			httpx.WriteError(w, err, requestID)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func (h *Handler) SetIngredients(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFrom(r.Context())
	requestID := httpx.RequestIDFrom(r.Context())

	var body struct {
		Ingredients []SupplierIngredientInput `json:"ingredients"`
	}
	if err := httpx.DecodeJSON(w, r, &body); err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}

	if err := h.svc.SetIngredients(r.Context(), actor.BusinessID, r.PathValue("id"), body.Ingredients); err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) PurchaseHistory(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFrom(r.Context())
	requestID := httpx.RequestIDFrom(r.Context())

	items, err := h.svc.PurchaseHistory(r.Context(), actor.OutletID, r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}
	httpx.WriteData(w, http.StatusOK, items, requestID)
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
