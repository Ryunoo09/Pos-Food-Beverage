package waste

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("catatan waste tidak ditemukan")

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Begin(ctx context.Context) (pgx.Tx, error) {
	return r.pool.Begin(ctx)
}

// ingredientInfo mengambil identitas bahan beserta satuan dasarnya.
func (r *Repository) ingredientInfo(ctx context.Context, businessID, ingredientID string) (unitID int, unitCode, code, name string, found bool, err error) {
	const q = `
		SELECT i.base_unit_id, u.code, i.code, i.name
		FROM ingredients i JOIN units u ON u.id = i.base_unit_id
		WHERE i.id = $1 AND i.business_id = $2 AND i.is_active`
	err = r.pool.QueryRow(ctx, q, ingredientID, businessID).Scan(&unitID, &unitCode, &code, &name)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, "", "", "", false, nil
	}
	if err != nil {
		return 0, "", "", "", false, fmt.Errorf("ingredientInfo: %w", err)
	}
	return unitID, unitCode, code, name, true, nil
}

// UnitToBase menghitung faktor konversi satuan ke satuan dasar bahan.
func (r *Repository) UnitToBase(ctx context.Context, ingredientID string, unitID int) (string, bool, error) {
	const q = `
		SELECT COALESCE(iuc.base_quantity, u.to_base_factor)::text
		FROM units u
		LEFT JOIN ingredient_unit_conversions iuc
		       ON iuc.unit_id = u.id AND iuc.ingredient_id = $1
		WHERE u.id = $2`
	var factor string
	err := r.pool.QueryRow(ctx, q, ingredientID, unitID).Scan(&factor)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("unitToBase: %w", err)
	}
	return factor, true, nil
}

// variantInfo mengambil mode stok produk dan bahan stoknya (bila ada).
func (r *Repository) variantInfo(ctx context.Context, businessID, variantID string) (name, stockMode, stockIngredient string, found bool, err error) {
	const q = `
		SELECT p.name || CASE WHEN v.name IS NULL THEN '' ELSE ' - ' || v.name END,
		       p.stock_mode, COALESCE(p.stock_ingredient_id::text, '')
		FROM product_variants v JOIN products p ON p.id = v.product_id
		WHERE v.id = $1 AND p.business_id = $2`
	err = r.pool.QueryRow(ctx, q, variantID, businessID).Scan(&name, &stockMode, &stockIngredient)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", "", false, nil
	}
	if err != nil {
		return "", "", "", false, fmt.Errorf("variantInfo: %w", err)
	}
	return name, stockMode, stockIngredient, true, nil
}

type recipeComponent struct {
	IngredientID string
	Quantity     string
	WasteFactor  string
}

func (r *Repository) activeRecipe(ctx context.Context, variantID string) ([]recipeComponent, error) {
	const q = `
		SELECT ri.ingredient_id::text, ri.quantity::text, ri.waste_factor_percent::text
		FROM recipes rec JOIN recipe_items ri ON ri.recipe_id = rec.id
		WHERE rec.variant_id = $1 AND rec.status = 'ACTIVE'
		ORDER BY ri.ingredient_id`
	rows, err := r.pool.Query(ctx, q, variantID)
	if err != nil {
		return nil, fmt.Errorf("activeRecipe: %w", err)
	}
	defer rows.Close()

	items := []recipeComponent{}
	for rows.Next() {
		var c recipeComponent
		if err := rows.Scan(&c.IngredientID, &c.Quantity, &c.WasteFactor); err != nil {
			return nil, fmt.Errorf("activeRecipe: scan: %w", err)
		}
		items = append(items, c)
	}
	return items, rows.Err()
}

func (r *Repository) LockInventory(ctx context.Context, tx pgx.Tx, outletID, ingredientID string) (qty, avg string, err error) {
	if _, err := tx.Exec(ctx, `
		INSERT INTO inventory (outlet_id, ingredient_id) VALUES ($1, $2)
		ON CONFLICT (outlet_id, ingredient_id) DO NOTHING`, outletID, ingredientID); err != nil {
		return "", "", fmt.Errorf("lockInventory: ensure: %w", err)
	}
	if err := tx.QueryRow(ctx, `
		SELECT quantity_on_hand::text, avg_unit_cost::text
		FROM inventory WHERE outlet_id = $1 AND ingredient_id = $2 FOR UPDATE`,
		outletID, ingredientID).Scan(&qty, &avg); err != nil {
		return "", "", fmt.Errorf("lockInventory: lock: %w", err)
	}
	return qty, avg, nil
}

