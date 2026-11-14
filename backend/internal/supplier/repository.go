package supplier

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("supplier tidak ditemukan")

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) List(ctx context.Context, businessID, query string, active *bool) ([]Supplier, error) {
	const q = `
		SELECT id, name, contact_person, phone, email, address, note, is_active
		FROM suppliers
		WHERE business_id = $1
		  AND ($2 = '' OR name ILIKE '%' || $2 || '%')
		  AND ($3::bool IS NULL OR is_active = $3)
		ORDER BY name`

	rows, err := r.pool.Query(ctx, q, businessID, query, active)
	if err != nil {
		return nil, fmt.Errorf("listSuppliers: %w", err)
	}
	defer rows.Close()

	items := []Supplier{}
	for rows.Next() {
		var s Supplier
		if err := rows.Scan(&s.ID, &s.Name, &s.ContactPerson, &s.Phone, &s.Email,
			&s.Address, &s.Note, &s.IsActive); err != nil {
			return nil, fmt.Errorf("scan supplier: %w", err)
		}
		items = append(items, s)
	}
	return items, rows.Err()
}

func (r *Repository) Supplier(ctx context.Context, businessID, id string) (*Supplier, error) {
	const q = `
		SELECT id, name, contact_person, phone, email, address, note, is_active
		FROM suppliers WHERE id = $1 AND business_id = $2`

	s := &Supplier{}
	err := r.pool.QueryRow(ctx, q, id, businessID).Scan(&s.ID, &s.Name, &s.ContactPerson,
		&s.Phone, &s.Email, &s.Address, &s.Note, &s.IsActive)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("supplier: %w", err)
	}

	items, err := r.ingredients(ctx, id)
	if err != nil {
		return nil, err
	}
	s.Ingredients = items
	return s, nil
}

func (r *Repository) ingredients(ctx context.Context, supplierID string) ([]SupplierIngredient, error) {
	const q = `
		SELECT si.ingredient_id, i.name, si.purchase_unit_id, si.last_price::text,
		       si.lead_time_days, si.is_preferred
		FROM supplier_ingredients si
		JOIN ingredients i ON i.id = si.ingredient_id
		WHERE si.supplier_id = $1
		ORDER BY i.name`

	rows, err := r.pool.Query(ctx, q, supplierID)
	if err != nil {
		return nil, fmt.Errorf("supplierIngredients: %w", err)
	}
	defer rows.Close()

	items := []SupplierIngredient{}
	for rows.Next() {
		var si SupplierIngredient
		if err := rows.Scan(&si.IngredientID, &si.IngredientName, &si.PurchaseUnitID,
			&si.LastPrice, &si.LeadTimeDays, &si.IsPreferred); err != nil {
			return nil, fmt.Errorf("scan supplier ingredient: %w", err)
		}
		items = append(items, si)
	}
	return items, rows.Err()
}

func (r *Repository) Create(ctx context.Context, businessID string, in SupplierInput) (*Supplier, error) {
	const q = `
		INSERT INTO suppliers (business_id, name, contact_person, phone, email, address, note)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id`

	var id string
	if err := r.pool.QueryRow(ctx, q, businessID, in.Name, in.ContactPerson, in.Phone,
		in.Email, in.Address, in.Note).Scan(&id); err != nil {
		return nil, fmt.Errorf("createSupplier: %w", err)
	}
	return r.Supplier(ctx, businessID, id)
}

func (r *Repository) Update(ctx context.Context, businessID, id string, in SupplierInput) (*Supplier, error) {
	const q = `
		UPDATE suppliers
		SET name = $3, contact_person = $4, phone = $5, email = $6, address = $7,
		    note = $8, updated_at = now()
		WHERE id = $1 AND business_id = $2
		RETURNING id`

	var updated string
	err := r.pool.QueryRow(ctx, q, id, businessID, in.Name, in.ContactPerson, in.Phone,
		in.Email, in.Address, in.Note).Scan(&updated)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("updateSupplier: %w", err)
	}
	return r.Supplier(ctx, businessID, id)
}

func (r *Repository) SetActive(ctx context.Context, businessID, id string, active bool) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE suppliers SET is_active = $3, updated_at = now() WHERE id = $1 AND business_id = $2`,
		id, businessID, active)
	if err != nil {
		return fmt.Errorf("setSupplierActive: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) SetIngredients(ctx context.Context, businessID, id string, items []SupplierIngredientInput) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("setSupplierIngredients: begin: %w", err)
	}
	defer tx.Rollback(ctx)

	var exists bool
	if err := tx.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM suppliers WHERE id = $1 AND business_id = $2)`,
		id, businessID).Scan(&exists); err != nil {
		return fmt.Errorf("setSupplierIngredients: supplier: %w", err)
	}
	if !exists {
		return ErrNotFound
	}

	var valid int
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.IngredientID)
	}
	if err := tx.QueryRow(ctx,
		`SELECT count(*)::int FROM ingredients WHERE business_id = $1 AND id = ANY($2::uuid[])`,
		businessID, ids).Scan(&valid); err != nil {
		return fmt.Errorf("setSupplierIngredients: ingredients: %w", err)
	}
	if valid != len(ids) {
		return ErrNotFound
	}

	if _, err := tx.Exec(ctx, `DELETE FROM supplier_ingredients WHERE supplier_id = $1`, id); err != nil {
		return fmt.Errorf("setSupplierIngredients: clear: %w", err)
	}
	for _, item := range items {
		const insert = `
			INSERT INTO supplier_ingredients
			    (supplier_id, ingredient_id, purchase_unit_id, last_price, lead_time_days, is_preferred)
			VALUES ($1, $2, $3, $4, $5, $6)`
		if _, err := tx.Exec(ctx, insert, id, item.IngredientID, item.PurchaseUnitID,
			item.LastPrice, item.LeadTimeDays, item.IsPreferred); err != nil {
			return fmt.Errorf("setSupplierIngredients: insert: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("setSupplierIngredients: commit: %w", err)
	}
	return nil
}

// PurchaseHistory mengambil riwayat PO per supplier pada satu outlet.
func (r *Repository) PurchaseHistory(ctx context.Context, outletID, supplierID string) ([]PurchaseHistoryRow, error) {
	const q = `
		SELECT po.id, po.code, po.status, po.order_date::text, po.total_amount::text,
		       (SELECT count(*)::int FROM purchase_order_items poi WHERE poi.purchase_order_id = po.id)
		FROM purchase_orders po
		WHERE po.outlet_id = $1 AND po.supplier_id = $2
		ORDER BY po.order_date DESC, po.created_at DESC
		LIMIT 100`

	rows, err := r.pool.Query(ctx, q, outletID, supplierID)
	if err != nil {
		return nil, fmt.Errorf("purchaseHistory: %w", err)
	}
	defer rows.Close()

	items := []PurchaseHistoryRow{}
	for rows.Next() {
		var h PurchaseHistoryRow
		if err := rows.Scan(&h.ID, &h.Code, &h.Status, &h.OrderDate, &h.TotalAmount, &h.ItemCount); err != nil {
			return nil, fmt.Errorf("purchaseHistory: scan: %w", err)
		}
		items = append(items, h)
	}
	return items, rows.Err()
}
