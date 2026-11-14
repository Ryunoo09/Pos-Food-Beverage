// Package audit menulis dan membaca audit log append-only (F20).
// Audit ditulis pada transaksi bisnis yang sama agar tidak ada aksi tanpa jejak.
package audit

import (
	"encoding/json"
	"time"
)

// Actor adalah identitas pelaku aksi; diisi dari auth.Actor.
type Actor struct {
	BusinessID string
	OutletID   string
	UserID     string
	Roles      []string
	IPAddress  string
	UserAgent  string
	RequestID  string
}

type Entry struct {
	Actor        Actor
	Action       string
	EntityType   string
	EntityID     string
	Before       any
	After        any
	Reason       string
	SelfApproved bool
}

type Log struct {
	ID         int64           `json:"id"`
	OccurredAt time.Time       `json:"occurred_at"`
	UserID     *string         `json:"user_id,omitempty"`
	Roles      []string        `json:"user_roles,omitempty"`
	Action     string          `json:"action"`
	EntityType *string         `json:"entity_type,omitempty"`
	EntityID   *string         `json:"entity_id,omitempty"`
	OutletID   *string         `json:"outlet_id,omitempty"`
	Before     json.RawMessage `json:"before_data,omitempty"`
	After      json.RawMessage `json:"after_data,omitempty"`
	Reason     *string         `json:"reason,omitempty"`
	IPAddress  *string         `json:"ip_address,omitempty"`
	UserAgent  *string         `json:"user_agent,omitempty"`
	RequestID  *string         `json:"request_id,omitempty"`
}

type Filter struct {
	Action     string
	EntityType string
	EntityID   string
	UserID     string
	From       string
	To         string
	Page       int
	PerPage    int
}
