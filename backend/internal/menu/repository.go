package menu

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound = errors.New("data menu tidak ditemukan")
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Begin(ctx context.Context) (pgx.Tx, error) {
	return r.pool.Begin(ctx)
}

func (r *Repository) ListCategories(ctx context.Context, businessID string) ([]Category, error) {
	const q = `
		SELECT id, name, sort_order, is_active, created_at
		FROM categories
		WHERE business_id = $1
		ORDER BY sort_order, name`

	rows, err := r.pool.Query(ctx, q, businessID)
	if err != nil {
		return nil, fmt.Errorf("listCategories: %w", err)
	}
	defer rows.Close()

	items := []Category{}
	for rows.Next() {
		var c Category
		if err := rows.Scan(&c.ID, &c.Name, &c.SortOrder, &c.IsActive, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan category: %w", err)
		}
		items = append(items, c)
	}
	return items, rows.Err()
}

func (r *Repository) Category(ctx context.Context, businessID, id string) (*Category, error) {
	const q = `SELECT id, name, sort_order, is_active, created_at FROM categories WHERE id = $1 AND business_id = $2`

	c := &Category{}
	err := r.pool.QueryRow(ctx, q, id, businessID).Scan(&c.ID, &c.Name, &c.SortOrder, &c.IsActive, &c.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("category: %w", err)
	}
	return c, nil
}

func (r *Repository) CreateCategory(ctx context.Context, businessID string, in CategoryInput) (*Category, error) {
	const q = `
		INSERT INTO categories (business_id, name, sort_order)
		VALUES ($1, $2, $3)
		RETURNING id, name, sort_order, is_active, created_at`

	c := &Category{}
	if err := r.pool.QueryRow(ctx, q, businessID, in.Name, in.SortOrder).Scan(
		&c.ID, &c.Name, &c.SortOrder, &c.IsActive, &c.CreatedAt,
	); err != nil {
		return nil, fmt.Errorf("createCategory: %w", err)
	}
	return c, nil
}

func (r *Repository) UpdateCategory(ctx context.Context, businessID, id string, in CategoryInput, isActive bool) (*Category, error) {
	const q = `
		UPDATE categories
		SET name = $3, sort_order = $4, is_active = $5, updated_at = now()
		WHERE id = $1 AND business_id = $2
		RETURNING id, name, sort_order, is_active, created_at`

	c := &Category{}
	err := r.pool.QueryRow(ctx, q, id, businessID, in.Name, in.SortOrder, isActive).Scan(
		&c.ID, &c.Name, &c.SortOrder, &c.IsActive, &c.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("updateCategory: %w", err)
	}
	return c, nil
}

func (r *Repository) DeleteCategory(ctx context.Context, businessID, id string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM categories WHERE id = $1 AND business_id = $2`, id, businessID)
	if err != nil {
		return fmt.Errorf("deleteCategory: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
