// Package kitchen mengelola Kitchen Display System: tiket dapur, status item,
// dan prioritas. DTO tidak memuat field harga sehingga aman untuk staf dapur.
package kitchen

import (
	"context"
	"net/http"
	"strings"

	"github.com/ryunoo09/pos-food-beverage/backend/internal/auth"
	"github.com/ryunoo09/pos-food-beverage/backend/internal/httpx"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// List mengembalikan tiket aktif; default NEW/COOKING/READY.
func (s *Service) List(ctx context.Context, actor auth.Actor, f ListFilter) ([]Order, error) {
	if actor.OutletID == "" {
		return nil, outletMissing()
	}
	if len(f.Statuses) == 0 {
		f.Statuses = []string{"NEW", "COOKING", "READY"}
	}
	for _, st := range f.Statuses {
		if !validStatus(st) {
			return nil, httpx.NewError(http.StatusBadRequest, "VALIDATION_ERROR", "status tidak valid.")
		}
	}
	f.Station = strings.TrimSpace(f.Station)
	return s.repo.List(ctx, actor.OutletID, f)
}

func (s *Service) History(ctx context.Context, actor auth.Actor, f HistoryFilter) ([]Order, int, error) {
	if actor.OutletID == "" {
		return nil, 0, outletMissing()
	}
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PerPage < 1 || f.PerPage > 100 {
		f.PerPage = 20
	}
	return s.repo.History(ctx, actor.OutletID, f)
}

// UpdateItemStatus mengubah status satu item dengan optimistic lock pada version
// lalu menyimpulkan ulang status tiket. Mengembalikan tiket akhir dan status
// lama item (untuk payload SSE).
func (s *Service) UpdateItemStatus(ctx context.Context, actor auth.Actor, itemID string, in StatusInput) (*Order, string, error) {
	if actor.OutletID == "" {
		return nil, "", outletMissing()
	}
	status := strings.ToUpper(strings.TrimSpace(in.Status))
	if !validStatus(status) {
		return nil, "", httpx.NewError(http.StatusBadRequest, "VALIDATION_ERROR",
			"status harus NEW, COOKING, READY, atau SERVED.")
	}
	if in.Version < 1 {
		return nil, "", httpx.NewError(http.StatusBadRequest, "VALIDATION_ERROR", "version wajib diisi.")
	}

	tx, err := s.repo.Begin(ctx)
	if err != nil {
		return nil, "", err
	}
	defer tx.Rollback(ctx)

	item, ok, err := s.repo.ItemForUpdate(ctx, tx, itemID)
	if err != nil {
		return nil, "", err
	}
	if !ok || item.outletID != actor.OutletID {
		return nil, "", itemNotFound()
	}

	updated, err := s.repo.SetItemStatus(ctx, tx, itemID, status, in.Version)
	if err != nil {
		return nil, "", err
	}
	if !updated {
		return nil, "", httpx.NewError(http.StatusConflict, "VERSION_CONFLICT",
			"Item sudah diubah pengguna lain; muat ulang tiket.")
	}
	if err := s.repo.RecomputeOrderStatus(ctx, tx, item.kitchenOrderID); err != nil {
		return nil, "", err
	}
	oldStatus := item.Status
	if err := tx.Commit(ctx); err != nil {
		return nil, "", err
	}
	o, err := s.repo.Get(ctx, actor.OutletID, item.kitchenOrderID)
	if err != nil {
		return nil, "", err
	}
	return o, oldStatus, nil
}

// SetPriority menandai tiket RUSH/NORMAL untuk mempercepat antrean.
func (s *Service) SetPriority(ctx context.Context, actor auth.Actor, id string, in PriorityInput) (*Order, error) {
	if actor.OutletID == "" {
		return nil, outletMissing()
	}
	priority := strings.ToUpper(strings.TrimSpace(in.Priority))
	if priority != "NORMAL" && priority != "RUSH" {
		return nil, httpx.NewError(http.StatusBadRequest, "VALIDATION_ERROR",
			"priority harus NORMAL atau RUSH.")
	}
	ok, err := s.repo.SetPriority(ctx, actor.OutletID, id, priority)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ticketNotFound()
	}
	return s.repo.Get(ctx, actor.OutletID, id)
}

func validStatus(v string) bool {
	switch v {
	case "NEW", "COOKING", "READY", "SERVED":
		return true
	}
	return false
}

func itemNotFound() error {
	return httpx.NewError(http.StatusNotFound, "KITCHEN_ITEM_NOT_FOUND", "Item dapur tidak ditemukan.")
}

func ticketNotFound() error {
	return httpx.NewError(http.StatusNotFound, "KITCHEN_ORDER_NOT_FOUND", "Tiket dapur tidak ditemukan.")
}

func outletMissing() error {
	return httpx.NewError(http.StatusUnprocessableEntity, "OUTLET_NOT_CONFIGURED",
		"Outlet belum dikonfigurasi.")
}
