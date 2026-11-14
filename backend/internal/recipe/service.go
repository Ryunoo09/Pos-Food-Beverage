package recipe

import (
	"context"
	"errors"
	"net/http"

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

func (s *Service) Active(ctx context.Context, actor auth.Actor, variantID string, withCost bool) (*Recipe, error) {
	rc, err := s.repo.Active(ctx, actor.BusinessID, actor.OutletID, variantID)
	if errors.Is(err, ErrNotFound) {
		return nil, httpx.NewError(http.StatusNotFound, "RECIPE_NOT_FOUND",
			"Varian belum punya resep aktif.")
	}
	if err != nil {
		return nil, err
	}

	total := money.Decimal{}
	hundred, _ := money.Parse("100")
	for i := range rc.Items {
		item := &rc.Items[i]
		qty, _ := money.Parse(item.Quantity)
		waste, _ := money.Parse(item.WasteFactorPercent)
		cost, _ := money.Parse(item.avgCost)

		effective := qty.Mul(waste.Add(hundred).Div(hundred))
		line := money.RoundHalfUp(effective.Mul(cost), 2)
		total = total.Add(line)

		if withCost {
			formatted := money.Format(line)
			item.LineCost = &formatted
		}
	}
	if withCost {
		formatted := money.Format(total)
		rc.TotalCost = &formatted
	}
	return rc, nil
}

func (s *Service) Versions(ctx context.Context, actor auth.Actor, variantID string) ([]Recipe, error) {
	return s.repo.Versions(ctx, actor.BusinessID, variantID)
}

func (s *Service) Create(ctx context.Context, actor auth.Actor, variantID string, in RecipeInput) (*Recipe, error) {
	ok, err := s.repo.VariantExists(ctx, actor.BusinessID, variantID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, httpx.NewError(http.StatusNotFound, "VARIANT_NOT_FOUND", "Varian tidak ditemukan.")
	}

	items, ids, err := validateItems(in.Items)
	if err != nil {
		return nil, err
	}
	in.Items = items

	ok, err = s.repo.IngredientsExist(ctx, actor.BusinessID, ids)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, httpx.NewError(http.StatusUnprocessableEntity, "INGREDIENT_NOT_FOUND",
			"Ada bahan resep yang tidak ditemukan.")
	}

	return s.repo.CreateVersion(ctx, actor.UserID, variantID, in)
}

func validateItems(items []RecipeItemInput) ([]RecipeItemInput, []string, error) {
	if len(items) == 0 {
		return nil, nil, httpx.NewError(http.StatusBadRequest, "VALIDATION_ERROR",
			"Resep minimal punya satu bahan.")
	}

	ids := make([]string, 0, len(items))
	seen := map[string]bool{}
	for i := range items {
		item := &items[i]
		if item.IngredientID == "" {
			return nil, nil, httpx.NewError(http.StatusBadRequest, "VALIDATION_ERROR", "ingredient_id wajib diisi.")
		}
		if seen[item.IngredientID] {
			return nil, nil, httpx.NewError(http.StatusBadRequest, "VALIDATION_ERROR",
				"Bahan yang sama tidak boleh muncul dua kali dalam satu resep.")
		}
		seen[item.IngredientID] = true
		ids = append(ids, item.IngredientID)

		if item.WasteFactorPercent == "" {
			item.WasteFactorPercent = "0"
		}
		qty, err := money.Parse(item.Quantity)
		if err != nil || !qty.IsPositive() {
			return nil, nil, httpx.NewError(http.StatusBadRequest, "VALIDATION_ERROR",
				"quantity harus lebih dari nol.")
		}
		waste, err := money.Parse(item.WasteFactorPercent)
		if err != nil || waste.IsNegative() {
			return nil, nil, httpx.NewError(http.StatusBadRequest, "VALIDATION_ERROR",
				"waste_factor_percent tidak valid.")
		}
		item.Quantity = qty.String()
		item.WasteFactorPercent = waste.String()
	}
	return items, ids, nil
}
