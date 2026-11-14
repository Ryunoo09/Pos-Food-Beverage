package order

import (
	"net/http"
	"strconv"
	"strings"

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

	page, perPage := pagination(r)
	filter := Filter{Query: strings.TrimSpace(r.URL.Query().Get("q")), Page: page, PerPage: perPage}
	if raw := r.URL.Query().Get("status"); raw != "" {
		filter.Status = &raw
	}

	items, total, err := h.svc.List(r.Context(), actor, filter)
	if err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}
	httpx.WritePage(w, items, httpx.PageMeta{
		Page:       page,
		PerPage:    perPage,
		Total:      total,
		TotalPages: totalPages(total, perPage),
		RequestID:  requestID,
	})
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFrom(r.Context())
	requestID := httpx.RequestIDFrom(r.Context())

	o, err := h.svc.Get(r.Context(), actor, r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}
	httpx.WriteData(w, http.StatusOK, o, requestID)
}

func (h *Handler) CreateDraft(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFrom(r.Context())
	requestID := httpx.RequestIDFrom(r.Context())

	var in DraftInput
	if err := httpx.DecodeJSON(w, r, &in); err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}

	o, err := h.svc.CreateDraft(r.Context(), actor, in)
	if err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}
	httpx.WriteData(w, http.StatusCreated, o, requestID)
}

func (h *Handler) AddItem(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFrom(r.Context())
	requestID := httpx.RequestIDFrom(r.Context())

	var in ItemInput
	if err := httpx.DecodeJSON(w, r, &in); err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}

	o, err := h.svc.AddItem(r.Context(), actor, r.PathValue("id"), in)
	if err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}
	httpx.WriteData(w, http.StatusCreated, o, requestID)
}

func (h *Handler) UpdateItem(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFrom(r.Context())
	requestID := httpx.RequestIDFrom(r.Context())

	var in UpdateItemInput
	if err := httpx.DecodeJSON(w, r, &in); err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}

	o, err := h.svc.UpdateItem(r.Context(), actor, r.PathValue("id"), r.PathValue("itemID"), in)
	if err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}
	httpx.WriteData(w, http.StatusOK, o, requestID)
}

func (h *Handler) DeleteItem(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFrom(r.Context())
	requestID := httpx.RequestIDFrom(r.Context())

	o, err := h.svc.DeleteItem(r.Context(), actor, r.PathValue("id"), r.PathValue("itemID"))
	if err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}
	httpx.WriteData(w, http.StatusOK, o, requestID)
}

func (h *Handler) Confirm(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFrom(r.Context())
	requestID := httpx.RequestIDFrom(r.Context())

	o, err := h.svc.Confirm(r.Context(), actor, r.PathValue("id"), r.Header.Get("Idempotency-Key"))
	if err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}
	httpx.WriteData(w, http.StatusOK, o, requestID)
}

func (h *Handler) Cancel(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFrom(r.Context())
	requestID := httpx.RequestIDFrom(r.Context())

	var in CancelInput
	if r.ContentLength != 0 {
		if err := httpx.DecodeJSON(w, r, &in); err != nil {
			httpx.WriteError(w, err, requestID)
			return
		}
	}

	o, err := h.svc.Cancel(r.Context(), actor, r.PathValue("id"), in)
	if err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}
	httpx.WriteData(w, http.StatusOK, o, requestID)
}

func (h *Handler) VoidItem(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFrom(r.Context())
	requestID := httpx.RequestIDFrom(r.Context())

	var in VoidInput
	if r.ContentLength != 0 {
		if err := httpx.DecodeJSON(w, r, &in); err != nil {
			httpx.WriteError(w, err, requestID)
			return
		}
	}

	o, err := h.svc.VoidItem(r.Context(), actor, r.PathValue("id"), r.PathValue("itemID"), in)
	if err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}
	httpx.WriteData(w, http.StatusOK, o, requestID)
}

func (h *Handler) MoveTable(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFrom(r.Context())
	requestID := httpx.RequestIDFrom(r.Context())

	var in MoveTableInput
	if err := httpx.DecodeJSON(w, r, &in); err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}

	o, err := h.svc.MoveTable(r.Context(), actor, r.PathValue("id"), in)
	if err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}
	httpx.WriteData(w, http.StatusOK, o, requestID)
}

func (h *Handler) Receipt(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFrom(r.Context())
	requestID := httpx.RequestIDFrom(r.Context())

	receipt, err := h.svc.Receipt(r.Context(), actor, r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}
	httpx.WriteData(w, http.StatusOK, receipt, requestID)
}

func (h *Handler) ReprintReceipt(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFrom(r.Context())
	requestID := httpx.RequestIDFrom(r.Context())

	receipt, err := h.svc.ReprintReceipt(r.Context(), actor, r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}
	httpx.WriteData(w, http.StatusOK, receipt, requestID)
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

func totalPages(total, perPage int) int {
	if perPage == 0 {
		return 0
	}
	return (total + perPage - 1) / perPage
}
