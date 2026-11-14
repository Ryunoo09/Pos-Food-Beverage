package menu

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/ryunoo09/pos-food-beverage/backend/internal/auth"
	"github.com/ryunoo09/pos-food-beverage/backend/internal/httpx"
	"github.com/ryunoo09/pos-food-beverage/backend/internal/pkg/money"
)

const maxNameLength = 80

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) ListCategories(ctx context.Context, businessID string) ([]Category, error) {
	return s.repo.ListCategories(ctx, businessID)
}

func (s *Service) Category(ctx context.Context, businessID, id string) (*Category, error) {
	c, err := s.repo.Category(ctx, businessID, id)
	if errors.Is(err, ErrNotFound) {
		return nil, notFound()
	}
	return c, err
}

func (s *Service) CreateCategory(ctx context.Context, businessID string, in CategoryInput) (*Category, error) {
	in, err := validateCategory(in)
	if err != nil {
		return nil, err
	}

	c, err := s.repo.CreateCategory(ctx, businessID, in)
	if err != nil {
		return nil, mapCategoryError(err)
	}
	return c, nil
}

func (s *Service) UpdateCategory(ctx context.Context, businessID, id string, in CategoryInput) (*Category, error) {
	in, err := validateCategory(in)
	if err != nil {
		return nil, err
	}

	isActive := true
	if in.IsActive != nil {
		isActive = *in.IsActive
	}

	c, err := s.repo.UpdateCategory(ctx, businessID, id, in, isActive)
	if errors.Is(err, ErrNotFound) {
		return nil, notFound()
	}
	if err != nil {
		return nil, mapCategoryError(err)
	}
	return c, nil
}

func (s *Service) DeleteCategory(ctx context.Context, businessID, id string) error {
	err := s.repo.DeleteCategory(ctx, businessID, id)
	if errors.Is(err, ErrNotFound) {
		return notFound()
	}
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return httpx.NewError(http.StatusUnprocessableEntity, "CATEGORY_IN_USE",
				"Kategori masih dipakai produk. Nonaktifkan kategori atau pindahkan produknya.")
		}
		return err
	}
	return nil
}

func validateCategory(in CategoryInput) (CategoryInput, error) {
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" || utf8.RuneCountInString(in.Name) > maxNameLength {
		return in, httpx.NewError(http.StatusBadRequest, "VALIDATION_ERROR",
			"Nama kategori wajib diisi, maksimal 80 karakter.")
	}
	return in, nil
}

func mapCategoryError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return httpx.NewError(http.StatusConflict, "CATEGORY_NAME_EXISTS",
			"Nama kategori sudah dipakai.")
	}
	return err
}

func notFound() error {
	return httpx.NewError(http.StatusNotFound, "CATEGORY_NOT_FOUND", "Kategori tidak ditemukan.")
}

func (s *Service) ListProducts(ctx context.Context, businessID string, f ProductFilter) ([]Product, int, error) {
	return s.repo.ListProducts(ctx, businessID, f)
}

func (s *Service) Product(ctx context.Context, businessID, id string) (*Product, error) {
	p, err := s.repo.Product(ctx, businessID, id)
	if errors.Is(err, ErrNotFound) {
		return nil, productNotFound()
	}
	return p, err
}

func (s *Service) CreateProduct(ctx context.Context, actor auth.Actor, in ProductInput) (*Product, error) {
	in, err := s.validateProduct(ctx, actor.BusinessID, in, true)
	if err != nil {
		return nil, err
	}

	p, err := s.repo.CreateProduct(ctx, actor.BusinessID, actor.UserID, in)
	if err != nil {
		return nil, mapProductError(err)
	}
	return p, nil
}

func (s *Service) UpdateProduct(ctx context.Context, actor auth.Actor, id string, in ProductInput) (*Product, error) {
	in, err := s.validateProduct(ctx, actor.BusinessID, in, false)
	if err != nil {
		return nil, err
	}

	p, err := s.repo.UpdateProduct(ctx, actor.BusinessID, id, in)
	if errors.Is(err, ErrNotFound) {
		return nil, productNotFound()
	}
	if err != nil {
		return nil, mapProductError(err)
	}
	return p, nil
}

func (s *Service) SetProductActive(ctx context.Context, businessID, id string, active bool) error {
	err := s.repo.SetProductActive(ctx, businessID, id, active)
	if errors.Is(err, ErrNotFound) {
		return productNotFound()
	}
	return err
}

