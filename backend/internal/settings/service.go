package settings

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

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

func (s *Service) Settings(ctx context.Context, actor auth.Actor) (*OutletSettings, error) {
	if actor.OutletID == "" {
		return nil, outletMissing()
	}
	item, err := s.repo.Settings(ctx, actor.OutletID)
	if errors.Is(err, ErrNotFound) {
		return nil, outletMissing()
	}
	return item, err
}

func (s *Service) UpdateSettings(ctx context.Context, actor auth.Actor, in OutletSettingsInput) (*OutletSettings, error) {
	if actor.OutletID == "" {
		return nil, outletMissing()
	}

	if in.RoundingMode != nil {
		switch *in.RoundingMode {
		case "NONE", "UP_100", "NEAREST_100", "NEAREST_500":
		default:
			return nil, httpx.NewError(http.StatusBadRequest, "VALIDATION_ERROR", "rounding_mode tidak valid.")
		}
	}

	var err error
	if in.DiscountCashierMaxPercent, err = normalizePercent(in.DiscountCashierMaxPercent); err != nil {
		return nil, err
	}
	if in.DiscountApprovalThreshold, err = normalizeAmount(in.DiscountApprovalThreshold); err != nil {
		return nil, err
	}
	if in.RefundApprovalThreshold, err = normalizeAmount(in.RefundApprovalThreshold); err != nil {
		return nil, err
	}

	return s.repo.UpdateSettings(ctx, actor.OutletID, in)
}

func (s *Service) Taxes(ctx context.Context, actor auth.Actor) ([]TaxConfig, error) {
	if actor.OutletID == "" {
		return nil, outletMissing()
	}
	return s.repo.Taxes(ctx, actor.OutletID)
}

func (s *Service) CreateTax(ctx context.Context, actor auth.Actor, in TaxConfigInput) (*TaxConfig, error) {
	if actor.OutletID == "" {
		return nil, outletMissing()
	}

	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		return nil, httpx.NewError(http.StatusBadRequest, "VALIDATION_ERROR", "Nama pajak/layanan wajib diisi.")
	}
	if in.Type != "TAX" && in.Type != "SERVICE" {
		return nil, httpx.NewError(http.StatusBadRequest, "VALIDATION_ERROR", "type harus TAX atau SERVICE.")
	}

	rate, err := money.Parse(in.RatePercent)
	if err != nil || rate.IsNegative() {
		return nil, httpx.NewError(http.StatusBadRequest, "VALIDATION_ERROR", "rate_percent tidak valid.")
	}
	if rate.GreaterThan(hundred()) {
		return nil, httpx.NewError(http.StatusBadRequest, "VALIDATION_ERROR", "rate_percent maksimal 100.")
	}
	in.RatePercent = rate.StringFixed(4)

	for _, st := range in.AppliesTo {
		switch st {
		case "DINE_IN", "TAKEAWAY", "PICKUP", "DELIVERY":
		default:
			return nil, httpx.NewError(http.StatusBadRequest, "VALIDATION_ERROR",
				"applies_to hanya boleh DINE_IN, TAKEAWAY, PICKUP, DELIVERY.")
		}
	}
	if in.EffectiveFrom != nil {
		if _, err := time.Parse("2006-01-02", *in.EffectiveFrom); err != nil {
			return nil, httpx.NewError(http.StatusBadRequest, "VALIDATION_ERROR",
				"effective_from harus YYYY-MM-DD.")
		}
	}

	return s.repo.CreateTax(ctx, actor.OutletID, in)
}

func (s *Service) SetTaxActive(ctx context.Context, actor auth.Actor, id string, active bool) error {
	if actor.OutletID == "" {
		return outletMissing()
	}
	err := s.repo.SetTaxActive(ctx, actor.OutletID, id, active)
	if errors.Is(err, ErrNotFound) {
		return httpx.NewError(http.StatusNotFound, "TAX_NOT_FOUND", "Konfigurasi pajak tidak ditemukan.")
	}
	return err
}

func normalizePercent(raw *string) (*string, error) {
	if raw == nil {
		return nil, nil
	}
	value, err := money.Parse(*raw)
	if err != nil || value.IsNegative() || value.GreaterThan(hundred()) {
		return nil, httpx.NewError(http.StatusBadRequest, "VALIDATION_ERROR", "Persentase tidak valid (0-100).")
	}
	formatted := value.StringFixed(4)
	return &formatted, nil
}

func normalizeAmount(raw *string) (*string, error) {
	if raw == nil {
		return nil, nil
	}
	value, err := money.Parse(*raw)
	if err != nil || value.IsNegative() {
		return nil, httpx.NewError(http.StatusBadRequest, "VALIDATION_ERROR", "Nilai uang tidak valid.")
	}
	formatted := money.Format(value)
	return &formatted, nil
}

func hundred() money.Decimal {
	d, _ := money.Parse("100")
	return d
}

func outletMissing() error {
	return httpx.NewError(http.StatusUnprocessableEntity, "OUTLET_NOT_CONFIGURED",
		"Outlet belum dikonfigurasi.")
}
