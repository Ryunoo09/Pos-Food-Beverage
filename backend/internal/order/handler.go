package order

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
	// hub SSE untuk publish event realtime; nil = publish dimatikan.
	hub *sse.Hub
}

func NewHandler(svc *Service, hub *sse.Hub) *Handler {
	return &Handler{svc: svc, hub: hub}
}

// publishConfirm mengirim event SSE setelah commit: tiket dapur dibuat +
// status meja menjadi OCCUPIED (bila dine-in).
func (h *Handler) publishConfirm(o *Order, outletID string) {
	items := make([]sse.KitchenItem, 0, len(o.Items))
	for _, it := range o.Items {
		if it.Status != "ACTIVE" {
			continue
		}
		items = append(items, sse.KitchenItem{
			Name:     it.ProductName,
			Variant:  it.VariantName,
			Note:     strVal(it.Note),
			Quantity: it.Quantity,
		})
	}
	var createdAt *time.Time = nil
	if o.ConfirmedAt != nil {
		createdAt = o.ConfirmedAt
	}
	h.hub.Publish(outletID, sse.EventTypeKitchenOrderCreated, sse.KitchenOrderCreatedPayload{
		OrderID:     o.ID,
		OrderNo:     o.OrderNo,
		QueueNo:     o.QueueNo,
		ServiceType: o.ServiceType,
		TableName:   o.TableName,
		RoundNo:     1,
		Priority:    o.Priority,
		Items:       items,
		CreatedAt:   timePtr(createdAt),
	})
	h.hub.Publish(outletID, sse.EventTypeOrderStatusChanged, sse.OrderStatusChangedPayload{
		OrderID:   o.ID,
		OldStatus: "DRAFT",
		NewStatus: o.Status,
		Timestamp: timePtr(createdAt),
	})
	if o.TableID != nil {
		no := strVal(o.TableName)
		h.hub.Publish(outletID, sse.EventTypeTableStatusChanged, sse.TableStatusChangedPayload{
			TableID: *o.TableID, TableNo: no,
			OldStatus: "AVAILABLE", NewStatus: "OCCUPIED",
			Timestamp: timePtr(createdAt),
		})
	}
}

func timePtr(p *time.Time) time.Time {
	if p != nil {
		return *p
	}
	return time.Time{}
}

func strVal(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFrom(r.Context())
	requestID := httpx.RequestIDFrom(r.Context())

	page, perPage := pagination(r)
	filter := Filter{Query: strings.TrimSpace(r.URL.Query().Get("q")), Page: page, PerPage: perPage}
	if raw := r.URL.Query().Get("status"); raw != "" {
		filter.Status = &raw
	}

	items, total, err := h.svc.List(r.Context(), actor, filter)
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

func (h *Handler) CreateDraft(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFrom(r.Context())
	requestID := httpx.RequestIDFrom(r.Context())

	var in DraftInput
	if err := httpx.DecodeJSON(w, r, &in); err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}

	o, err := h.svc.CreateDraft(r.Context(), actor, in)
	if err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}
	httpx.WriteData(w, http.StatusCreated, o, requestID)
}

func (h *Handler) AddItem(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFrom(r.Context())
	requestID := httpx.RequestIDFrom(r.Context())

	var in ItemInput
	if err := httpx.DecodeJSON(w, r, &in); err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}

	o, err := h.svc.AddItem(r.Context(), actor, r.PathValue("id"), in)
	if err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}
	httpx.WriteData(w, http.StatusCreated, o, requestID)
}

func (h *Handler) UpdateItem(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFrom(r.Context())
	requestID := httpx.RequestIDFrom(r.Context())

	var in UpdateItemInput
	if err := httpx.DecodeJSON(w, r, &in); err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}

	o, err := h.svc.UpdateItem(r.Context(), actor, r.PathValue("id"), r.PathValue("itemID"), in)
	if err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}
	httpx.WriteData(w, http.StatusOK, o, requestID)
}

func (h *Handler) DeleteItem(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFrom(r.Context())
	requestID := httpx.RequestIDFrom(r.Context())

	o, err := h.svc.DeleteItem(r.Context(), actor, r.PathValue("id"), r.PathValue("itemID"))
	if err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}
	httpx.WriteData(w, http.StatusOK, o, requestID)
}

