package inventory

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ryunoo09/pos-food-beverage/backend/internal/pkg/money"
)

var ErrNotFound = errors.New("data inventaris tidak ditemukan")

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Units(ctx context.Context) ([]Unit, error) {
	const q = `
		SELECT id, code, name, dimension, to_base_factor::text, is_base
		FROM units ORDER BY dimension, to_base_factor, id`

	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("units: %w", err)
	}
	defer rows.Close()

	items := []Unit{}
	for rows.Next() {
		var u Unit
		if err := rows.Scan(&u.ID, &u.Code, &u.Name, &u.Dimension, &u.ToBaseFactor, &u.IsBase); err != nil {
			return nil, fmt.Errorf("scan unit: %w", err)
		}
		items = append(items, u)
	}
	return items, rows.Err()
}

func (r *Repository) UnitExists(ctx context.Context, id int16) (bool, error) {
	var ok bool
	err := r.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM units WHERE id = $1)`, id).Scan(&ok)
	if err != nil {
		return false, fmt.Errorf("unitExists: %w", err)
	}
	return ok, nil
}

func (r *Repository) Categories(ctx context.Context, businessID string) ([]IngredientCategory, error) {
	const q = `
		SELECT id, name, is_active FROM ingredient_categories
		WHERE business_id = $1 ORDER BY name`

	rows, err := r.pool.Query(ctx, q, businessID)
	if err != nil {
		return nil, fmt.Errorf("ingredientCategories: %w", err)
	}
	defer rows.Close()

	items := []IngredientCategory{}
	for rows.Next() {
		var c IngredientCategory
		if err := rows.Scan(&c.ID, &c.Name, &c.IsActive); err != nil {
			return nil, fmt.Errorf("scan ingredient category: %w", err)
		}
		items = append(items, c)
	}
	return items, rows.Err()
}

func (r *Repository) CategoryExists(ctx context.Context, businessID, id string) (bool, error) {
	var ok bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM ingredient_categories WHERE id = $1 AND business_id = $2)`,
		id, businessID).Scan(&ok)
	if err != nil {
		return false, fmt.Errorf("ingredientCategoryExists: %w", err)
	}
	return ok, nil
}

func (r *Repository) CreateCategory(ctx context.Context, businessID, name string) (*IngredientCategory, error) {
	const q = `
		INSERT INTO ingredient_categories (business_id, name) VALUES ($1, $2)
		RETURNING id, name, is_active`

	c := &IngredientCategory{}
	if err := r.pool.QueryRow(ctx, q, businessID, name).Scan(&c.ID, &c.Name, &c.IsActive); err != nil {
		return nil, fmt.Errorf("createIngredientCategory: %w", err)
	}
	return c, nil
}

func (r *Repository) UpdateCategory(ctx context.Context, businessID, id, name string, active bool) (*IngredientCategory, error) {
	const q = `
		UPDATE ingredient_categories
		SET name = $3, is_active = $4, updated_at = now()
		WHERE id = $1 AND business_id = $2
		RETURNING id, name, is_active`

	c := &IngredientCategory{}
	err := r.pool.QueryRow(ctx, q, id, businessID, name, active).Scan(&c.ID, &c.Name, &c.IsActive)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("updateIngredientCategory: %w", err)
	}
	return c, nil
}

