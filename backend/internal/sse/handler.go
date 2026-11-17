// Package sse menyediakan endpoint stream SSE dan wiring publish event
// dari modul order, kitchen, table, dan payment.
package sse

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/ryunoo09/pos-food-beverage/backend/internal/auth"
	"github.com/ryunoo09/pos-food-beverage/backend/internal/httpx"
)

// Handler menangani GET /api/v1/kitchen/stream.
// Auth: Bearer token di header Authorization (EventSource tidak bisa kirim
// header, jadi klien juga boleh lewat ?token=... query param).
type Handler struct {
	hub *Hub
}

// NewHandler membuat handler SSE.
func NewHandler(hub *Hub) *Handler {
	return &Handler{hub: hub}
}

// Stream melayani koneksi SSE: subscribe, replay, lalu loop tulis.
func (h *Handler) Stream(w http.ResponseWriter, r *http.Request) {
	actor, ok := auth.ActorFrom(r.Context())
	if !ok {
		httpx.WriteError(w, httpx.NewError(http.StatusUnauthorized, "AUTH_TOKEN_MISSING",
			"Token akses tidak ditemukan."), httpx.RequestIDFrom(r.Context()))
		return
	}
	if actor.OutletID == "" {
		httpx.WriteError(w, httpx.NewError(http.StatusUnprocessableEntity, "OUTLET_NOT_CONFIGURED",
			"Outlet belum dikonfigurasi."), httpx.RequestIDFrom(r.Context()))
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		httpx.WriteError(w, httpx.NewError(http.StatusInternalServerError, "SSE_FLUSH_UNSUPPORTED",
			"Server tidak mendukung SSE."), httpx.RequestIDFrom(r.Context()))
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	// Last-Event-ID: dari header, atau query param last_event_id (klien useSSE
	// mengirim ?last_event_id=... karena EventSource tidak set header custom).
	lastID := int64(0)
	if raw := r.URL.Query().Get("last_event_id"); raw != "" {
		lastID, _ = strconv.ParseInt(raw, 10, 64)
	}
	if lastID == 0 {
		if raw := r.Header.Get("Last-Event-ID"); raw != "" {
			lastID, _ = strconv.ParseInt(raw, 10, 64)
		}
	}

	ch, unsub := h.hub.Subscribe(actor.OutletID, lastID, actor.Permissions...)
	defer unsub()

	// Sapa awal: pastikan koneksi flush pertama.
	fmt.Fprintf(w, ": connected\n\n")
	flusher.Flush()

	for {
		select {
		case event := <-ch:
			if err := writeEvent(w, event); err != nil {
				return
			}
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

// writeEvent menulis satu frame SSE: "id: N\nevent: type\ndata: payload\n\n".
// Event full_reload (ID=0, Type=full_reload, Payload kosong) hanya mengirim
// event type tanpa data.
func writeEvent(w http.ResponseWriter, e Event) error {
	var err error
	if e.ID > 0 {
		_, err = fmt.Fprintf(w, "id: %d\n", e.ID)
	}
	if e.Type != "" {
		_, err = fmt.Fprintf(w, "event: %s\n", e.Type)
	}
	if len(e.Payload) > 0 {
		_, err = fmt.Fprintf(w, "data: %s\n", e.Payload)
	}
	if err != nil {
		return err
	}
	_, err = fmt.Fprint(w, "\n")
	return err
}