func (h *Handler) Confirm(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFrom(r.Context())
	requestID := httpx.RequestIDFrom(r.Context())

	o, err := h.svc.Confirm(r.Context(), actor, r.PathValue("id"), r.Header.Get("Idempotency-Key"))
	if err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}
	// Emit SSE setelah commit: tiket dapur baru + status meja (bila dine-in).
	if h.hub != nil && actor.OutletID != "" {
		h.publishConfirm(o, actor.OutletID)
	}
	httpx.WriteData(w, http.StatusOK, o, requestID)
}

func (h *Handler) Cancel(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFrom(r.Context())
	requestID := httpx.RequestIDFrom(r.Context())

	var in CancelInput
	if r.ContentLength != 0 {
		if err := httpx.DecodeJSON(w, r, &in); err != nil {
			httpx.WriteError(w, err, requestID)
			return
		}
	}

	o, err := h.svc.Cancel(r.Context(), actor, r.PathValue("id"), in)
	if err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}
	if h.hub != nil && actor.OutletID != "" {
		// ponytail: old_status tidak dikembalikan service; konsumen memakai new_status.
		h.hub.Publish(actor.OutletID, sse.EventTypeOrderStatusChanged, sse.OrderStatusChangedPayload{
			OrderID:   o.ID,
			NewStatus: o.Status,
			Timestamp: time.Now().UTC(),
		})
		if o.TableID != nil {
			h.hub.Publish(actor.OutletID, sse.EventTypeTableStatusChanged, sse.TableStatusChangedPayload{
				TableID:   *o.TableID,
				TableNo:   strVal(o.TableName),
				OldStatus: "OCCUPIED",
				NewStatus: "AVAILABLE",
				Timestamp: time.Now().UTC(),
			})
		}
	}
	httpx.WriteData(w, http.StatusOK, o, requestID)
}

func (h *Handler) VoidItem(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFrom(r.Context())
	requestID := httpx.RequestIDFrom(r.Context())

	var in VoidInput
	if r.ContentLength != 0 {
		if err := httpx.DecodeJSON(w, r, &in); err != nil {
			httpx.WriteError(w, err, requestID)
			return
		}
	}

	o, err := h.svc.VoidItem(r.Context(), actor, r.PathValue("id"), r.PathValue("itemID"), in)
	if err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}
	if h.hub != nil && actor.OutletID != "" {
		reason := strings.TrimSpace(in.ReasonCode)
		if note := strings.TrimSpace(strVal(in.Note)); note != "" {
			reason = note
		}
		h.hub.Publish(actor.OutletID, sse.EventTypeKitchenItemVoided, sse.ItemVoidedPayload{
			OrderID:   o.ID,
			ItemID:    r.PathValue("itemID"),
			Reason:    reason,
			VoidedBy:  actor.UserID,
			Timestamp: time.Now().UTC(),
		})
	}
	httpx.WriteData(w, http.StatusOK, o, requestID)
}

func (h *Handler) MoveTable(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFrom(r.Context())
	requestID := httpx.RequestIDFrom(r.Context())

	var in MoveTableInput
	if err := httpx.DecodeJSON(w, r, &in); err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}

	o, err := h.svc.MoveTable(r.Context(), actor, r.PathValue("id"), in)
	if err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}
	httpx.WriteData(w, http.StatusOK, o, requestID)
}

func (h *Handler) Receipt(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFrom(r.Context())
	requestID := httpx.RequestIDFrom(r.Context())

	receipt, err := h.svc.Receipt(r.Context(), actor, r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}
	httpx.WriteData(w, http.StatusOK, receipt, requestID)
}

func (h *Handler) ReprintReceipt(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFrom(r.Context())
	requestID := httpx.RequestIDFrom(r.Context())

	receipt, err := h.svc.ReprintReceipt(r.Context(), actor, r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}
	httpx.WriteData(w, http.StatusOK, receipt, requestID)
}

func pagination(r *http.Request) (int, int) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
	if page < 1 {
		page = 1
	}
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
