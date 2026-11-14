package discount

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/ryunoo09/pos-food-beverage/backend/internal/auth"
	"github.com/ryunoo09/pos-food-beverage/backend/internal/httpx"
	"github.com/ryunoo09/pos-food-beverage/backend/internal/pkg/money"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) List(ctx context.Context, actor auth.Actor, active *bool) ([]Discount, error) {
	if actor.OutletID == "" {
		return nil, outletMissing()
	}
	return s.repo.List(ctx, actor.OutletID, active)
}

func (s *Service) Discount(ctx context.Context, actor auth.Actor, id string) (*Discount, error) {
	if actor.OutletID == "" {
		return nil, outletMissing()
	}
	item, err := s.repo.Discount(ctx, actor.OutletID, id)
	if errors.Is(err, ErrNotFound) {
		return nil, notFound()
	}
	return item, err
}

func (s *Service) Create(ctx context.Context, actor auth.Actor, in Input) (*Discount, error) {
	if actor.OutletID == "" {
		return nil, outletMissing()
	}
	in, err := validate(in)
	if err != nil {
		return nil, err
	}

	item, err := s.repo.Create(ctx, actor.OutletID, actor.UserID, in)
	if err != nil {
		return nil, mapError(err)
	}
	return item, nil
}

func (s *Service) Update(ctx context.Context, actor auth.Actor, id string, in Input) (*Discount, error) {
	if actor.OutletID == "" {
		return nil, outletMissing()
	}
	in, err := validate(in)
	if err != nil {
		return nil, err
	}

	item, err := s.repo.Update(ctx, actor.OutletID, id, in)
	if errors.Is(err, ErrNotFound) {
		return nil, notFound()
	}
	if err != nil {
		return nil, mapError(err)
	}
	return item, nil
}

func (s *Service) SetActive(ctx context.Context, actor auth.Actor, id string, active bool) error {
	if actor.OutletID == "" {
		return outletMissing()
	}
	err := s.repo.SetActive(ctx, actor.OutletID, id, active)
	if errors.Is(err, ErrNotFound) {
		return notFound()
	}
	return err
}

func validate(in Input) (Input, error) {
	in.Code = strings.TrimSpace(in.Code)
	in.Name = strings.TrimSpace(in.Name)
	if in.Code == "" || in.Name == "" {
		return in, httpx.NewError(http.StatusBadRequest, "VALIDATION_ERROR",
			"Kode dan nama diskon wajib diisi.")
	}
	if in.DiscountType != "PERCENT" && in.DiscountType != "AMOUNT" {
		return in, httpx.NewError(http.StatusBadRequest, "VALIDATION_ERROR",
			"discount_type harus PERCENT atau AMOUNT.")
	}
	if in.Scope != "ITEM" && in.Scope != "ORDER" {
		return in, httpx.NewError(http.StatusBadRequest, "VALIDATION_ERROR",
			"scope harus ITEM atau ORDER.")
	}

	value, err := money.Parse(in.Value)
	if err != nil || value.IsNegative() {
		return in, httpx.NewError(http.StatusBadRequest, "VALIDATION_ERROR", "value tidak valid.")
	}
	if in.DiscountType == "PERCENT" && value.GreaterThan(hundred()) {
		return in, httpx.NewError(http.StatusBadRequest, "VALIDATION_ERROR", "Diskon persen maksimal 100.")
	}
	in.Value = value.StringFixed(4)

	minSpend, err := money.Parse(in.MinSpend)
	if in.MinSpend == "" {
		minSpend, err = money.Parse("0")
	}
	if err != nil || minSpend.IsNegative() {
		return in, httpx.NewError(http.StatusBadRequest, "VALIDATION_ERROR", "min_spend tidak valid.")
	}
	in.MinSpend = money.Format(minSpend)

	if in.MaxDiscount != nil {
		max, err := money.Parse(*in.MaxDiscount)
		if err != nil || max.IsNegative() {
			return in, httpx.NewError(http.StatusBadRequest, "VALIDATION_ERROR", "max_discount tidak valid.")
		}
		formatted := money.Format(max)
		in.MaxDiscount = &formatted
	}

	for _, day := range in.DaysOfWeek {
		if day < 0 || day > 6 {
			return in, httpx.NewError(http.StatusBadRequest, "VALIDATION_ERROR",
				"days_of_week harus 0-6 (Minggu-Sabtu).")
		}
	}

	starts, err := parseTime(in.StartsAt)
	if err != nil {
		return in, err
	}
	ends, err := parseTime(in.EndsAt)
	if err != nil {
		return in, err
	}
	if starts != nil && ends != nil && ends.Before(*starts) {
		return in, httpx.NewError(http.StatusBadRequest, "VALIDATION_ERROR",
			"ends_at harus setelah starts_at.")
	}
	return in, nil
}

func parseTime(raw *string) (*time.Time, error) {
	if raw == nil || *raw == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, *raw)
	if err != nil {
		return nil, httpx.NewError(http.StatusBadRequest, "VALIDATION_ERROR",
			"Format waktu harus RFC3339, contoh 2026-11-14T00:00:00Z.")
	}
	return &parsed, nil
}

func mapError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return httpx.NewError(http.StatusConflict, "DISCOUNT_CODE_EXISTS", "Kode diskon sudah dipakai.")
	}
	return err
}

func hundred() money.Decimal {
	d, _ := money.Parse("100")
	return d
}

func notFound() error {
	return httpx.NewError(http.StatusNotFound, "DISCOUNT_NOT_FOUND", "Diskon tidak ditemukan.")
}

func outletMissing() error {
	return httpx.NewError(http.StatusUnprocessableEntity, "OUTLET_NOT_CONFIGURED",
		"Outlet belum dikonfigurasi.")
}
