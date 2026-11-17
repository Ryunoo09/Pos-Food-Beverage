package kitchen

import (
	"net/http"
	"strconv"
	"strings"
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

	filter := ListFilter{Station: r.URL.Query().Get("station")}
	if raw := strings.TrimSpace(r.URL.Query().Get("status")); raw != "" {
		for _, part := range strings.Split(raw, ",") {
			if st := strings.ToUpper(strings.TrimSpace(part)); st != "" {
				filter.Statuses = append(filter.Statuses, st)
			}
		}
	}

	items, err := h.svc.List(r.Context(), actor, filter)
	if err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}
	httpx.WriteData(w, http.StatusOK, items, requestID)
}

func (h *Handler) History(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFrom(r.Context())
	requestID := httpx.RequestIDFrom(r.Context())

	page, perPage := pagination(r)
	filter := HistoryFilter{From: timeParam(r.URL.Query().Get("from")), To: timeParam(r.URL.Query().Get("to")), Page: page, PerPage: perPage}

	items, total, err := h.svc.History(r.Context(), actor, filter)
	if err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}
	httpx.WritePage(w, items, httpx.PageMeta{
		Page:       page,
		PerPage:    perPage,
		Total:      total,
		TotalPages: totalPages(total, perPage),
		RequestID:  requestID,
	})
}

func (h *Handler) UpdateItemStatus(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFrom(r.Context())
	requestID := httpx.RequestIDFrom(r.Context())

	var in StatusInput
	if err := httpx.DecodeJSON(w, r, &in); err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}

	o, oldStatus, err := h.svc.UpdateItemStatus(r.Context(), actor, r.PathValue("id"), in)
	if err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}
	if o != nil && h.hub != nil && actor.OutletID != "" {
		h.hub.Publish(actor.OutletID, sse.EventTypeKitchenItemStatusChanged, sse.ItemStatusChangedPayload{
			OrderID:   o.OrderID,
			ItemID:    r.PathValue("id"),
			OldStatus: oldStatus,
			NewStatus: in.Status,
			UpdatedBy: actor.UserID,
			Timestamp: time.Now().UTC(),
		})
	}
	httpx.WriteData(w, http.StatusOK, o, requestID)
}

func (h *Handler) SetPriority(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFrom(r.Context())
	requestID := httpx.RequestIDFrom(r.Context())

	var in PriorityInput
	if err := httpx.DecodeJSON(w, r, &in); err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}

	o, err := h.svc.SetPriority(r.Context(), actor, r.PathValue("id"), in)
	if err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}
	httpx.WriteData(w, http.StatusOK, o, requestID)
}

func pagination(r *http.Request) (int, int) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}
	return page, perPage
}

func totalPages(total, perPage int) int {
	if perPage == 0 {
		return 0
	}
	return (total + perPage - 1) / perPage
}

// timeParam mem-parsing waktu RFC3339; kosong/tidak valid dianggap tidak ada.
func timeParam(raw string) *time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil
	}
	return &t
}