func (r *Repository) DeleteCategory(ctx context.Context, businessID, id string) error {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM ingredient_categories WHERE id = $1 AND business_id = $2`, id, businessID)
	if err != nil {
		return fmt.Errorf("deleteIngredientCategory: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) ListIngredients(ctx context.Context, businessID, outletID string, f IngredientFilter) ([]Ingredient, int, error) {
	const q = `
		SELECT i.id, i.code, i.name, i.category_id, c.name, i.base_unit_id, u.code,
		       i.default_purchase_unit_id, i.min_stock::text, i.is_active, i.is_stock_product,
		       COALESCE(inv.quantity_on_hand, 0)::text, COALESCE(inv.avg_unit_cost, 0)::text,
		       count(*) OVER ()::int
		FROM ingredients i
		JOIN units u ON u.id = i.base_unit_id
		LEFT JOIN ingredient_categories c ON c.id = i.category_id
		LEFT JOIN inventory inv ON inv.ingredient_id = i.id AND inv.outlet_id = $2
		WHERE i.business_id = $1
		  AND ($3 = '' OR i.name ILIKE '%' || $3 || '%' OR i.code ILIKE '%' || $3 || '%')
		  AND ($4::bool IS NULL OR i.is_active = $4)
		ORDER BY i.name
		LIMIT $5 OFFSET $6`

	rows, err := r.pool.Query(ctx, q, businessID, outletID, f.Query, f.Active,
		f.PerPage, (f.Page-1)*f.PerPage)
	if err != nil {
		return nil, 0, fmt.Errorf("listIngredients: %w", err)
	}
	defer rows.Close()

	items := []Ingredient{}
	total := 0
	for rows.Next() {
		var i Ingredient
		if err := rows.Scan(&i.ID, &i.Code, &i.Name, &i.CategoryID, &i.CategoryName,
			&i.BaseUnitID, &i.BaseUnitCode, &i.DefaultPurchaseUnit, &i.MinStock,
			&i.IsActive, &i.IsStockProduct, &i.QuantityOnHand, &i.AvgUnitCost, &total); err != nil {
			return nil, 0, fmt.Errorf("scan ingredient: %w", err)
		}
		items = append(items, i)
	}
	return items, total, rows.Err()
}

func (r *Repository) Ingredient(ctx context.Context, businessID, outletID, id string) (*Ingredient, error) {
	const q = `
		SELECT i.id, i.code, i.name, i.category_id, c.name, i.base_unit_id, u.code,
		       i.default_purchase_unit_id, i.min_stock::text, i.is_active, i.is_stock_product,
		       COALESCE(inv.quantity_on_hand, 0)::text, COALESCE(inv.avg_unit_cost, 0)::text
		FROM ingredients i
		JOIN units u ON u.id = i.base_unit_id
		LEFT JOIN ingredient_categories c ON c.id = i.category_id
		LEFT JOIN inventory inv ON inv.ingredient_id = i.id AND inv.outlet_id = $3
		WHERE i.id = $1 AND i.business_id = $2`

	i := &Ingredient{}
	err := r.pool.QueryRow(ctx, q, id, businessID, outletID).Scan(&i.ID, &i.Code, &i.Name,
		&i.CategoryID, &i.CategoryName, &i.BaseUnitID, &i.BaseUnitCode, &i.DefaultPurchaseUnit,
		&i.MinStock, &i.IsActive, &i.IsStockProduct, &i.QuantityOnHand, &i.AvgUnitCost)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("ingredient: %w", err)
	}
	return i, nil
}

func (r *Repository) CreateIngredient(ctx context.Context, businessID, outletID string, in IngredientInput) (*Ingredient, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("createIngredient: begin: %w", err)
	}
	defer tx.Rollback(ctx)

	const q = `
		INSERT INTO ingredients (business_id, code, name, category_id, base_unit_id,
		                         default_purchase_unit_id, min_stock, is_stock_product)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id`

	var id string
	if err := tx.QueryRow(ctx, q, businessID, in.Code, in.Name, in.CategoryID,
		in.BaseUnitID, in.DefaultPurchaseUnit, in.MinStock, in.IsStockProduct).Scan(&id); err != nil {
		return nil, fmt.Errorf("createIngredient: %w", err)
	}

	if outletID != "" {
		const inv = `INSERT INTO inventory (outlet_id, ingredient_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`
		if _, err := tx.Exec(ctx, inv, outletID, id); err != nil {
			return nil, fmt.Errorf("createIngredient: inventory: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("createIngredient: commit: %w", err)
	}
	return r.Ingredient(ctx, businessID, outletID, id)
}

func (r *Repository) UpdateIngredient(ctx context.Context, businessID, outletID, id string, in IngredientInput) (*Ingredient, error) {
	const q = `
		UPDATE ingredients
		SET code = $3, name = $4, category_id = $5, base_unit_id = $6,
		    default_purchase_unit_id = $7, min_stock = $8, is_stock_product = $9,
		    updated_at = now()
		WHERE id = $1 AND business_id = $2
		RETURNING id`

	var updated string
	err := r.pool.QueryRow(ctx, q, id, businessID, in.Code, in.Name, in.CategoryID,
		in.BaseUnitID, in.DefaultPurchaseUnit, in.MinStock, in.IsStockProduct).Scan(&updated)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("updateIngredient: %w", err)
	}
	return r.Ingredient(ctx, businessID, outletID, id)
}

func (r *Repository) HasMovements(ctx context.Context, outletID, ingredientID string) (bool, error) {
	var ok bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM stock_movements WHERE outlet_id = $1 AND ingredient_id = $2)`,
		outletID, ingredientID).Scan(&ok)
	if err != nil {
		return false, fmt.Errorf("hasMovements: %w", err)
	}
	return ok, nil
}

