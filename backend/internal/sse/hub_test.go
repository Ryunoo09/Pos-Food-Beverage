package sse

import (
	"testing"
	"time"
)

// TestPublish_SequentialIDs: setiap publish mengembalikan ID monoton naik.
func TestPublish_SequentialIDs(t *testing.T) {
	h := NewHub()
	outlet := "o1"

	a := h.Publish(outlet, "e1", map[string]int{"n": 1})
	b := h.Publish(outlet, "e2", map[string]int{"n": 2})
	if b <= a {
		t.Fatalf("ID tidak monoton: %d, %d", a, b)
	}
}

// TestSubscribe_Replay: klien yang connect dengan lastID menerima event
// setelah lastID dari buffer.
func TestSubscribe_Replay(t *testing.T) {
	h := NewHub()
	outlet := "o1"

	// 3 event sebelum subscribe.
	id1 := h.Publish(outlet, "e1", nil)
	id2 := h.Publish(outlet, "e2", nil)
	_ = id2

	// Subscribe dengan lastID = id1 → harus dapat event id2+.
	ch, unsub := h.Subscribe(outlet, id1)
	defer unsub()

	select {
	case ev := <-ch:
		if ev.ID <= id1 {
			t.Fatalf("replay seharusnya mulai setelah lastID=%d, dapat %d", id1, ev.ID)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("tidak ada event replay dalam 500ms")
	}
}

// TestSubscribe_FullReload: lastID yang tidak ada di buffer (sudah lewat TTL
// atau server restart) memicu full_reload.
func TestSubscribe_FullReload(t *testing.T) {
	h := NewHub()
	// Set bufferTTL pendek untuk uji
	h.bufferTTL = time.Millisecond
	outlet := "o1"

	_ = h.Publish(outlet, "e1", nil)
	time.Sleep(5 * time.Millisecond) // lewat TTL

	ch, unsub := h.Subscribe(outlet, 999) // ID tidak ada di buffer
	defer unsub()

	select {
	case ev := <-ch:
		if ev.Type != "full_reload" {
			t.Fatalf("harus full_reload, dapat %s", ev.Type)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("tidak ada full_reload dalam 500ms")
	}
}

// TestSubscribe_NewClient: klien baru (lastID=0) menerima event live.
func TestSubscribe_NewClient(t *testing.T) {
	h := NewHub()
	outlet := "o1"

	ch, unsub := h.Subscribe(outlet, 0)
	defer unsub()

	h.Publish(outlet, "kitchen.order.created", map[string]string{"order_id": "x"})

	select {
	case ev := <-ch:
		if ev.Type != "kitchen.order.created" {
			t.Fatalf("harus kitchen.order.created, dapat %s", ev.Type)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("event live tidak sampai")
	}
}

// TestUnsubscribe: setelah unsub, klien tidak menerima event lagi.
func TestUnsubscribe(t *testing.T) {
	h := NewHub()
	outlet := "o1"

	ch, unsub := h.Subscribe(outlet, 0)
	unsub()

	h.Publish(outlet, "e", nil)

	select {
	case ev := <-ch:
		t.Fatalf("client yang sudah unsub masih menerima: %v", ev)
	case <-time.After(200 * time.Millisecond):
		// diharapkan: tidak ada
	}
}

// TestMultiClient: dua klien outlet sama menerima event yang sama.
func TestMultiClient(t *testing.T) {
	h := NewHub()
	outlet := "o1"

	ch1, un1 := h.Subscribe(outlet, 0)
	defer un1()
	ch2, un2 := h.Subscribe(outlet, 0)
	defer un2()

	h.Publish(outlet, "table.status_changed", map[string]string{"table_id": "t1"})

	select {
	case ev := <-ch1:
		if ev.Type != "table.status_changed" {
			t.Fatalf("client1 dapat %s", ev.Type)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("client1 tidak menerima")
	}
	select {
	case ev := <-ch2:
		if ev.Type != "table.status_changed" {
			t.Fatalf("client2 dapat %s", ev.Type)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("client2 tidak menerima")
	}
}