func (s *Service) validateProduct(ctx context.Context, businessID string, in ProductInput, withVariants bool) (ProductInput, error) {
	in.Name = strings.TrimSpace(in.Name)
	in.SKU = strings.TrimSpace(in.SKU)
	if in.Name == "" || in.SKU == "" || utf8.RuneCountInString(in.Name) > maxNameLength {
		return in, httpx.NewError(http.StatusBadRequest, "VALIDATION_ERROR",
			"Nama dan SKU produk wajib diisi, nama maksimal 80 karakter.")
	}
	if in.ProductType != "FOOD" && in.ProductType != "BEVERAGE" && in.ProductType != "OTHER" {
		return in, httpx.NewError(http.StatusBadRequest, "VALIDATION_ERROR", "Tipe produk tidak valid.")
	}
	switch in.StockMode {
	case "TRACK_BY_RECIPE", "TRACK_AS_STOCK_ITEM", "NO_TRACKING":
	default:
		return in, httpx.NewError(http.StatusBadRequest, "VALIDATION_ERROR", "Mode stok tidak valid.")
	}
	if in.KitchenStation != nil && *in.KitchenStation != "KITCHEN" && *in.KitchenStation != "BAR" {
		return in, httpx.NewError(http.StatusBadRequest, "VALIDATION_ERROR", "Station dapur tidak valid.")
	}

	if _, err := s.repo.Category(ctx, businessID, in.CategoryID); err != nil {
		if errors.Is(err, ErrNotFound) {
			return in, httpx.NewError(http.StatusUnprocessableEntity, "CATEGORY_NOT_FOUND",
				"Kategori produk tidak ditemukan.")
		}
		return in, err
	}

	if !withVariants {
		in.Variants = nil
		return in, nil
	}
	if len(in.Variants) == 0 {
		return in, httpx.NewError(http.StatusBadRequest, "VALIDATION_ERROR", "Minimal satu varian produk.")
	}

	defaults := 0
	for i := range in.Variants {
		v := &in.Variants[i]
		v.Name = strings.TrimSpace(v.Name)
		if v.Name == "" {
			return in, httpx.NewError(http.StatusBadRequest, "VALIDATION_ERROR", "Nama varian wajib diisi.")
		}
		price, err := money.Parse(v.Price)
		if err != nil || price.IsNegative() {
			return in, httpx.NewError(http.StatusBadRequest, "VALIDATION_ERROR", "Harga varian tidak valid.")
		}
		v.Price = money.Format(price)
		if v.IsDefault {
			defaults++
		}
	}
	if defaults > 1 {
		return in, httpx.NewError(http.StatusBadRequest, "VALIDATION_ERROR", "Hanya satu varian default.")
	}
	if defaults == 0 {
		in.Variants[0].IsDefault = true
	}
	return in, nil
}

func mapProductError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			return httpx.NewError(http.StatusConflict, "PRODUCT_SKU_EXISTS", "SKU produk sudah dipakai.")
		case "23503":
			return httpx.NewError(http.StatusUnprocessableEntity, "CATEGORY_NOT_FOUND",
				"Kategori produk tidak ditemukan.")
		}
	}
	return err
}

func productNotFound() error {
	return httpx.NewError(http.StatusNotFound, "PRODUCT_NOT_FOUND", "Produk tidak ditemukan.")
}

func (s *Service) ListVariants(ctx context.Context, businessID, productID string) ([]Variant, error) {
	if _, err := s.Product(ctx, businessID, productID); err != nil {
		return nil, err
	}
	return s.repo.variants(ctx, productID)
}

func (s *Service) CreateVariant(ctx context.Context, businessID, productID string, in VariantInput) (*Variant, error) {
	if _, err := s.Product(ctx, businessID, productID); err != nil {
		return nil, err
	}

	in, err := validateVariant(in)
	if err != nil {
		return nil, err
	}

	v, err := s.repo.CreateVariant(ctx, productID, in)
	if err != nil {
		return nil, mapProductError(err)
	}
	return v, nil
}

func (s *Service) UpdateVariant(ctx context.Context, actor auth.Actor, variantID string, in VariantInput) (*Variant, error) {
	in, err := validateVariant(in)
	if err != nil {
		return nil, err
	}

	// ponytail: harga boleh berubah hanya dengan products.update_price; riwayat harga
	// dicatat pada commit berikutnya bersama tabel price_histories.
	if !hasPermission(actor.Permissions, "products.update_price") {
		current, err := s.repo.Variant(ctx, actor.BusinessID, variantID)
		if errors.Is(err, ErrNotFound) {
			return nil, httpx.NewError(http.StatusNotFound, "VARIANT_NOT_FOUND", "Varian tidak ditemukan.")
		}
		if err != nil {
			return nil, err
		}
		parsed, err := money.Parse(in.Price)
		if err != nil || !parsed.Equal(mustParse(current.Price)) {
			return nil, httpx.NewError(http.StatusForbidden, "PRODUCT_PRICE_FORBIDDEN",
				"Tidak punya izin mengubah harga produk.")
		}
		in.Price = current.Price
	}

	v, err := s.repo.UpdateVariant(ctx, actor.BusinessID, variantID, in, activeOr(in.IsActive, true))
	if errors.Is(err, ErrNotFound) {
		return nil, httpx.NewError(http.StatusNotFound, "VARIANT_NOT_FOUND", "Varian tidak ditemukan.")
	}
	if err != nil {
		return nil, mapProductError(err)
	}
	return v, nil
}

func (s *Service) SetAvailability(ctx context.Context, businessID, productID string, available bool) error {
	err := s.repo.SetProductAvailability(ctx, businessID, productID, available)
	if errors.Is(err, ErrNotFound) {
		return productNotFound()
	}
	return err
}

func validateVariant(in VariantInput) (VariantInput, error) {
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		return in, httpx.NewError(http.StatusBadRequest, "VALIDATION_ERROR", "Nama varian wajib diisi.")
	}
	price, err := money.Parse(in.Price)
	if err != nil || price.IsNegative() {
		return in, httpx.NewError(http.StatusBadRequest, "VALIDATION_ERROR", "Harga varian tidak valid.")
	}
	in.Price = money.Format(price)
	return in, nil
}

func activeOr(v *bool, fallback bool) bool {
	if v == nil {
		return fallback
	}
	return *v
}

func mustParse(s string) money.Decimal {
	d, _ := money.Parse(s)
	return d
}

func hasPermission(list []string, want string) bool {
	for _, p := range list {
		if p == want {
			return true
		}
	}
	return false
}
