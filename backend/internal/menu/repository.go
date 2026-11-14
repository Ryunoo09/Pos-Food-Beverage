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

func (r *Repository) ListProducts(ctx context.Context, businessID string, f ProductFilter) ([]Product, int, error) {
	const q = `
		SELECT p.id, p.category_id, c.name, p.sku, p.name, p.description,
		       p.product_type, p.stock_mode, p.stock_ingredient_id, p.kitchen_station,
		       p.allow_zero_price, p.is_active, p.is_available, p.sort_order,
		       COALESCE((SELECT v.price FROM product_variants v
		                 WHERE v.product_id = p.id AND v.is_default LIMIT 1), 0)::text,
		       count(*) OVER ()::int
		FROM products p
		JOIN categories c ON c.id = p.category_id
		WHERE p.business_id = $1
		  AND ($2 = '' OR p.name ILIKE '%' || $2 || '%' OR p.sku ILIKE '%' || $2 || '%')
		  AND (NULLIF($3, '')::uuid IS NULL OR p.category_id = NULLIF($3, '')::uuid)
		  AND ($4::bool IS NULL OR p.is_active = $4)
		  AND ($5::bool IS NULL OR p.is_available = $5)
		ORDER BY p.sort_order, p.name
		LIMIT $6 OFFSET $7`

	rows, err := r.pool.Query(ctx, q, businessID, f.Query, f.CategoryID,
		f.IsActive, f.Available, f.PerPage, (f.Page-1)*f.PerPage)
	if err != nil {
		return nil, 0, fmt.Errorf("listProducts: %w", err)
	}
	defer rows.Close()

	items := []Product{}
	total := 0
	for rows.Next() {
		var p Product
		if err := rows.Scan(&p.ID, &p.CategoryID, &p.CategoryName, &p.SKU, &p.Name, &p.Description,
			&p.ProductType, &p.StockMode, &p.StockIngredient, &p.KitchenStation,
			&p.AllowZeroPrice, &p.IsActive, &p.IsAvailable, &p.SortOrder, &p.Price, &total); err != nil {
			return nil, 0, fmt.Errorf("scan product: %w", err)
		}
		items = append(items, p)
	}
	return items, total, rows.Err()
}

func (r *Repository) Product(ctx context.Context, businessID, id string) (*Product, error) {
	const q = `
		SELECT p.id, p.category_id, c.name, p.sku, p.name, p.description,
		       p.product_type, p.stock_mode, p.stock_ingredient_id, p.kitchen_station,
		       p.allow_zero_price, p.is_active, p.is_available, p.sort_order,
		       COALESCE((SELECT v.price FROM product_variants v
		                 WHERE v.product_id = p.id AND v.is_default LIMIT 1), 0)::text
		FROM products p
		JOIN categories c ON c.id = p.category_id
		WHERE p.id = $1 AND p.business_id = $2`

	p := &Product{}
	err := r.pool.QueryRow(ctx, q, id, businessID).Scan(&p.ID, &p.CategoryID, &p.CategoryName,
		&p.SKU, &p.Name, &p.Description, &p.ProductType, &p.StockMode, &p.StockIngredient,
		&p.KitchenStation, &p.AllowZeroPrice, &p.IsActive, &p.IsAvailable, &p.SortOrder, &p.Price)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("product: %w", err)
	}

	variants, err := r.variants(ctx, id)
	if err != nil {
		return nil, err
	}
	p.Variants = variants
	return p, nil
}

func (r *Repository) variants(ctx context.Context, productID string) ([]Variant, error) {
	const q = `
		SELECT id, name, COALESCE(sku_suffix, ''), price::text, is_default, is_active, sort_order
		FROM product_variants
		WHERE product_id = $1
		ORDER BY sort_order, name`

	rows, err := r.pool.Query(ctx, q, productID)
	if err != nil {
		return nil, fmt.Errorf("variants: %w", err)
	}
	defer rows.Close()

	items := []Variant{}
	for rows.Next() {
		var v Variant
		if err := rows.Scan(&v.ID, &v.Name, &v.SKU, &v.Price, &v.IsDefault, &v.IsActive, &v.SortOrder); err != nil {
			return nil, fmt.Errorf("scan variant: %w", err)
		}
		items = append(items, v)
	}
	return items, rows.Err()
}

func (r *Repository) CreateProduct(ctx context.Context, businessID, userID string, in ProductInput) (*Product, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("createProduct: begin: %w", err)
	}
	defer tx.Rollback(ctx)

	const insertProduct = `
		INSERT INTO products (business_id, category_id, sku, name, description, product_type,
		                      stock_mode, stock_ingredient_id, kitchen_station, allow_zero_price,
		                      sort_order, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING id`

	var id string
	if err := tx.QueryRow(ctx, insertProduct, businessID, in.CategoryID, in.SKU, in.Name,
		in.Description, in.ProductType, in.StockMode, in.StockIngredient, in.KitchenStation,
		in.AllowZeroPrice, in.SortOrder, userID).Scan(&id); err != nil {
		return nil, fmt.Errorf("createProduct: %w", err)
	}

	for _, v := range in.Variants {
		const insertVariant = `
			INSERT INTO product_variants (product_id, name, sku_suffix, price, is_default, sort_order)
			VALUES ($1, $2, $3, $4, $5, $6)`
		if _, err := tx.Exec(ctx, insertVariant, id, v.Name, nullIfEmpty(v.SKU), v.Price, v.IsDefault, v.SortOrder); err != nil {
			return nil, fmt.Errorf("createProduct: variant: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("createProduct: commit: %w", err)
	}
	return r.Product(ctx, businessID, id)
}

func (r *Repository) UpdateProduct(ctx context.Context, businessID, id string, in ProductInput) (*Product, error) {
	const q = `
		UPDATE products
		SET category_id = $3, sku = $4, name = $5, description = $6, product_type = $7,
		    stock_mode = $8, stock_ingredient_id = $9, kitchen_station = $10,
		    allow_zero_price = $11, sort_order = $12, updated_at = now()
		WHERE id = $1 AND business_id = $2
		RETURNING id`

	var updated string
	err := r.pool.QueryRow(ctx, q, id, businessID, in.CategoryID, in.SKU, in.Name,
		in.Description, in.ProductType, in.StockMode, in.StockIngredient, in.KitchenStation,
		in.AllowZeroPrice, in.SortOrder).Scan(&updated)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("updateProduct: %w", err)
	}
	return r.Product(ctx, businessID, id)
}

func (r *Repository) SetProductActive(ctx context.Context, businessID, id string, active bool) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE products SET is_active = $3, updated_at = now() WHERE id = $1 AND business_id = $2`,
		id, businessID, active)
	if err != nil {
		return fmt.Errorf("setProductActive: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func nullIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