func (r *Repository) CreateRecord(ctx context.Context, tx pgx.Tx, outletID, wasteType, userID string,
	ingredientID, variantID *string, quantity string, unitID *int, costValue string, reasonCode, note, orderID *string) (string, error) {
	const q = `
		INSERT INTO waste_records (outlet_id, waste_type, ingredient_id, variant_id, quantity, unit_id,
		                           cost_value, reason_code, note, order_id, recorded_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id`

	var id string
	if err := tx.QueryRow(ctx, q, outletID, wasteType, ingredientID, variantID, quantity, unitID,
		costValue, reasonCode, note, orderID, userID).Scan(&id); err != nil {
		return "", fmt.Errorf("createWasteRecord: %w", err)
	}
	return id, nil
}

func (r *Repository) InsertWasteMovement(ctx context.Context, tx pgx.Tx, outletID, ingredientID, recordID string,
	delta, unitCost, totalCost, balance, avgAfter, reason string, shortage bool, userID string) (string, error) {
	const q = `
		INSERT INTO stock_movements (outlet_id, ingredient_id, movement_type, quantity_delta,
		                             unit_cost, total_cost, balance_after, avg_cost_after,
		                             reference_type, reference_id, is_shortage, reason, created_by)
		VALUES ($1, $2, 'WASTE', $3, $4, $5, $6, $7, 'WASTE', $8, $9, $10, $11)
		RETURNING id`

	var id string
	if err := tx.QueryRow(ctx, q, outletID, ingredientID, delta, unitCost, totalCost, balance, avgAfter,
		recordID, shortage, reason, userID).Scan(&id); err != nil {
		return "", fmt.Errorf("insertWasteMovement: %w", err)
	}
	return id, nil
}

func (r *Repository) InsertReversalMovement(ctx context.Context, tx pgx.Tx, outletID, ingredientID, recordID string,
	delta, unitCost, totalCost, balance, avgAfter, userID string) error {
	const q = `
		INSERT INTO stock_movements (outlet_id, ingredient_id, movement_type, quantity_delta,
		                             unit_cost, total_cost, balance_after, avg_cost_after,
		                             reference_type, reference_id, created_by)
		VALUES ($1, $2, 'WASTE_REVERSAL', $3, $4, $5, $6, $7, 'WASTE', $8, $9)`

	if _, err := tx.Exec(ctx, q, outletID, ingredientID, delta, unitCost, totalCost, balance, avgAfter,
		recordID, userID); err != nil {
		return fmt.Errorf("insertWasteReversal: %w", err)
	}
	return nil
}

func (r *Repository) UpdateInventory(ctx context.Context, tx pgx.Tx, outletID, ingredientID, balance string) error {
	if _, err := tx.Exec(ctx, `
		UPDATE inventory SET quantity_on_hand = $3, last_movement_at = now(),
		       version = version + 1, updated_at = now()
		WHERE outlet_id = $1 AND ingredient_id = $2`,
		outletID, ingredientID, balance); err != nil {
		return fmt.Errorf("updateInventoryAfterWaste: %w", err)
	}
	return nil
}

func (r *Repository) AllowNegativeStock(ctx context.Context, outletID string) (bool, error) {
	var ok bool
	err := r.pool.QueryRow(ctx,
		`SELECT allow_negative_stock FROM outlet_settings WHERE outlet_id = $1`, outletID).Scan(&ok)
	if errors.Is(err, pgx.ErrNoRows) {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("allowNegativeStock: %w", err)
	}
	return ok, nil
}

func (r *Repository) Record(ctx context.Context, outletID, id string) (*Record, error) {
	const q = `
		SELECT w.id, w.waste_type, w.ingredient_id::text, i.code, i.name, w.variant_id::text,
		       CASE WHEN w.variant_id IS NULL THEN NULL
		            ELSE p.name || CASE WHEN v.name IS NULL THEN '' ELSE ' - ' || v.name END END,
		       w.quantity::text, u.code, w.cost_value::text, w.reason_code, w.note,
		       w.order_id::text, w.status, w.recorded_at
		FROM waste_records w
		LEFT JOIN ingredients i ON i.id = w.ingredient_id
		LEFT JOIN product_variants v ON v.id = w.variant_id
		LEFT JOIN products p ON p.id = v.product_id
		LEFT JOIN units u ON u.id = w.unit_id
		WHERE w.id = $1 AND w.outlet_id = $2`

	o := &Record{}
	err := r.pool.QueryRow(ctx, q, id, outletID).Scan(&o.ID, &o.Type, &o.IngredientID, &o.IngredientCode,
		&o.IngredientName, &o.VariantID, &o.VariantName, &o.Quantity, &o.UnitCode, &o.CostValue,
		&o.ReasonCode, &o.Note, &o.OrderID, &o.Status, &o.RecordedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("wasteRecord: %w", err)
	}
	return o, nil
}

