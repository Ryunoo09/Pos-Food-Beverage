package waste

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

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFrom(r.Context())
	requestID := httpx.RequestIDFrom(r.Context())

	var in Input
	if err := httpx.DecodeJSON(w, r, &in); err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}
	rec, err := h.svc.Record(r.Context(), actor, in)
	if err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}
	httpx.WriteData(w, http.StatusCreated, rec, requestID)
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFrom(r.Context())
	requestID := httpx.RequestIDFrom(r.Context())

	q := r.URL.Query()
	page, perPage := pagination(r)
	f := Filter{
		Type:       q.Get("waste_type"),
		ReasonCode: q.Get("reason_code"),
		From:       q.Get("from"),
		To:         q.Get("to"),
		Page:       page,
		PerPage:    perPage,
	}
	items, total, err := h.svc.List(r.Context(), actor, f)
	if err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}
	httpx.WritePage(w, items, httpx.PageMeta{
		Page: page, PerPage: perPage, Total: total,
		TotalPages: (total + perPage - 1) / perPage, RequestID: requestID,
	})
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFrom(r.Context())
	requestID := httpx.RequestIDFrom(r.Context())

	rec, err := h.svc.Get(r.Context(), actor, r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}
	httpx.WriteData(w, http.StatusOK, rec, requestID)
}

func (h *Handler) Reverse(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFrom(r.Context())
	requestID := httpx.RequestIDFrom(r.Context())

	rec, err := h.svc.Reverse(r.Context(), actor, r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}
	httpx.WriteData(w, http.StatusOK, rec, requestID)
}

func pagination(r *http.Request) (page, perPage int) {
	page, _ = strconv.Atoi(r.URL.Query().Get("page"))
	perPage, _ = strconv.Atoi(r.URL.Query().Get("per_page"))
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}
	return page, perPage
}
