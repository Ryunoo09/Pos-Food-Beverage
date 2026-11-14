package payment

import (
	"context"
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

	var in CreateInput
	if err := httpx.DecodeJSON(w, r, &in); err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}
	result, err := h.svc.Create(r.Context(), actor, r.PathValue("id"), r.Header.Get("Idempotency-Key"), in)
	if err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}
	httpx.WriteData(w, http.StatusCreated, result, requestID)
}

func (h *Handler) Void(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFrom(r.Context())
	requestID := httpx.RequestIDFrom(r.Context())

	var in VoidInput
	if err := httpx.DecodeJSON(w, r, &in); err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}
	result, err := h.svc.Void(r.Context(), actor, r.PathValue("id"), r.Header.Get("Idempotency-Key"), in)
	if err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}
	httpx.WriteData(w, http.StatusCreated, result, requestID)
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFrom(r.Context())
	requestID := httpx.RequestIDFrom(r.Context())

	items, err := h.svc.List(r.Context(), actor, r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}
	httpx.WriteData(w, http.StatusOK, items, requestID)
}

func (h *Handler) CreateRefund(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFrom(r.Context())
	requestID := httpx.RequestIDFrom(r.Context())

	var in RefundInput
	if err := httpx.DecodeJSON(w, r, &in); err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}
	result, status, err := h.svc.CreateRefund(r.Context(), actor, r.PathValue("id"),
		r.Header.Get("Idempotency-Key"), in)
	if err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}
	httpx.WriteData(w, status, result, requestID)
}

func (h *Handler) ApproveRefund(w http.ResponseWriter, r *http.Request) {
	h.decideRefund(w, r, h.svc.ApproveRefund)
}

func (h *Handler) RejectRefund(w http.ResponseWriter, r *http.Request) {
	h.decideRefund(w, r, h.svc.RejectRefund)
}

func (h *Handler) decideRefund(w http.ResponseWriter, r *http.Request,
	action func(ctx context.Context, actor auth.Actor, refundID string, in RefundDecisionInput) (*RefundResult, error)) {
	actor, _ := auth.ActorFrom(r.Context())
	requestID := httpx.RequestIDFrom(r.Context())

	var in RefundDecisionInput
	if err := httpx.DecodeJSON(w, r, &in); err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}
	result, err := action(r.Context(), actor, r.PathValue("id"), in)
	if err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}
	httpx.WriteData(w, http.StatusOK, result, requestID)
}

func (h *Handler) ListRefunds(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFrom(r.Context())
	requestID := httpx.RequestIDFrom(r.Context())

	f := RefundFilter{
		OrderID: r.URL.Query().Get("order_id"),
		Status:  r.URL.Query().Get("status"),
		Page:    atoiDefault(r.URL.Query().Get("page"), 1),
		PerPage: atoiDefault(r.URL.Query().Get("per_page"), 20),
	}
	items, meta, err := h.svc.ListRefunds(r.Context(), actor, f)
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
