// Package sse mengimplementasikan Server-Sent Events hub per outlet untuk
// KDS, denah meja, dan notifikasi kasir. Event disimpan di ring buffer
// in-memory (100 event terakhir, TTL 5 menit) untuk replay saat reconnect
// dengan Last-Event-ID.
package sse

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"
)

// Event adalah satu payload SSE yang dikirim ke klien.
type Event struct {
	ID      int64
	Type    string
	Payload json.RawMessage
}

// Client adalah satu koneksi SSE terikat ke outlet.
type Client struct {
	outletID string
	ch       chan Event
}

// Hub mengelola langganan SSE per outlet dengan ring buffer replay.
// Sequence ID satu global (sederhana, cukup untuk satu monolith).
type Hub struct {
	mu      sync.RWMutex
	clients map[string][]*Client

	nextID int64 // di bawah h.mu

	bufferMu  sync.RWMutex
	buffers   map[string][]bufferedEvent
	bufferIdx map[string]map[int64]int // outlet → event id → index di buffer
	bufferLimit int
	bufferTTL   time.Duration
}

type bufferedEvent struct {
	event Event
	age   time.Time
}

// NewHub membuat hub: buffer 100 event, TTL 5 menit.
func NewHub() *Hub {
	return &Hub{
		clients:     map[string][]*Client{},
		buffers:     map[string][]bufferedEvent{},
		bufferIdx:   map[string]map[int64]int{},
		bufferLimit: 100,
		bufferTTL:   5 * time.Minute,
	}
}

// Subscribe mendaftarkan klien baru untuk outlet dan mengembalikan channel
// event serta fungsi unsubscribe. Event terlewat (lastID < terbaru) di-replay
// dari buffer; bila lastID tidak ditemukan di buffer, kirim full_reload.
func (h *Hub) Subscribe(outletID string, lastID int64) (<-chan Event, func()) {
	ch := make(chan Event, 16)

	h.mu.Lock()
	h.clients[outletID] = append(h.clients[outletID], &Client{outletID: outletID, ch: ch})
	h.mu.Unlock()

	if lastID > 0 {
		h.replay(outletID, lastID, ch)
	}

	unsub := func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		clients := h.clients[outletID]
		for i, c := range clients {
			if c.ch == ch {
				h.clients[outletID] = append(clients[:i], clients[i+1:]...)
				break
			}
		}
	}
	return ch, unsub
}

// Publish mengirim event ke semua klien outlet dan menyimpannya di ring
// buffer. Dipanggil SETELAH commit DB. ID event monoton global.
func (h *Hub) Publish(outletID, eventType string, payload any) int64 {
	var id int64
	var raw []byte

	h.mu.Lock()
	h.nextID++
	id = h.nextID
	h.mu.Unlock()

	if payload != nil {
		var err error
		raw, err = json.Marshal(payload)
		if err != nil {
			slog.Warn("sse: marshal payload gagal", "event_type", eventType, "error", err)
			return id
		}
	}

	event := Event{ID: id, Type: eventType, Payload: raw}
	h.pushBuffer(outletID, event)

	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, c := range h.clients[outletID] {
		select {
		case c.ch <- event:
		default: // klien lambat; biarkan, tidak block publisher
		}
	}
	return id
}

// Ping mengirim event ping per outlet tiap 30 detik (goroutine terpisah,
// dipanggil StartPingLoop).
func (h *Hub) Ping(outletID string) {
	h.Publish(outletID, "ping", map[string]any{"timestamp": time.Now().Unix()})
}

// StartPingLoop menjalankan loop ping sampai ctx dibatalkan.
func (h *Hub) StartPingLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			h.mu.RLock()
			outlets := make([]string, 0, len(h.clients))
			for o := range h.clients {
				outlets = append(outlets, o)
			}
			h.mu.RUnlock()
			for _, o := range outlets {
				h.Ping(o)
			}
		case <-ctx.Done():
			return
		}
	}
}

// pushBuffer menyimpan event di ring buffer outlet (limit + TTL).
func (h *Hub) pushBuffer(outletID string, event Event) {
	now := time.Now()
	h.bufferMu.Lock()
	defer h.bufferMu.Unlock()

	buf := h.buffers[outletID]
	buf = append(buf, bufferedEvent{event: event, age: now})
	if len(buf) > h.bufferLimit {
		buf = buf[len(buf)-h.bufferLimit:]
	}
	// Bersihkan lewat TTL dan bangun index baru.
	idx := map[int64]int{}
	kept := make([]bufferedEvent, 0, len(buf))
	for i, b := range buf {
		if now.Sub(b.age) <= h.bufferTTL {
			idx[b.event.ID] = i
			kept = append(kept, b)
		}
	}
	h.buffers[outletID] = kept
	h.bufferIdx[outletID] = idx
}

// replay mengirim event setelah lastID dari buffer; bila lastID tidak ada di
// buffer (gap), kirim full_reload supaya klien load ulang lewat REST.
func (h *Hub) replay(outletID string, lastID int64, ch chan Event) {
	h.bufferMu.RLock()
	idx := h.bufferIdx[outletID]
	buf := h.buffers[outletID]
	h.bufferMu.RUnlock()

	// Cari posisi pertama event dengan ID > lastID.
	pos, found := idx[lastID]
	if found {
		pos++
	}
	if !found {
		// lastID tidak ada di buffer: cek apakah seluruh buffer sudah > lastID.
		if len(buf) == 0 || buf[0].event.ID > lastID {
			pos = 0
		} else {
			ch <- Event{Type: "full_reload"} // ID 0: sinyal khusus, bukan dari hub
			return
		}
	}
	for _, b := range buf[pos:] {
		if b.event.ID > lastID {
			ch <- b.event
		}
	}
}