func (r *Repository) List(ctx context.Context, outletID string, f Filter) ([]Record, int, error) {
	const q = `
		SELECT w.id, w.waste_type, w.ingredient_id::text, i.code, i.name, w.variant_id::text,
		       CASE WHEN w.variant_id IS NULL THEN NULL
		            ELSE p.name || CASE WHEN v.name IS NULL THEN '' ELSE ' - ' || v.name END END,
		       w.quantity::text, u.code, w.cost_value::text, w.reason_code, w.note,
		       w.order_id::text, w.status, w.recorded_at, count(*) OVER ()::int
		FROM waste_records w
		LEFT JOIN ingredients i ON i.id = w.ingredient_id
		LEFT JOIN product_variants v ON v.id = w.variant_id
		LEFT JOIN products p ON p.id = v.product_id
		LEFT JOIN units u ON u.id = w.unit_id
		WHERE w.outlet_id = $1
		  AND ($2 = '' OR w.waste_type = $2)
		  AND ($3 = '' OR w.reason_code = $3)
		  AND ($4 = '' OR w.recorded_at >= $4::date)
		  AND ($5 = '' OR w.recorded_at < ($5::date + 1))
		ORDER BY w.recorded_at DESC
		LIMIT $6 OFFSET $7`

	rows, err := r.pool.Query(ctx, q, outletID, f.Type, f.ReasonCode, f.From, f.To, f.PerPage, (f.Page-1)*f.PerPage)
	if err != nil {
		return nil, 0, fmt.Errorf("listWasteRecords: %w", err)
	}
	defer rows.Close()

	items := []Record{}
	total := 0
	for rows.Next() {
		var o Record
		if err := rows.Scan(&o.ID, &o.Type, &o.IngredientID, &o.IngredientCode, &o.IngredientName,
			&o.VariantID, &o.VariantName, &o.Quantity, &o.UnitCode, &o.CostValue, &o.ReasonCode,
			&o.Note, &o.OrderID, &o.Status, &o.RecordedAt, &total); err != nil {
			return nil, 0, fmt.Errorf("listWasteRecords: scan: %w", err)
		}
		items = append(items, o)
	}
	return items, total, rows.Err()
}

// LockRecord mengunci catatan waste dan mengembalikan statusnya.
func (r *Repository) LockRecord(ctx context.Context, tx pgx.Tx, outletID, id string) (string, bool, error) {
	var status string
	err := tx.QueryRow(ctx,
		`SELECT status FROM waste_records WHERE id = $1 AND outlet_id = $2 FOR UPDATE`,
		id, outletID).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("lockWasteRecord: %w", err)
	}
	return status, true, nil
}

func (r *Repository) SetReversed(ctx context.Context, tx pgx.Tx, outletID, id, userID string) error {
	if _, err := tx.Exec(ctx,
		`UPDATE waste_records SET status = 'REVERSED', approved_by = $3 WHERE id = $1 AND outlet_id = $2`,
		id, outletID, userID); err != nil {
		return fmt.Errorf("setWasteReversed: %w", err)
	}
	return nil
}

type originalMovement struct {
	IngredientID string
	Quantity     string
	UnitCost     string
}

func (r *Repository) OriginalMovements(ctx context.Context, tx pgx.Tx, recordID string) ([]originalMovement, error) {
	const q = `
		SELECT ingredient_id::text, quantity_delta::text, unit_cost::text
		FROM stock_movements
		WHERE reference_type = 'WASTE' AND reference_id = $1 AND movement_type = 'WASTE'
		ORDER BY ingredient_id`
	rows, err := tx.Query(ctx, q, recordID)
	if err != nil {
		return nil, fmt.Errorf("originalWasteMovements: %w", err)
	}
	defer rows.Close()

	items := []originalMovement{}
	for rows.Next() {
		var m originalMovement
		if err := rows.Scan(&m.IngredientID, &m.Quantity, &m.UnitCost); err != nil {
			return nil, fmt.Errorf("originalWasteMovements: scan: %w", err)
		}
		items = append(items, m)
	}
	return items, rows.Err()
}