func (r *Repository) SetIngredientActive(ctx context.Context, businessID, id string, active bool) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE ingredients SET is_active = $3, updated_at = now() WHERE id = $1 AND business_id = $2`,
		id, businessID, active)
	if err != nil {
		return fmt.Errorf("setIngredientActive: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) Movements(ctx context.Context, outletID, ingredientID string, limit int) ([]StockMovement, error) {
	const q = `
		SELECT id, movement_type, quantity_delta::text, unit_cost::text, total_cost::text,
		       balance_after::text, avg_cost_after::text, reference_type, reference_id,
		       is_shortage, reason, created_at
		FROM stock_movements
		WHERE outlet_id = $1 AND ingredient_id = $2
		ORDER BY created_at DESC
		LIMIT $3`

	rows, err := r.pool.Query(ctx, q, outletID, ingredientID, limit)
	if err != nil {
		return nil, fmt.Errorf("movements: %w", err)
	}
	defer rows.Close()

	items := []StockMovement{}
	for rows.Next() {
		var m StockMovement
		if err := rows.Scan(&m.ID, &m.MovementType, &m.QuantityDelta, &m.UnitCost,
			&m.TotalCost, &m.BalanceAfter, &m.AvgCostAfter, &m.ReferenceType,
			&m.ReferenceID, &m.IsShortage, &m.Reason, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan movement: %w", err)
		}
		items = append(items, m)
	}
	return items, rows.Err()
}

// StockLevels mengambil saldo stok per bahan untuk daftar inventaris.
func (r *Repository) StockLevels(ctx context.Context, businessID, outletID string, f StockFilter) ([]StockLevel, int, error) {
	const q = `
		SELECT i.id, i.code, i.name, u.code,
		       COALESCE(inv.quantity_on_hand, 0)::text, i.min_stock::text,
		       COALESCE(inv.avg_unit_cost, 0)::text,
		       ROUND(COALESCE(inv.quantity_on_hand, 0) * COALESCE(inv.avg_unit_cost, 0), 2)::text,
		       (COALESCE(inv.quantity_on_hand, 0) <= i.min_stock) AS is_low,
		       count(*) OVER ()::int
		FROM ingredients i
		JOIN units u ON u.id = i.base_unit_id
		LEFT JOIN inventory inv ON inv.ingredient_id = i.id AND inv.outlet_id = $2
		WHERE i.business_id = $1
		  AND ($3 = '' OR i.name ILIKE '%' || $3 || '%' OR i.code ILIKE '%' || $3 || '%')
		  AND ($4 = false OR COALESCE(inv.quantity_on_hand, 0) <= i.min_stock)
		ORDER BY is_low DESC, i.name
		LIMIT $5 OFFSET $6`

	rows, err := r.pool.Query(ctx, q, businessID, outletID, f.Query, f.LowOnly,
		f.PerPage, (f.Page-1)*f.PerPage)
	if err != nil {
		return nil, 0, fmt.Errorf("stockLevels: %w", err)
	}
	defer rows.Close()

	items := []StockLevel{}
	total := 0
	for rows.Next() {
		var s StockLevel
		if err := rows.Scan(&s.IngredientID, &s.Code, &s.Name, &s.BaseUnitCode,
			&s.QuantityOnHand, &s.MinStock, &s.AvgUnitCost, &s.StockValue, &s.IsLow, &total); err != nil {
			return nil, 0, fmt.Errorf("stockLevels: scan: %w", err)
		}
		items = append(items, s)
	}
	return items, total, rows.Err()
}

// AdjustSettings membaca ambang persetujuan dan izin self-approval penyesuaian stok.
func (r *Repository) AdjustSettings(ctx context.Context, outletID string) (threshold string, allowSelf bool, err error) {
	const q = `SELECT adjustment_approval_threshold::text, allow_self_approval_adjustment
	           FROM outlet_settings WHERE outlet_id = $1`
	err = r.pool.QueryRow(ctx, q, outletID).Scan(&threshold, &allowSelf)
	if errors.Is(err, pgx.ErrNoRows) {
		return "500000.00", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("adjustSettings: %w", err)
	}
	return threshold, allowSelf, nil
}

// CurrentStock membaca saldo dan biaya rata-rata terkini satu bahan.
func (r *Repository) CurrentStock(ctx context.Context, outletID, ingredientID string) (qty, avg string, found bool, err error) {
	const q = `SELECT quantity_on_hand::text, avg_unit_cost::text
	           FROM inventory WHERE outlet_id = $1 AND ingredient_id = $2`
	err = r.pool.QueryRow(ctx, q, outletID, ingredientID).Scan(&qty, &avg)
	if errors.Is(err, pgx.ErrNoRows) {
		return "0", "0", true, nil
	}
	if err != nil {
		return "", "", false, fmt.Errorf("currentStock: %w", err)
	}
	return qty, avg, true, nil
}

// IngredientExists memastikan bahan milik bisnis yang benar.
func (r *Repository) IngredientExists(ctx context.Context, businessID, ingredientID string) (bool, error) {
	var ok bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM ingredients WHERE id = $1 AND business_id = $2)`,
		ingredientID, businessID).Scan(&ok)
	if err != nil {
		return false, fmt.Errorf("ingredientExists: %w", err)
	}
	return ok, nil
}

