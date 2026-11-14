package order

import (
	"context"
	"fmt"
	"net/http"
	"sort"

	"github.com/jackc/pgx/v5"

	"github.com/ryunoo09/pos-food-beverage/backend/internal/httpx"
	"github.com/ryunoo09/pos-food-beverage/backend/internal/pkg/money"
)

// stockLine adalah kebutuhan satu bahan untuk satu item pesanan.
type stockLine struct {
	OrderItemID  string
	IngredientID string
	Source       string // RECIPE | MODIFIER | STOCK_ITEM
	Quantity     money.Decimal
}

var hundredDec, _ = money.Parse("100")

var zeroDec, _ = money.Parse("0")

// StockLines menghitung kebutuhan bahan tiap item aktif dari resep varian,
// resep modifier, dan mode stok langsung. Item TRACK_BY_RECIPE tanpa resep
// ditandai cost_unknown.
func (r *Repository) StockLines(ctx context.Context, tx pgx.Tx, orderID string) ([]stockLine, []string, error) {
	const itemQ = `
		SELECT oi.id, oi.variant_id, oi.quantity, p.stock_mode,
		       COALESCE(p.stock_ingredient_id::text, '')
		FROM order_items oi
		JOIN products p ON p.id = oi.product_id
		WHERE oi.order_id = $1 AND oi.status = 'ACTIVE'
		ORDER BY oi.created_at`

	rows, err := tx.Query(ctx, itemQ, orderID)
	if err != nil {
		return nil, nil, fmt.Errorf("stockLines: items: %w", err)
	}
	type itemInfo struct {
		id        string
		variantID string
		quantity  int
		mode      string
		stockIng  string
	}
	items := []itemInfo{}
	variants := []string{}
	itemIDs := []string{}
	for rows.Next() {
		var it itemInfo
		if err := rows.Scan(&it.id, &it.variantID, &it.quantity, &it.mode, &it.stockIng); err != nil {
			rows.Close()
			return nil, nil, fmt.Errorf("stockLines: scan item: %w", err)
		}
		items = append(items, it)
		variants = append(variants, it.variantID)
		itemIDs = append(itemIDs, it.id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	if len(items) == 0 {
		return nil, nil, nil
	}

	type component struct {
		ingredientID string
		quantity     money.Decimal
	}
	recipeByVariant := map[string][]component{}
	hasRecipe := map[string]bool{}

	// Komponen resep per varian.
	rrows, err := tx.Query(ctx, `
		SELECT rec.variant_id, ri.ingredient_id, ri.quantity::text, ri.waste_factor_percent::text
		FROM recipes rec
		JOIN recipe_items ri ON ri.recipe_id = rec.id
		WHERE rec.status = 'ACTIVE' AND rec.variant_id = ANY($1::uuid[])`, variants)
	if err != nil {
		return nil, nil, fmt.Errorf("stockLines: recipes: %w", err)
	}
	for rrows.Next() {
		var variantID, ingID, qtyStr, wasteStr string
		if err := rrows.Scan(&variantID, &ingID, &qtyStr, &wasteStr); err != nil {
			rrows.Close()
			return nil, nil, fmt.Errorf("stockLines: scan recipe: %w", err)
		}
		qty, err := money.Parse(qtyStr)
		if err != nil {
			rrows.Close()
			return nil, nil, err
		}
		waste, err := money.Parse(wasteStr)
		if err != nil {
			rrows.Close()
			return nil, nil, err
		}
		withWaste := qty.Mul(hundredDec.Add(waste)).Div(hundredDec)
		recipeByVariant[variantID] = append(recipeByVariant[variantID], component{ingID, withWaste})
		hasRecipe[variantID] = true
	}
	rrows.Close()
	if err := rrows.Err(); err != nil {
		return nil, nil, err
	}

	// Komponen resep modifier: ADD menambah, REPLACE menambah pengganti dan
	// mengurangi bahan asal (BR-F08).
	type modComponent struct {
		orderItemID  string
		ingredientID string
		quantity     money.Decimal
		negative     bool
	}
	mods := []modComponent{}
	mrows, err := tx.Query(ctx, `
		SELECT oim.order_item_id, mori.ingredient_id, mori.quantity::text, mori.mode,
		       COALESCE(mori.replaces_ingredient_id::text, '')
		FROM order_item_modifiers oim
		JOIN modifier_option_recipe_items mori ON mori.option_id = oim.option_id
		WHERE oim.order_item_id = ANY($1::uuid[])`, itemIDs)
	if err != nil {
		return nil, nil, fmt.Errorf("stockLines: modifier recipes: %w", err)
	}
	for mrows.Next() {
		var orderItemID, ingID, qtyStr, mode, replaces string
		if err := mrows.Scan(&orderItemID, &ingID, &qtyStr, &mode, &replaces); err != nil {
			mrows.Close()
			return nil, nil, fmt.Errorf("stockLines: scan modifier recipe: %w", err)
		}
		qty, err := money.Parse(qtyStr)
		if err != nil {
			mrows.Close()
			return nil, nil, err
		}
		mods = append(mods, modComponent{orderItemID, ingID, qty, false})
		if mode == "REPLACE" && replaces != "" {
			mods = append(mods, modComponent{orderItemID, replaces, qty, true})
		}
	}
	mrows.Close()
	if err := mrows.Err(); err != nil {
		return nil, nil, err
	}

	modsByItem := map[string][]modComponent{}
	for _, m := range mods {
		modsByItem[m.orderItemID] = append(modsByItem[m.orderItemID], m)
	}

	lines := []stockLine{}
	unknown := []string{}
	for _, it := range items {
		qtyDec := money.FromInt(int64(it.quantity))

		perIngredient := map[string]money.Decimal{}
		sources := map[string]string{}
		for _, c := range recipeByVariant[it.variantID] {
			perIngredient[c.ingredientID] = perIngredient[c.ingredientID].Add(c.quantity.Mul(qtyDec))
			sources[c.ingredientID] = "RECIPE"
		}
		for _, m := range modsByItem[it.id] {
			contrib := m.quantity.Mul(qtyDec)
			if m.negative {
				perIngredient[m.ingredientID] = perIngredient[m.ingredientID].Sub(contrib)
			} else {
				perIngredient[m.ingredientID] = perIngredient[m.ingredientID].Add(contrib)
				if sources[m.ingredientID] == "" {
					sources[m.ingredientID] = "MODIFIER"
				}
			}
		}
		switch it.mode {
		case "TRACK_BY_RECIPE":
			if !hasRecipe[it.variantID] {
				unknown = append(unknown, it.id)
			}
		case "TRACK_AS_STOCK_ITEM":
			if it.stockIng == "" {
				unknown = append(unknown, it.id)
				break
			}
			perIngredient[it.stockIng] = perIngredient[it.stockIng].Add(qtyDec)
			sources[it.stockIng] = "STOCK_ITEM"
		}

		for ingID, qty := range perIngredient {
			if !qty.IsPositive() {
				continue
			}
			source := sources[ingID]
			if source == "" {
				source = "RECIPE"
			}
			lines = append(lines, stockLine{it.id, ingID, source, qty})
		}
	}
	return lines, unknown, nil
}

// ApplyStock mengurangi stok dalam satu transaksi dengan lock urut
// ingredient_id ASC (cegah deadlock), menulis movement SALE, dan menghitung HPP.
func (r *Repository) ApplyStock(ctx context.Context, tx pgx.Tx, outletID, orderID, userID string, lines []stockLine, allowNegative bool) error {
	if len(lines) == 0 {
		return nil
	}

	perIngredient := map[string]money.Decimal{}
	for _, l := range lines {
		perIngredient[l.IngredientID] = perIngredient[l.IngredientID].Add(l.Quantity)
	}
	ingredients := make([]string, 0, len(perIngredient))
	for id := range perIngredient {
		ingredients = append(ingredients, id)
	}
	sort.Strings(ingredients)

	avgCost := map[string]money.Decimal{}
	for _, ingID := range ingredients {
		required := perIngredient[ingID]

		if _, err := tx.Exec(ctx,
			`INSERT INTO inventory (outlet_id, ingredient_id) VALUES ($1, $2)
			 ON CONFLICT (outlet_id, ingredient_id) DO NOTHING`, outletID, ingID); err != nil {
			return fmt.Errorf("applyStock: init inventory: %w", err)
		}

		var onHandStr, avgStr string
		const lock = `SELECT quantity_on_hand::text, avg_unit_cost::text
		              FROM inventory WHERE outlet_id = $1 AND ingredient_id = $2 FOR UPDATE`
		if err := tx.QueryRow(ctx, lock, outletID, ingID).Scan(&onHandStr, &avgStr); err != nil {
			return fmt.Errorf("applyStock: lock inventory: %w", err)
		}
		onHand, err := money.Parse(onHandStr)
		if err != nil {
			return err
		}
		avg, err := money.Parse(avgStr)
		if err != nil {
			return err
		}
		avgCost[ingID] = avg

		balance := onHand.Sub(required)
		if balance.IsNegative() && !allowNegative {
			return insufficientStock()
		}
		totalCost := money.RoundHalfUp(required.Mul(avg), 2)

		const movement = `
			INSERT INTO stock_movements (outlet_id, ingredient_id, movement_type, quantity_delta,
			                             unit_cost, total_cost, balance_after, avg_cost_after,
			                             reference_type, reference_id, is_shortage, created_by)
			VALUES ($1, $2, 'SALE', $3, $4, $5, $6, $4, 'ORDER', $7, $8, $9)`
		if _, err := tx.Exec(ctx, movement, outletID, ingID, required.Neg().StringFixed(4),
			avg.StringFixed(6), money.Format(totalCost), balance.StringFixed(4),
			orderID, balance.IsNegative(), userID); err != nil {
			return fmt.Errorf("applyStock: movement: %w", err)
		}

		const update = `
			UPDATE inventory SET quantity_on_hand = $3, last_movement_at = now(),
			       version = version + 1, updated_at = now()
			WHERE outlet_id = $1 AND ingredient_id = $2`
		if _, err := tx.Exec(ctx, update, outletID, ingID, balance.StringFixed(4)); err != nil {
			return fmt.Errorf("applyStock: update inventory: %w", err)
		}
	}

	itemCogs := map[string]money.Decimal{}
	for _, l := range lines {
		cost := money.RoundHalfUp(l.Quantity.Mul(avgCost[l.IngredientID]), 2)
		const insert = `
			INSERT INTO order_item_consumptions (order_item_id, ingredient_id, source,
			                                     quantity_base, unit_cost, total_cost)
			VALUES ($1, $2, $3, $4, $5, $6)`
		if _, err := tx.Exec(ctx, insert, l.OrderItemID, l.IngredientID, l.Source,
			l.Quantity.StringFixed(4), avgCost[l.IngredientID].StringFixed(6),
			money.Format(cost)); err != nil {
			return fmt.Errorf("applyStock: consumption: %w", err)
		}
		itemCogs[l.OrderItemID] = itemCogs[l.OrderItemID].Add(cost)
	}

	totalCogs := zeroDec
	for itemID, cogs := range itemCogs {
		const upd = `UPDATE order_items
			SET line_cogs = $2, unit_cogs = ROUND(($2::numeric / NULLIF(quantity, 0)), 4)
			WHERE id = $1`
		if _, err := tx.Exec(ctx, upd, itemID, money.Format(cogs)); err != nil {
			return fmt.Errorf("applyStock: item cogs: %w", err)
		}
		totalCogs = totalCogs.Add(cogs)
	}

	const updOrder = `UPDATE orders SET total_cogs = $2, updated_at = now() WHERE id = $1`
	if _, err := tx.Exec(ctx, updOrder, orderID, money.Format(totalCogs)); err != nil {
		return fmt.Errorf("applyStock: order cogs: %w", err)
	}
	return nil
}

// MarkCostUnknown menandai item yang HPP-nya tidak dapat dihitung.
func (r *Repository) MarkCostUnknown(ctx context.Context, tx pgx.Tx, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := tx.Exec(ctx, `UPDATE order_items SET cost_unknown = true WHERE id = ANY($1::uuid[])`, ids)
	if err != nil {
		return fmt.Errorf("markCostUnknown: %w", err)
	}
	return nil
}

func insufficientStock() error {
	return httpx.NewError(http.StatusUnprocessableEntity, "INSUFFICIENT_STOCK",
		"Stok bahan tidak cukup untuk mengonfirmasi pesanan.")
}
