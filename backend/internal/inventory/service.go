package inventory

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/shopspring/decimal"

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

func (s *Service) Units(ctx context.Context) ([]Unit, error) {
	return s.repo.Units(ctx)
}

func (s *Service) Categories(ctx context.Context, businessID string) ([]IngredientCategory, error) {
	return s.repo.Categories(ctx, businessID)
}

func (s *Service) CreateCategory(ctx context.Context, businessID string, in IngredientCategoryInput) (*IngredientCategory, error) {
	name, err := validateName(in.Name, "Nama kategori bahan wajib diisi.")
	if err != nil {
		return nil, err
	}

	c, err := s.repo.CreateCategory(ctx, businessID, name)
	if err != nil {
		return nil, mapError(err, "INGREDIENT_CATEGORY_NAME_EXISTS", "Nama kategori bahan sudah dipakai.")
	}
	return c, nil
}

func (s *Service) UpdateCategory(ctx context.Context, businessID, id string, in IngredientCategoryInput) (*IngredientCategory, error) {
	name, err := validateName(in.Name, "Nama kategori bahan wajib diisi.")
	if err != nil {
		return nil, err
	}

	active := true
	if in.IsActive != nil {
		active = *in.IsActive
	}

	c, err := s.repo.UpdateCategory(ctx, businessID, id, name, active)
	if errors.Is(err, ErrNotFound) {
		return nil, categoryNotFound()
	}
	if err != nil {
		return nil, mapError(err, "INGREDIENT_CATEGORY_NAME_EXISTS", "Nama kategori bahan sudah dipakai.")
	}
	return c, nil
}

func (s *Service) DeleteCategory(ctx context.Context, businessID, id string) error {
	err := s.repo.DeleteCategory(ctx, businessID, id)
	if errors.Is(err, ErrNotFound) {
		return categoryNotFound()
	}
	return err
}

func (s *Service) ListIngredients(ctx context.Context, businessID, outletID string, f IngredientFilter) ([]Ingredient, int, error) {
	return s.repo.ListIngredients(ctx, businessID, outletID, f)
}

func (s *Service) Ingredient(ctx context.Context, businessID, outletID, id string) (*Ingredient, error) {
	i, err := s.repo.Ingredient(ctx, businessID, outletID, id)
	if errors.Is(err, ErrNotFound) {
		return nil, ingredientNotFound()
	}
	return i, err
}

func (s *Service) CreateIngredient(ctx context.Context, actor auth.Actor, in IngredientInput) (*Ingredient, error) {
	in, err := s.validateIngredient(ctx, actor.BusinessID, in)
	if err != nil {
		return nil, err
	}

	i, err := s.repo.CreateIngredient(ctx, actor.BusinessID, actor.OutletID, in)
	if err != nil {
		return nil, mapError(err, "INGREDIENT_CODE_EXISTS", "Kode bahan sudah dipakai.")
	}
	return i, nil
}

func (s *Service) UpdateIngredient(ctx context.Context, actor auth.Actor, id string, in IngredientInput) (*Ingredient, error) {
	in, err := s.validateIngredient(ctx, actor.BusinessID, in)
	if err != nil {
		return nil, err
	}

	current, err := s.Ingredient(ctx, actor.BusinessID, actor.OutletID, id)
	if err != nil {
		return nil, err
	}
	if current.BaseUnitID != in.BaseUnitID {
		used, err := s.repo.HasMovements(ctx, actor.OutletID, id)
		if err != nil {
			return nil, err
		}
		if used {
			return nil, httpx.NewError(http.StatusUnprocessableEntity, "BASE_UNIT_LOCKED",
				"Satuan dasar tidak bisa diubah setelah bahan punya riwayat stok.")
		}
	}

	i, err := s.repo.UpdateIngredient(ctx, actor.BusinessID, actor.OutletID, id, in)
	if errors.Is(err, ErrNotFound) {
		return nil, ingredientNotFound()
	}
	if err != nil {
		return nil, mapError(err, "INGREDIENT_CODE_EXISTS", "Kode bahan sudah dipakai.")
	}
	return i, nil
}

