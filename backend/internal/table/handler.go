package table

import (
	"net/http"
	"time"

	"github.com/ryunoo09/pos-food-beverage/backend/internal/auth"
	"github.com/ryunoo09/pos-food-beverage/backend/internal/httpx"
	"github.com/ryunoo09/pos-food-beverage/backend/internal/sse"
)

type Handler struct {
	svc *Service
	hub *sse.Hub
}

func NewHandler(svc *Service, hub *sse.Hub) *Handler {
	return &Handler{svc: svc, hub: hub}
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFrom(r.Context())
	requestID := httpx.RequestIDFrom(r.Context())

	filter := Filter{
		Area:     r.URL.Query().Get("area"),
		IsActive: boolParam(r.URL.Query().Get("is_active")),
	}
	items, err := h.svc.List(r.Context(), actor, filter)
	if err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}
	httpx.WriteData(w, http.StatusOK, items, requestID)
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFrom(r.Context())
	requestID := httpx.RequestIDFrom(r.Context())

	t, err := h.svc.Table(r.Context(), actor, r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}
	httpx.WriteData(w, http.StatusOK, t, requestID)
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFrom(r.Context())
	requestID := httpx.RequestIDFrom(r.Context())

	var in Input
	if err := httpx.DecodeJSON(w, r, &in); err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}

	t, err := h.svc.Create(r.Context(), actor, in)
	if err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}
	httpx.WriteData(w, http.StatusCreated, t, requestID)
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFrom(r.Context())
	requestID := httpx.RequestIDFrom(r.Context())

	var in Input
	if err := httpx.DecodeJSON(w, r, &in); err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}

	t, err := h.svc.Update(r.Context(), actor, r.PathValue("id"), in)
	if err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}
	httpx.WriteData(w, http.StatusOK, t, requestID)
}

func (h *Handler) SetStatus(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFrom(r.Context())
	requestID := httpx.RequestIDFrom(r.Context())

	var in StatusInput
	if err := httpx.DecodeJSON(w, r, &in); err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}

	old, err := h.svc.SetStatus(r.Context(), actor, r.PathValue("id"), in)
	if err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}
	if h.hub != nil && actor.OutletID != "" {
		t, _ := h.svc.Table(r.Context(), actor, r.PathValue("id"))
		no := ""
		if t != nil {
			no = t.Number
		}
		h.hub.Publish(actor.OutletID, sse.EventTypeTableStatusChanged, sse.TableStatusChangedPayload{
			TableID: r.PathValue("id"), TableNo: no,
			OldStatus: old, NewStatus: in.Status,
			Timestamp: time.Now().UTC(),
		})
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFrom(r.Context())
	requestID := httpx.RequestIDFrom(r.Context())

	if err := h.svc.Delete(r.Context(), actor, r.PathValue("id")); err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}
	w.WriteHeader(http.StatusNoContent)
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
