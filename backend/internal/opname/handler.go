package opname

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

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFrom(r.Context())
	requestID := httpx.RequestIDFrom(r.Context())

	var in CreateInput
	if err := httpx.DecodeJSON(w, r, &in); err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}

	o, err := h.svc.Create(r.Context(), actor, in)
	if err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}
	httpx.WriteData(w, http.StatusCreated, o, requestID)
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

func (h *Handler) Start(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFrom(r.Context())
	requestID := httpx.RequestIDFrom(r.Context())

	o, err := h.svc.Start(r.Context(), actor, r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}
	httpx.WriteData(w, http.StatusOK, o, requestID)
}

func (h *Handler) Count(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFrom(r.Context())
	requestID := httpx.RequestIDFrom(r.Context())

	var in CountInput
	if err := httpx.DecodeJSON(w, r, &in); err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}

	o, err := h.svc.Count(r.Context(), actor, r.PathValue("id"), in)
	if err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}
	httpx.WriteData(w, http.StatusOK, o, requestID)
}

func (h *Handler) Submit(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFrom(r.Context())
	requestID := httpx.RequestIDFrom(r.Context())

	o, err := h.svc.Submit(r.Context(), actor, r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}
	httpx.WriteData(w, http.StatusOK, o, requestID)
}

func (h *Handler) Approve(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFrom(r.Context())
	requestID := httpx.RequestIDFrom(r.Context())

	o, err := h.svc.Approve(r.Context(), actor, r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}
	httpx.WriteData(w, http.StatusOK, o, requestID)
}

func (h *Handler) Reject(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFrom(r.Context())
	requestID := httpx.RequestIDFrom(r.Context())

	var in RejectInput
	if err := httpx.DecodeJSON(w, r, &in); err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}

	o, err := h.svc.Reject(r.Context(), actor, r.PathValue("id"), in)
	if err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}
	httpx.WriteData(w, http.StatusOK, o, requestID)
}

func (h *Handler) Cancel(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFrom(r.Context())
	requestID := httpx.RequestIDFrom(r.Context())

	o, err := h.svc.Cancel(r.Context(), actor, r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}
	httpx.WriteData(w, http.StatusOK, o, requestID)
}

// Report memakai detail sesi yang sama; ringkasan selisih sudah ikut di item.
func (h *Handler) Report(w http.ResponseWriter, r *http.Request) {
	h.Get(w, r)
}
