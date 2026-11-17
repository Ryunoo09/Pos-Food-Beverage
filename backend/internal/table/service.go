package table

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/ryunoo09/pos-food-beverage/backend/internal/auth"
	"github.com/ryunoo09/pos-food-beverage/backend/internal/httpx"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) List(ctx context.Context, actor auth.Actor, f Filter) ([]Table, error) {
	if actor.OutletID == "" {
		return nil, outletMissing()
	}
	f.Area = strings.TrimSpace(f.Area)
	return s.repo.List(ctx, actor.OutletID, f)
}

func (s *Service) Table(ctx context.Context, actor auth.Actor, id string) (*Table, error) {
	if actor.OutletID == "" {
		return nil, outletMissing()
	}
	t, err := s.repo.Table(ctx, actor.OutletID, id)
	if errors.Is(err, ErrNotFound) {
		return nil, notFound()
	}
	return t, err
}

func (s *Service) Create(ctx context.Context, actor auth.Actor, in Input) (*Table, error) {
	if actor.OutletID == "" {
		return nil, outletMissing()
	}
	in, err := validate(in)
	if err != nil {
		return nil, err
	}
	t, err := s.repo.Create(ctx, actor.OutletID, in)
	if err != nil {
		return nil, mapError(err)
	}
	return t, nil
}

func (s *Service) Update(ctx context.Context, actor auth.Actor, id string, in Input) (*Table, error) {
	if actor.OutletID == "" {
		return nil, outletMissing()
	}
	in, err := validate(in)
	if err != nil {
		return nil, err
	}
	t, err := s.repo.Update(ctx, actor.OutletID, id, in)
	if errors.Is(err, ErrNotFound) {
		return nil, notFound()
	}
	if err != nil {
		return nil, mapError(err)
	}
	return t, nil
}

func (s *Service) SetStatus(ctx context.Context, actor auth.Actor, id string, in StatusInput) (string, error) {
	if actor.OutletID == "" {
		return "", outletMissing()
	}
	status := strings.ToUpper(strings.TrimSpace(in.Status))
	if !validStatus(status) {
		return "", httpx.NewError(http.StatusBadRequest, "VALIDATION_ERROR",
			"status harus AVAILABLE, OCCUPIED, RESERVED, atau CLEANING.")
	}
	old, err := s.repo.SetStatus(ctx, actor.OutletID, id, status)
	if errors.Is(err, ErrNotFound) {
		return "", notFound()
	}
	return old, err
}

func (s *Service) Delete(ctx context.Context, actor auth.Actor, id string) error {
	if actor.OutletID == "" {
		return outletMissing()
	}
	busy, err := s.repo.HasActiveOrder(ctx, actor.OutletID, id)
	if err != nil {
		return err
	}
	if busy {
		return httpx.NewError(http.StatusUnprocessableEntity, "TABLE_IN_USE",
			"Meja masih dipakai pesanan aktif.")
	}
	err = s.repo.Deactivate(ctx, actor.OutletID, id)
	if errors.Is(err, ErrNotFound) {
		return notFound()
	}
	return err
}

func validate(in Input) (Input, error) {
	in.Number = strings.TrimSpace(in.Number)
	if in.Number == "" {
		return in, httpx.NewError(http.StatusBadRequest, "VALIDATION_ERROR", "number wajib diisi.")
	}
	if in.Area != nil {
		area := strings.TrimSpace(*in.Area)
		if area == "" {
			in.Area = nil
		} else {
			in.Area = &area
		}
	}
	if in.Capacity != nil && *in.Capacity <= 0 {
		return in, httpx.NewError(http.StatusBadRequest, "VALIDATION_ERROR", "capacity harus lebih dari nol.")
	}
	return in, nil
}

func validStatus(v string) bool {
	switch v {
	case "AVAILABLE", "OCCUPIED", "RESERVED", "CLEANING":
		return true
	}
	return false
}

func mapError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return httpx.NewError(http.StatusConflict, "TABLE_NUMBER_EXISTS",
			"Nomor meja sudah dipakai pada outlet ini.")
	}
	return err
}

func notFound() error {
	return httpx.NewError(http.StatusNotFound, "TABLE_NOT_FOUND", "Meja tidak ditemukan.")
}

func outletMissing() error {
	return httpx.NewError(http.StatusUnprocessableEntity, "OUTLET_NOT_CONFIGURED",
		"Outlet belum dikonfigurasi.")
}