func (s *Service) SetIngredientActive(ctx context.Context, businessID, id string, active bool) error {
	err := s.repo.SetIngredientActive(ctx, businessID, id, active)
	if errors.Is(err, ErrNotFound) {
		return ingredientNotFound()
	}
	return err
}

func (s *Service) Movements(ctx context.Context, actor auth.Actor, ingredientID string) ([]StockMovement, error) {
	if _, err := s.Ingredient(ctx, actor.BusinessID, actor.OutletID, ingredientID); err != nil {
		return nil, err
	}
	if actor.OutletID == "" {
		return nil, httpx.NewError(http.StatusUnprocessableEntity, "OUTLET_NOT_CONFIGURED",
			"Outlet belum dikonfigurasi.")
	}
	return s.repo.Movements(ctx, actor.OutletID, ingredientID, 100)
}

// Stock mengembalikan saldo stok per bahan; biaya disembunyikan tanpa inventory.view_cost.
func (s *Service) Stock(ctx context.Context, actor auth.Actor, f StockFilter) ([]StockLevel, int, error) {
	if actor.OutletID == "" {
		return nil, 0, httpx.NewError(http.StatusUnprocessableEntity, "OUTLET_NOT_CONFIGURED",
			"Outlet belum dikonfigurasi.")
	}
	f.Query = strings.TrimSpace(f.Query)
	items, total, err := s.repo.StockLevels(ctx, actor.BusinessID, actor.OutletID, f)
	if err != nil {
		return nil, 0, err
	}
	if !hasPermission(actor, "inventory.view_cost") {
		for i := range items {
			items[i].AvgUnitCost = ""
			items[i].StockValue = ""
		}
	}
	return items, total, nil
}

// AdjustResult adalah saldo setelah penyesuaian manual.
type AdjustResult struct {
	IngredientID   string `json:"ingredient_id"`
	QuantityOnHand string `json:"quantity_on_hand"`
}

// Adjust menyesuaikan saldo stok manual. Nilai penyesuaian di atas ambang
// (BR-F09-2) butuh permission inventory.approve_adjustment.
func (s *Service) Adjust(ctx context.Context, actor auth.Actor, ingredientID string, in AdjustInput) (*AdjustResult, error) {
	if actor.OutletID == "" {
		return nil, httpx.NewError(http.StatusUnprocessableEntity, "OUTLET_NOT_CONFIGURED",
			"Outlet belum dikonfigurasi.")
	}
	reason := strings.TrimSpace(in.Reason)
	if reason == "" {
		return nil, httpx.NewError(http.StatusBadRequest, "VALIDATION_ERROR", "reason wajib diisi.")
	}
	delta, err := money.Parse(strings.TrimSpace(in.QuantityDelta))
	if err != nil {
		return nil, httpx.NewError(http.StatusBadRequest, "VALIDATION_ERROR", "quantity_delta tidak valid.")
	}
	if delta.IsZero() {
		return nil, httpx.NewError(http.StatusBadRequest, "VALIDATION_ERROR", "quantity_delta tidak boleh nol.")
	}
	exists, err := s.repo.IngredientExists(ctx, actor.BusinessID, ingredientID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, ingredientNotFound()
	}

	qtyStr, avgStr, _, err := s.repo.CurrentStock(ctx, actor.OutletID, ingredientID)
	if err != nil {
		return nil, err
	}
	threshold, allowSelf, err := s.repo.AdjustSettings(ctx, actor.OutletID)
	if err != nil {
		return nil, err
	}
	qty, err := money.Parse(qtyStr)
	if err != nil {
		return nil, err
	}
	avg, err := money.Parse(avgStr)
	if err != nil {
		return nil, err
	}
	limit, err := money.Parse(threshold)
	if err != nil {
		return nil, err
	}

	value := delta.Abs().Mul(avg)
	needsApproval := needsAdjustmentApproval(delta, qty, value, limit)
	if needsApproval && !hasPermission(actor, "inventory.approve_adjustment") && !allowSelf {
		return nil, httpx.Errorf(http.StatusUnprocessableEntity, "ADJUSTMENT_APPROVAL_REQUIRED",
			"Penyesuaian di atas ambang butuh persetujuan manager.", map[string]string{
				"threshold_amount": threshold,
				"adjustment_value": money.Format(value),
			})
	}

	balance, err := s.repo.Adjust(ctx, actor.OutletID, ingredientID, actor.UserID,
		delta.String(), reason, in.Note)
	if err != nil {
		return nil, err
	}
	return &AdjustResult{IngredientID: ingredientID, QuantityOnHand: balance}, nil
}