// Adjust menulis movement ADJUSTMENT dan memperbarui saldo dalam satu transaksi.
func (r *Repository) Adjust(ctx context.Context, outletID, ingredientID, userID, deltaStr, reason string, note *string) (string, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("adjust: begin: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
		INSERT INTO inventory (outlet_id, ingredient_id) VALUES ($1, $2)
		ON CONFLICT (outlet_id, ingredient_id) DO NOTHING`, outletID, ingredientID); err != nil {
		return "", fmt.Errorf("adjust: ensure inventory: %w", err)
	}

	var qtyStr, avgStr string
	if err := tx.QueryRow(ctx, `
		SELECT quantity_on_hand::text, avg_unit_cost::text
		FROM inventory WHERE outlet_id = $1 AND ingredient_id = $2 FOR UPDATE`,
		outletID, ingredientID).Scan(&qtyStr, &avgStr); err != nil {
		return "", fmt.Errorf("adjust: lock: %w", err)
	}

	delta, err := money.Parse(deltaStr)
	if err != nil {
		return "", err
	}
	qty, err := money.Parse(qtyStr)
	if err != nil {
		return "", err
	}
	avg, err := money.Parse(avgStr)
	if err != nil {
		return "", err
	}

	balance := qty.Add(delta)
	totalCost := money.RoundHalfUp(delta.Mul(avg), 2)

	reasonText := reason
	if note != nil && strings.TrimSpace(*note) != "" {
		reasonText = reason + ": " + strings.TrimSpace(*note)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO stock_movements (outlet_id, ingredient_id, movement_type, quantity_delta,
		                             unit_cost, total_cost, balance_after, avg_cost_after,
		                             reference_type, reason, created_by)
		VALUES ($1, $2, 'ADJUSTMENT', $3, $4, $5, $6, $4, 'MANUAL', $7, $8)`,
		outletID, ingredientID, delta.StringFixed(4), avg.StringFixed(6),
		money.Format(totalCost), balance.StringFixed(4), reasonText, userID); err != nil {
		return "", fmt.Errorf("adjust: movement: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		UPDATE inventory SET quantity_on_hand = $3, last_movement_at = now(),
		       version = version + 1, updated_at = now()
		WHERE outlet_id = $1 AND ingredient_id = $2`,
		outletID, ingredientID, balance.StringFixed(4)); err != nil {
		return "", fmt.Errorf("adjust: update: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("adjust: commit: %w", err)
	}
	return balance.StringFixed(4), nil
}
