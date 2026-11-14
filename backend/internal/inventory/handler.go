package inventory

import (
	"net/http"
	"strconv"

	"github.com/ryunoo09/pos-food-beverage/backend/internal/auth"
	"github.com/ryunoo09/pos-food-beverage/backend/internal/httpx"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Units(w http.ResponseWriter, r *http.Request) {
	requestID := httpx.RequestIDFrom(r.Context())

	items, err := h.svc.Units(r.Context())
	if err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}
	httpx.WriteData(w, http.StatusOK, items, requestID)
}

func (h *Handler) ListCategories(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFrom(r.Context())
	requestID := httpx.RequestIDFrom(r.Context())

	items, err := h.svc.Categories(r.Context(), actor.BusinessID)
	if err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}
	httpx.WriteData(w, http.StatusOK, items, requestID)
}

func (h *Handler) CreateCategory(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFrom(r.Context())
	requestID := httpx.RequestIDFrom(r.Context())

	var in IngredientCategoryInput
	if err := httpx.DecodeJSON(w, r, &in); err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}

	item, err := h.svc.CreateCategory(r.Context(), actor.BusinessID, in)
	if err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}
	httpx.WriteData(w, http.StatusCreated, item, requestID)
}

func (h *Handler) UpdateCategory(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFrom(r.Context())
	requestID := httpx.RequestIDFrom(r.Context())

	var in IngredientCategoryInput
	if err := httpx.DecodeJSON(w, r, &in); err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}

	item, err := h.svc.UpdateCategory(r.Context(), actor.BusinessID, r.PathValue("id"), in)
	if err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}
	httpx.WriteData(w, http.StatusOK, item, requestID)
}

func (h *Handler) DeleteCategory(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFrom(r.Context())
	requestID := httpx.RequestIDFrom(r.Context())

	if err := h.svc.DeleteCategory(r.Context(), actor.BusinessID, r.PathValue("id")); err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ListIngredients(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFrom(r.Context())
	requestID := httpx.RequestIDFrom(r.Context())

	q := r.URL.Query()
	page, perPage := pagination(r)
	filter := IngredientFilter{
		Query:   q.Get("q"),
		Active:  boolParam(q.Get("is_active")),
		Page:    page,
		PerPage: perPage,
	}

	items, total, err := h.svc.ListIngredients(r.Context(), actor.BusinessID, actor.OutletID, filter)
	if err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}
	httpx.WritePage(w, items, httpx.PageMeta{
		Page: page, PerPage: perPage, Total: total,
		TotalPages: (total + perPage - 1) / perPage, RequestID: requestID,
	})
}

func (h *Handler) GetIngredient(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFrom(r.Context())
	requestID := httpx.RequestIDFrom(r.Context())

	item, err := h.svc.Ingredient(r.Context(), actor.BusinessID, actor.OutletID, r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}
	httpx.WriteData(w, http.StatusOK, item, requestID)
}

func (h *Handler) CreateIngredient(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFrom(r.Context())
	requestID := httpx.RequestIDFrom(r.Context())

	var in IngredientInput
	if err := httpx.DecodeJSON(w, r, &in); err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}

	item, err := h.svc.CreateIngredient(r.Context(), actor, in)
	if err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}
	httpx.WriteData(w, http.StatusCreated, item, requestID)
}

func (h *Handler) UpdateIngredient(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFrom(r.Context())
	requestID := httpx.RequestIDFrom(r.Context())

	var in IngredientInput
	if err := httpx.DecodeJSON(w, r, &in); err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}

	item, err := h.svc.UpdateIngredient(r.Context(), actor, r.PathValue("id"), in)
	if err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}
	httpx.WriteData(w, http.StatusOK, item, requestID)
}

func (h *Handler) SetIngredientActive(active bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, _ := auth.ActorFrom(r.Context())
		requestID := httpx.RequestIDFrom(r.Context())

		if err := h.svc.SetIngredientActive(r.Context(), actor.BusinessID, r.PathValue("id"), active); err != nil {
			httpx.WriteError(w, err, requestID)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func (h *Handler) Movements(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFrom(r.Context())
	requestID := httpx.RequestIDFrom(r.Context())

	items, err := h.svc.Movements(r.Context(), actor, r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}
	httpx.WriteData(w, http.StatusOK, items, requestID)
}

func (h *Handler) Stock(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFrom(r.Context())
	requestID := httpx.RequestIDFrom(r.Context())

	page, perPage := pagination(r)
	q := r.URL.Query()
	low := boolParam(q.Get("low"))
	filter := StockFilter{
		Query:   q.Get("q"),
		LowOnly: low != nil && *low,
		Page:    page,
		PerPage: perPage,
	}

	items, total, err := h.svc.Stock(r.Context(), actor, filter)
	if err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}
	httpx.WritePage(w, items, httpx.PageMeta{
		Page: page, PerPage: perPage, Total: total,
		TotalPages: (total + perPage - 1) / perPage, RequestID: requestID,
	})
}

func (h *Handler) StockMovements(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFrom(r.Context())
	requestID := httpx.RequestIDFrom(r.Context())

	items, err := h.svc.Movements(r.Context(), actor, r.PathValue("ingredientId"))
	if err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}
	httpx.WriteData(w, http.StatusOK, items, requestID)
}

func (h *Handler) Adjust(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFrom(r.Context())
	requestID := httpx.RequestIDFrom(r.Context())

	var in AdjustInput
	if err := httpx.DecodeJSON(w, r, &in); err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}

	result, err := h.svc.Adjust(r.Context(), actor, r.PathValue("ingredientId"), in)
	if err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}
	httpx.WriteData(w, http.StatusOK, result, requestID)
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

func pagination(r *http.Request) (int, int) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}
	return page, perPage
}
