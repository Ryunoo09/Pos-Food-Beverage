package audit

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/ryunoo09/pos-food-beverage/backend/internal/auth"
	"github.com/ryunoo09/pos-food-beverage/backend/internal/httpx"
)

type Handler struct {
	writer *Writer
}

func NewHandler(writer *Writer) *Handler {
	return &Handler{writer: writer}
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFrom(r.Context())
	requestID := httpx.RequestIDFrom(r.Context())
	if actor.BusinessID == "" {
		httpx.WriteError(w, httpx.NewError(http.StatusForbidden, "BUSINESS_REQUIRED",
			"Pengguna tidak terhubung ke bisnis."), requestID)
		return
	}

	f := filterFrom(r)
	items, total, err := h.writer.List(r.Context(), actor.BusinessID, f)
	if err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}
	httpx.WritePage(w, items, httpx.PageMeta{
		Page: f.Page, PerPage: f.PerPage, Total: total,
		TotalPages: (total + f.PerPage - 1) / f.PerPage, RequestID: requestID,
	})
}

func (h *Handler) Export(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFrom(r.Context())
	requestID := httpx.RequestIDFrom(r.Context())
	if actor.BusinessID == "" {
		httpx.WriteError(w, httpx.NewError(http.StatusForbidden, "BUSINESS_REQUIRED",
			"Pengguna tidak terhubung ke bisnis."), requestID)
		return
	}

	f := filterFrom(r)
	items, err := h.writer.All(r.Context(), actor.BusinessID, f)
	if err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}

	// Ekspor itu sendiri diaudit (FR-F20-4). Gagal audit = gagal ekspor.
	if err := h.writer.Record(r.Context(), Entry{
		Actor: Actor{
			BusinessID: actor.BusinessID,
			OutletID:   actor.OutletID,
			UserID:     actor.UserID,
			Roles:      actor.Roles,
			IPAddress:  actor.IPAddress,
			UserAgent:  actor.UserAgent,
			RequestID:  actor.RequestID,
		},
		Action:     "audit.export",
		EntityType: "audit_log",
		Reason:     fmt.Sprintf("rows=%d", len(items)),
	}); err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="audit-logs.csv"`)
	cw := csv.NewWriter(w)
	_ = cw.Write([]string{"id", "occurred_at", "user_id", "user_roles", "action", "entity_type",
		"entity_id", "outlet_id", "before_data", "after_data", "reason", "ip_address",
		"user_agent", "request_id"})
	for _, l := range items {
		_ = cw.Write([]string{
			strconv.FormatInt(l.ID, 10),
			l.OccurredAt.Format("2006-01-02T15:04:05Z07:00"),
			text(l.UserID), strings.Join(l.Roles, "|"), l.Action,
			text(l.EntityType), text(l.EntityID), text(l.OutletID),
			raw(l.Before), raw(l.After), text(l.Reason),
			text(l.IPAddress), text(l.UserAgent), text(l.RequestID),
		})
	}
	cw.Flush()
}

func filterFrom(r *http.Request) Filter {
	q := r.URL.Query()
	f := Filter{
		Action:     q.Get("action"),
		EntityType: q.Get("entity_type"),
		EntityID:   q.Get("entity_id"),
		UserID:     q.Get("user_id"),
		From:       q.Get("from"),
		To:         q.Get("to"),
		Page:       atoiDefault(q.Get("page"), 1),
		PerPage:    atoiDefault(q.Get("per_page"), 20),
	}
	if f.PerPage > 100 {
		f.PerPage = 100
	}
	return f
}

func text(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func raw(b json.RawMessage) string {
	if len(b) == 0 {
		return ""
	}
	return string(b)
}

func atoiDefault(s string, def int) int {
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 {
		return def
	}
	return n
}
