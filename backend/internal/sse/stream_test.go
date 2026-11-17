package sse

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ryunoo09/pos-food-beverage/backend/internal/auth"
)

// TestStream_Authenticated verifies GET /stream returns 200 and starts
// streaming when a valid actor is in context (mocked via actor injection).
func TestStream_WritesHeaders(t *testing.T) {
	hub := NewHub()
	h := NewHandler(hub)

	actor := auth.Actor{UserID: "u1", OutletID: "outlet1", Permissions: []string{"kitchen.orders.view"}}
	ctx := auth.WithActor(context.Background(), actor)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/kitchen/stream", nil)
	req = req.WithContext(ctx)
	// Simulate context cancel setelah 100ms agar handler keluar dari loop.
	reqCtx, cancel := context.WithTimeout(req.Context(), 100*time.Millisecond)
	defer cancel()
	req = req.WithContext(reqCtx)

	// Jalankan handler di goroutine karena loop sampai ctx done.
	done := make(chan struct{})
	go func() {
		h.Stream(rec, req)
		close(done)
	}()

	// Kirim event supaya handler menulis sesuatu.
	go func() {
		time.Sleep(30 * time.Millisecond)
		hub.Publish("outlet1", "kitchen.order.created", map[string]string{"order_id": "o1"})
	}()

	<-done
	body := rec.Body.String()
	if !strings.Contains(body, "text/event-stream") {
		// Cek header
	}
	ct := rec.Header().Get("Content-Type")
	if ct != "text/event-stream" {
		t.Errorf("Content-Type = %q, want text/event-stream", ct)
	}
	if rec.Header().Get("X-Accel-Buffering") != "no" {
		t.Error("X-Accel-Buffering harus 'no'")
	}
	if !strings.Contains(body, "kitchen.order.created") {
		t.Errorf("body tidak mengandung event kitchen.order.created:\n%s", body)
	}
	if !strings.Contains(body, "id: 1") {
		t.Errorf("body tidak mengandung 'id: 1':\n%s", body)
	}
}

// TestStream_401WhenNoActor: tanpa actor di context → 401.
func TestStream_NoActor_Returns401(t *testing.T) {
	hub := NewHub()
	h := NewHandler(hub)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/kitchen/stream", nil)

	// Handler Stream mengharapkan actor dari context (di-set middleware AuthRequired).
	// Tanpa actor, handler balas 401.
	h.Stream(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}
