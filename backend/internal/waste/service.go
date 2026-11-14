package waste

import (
	"context"
	"errors"
	"net/http"
	"sort"
	"strings"

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

// Record mencatat waste bahan atau produk dan menulis movement WASTE.
func (s *Service) Record(ctx context.Context, actor auth.Actor, in Input) (*Record, error) {
	if actor.OutletID == "" {
		return nil, outletMissing()
	}
	wasteType := strings.ToUpper(strings.TrimSpace(in.WasteType))
	if wasteType != "INGREDIENT" && wasteType != "PRODUCT" {
		return nil, validation("waste_type harus INGREDIENT atau PRODUCT.")
	}
	qty, err := money.Parse(strings.TrimSpace(in.Quantity))
	if err != nil || !qty.IsPositive() {
		return nil, validation("quantity harus lebih dari 0.")
	}

	var (
		lines        []wasteLine
		ingredientID *string
		variantID    *string
		unitID       *int
	)
	switch wasteType {
	case "INGREDIENT":
		if in.IngredientID == nil || strings.TrimSpace(*in.IngredientID) == "" {
			return nil, validation("ingredient_id wajib untuk waste bahan.")
		}
		baseUnit, _, _, _, found, err := s.repo.ingredientInfo(ctx, actor.BusinessID, *in.IngredientID)
		if err != nil {
			return nil, err
		}
		if !found {
			return nil, httpx.NewError(http.StatusUnprocessableEntity, "INGREDIENT_NOT_FOUND",
				"Bahan tidak ditemukan atau tidak aktif.")
		}
		factor := money.FromInt(1)
		if in.UnitID != nil && *in.UnitID != baseUnit {
			factorStr, ok, err := s.repo.UnitToBase(ctx, *in.IngredientID, *in.UnitID)
			if err != nil {
				return nil, err
			}
			if !ok {
				return nil, httpx.NewError(http.StatusUnprocessableEntity, "UNIT_NOT_FOUND",
					"Satuan tidak ditemukan.")
			}
			factor, err = money.Parse(factorStr)
			if err != nil {
				return nil, err
			}
		}
		lines = []wasteLine{{IngredientID: *in.IngredientID, Quantity: qty.Mul(factor)}}
		ingredientID = in.IngredientID
		unitID = &baseUnit

	case "PRODUCT":
		if in.VariantID == nil || strings.TrimSpace(*in.VariantID) == "" {
			return nil, validation("variant_id wajib untuk waste produk.")
		}
		_, stockMode, stockIngredient, found, err := s.repo.variantInfo(ctx, actor.BusinessID, *in.VariantID)
		if err != nil {
			return nil, err
		}
		if !found {
			return nil, httpx.NewError(http.StatusUnprocessableEntity, "VARIANT_NOT_FOUND",
				"Varian produk tidak ditemukan.")
		}
		if stockMode == "TRACK_AS_STOCK_ITEM" {
			if stockIngredient == "" {
				return nil, httpx.NewError(http.StatusUnprocessableEntity, "VARIANT_NO_STOCK_ITEM",
					"Produk tidak punya bahan stok.")
			}
			lines = []wasteLine{{IngredientID: stockIngredient, Quantity: qty}}
		} else {
			components, err := s.repo.activeRecipe(ctx, *in.VariantID)
			if err != nil {
				return nil, err
			}
			if len(components) == 0 {
				return nil, httpx.NewError(http.StatusUnprocessableEntity, "VARIANT_NO_RECIPE",
					"Produk belum punya resep aktif.")
			}
			lines, err = recipeWasteLines(components, qty)
			if err != nil {
				return nil, err
			}
		}
		variantID = in.VariantID
	}

	// Kunci inventory urut ingredient_id agar tidak deadlock.
	sort.Slice(lines, func(i, j int) bool { return lines[i].IngredientID < lines[j].IngredientID })

	tx, err := s.repo.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	allowNegative, err := s.repo.AllowNegativeStock(ctx, actor.OutletID)
	if err != nil {
		return nil, err
	}

	type posted struct {
		line     wasteLine
		avg      money.Decimal
		balance  money.Decimal
		shortage bool
	}
	posts := make([]posted, 0, len(lines))
	costValue := money.Decimal{}
	for _, ln := range lines {
		qtyRaw, avgRaw, err := s.repo.LockInventory(ctx, tx, actor.OutletID, ln.IngredientID)
		if err != nil {
			return nil, err
		}
		onHand, err := money.Parse(qtyRaw)
		if err != nil {
			return nil, err
		}
		avg, err := money.Parse(avgRaw)
		if err != nil {
			return nil, err
		}
		balance := onHand.Sub(ln.Quantity)
		if balance.IsNegative() && !allowNegative {
			return nil, httpx.NewError(http.StatusUnprocessableEntity, "INSUFFICIENT_STOCK",
				"Stok tidak cukup untuk mencatat waste ini.")
		}
		costValue = costValue.Add(ln.Quantity.Mul(avg))
		posts = append(posts, posted{line: ln, avg: avg, balance: balance, shortage: balance.IsNegative()})
	}

	reason := trimNote(in.ReasonCode)
	recordID, err := s.repo.CreateRecord(ctx, tx, actor.OutletID, wasteType, actor.UserID,
		ingredientID, variantID, money.Format(qty), unitID, money.Format(costValue),
		reason, trimNote(in.Note), trimNote(in.OrderID))
	if err != nil {
		return nil, err
	}

	reasonText := "Waste"
	if reason != nil {
		reasonText = *reason
	}
	for _, p := range posts {
		delta := p.line.Quantity.Neg()
		totalCost := money.RoundHalfUp(p.line.Quantity.Mul(p.avg), 2)
		if _, err := s.repo.InsertWasteMovement(ctx, tx, actor.OutletID, p.line.IngredientID, recordID,
			qtyStr(delta), costStr(p.avg), amtStr(totalCost), qtyStr(p.balance), costStr(p.avg),
			reasonText, p.shortage, actor.UserID); err != nil {
			return nil, err
		}
		if err := s.repo.UpdateInventory(ctx, tx, actor.OutletID, p.line.IngredientID, qtyStr(p.balance)); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.repo.Record(ctx, actor.OutletID, recordID)
}

// Reverse membatalkan waste: menulis movement WASTE_REVERSAL dan mengembalikan stok.
func (s *Service) Reverse(ctx context.Context, actor auth.Actor, id string) (*Record, error) {
	if actor.OutletID == "" {
		return nil, outletMissing()
	}
	tx, err := s.repo.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	status, found, err := s.repo.LockRecord(ctx, tx, actor.OutletID, id)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, notFound()
	}
	if status != "POSTED" {
		return nil, httpx.NewError(http.StatusConflict, "WASTE_NOT_REVERSIBLE",
			"Waste ini tidak bisa dibatalkan pada status saat ini.")
	}

	movements, err := s.repo.OriginalMovements(ctx, tx, id)
	if err != nil {
		return nil, err
	}
	if len(movements) == 0 {
		return nil, httpx.NewError(http.StatusUnprocessableEntity, "WASTE_NO_MOVEMENT",
			"Tidak ada movement yang bisa dibalik.")
	}

	for _, m := range movements {
		qtyRaw, avgRaw, err := s.repo.LockInventory(ctx, tx, actor.OutletID, m.IngredientID)
		if err != nil {
			return nil, err
		}
		onHand, err := money.Parse(qtyRaw)
		if err != nil {
			return nil, err
		}
		avg, err := money.Parse(avgRaw)
		if err != nil {
			return nil, err
		}
		originalQty, err := money.Parse(m.Quantity)
		if err != nil {
			return nil, err
		}
		unitCost, err := money.Parse(m.UnitCost)
		if err != nil {
			return nil, err
		}
		restored := originalQty.Neg()
		balance := onHand.Add(restored)
		totalCost := money.RoundHalfUp(restored.Mul(unitCost), 2)
		if err := s.repo.InsertReversalMovement(ctx, tx, actor.OutletID, m.IngredientID, id,
			qtyStr(restored), costStr(unitCost), amtStr(totalCost), qtyStr(balance), costStr(avg),
			actor.UserID); err != nil {
			return nil, err
		}
		if err := s.repo.UpdateInventory(ctx, tx, actor.OutletID, m.IngredientID, qtyStr(balance)); err != nil {
			return nil, err
		}
	}

	if err := s.repo.SetReversed(ctx, tx, actor.OutletID, id, actor.UserID); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.repo.Record(ctx, actor.OutletID, id)
}

func (s *Service) Get(ctx context.Context, actor auth.Actor, id string) (*Record, error) {
	if actor.OutletID == "" {
		return nil, outletMissing()
	}
	o, err := s.repo.Record(ctx, actor.OutletID, id)
	if errors.Is(err, ErrNotFound) {
		return nil, notFound()
	}
	return o, err
}

func (s *Service) List(ctx context.Context, actor auth.Actor, f Filter) ([]Record, int, error) {
	if actor.OutletID == "" {
		return nil, 0, outletMissing()
	}
	return s.repo.List(ctx, actor.OutletID, f)
}

func validation(msg string) error {
	return httpx.NewError(http.StatusBadRequest, "VALIDATION_ERROR", msg)
}

func notFound() error {
	return httpx.NewError(http.StatusNotFound, "WASTE_NOT_FOUND", "Catatan waste tidak ditemukan.")
}

func outletMissing() error {
	return httpx.NewError(http.StatusUnprocessableEntity, "OUTLET_NOT_CONFIGURED",
		"Outlet belum dikonfigurasi.")
}

func trimNote(s *string) *string {
	if s == nil {
		return nil
	}
	v := strings.TrimSpace(*s)
	if v == "" {
		return nil
	}
	return &v
}

func amtStr(d money.Decimal) string {
	return money.Format(d)
}

func qtyStr(d money.Decimal) string {
	return d.Round(4).StringFixed(4)
}

func costStr(d money.Decimal) string {
	return d.Round(6).StringFixed(6)
}