func hasPermission(actor auth.Actor, code string) bool {
	for _, p := range actor.Permissions {
		if p == code {
			return true
		}
	}
	return false
}

// needsAdjustmentApproval menerapkan BR-F09-2: nilai penyesuaian di atas ambang
// ATAU lebih dari 20% saldo saat ini butuh persetujuan manager.
func needsAdjustmentApproval(delta, qty, value, limit money.Decimal) bool {
	if value.GreaterThan(limit) {
		return true
	}
	return !qty.IsZero() && delta.Abs().Div(qty.Abs()).GreaterThan(decimal.RequireFromString("0.20"))
}

func (s *Service) validateIngredient(ctx context.Context, businessID string, in IngredientInput) (IngredientInput, error) {
	in.Code = strings.TrimSpace(in.Code)
	name, err := validateName(in.Name, "Nama bahan wajib diisi.")
	if err != nil {
		return in, err
	}
	if in.Code == "" {
		return in, httpx.NewError(http.StatusBadRequest, "VALIDATION_ERROR", "Kode bahan wajib diisi.")
	}
	in.Name = name

	minStock, err := money.Parse(in.MinStock)
	if in.MinStock == "" {
		minStock = money.Decimal{}
		err = nil
	}
	if err != nil || minStock.IsNegative() {
		return in, httpx.NewError(http.StatusBadRequest, "VALIDATION_ERROR", "min_stock tidak valid.")
	}
	in.MinStock = minStock.String()

	exists, err := s.repo.UnitExists(ctx, in.BaseUnitID)
	if err != nil {
		return in, err
	}
	if !exists {
		return in, httpx.NewError(http.StatusUnprocessableEntity, "UNIT_NOT_FOUND", "Satuan tidak ditemukan.")
	}
	// ponytail: dimensi satuan beli tidak dicek terhadap satuan dasar; tambahkan
	// validasi MASS/VOLUME/COUNT saat konversi satuan beli dipakai di pembelian.
	if in.DefaultPurchaseUnit != nil {
		exists, err = s.repo.UnitExists(ctx, *in.DefaultPurchaseUnit)
		if err != nil {
			return in, err
		}
		if !exists {
			return in, httpx.NewError(http.StatusUnprocessableEntity, "UNIT_NOT_FOUND", "Satuan beli tidak ditemukan.")
		}
	}

	if in.CategoryID != nil && *in.CategoryID != "" {
		exists, err := s.repo.CategoryExists(ctx, businessID, *in.CategoryID)
		if err != nil {
			return in, err
		}
		if !exists {
			return in, httpx.NewError(http.StatusUnprocessableEntity, "INGREDIENT_CATEGORY_NOT_FOUND",
				"Kategori bahan tidak ditemukan.")
		}
	} else {
		in.CategoryID = nil
	}
	return in, nil
}

func validateName(raw, message string) (string, error) {
	name := strings.TrimSpace(raw)
	if name == "" {
		return "", httpx.NewError(http.StatusBadRequest, "VALIDATION_ERROR", message)
	}
	return name, nil
}

func mapError(err error, conflictCode, conflictMessage string) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			return httpx.NewError(http.StatusConflict, conflictCode, conflictMessage)
		case "23503":
			return httpx.NewError(http.StatusUnprocessableEntity, "INGREDIENT_CATEGORY_NOT_FOUND",
				"Kategori bahan tidak ditemukan.")
		}
	}
	return err
}

func categoryNotFound() error {
	return httpx.NewError(http.StatusNotFound, "INGREDIENT_CATEGORY_NOT_FOUND", "Kategori bahan tidak ditemukan.")
}

func ingredientNotFound() error {
	return httpx.NewError(http.StatusNotFound, "INGREDIENT_NOT_FOUND", "Bahan tidak ditemukan.")
}
