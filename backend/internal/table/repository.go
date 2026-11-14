package table

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("meja tidak ditemukan")

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

const columns = `id, area, number, capacity, status, is_active, sort_order, created_at`

func scan(row pgx.Row) (*Table, error) {
	t := &Table{}
	if err := row.Scan(&t.ID, &t.Area, &t.Number, &t.Capacity, &t.Status,
		&t.IsActive, &t.SortOrder, &t.CreatedAt); err != nil {
		return nil, err
	}
	return t, nil
}

func (r *Repository) List(ctx context.Context, outletID string, f Filter) ([]Table, error) {
	const q = `
		SELECT ` + columns + `
		FROM tables
		WHERE outlet_id = $1
		  AND ($2 = '' OR area = $2)
		  AND ($3::boolean IS NULL OR is_active = $3)
		ORDER BY area NULLS FIRST, sort_order, number`

	rows, err := r.pool.Query(ctx, q, outletID, f.Area, f.IsActive)
	if err != nil {
		return nil, fmt.Errorf("listTables: %w", err)
	}
	defer rows.Close()

	items := []Table{}
	for rows.Next() {
		t, err := scan(rows)
		if err != nil {
			return nil, fmt.Errorf("listTables: scan: %w", err)
		}
		items = append(items, *t)
	}
	return items, rows.Err()
}

func (r *Repository) Table(ctx context.Context, outletID, id string) (*Table, error) {
	const q = `SELECT ` + columns + ` FROM tables WHERE id = $1 AND outlet_id = $2`
	t, err := scan(r.pool.QueryRow(ctx, q, id, outletID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("table: %w", err)
	}
	return t, nil
}

func (r *Repository) Create(ctx context.Context, outletID string, in Input) (*Table, error) {
	const q = `
		INSERT INTO tables (outlet_id, area, number, capacity, sort_order, is_active)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING ` + columns

	active := true
	if in.IsActive != nil {
		active = *in.IsActive
	}
	t, err := scan(r.pool.QueryRow(ctx, q, outletID, in.Area, in.Number, in.Capacity, in.SortOrder, active))
	if err != nil {
		return nil, err
	}
	return t, nil
}

func (r *Repository) Update(ctx context.Context, outletID, id string, in Input) (*Table, error) {
	const q = `
		UPDATE tables
		SET area = $3, number = $4, capacity = $5, sort_order = $6,
		    is_active = COALESCE($7::boolean, is_active), updated_at = now()
		WHERE id = $1 AND outlet_id = $2
		RETURNING ` + columns

	t, err := scan(r.pool.QueryRow(ctx, q, id, outletID, in.Area, in.Number, in.Capacity, in.SortOrder, in.IsActive))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return t, nil
}

// SetStatus mengubah status operasional meja.
func (r *Repository) SetStatus(ctx context.Context, outletID, id, status string) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE tables SET status = $3, updated_at = now() WHERE id = $1 AND outlet_id = $2`,
		id, outletID, status)
	if err != nil {
		return fmt.Errorf("setTableStatus: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// HasActiveOrder melaporkan apakah meja masih dipakai pesanan aktif.
func (r *Repository) HasActiveOrder(ctx context.Context, outletID, id string) (bool, error) {
	var busy bool
	err := r.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM orders
			WHERE table_id = $1 AND outlet_id = $2 AND status NOT IN ('COMPLETED', 'CANCELLED')
		)`, id, outletID).Scan(&busy)
	if err != nil {
		return false, fmt.Errorf("hasActiveOrder: %w", err)
	}
	return busy, nil
}

// Deactivate menonaktifkan meja (soft delete); tidak ada hard delete data master.
func (r *Repository) Deactivate(ctx context.Context, outletID, id string) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE tables SET is_active = false, updated_at = now() WHERE id = $1 AND outlet_id = $2`,
		id, outletID)
	if err != nil {
		return fmt.Errorf("deactivateTable: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
