package supplier

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/ryunoo09/pos-food-beverage/backend/internal/httpx"
	"github.com/ryunoo09/pos-food-beverage/backend/internal/pkg/money"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) List(ctx context.Context, businessID, query string, active *bool) ([]Supplier, error) {
	return s.repo.List(ctx, businessID, query, active)
}

func (s *Service) Supplier(ctx context.Context, businessID, id string) (*Supplier, error) {
	item, err := s.repo.Supplier(ctx, businessID, id)
	if errors.Is(err, ErrNotFound) {
		return nil, notFound()
	}
	return item, err
}

func (s *Service) Create(ctx context.Context, businessID string, in SupplierInput) (*Supplier, error) {
	in, err := validate(in)
	if err != nil {
		return nil, err
	}

	item, err := s.repo.Create(ctx, businessID, in)
	if err != nil {
		return nil, mapError(err)
	}
	return item, nil
}

func (s *Service) Update(ctx context.Context, businessID, id string, in SupplierInput) (*Supplier, error) {
	in, err := validate(in)
	if err != nil {
		return nil, err
	}

	item, err := s.repo.Update(ctx, businessID, id, in)
	if errors.Is(err, ErrNotFound) {
		return nil, notFound()
	}
	if err != nil {
		return nil, mapError(err)
	}
	return item, nil
}

func (s *Service) SetActive(ctx context.Context, businessID, id string, active bool) error {
	err := s.repo.SetActive(ctx, businessID, id, active)
	if errors.Is(err, ErrNotFound) {
		return notFound()
	}
	return err
}

func (s *Service) SetIngredients(ctx context.Context, businessID, id string, items []SupplierIngredientInput) error {
	for i := range items {
		item := &items[i]
		if item.IngredientID == "" {
			return httpx.NewError(http.StatusBadRequest, "VALIDATION_ERROR", "ingredient_id wajib diisi.")
		}
		if item.LastPrice != nil {
			price, err := money.Parse(*item.LastPrice)
			if err != nil || price.IsNegative() {
				return httpx.NewError(http.StatusBadRequest, "VALIDATION_ERROR", "last_price tidak valid.")
			}
			formatted := money.Format(price)
			item.LastPrice = &formatted
		}
		if item.LeadTimeDays != nil && *item.LeadTimeDays < 0 {
			return httpx.NewError(http.StatusBadRequest, "VALIDATION_ERROR", "lead_time_days tidak valid.")
		}
	}

	err := s.repo.SetIngredients(ctx, businessID, id, items)
	if errors.Is(err, ErrNotFound) {
		return httpx.NewError(http.StatusUnprocessableEntity, "SUPPLIER_OR_INGREDIENT_NOT_FOUND",
			"Supplier atau bahan tidak ditemukan.")
	}
	return err
}

// PurchaseHistory mengembalikan riwayat PO supplier pada outlet aktif.
func (s *Service) PurchaseHistory(ctx context.Context, outletID, supplierID string) ([]PurchaseHistoryRow, error) {
	return s.repo.PurchaseHistory(ctx, outletID, supplierID)
}

func validate(in SupplierInput) (SupplierInput, error) {
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		return in, httpx.NewError(http.StatusBadRequest, "VALIDATION_ERROR", "Nama supplier wajib diisi.")
	}
	return in, nil
}

func mapError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return httpx.NewError(http.StatusConflict, "SUPPLIER_NAME_EXISTS", "Nama supplier sudah dipakai.")
	}
	return err
}

func notFound() error {
	return httpx.NewError(http.StatusNotFound, "SUPPLIER_NOT_FOUND", "Supplier tidak ditemukan.")
}
