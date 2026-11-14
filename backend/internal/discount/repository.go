package discount

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("diskon tidak ditemukan")

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) List(ctx context.Context, outletID string, active *bool) ([]Discount, error) {
	const q = `
		SELECT id, code, name, discount_type, value::text, scope, min_spend::text,
		       max_discount::text, starts_at, ends_at, days_of_week, requires_approval, is_active
		FROM discounts
		WHERE outlet_id = $1 AND ($2::bool IS NULL OR is_active = $2)
		ORDER BY is_active DESC, name`

	rows, err := r.pool.Query(ctx, q, outletID, active)
	if err != nil {
		return nil, fmt.Errorf("listDiscounts: %w", err)
	}
	defer rows.Close()

	items := []Discount{}
	for rows.Next() {
		var d Discount
		if err := rows.Scan(&d.ID, &d.Code, &d.Name, &d.DiscountType, &d.Value, &d.Scope,
			&d.MinSpend, &d.MaxDiscount, &d.StartsAt, &d.EndsAt, &d.DaysOfWeek,
			&d.RequiresApproval, &d.IsActive); err != nil {
			return nil, fmt.Errorf("scan discount: %w", err)
		}
		items = append(items, d)
	}
	return items, rows.Err()
}

func (r *Repository) Discount(ctx context.Context, outletID, id string) (*Discount, error) {
	const q = `
		SELECT id, code, name, discount_type, value::text, scope, min_spend::text,
		       max_discount::text, starts_at, ends_at, days_of_week, requires_approval, is_active
		FROM discounts WHERE id = $1 AND outlet_id = $2`

	d := &Discount{}
	err := r.pool.QueryRow(ctx, q, id, outletID).Scan(&d.ID, &d.Code, &d.Name,
		&d.DiscountType, &d.Value, &d.Scope, &d.MinSpend, &d.MaxDiscount, &d.StartsAt,
		&d.EndsAt, &d.DaysOfWeek, &d.RequiresApproval, &d.IsActive)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("discount: %w", err)
	}
	return d, nil
}

func (r *Repository) Create(ctx context.Context, outletID, userID string, in Input) (*Discount, error) {
	const q = `
		INSERT INTO discounts (outlet_id, code, name, discount_type, value, scope, min_spend,
		                       max_discount, starts_at, ends_at, days_of_week, requires_approval, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9::timestamptz, $10::timestamptz, $11, $12, $13)
		RETURNING id`

	var id string
	if err := r.pool.QueryRow(ctx, q, outletID, in.Code, in.Name, in.DiscountType, in.Value,
		in.Scope, in.MinSpend, in.MaxDiscount, in.StartsAt, in.EndsAt, in.DaysOfWeek,
		in.RequiresApproval, userID).Scan(&id); err != nil {
		return nil, fmt.Errorf("createDiscount: %w", err)
	}
	return r.Discount(ctx, outletID, id)
}

func (r *Repository) Update(ctx context.Context, outletID, id string, in Input) (*Discount, error) {
	const q = `
		UPDATE discounts
		SET code = $3, name = $4, discount_type = $5, value = $6, scope = $7, min_spend = $8,
		    max_discount = $9, starts_at = $10::timestamptz, ends_at = $11::timestamptz,
		    days_of_week = $12, requires_approval = $13, updated_at = now()
		WHERE id = $1 AND outlet_id = $2
		RETURNING id`

	var updated string
	err := r.pool.QueryRow(ctx, q, id, outletID, in.Code, in.Name, in.DiscountType,
		in.Value, in.Scope, in.MinSpend, in.MaxDiscount, in.StartsAt, in.EndsAt,
		in.DaysOfWeek, in.RequiresApproval).Scan(&updated)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("updateDiscount: %w", err)
	}
	return r.Discount(ctx, outletID, id)
}

func (r *Repository) SetActive(ctx context.Context, outletID, id string, active bool) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE discounts SET is_active = $3, updated_at = now() WHERE id = $1 AND outlet_id = $2`,
		id, outletID, active)
	if err != nil {
		return fmt.Errorf("setDiscountActive: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
