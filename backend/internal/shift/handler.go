package shift

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

func (h *Handler) Open(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFrom(r.Context())
	requestID := httpx.RequestIDFrom(r.Context())

	var in OpenInput
	if err := httpx.DecodeJSON(w, r, &in); err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}
	shift, err := h.svc.Open(r.Context(), actor, in)
	if err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}
	httpx.WriteData(w, http.StatusCreated, shift, requestID)
}

func (h *Handler) Current(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFrom(r.Context())
	requestID := httpx.RequestIDFrom(r.Context())

	shift, err := h.svc.Current(r.Context(), actor)
	if err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}
	httpx.WriteData(w, http.StatusOK, shift, requestID)
}

func (h *Handler) Summary(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFrom(r.Context())
	requestID := httpx.RequestIDFrom(r.Context())

	summary, err := h.svc.Summary(r.Context(), actor, r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}
	httpx.WriteData(w, http.StatusOK, summary, requestID)
}

func (h *Handler) Close(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFrom(r.Context())
	requestID := httpx.RequestIDFrom(r.Context())

	var in CloseInput
	if err := httpx.DecodeJSON(w, r, &in); err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}
	summary, err := h.svc.Close(r.Context(), actor, r.PathValue("id"), r.Header.Get("Idempotency-Key"), in)
	if err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}
	httpx.WriteData(w, http.StatusOK, summary, requestID)
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFrom(r.Context())
	requestID := httpx.RequestIDFrom(r.Context())

	f := Filter{
		Status:  r.URL.Query().Get("status"),
		Page:    atoiDefault(r.URL.Query().Get("page"), 1),
		PerPage: atoiDefault(r.URL.Query().Get("per_page"), 20),
	}
	items, meta, err := h.svc.List(r.Context(), actor, f)
	if err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}
	meta.TotalPages = (meta.Total + meta.PerPage - 1) / meta.PerPage
	meta.RequestID = requestID
	httpx.WritePage(w, items, meta)
}

func atoiDefault(s string, def int) int {
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 {
		return def
	}
	return n
}
