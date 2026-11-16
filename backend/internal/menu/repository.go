package menu

import (
	"context"
	"errors"
	"fmt"
	"time"

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

	rows, err := r.pool.Query(ctx,
		`SELECT modifier_group_id FROM product_modifier_groups
		 WHERE product_id = $1 ORDER BY sort_order`, id)
	if err != nil {
		return nil, fmt.Errorf("product modifier groups: %w", err)
	}
	defer rows.Close()
	p.ModifierGroupIDs = []string{}
	for rows.Next() {
		var groupID string
		if err := rows.Scan(&groupID); err != nil {
			return nil, fmt.Errorf("scan product modifier group: %w", err)
		}
		p.ModifierGroupIDs = append(p.ModifierGroupIDs, groupID)
	}
	return p, rows.Err()
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
			VALUES ($1, $2, $3, $4, $5, $6)
			RETURNING id, price::text`
		var variantID, price string
		if err := tx.QueryRow(ctx, insertVariant, id, v.Name, nullIfEmpty(v.SKU),
			v.Price, v.IsDefault, v.SortOrder).Scan(&variantID, &price); err != nil {
			return nil, fmt.Errorf("createProduct: variant: %w", err)
		}

		const history = `
			INSERT INTO price_histories (entity_type, entity_id, new_price, changed_by)
			VALUES ('VARIANT', $1, $2, $3)`
		if _, err := tx.Exec(ctx, history, variantID, price, userID); err != nil {
			return nil, fmt.Errorf("createProduct: variant history: %w", err)
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

func (r *Repository) CreateVariant(ctx context.Context, userID, productID string, in VariantInput) (*Variant, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("createVariant: begin: %w", err)
	}
	defer tx.Rollback(ctx)

	const q = `
		INSERT INTO product_variants (product_id, name, sku_suffix, price, is_default, sort_order)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, name, COALESCE(sku_suffix, ''), price::text, is_default, is_active, sort_order`

	v := &Variant{}
	if err := tx.QueryRow(ctx, q, productID, in.Name, nullIfEmpty(in.SKU), in.Price,
		in.IsDefault, in.SortOrder).Scan(&v.ID, &v.Name, &v.SKU, &v.Price,
		&v.IsDefault, &v.IsActive, &v.SortOrder); err != nil {
		return nil, fmt.Errorf("createVariant: %w", err)
	}

	const history = `
		INSERT INTO price_histories (entity_type, entity_id, new_price, changed_by)
		VALUES ('VARIANT', $1, $2, $3)`
	if _, err := tx.Exec(ctx, history, v.ID, v.Price, userID); err != nil {
		return nil, fmt.Errorf("createVariant: history: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("createVariant: commit: %w", err)
	}
	return v, nil
}

func (r *Repository) UpdateVariant(ctx context.Context, businessID, userID, variantID string, in VariantInput, isActive bool) (*Variant, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("updateVariant: begin: %w", err)
	}
	defer tx.Rollback(ctx)

	const current = `
		SELECT v.price::text
		FROM product_variants v
		JOIN products p ON p.id = v.product_id
		WHERE v.id = $1 AND p.business_id = $2
		FOR UPDATE OF v`

	var oldPrice string
	err = tx.QueryRow(ctx, current, variantID, businessID).Scan(&oldPrice)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("updateVariant: current: %w", err)
	}

	const q = `
		UPDATE product_variants v
		SET name = $3, sku_suffix = $4, price = $5, is_default = $6, is_active = $7,
		    sort_order = $8, updated_at = now()
		FROM products p
		WHERE v.id = $1 AND p.id = v.product_id AND p.business_id = $2
		RETURNING v.id, v.name, COALESCE(v.sku_suffix, ''), v.price::text,
		          v.is_default, v.is_active, v.sort_order`

	v := &Variant{}
	err = tx.QueryRow(ctx, q, variantID, businessID, in.Name, nullIfEmpty(in.SKU),
		in.Price, in.IsDefault, isActive, in.SortOrder).Scan(&v.ID, &v.Name, &v.SKU,
		&v.Price, &v.IsDefault, &v.IsActive, &v.SortOrder)
	if err != nil {
		return nil, fmt.Errorf("updateVariant: %w", err)
	}

	if v.Price != oldPrice {
		const history = `
			INSERT INTO price_histories (entity_type, entity_id, old_price, new_price, changed_by)
			VALUES ('VARIANT', $1, $2, $3, $4)`
		if _, err := tx.Exec(ctx, history, variantID, oldPrice, v.Price, userID); err != nil {
			return nil, fmt.Errorf("updateVariant: history: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("updateVariant: commit: %w", err)
	}
	return v, nil
}

type PriceHistory struct {
	OldPrice  *string   `json:"old_price"`
	NewPrice  string    `json:"new_price"`
	Reason    *string   `json:"reason,omitempty"`
	ChangedAt time.Time `json:"changed_at"`
}

func (r *Repository) PriceHistories(ctx context.Context, businessID, variantID string) ([]PriceHistory, error) {
	const q = `
		SELECT ph.old_price::text, ph.new_price::text, ph.reason, ph.changed_at
		FROM price_histories ph
		JOIN product_variants v ON v.id = ph.entity_id
		JOIN products p ON p.id = v.product_id
		WHERE ph.entity_type = 'VARIANT' AND ph.entity_id = $1 AND p.business_id = $2
		ORDER BY ph.changed_at DESC`

	rows, err := r.pool.Query(ctx, q, variantID, businessID)
	if err != nil {
		return nil, fmt.Errorf("priceHistories: %w", err)
	}
	defer rows.Close()

	items := []PriceHistory{}
	for rows.Next() {
		var h PriceHistory
		if err := rows.Scan(&h.OldPrice, &h.NewPrice, &h.Reason, &h.ChangedAt); err != nil {
			return nil, fmt.Errorf("scan price history: %w", err)
		}
		items = append(items, h)
	}
	return items, rows.Err()
}

func (r *Repository) SetProductAvailability(ctx context.Context, businessID, id string, available bool) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE products SET is_available = $3, updated_at = now() WHERE id = $1 AND business_id = $2`,
		id, businessID, available)
	if err != nil {
		return fmt.Errorf("setProductAvailability: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) Variant(ctx context.Context, businessID, variantID string) (*Variant, error) {
	const q = `
		SELECT v.id, v.name, COALESCE(v.sku_suffix, ''), v.price::text, v.is_default, v.is_active, v.sort_order
		FROM product_variants v
		JOIN products p ON p.id = v.product_id
		WHERE v.id = $1 AND p.business_id = $2`

	v := &Variant{}
	err := r.pool.QueryRow(ctx, q, variantID, businessID).Scan(&v.ID, &v.Name, &v.SKU,
		&v.Price, &v.IsDefault, &v.IsActive, &v.SortOrder)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("variant: %w", err)
	}
	return v, nil
}

func (r *Repository) ListModifierGroups(ctx context.Context, businessID string) ([]ModifierGroup, error) {
	const q = `
		SELECT id, name, min_select, max_select, is_required, sort_order, is_active
		FROM modifier_groups
		WHERE business_id = $1
		ORDER BY sort_order, name`

	rows, err := r.pool.Query(ctx, q, businessID)
	if err != nil {
		return nil, fmt.Errorf("listModifierGroups: %w", err)
	}
	defer rows.Close()

	items := []ModifierGroup{}
	for rows.Next() {
		var g ModifierGroup
		if err := rows.Scan(&g.ID, &g.Name, &g.MinSelect, &g.MaxSelect,
			&g.IsRequired, &g.SortOrder, &g.IsActive); err != nil {
			return nil, fmt.Errorf("scan modifier group: %w", err)
		}
		items = append(items, g)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for i := range items {
		options, err := r.modifierOptions(ctx, items[i].ID)
		if err != nil {
			return nil, err
		}
		items[i].Options = options
	}
	return items, nil
}

func (r *Repository) ModifierGroup(ctx context.Context, businessID, id string) (*ModifierGroup, error) {
	const q = `
		SELECT id, name, min_select, max_select, is_required, sort_order, is_active
		FROM modifier_groups
		WHERE id = $1 AND business_id = $2`

	g := &ModifierGroup{}
	err := r.pool.QueryRow(ctx, q, id, businessID).Scan(&g.ID, &g.Name, &g.MinSelect,
		&g.MaxSelect, &g.IsRequired, &g.SortOrder, &g.IsActive)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("modifierGroup: %w", err)
	}

	options, err := r.modifierOptions(ctx, g.ID)
	if err != nil {
		return nil, err
	}
	g.Options = options
	return g, nil
}

func (r *Repository) modifierOptions(ctx context.Context, groupID string) ([]ModifierOption, error) {
	const q = `
		SELECT id, name, price_delta::text, is_default, is_active, sort_order
		FROM modifier_options
		WHERE group_id = $1
		ORDER BY sort_order, name`

	rows, err := r.pool.Query(ctx, q, groupID)
	if err != nil {
		return nil, fmt.Errorf("modifierOptions: %w", err)
	}
	defer rows.Close()

	items := []ModifierOption{}
	for rows.Next() {
		var o ModifierOption
		if err := rows.Scan(&o.ID, &o.Name, &o.PriceDelta, &o.IsDefault, &o.IsActive, &o.SortOrder); err != nil {
			return nil, fmt.Errorf("scan modifier option: %w", err)
		}
		items = append(items, o)
	}
	return items, rows.Err()
}

func (r *Repository) CreateModifierGroup(ctx context.Context, businessID string, in ModifierGroupInput) (*ModifierGroup, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("createModifierGroup: begin: %w", err)
	}
	defer tx.Rollback(ctx)

	const q = `
		INSERT INTO modifier_groups (business_id, name, min_select, max_select, is_required, sort_order)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id`

	var id string
	if err := tx.QueryRow(ctx, q, businessID, in.Name, in.MinSelect, in.MaxSelect,
		in.IsRequired, in.SortOrder).Scan(&id); err != nil {
		return nil, fmt.Errorf("createModifierGroup: %w", err)
	}

	for _, o := range in.Options {
		const insert = `
			INSERT INTO modifier_options (group_id, name, price_delta, is_default, sort_order)
			VALUES ($1, $2, $3, $4, $5)`
		if _, err := tx.Exec(ctx, insert, id, o.Name, o.PriceDelta, o.IsDefault, o.SortOrder); err != nil {
			return nil, fmt.Errorf("createModifierGroup: option: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("createModifierGroup: commit: %w", err)
	}
	return r.ModifierGroup(ctx, businessID, id)
}

func (r *Repository) UpdateModifierGroup(ctx context.Context, businessID, id string, in ModifierGroupInput, isActive bool) (*ModifierGroup, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("updateModifierGroup: begin: %w", err)
	}
	defer tx.Rollback(ctx)

	const q = `
		UPDATE modifier_groups
		SET name = $3, min_select = $4, max_select = $5, is_required = $6,
		    sort_order = $7, is_active = $8, updated_at = now()
		WHERE id = $1 AND business_id = $2
		RETURNING id`

	var updated string
	err = tx.QueryRow(ctx, q, id, businessID, in.Name, in.MinSelect, in.MaxSelect,
		in.IsRequired, in.SortOrder, isActive).Scan(&updated)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("updateModifierGroup: %w", err)
	}

	kept := make([]string, 0, len(in.Options))
	for _, o := range in.Options {
		if o.ID == "" {
			const insert = `
				INSERT INTO modifier_options (group_id, name, price_delta, is_default, sort_order)
				VALUES ($1, $2, $3, $4, $5)`
			if _, err := tx.Exec(ctx, insert, id, o.Name, o.PriceDelta, o.IsDefault, o.SortOrder); err != nil {
				return nil, fmt.Errorf("updateModifierGroup: insert option: %w", err)
			}
			continue
		}
		const update = `
			UPDATE modifier_options
			SET name = $3, price_delta = $4, is_default = $5, sort_order = $6,
			    is_active = true, updated_at = now()
			WHERE id = $1 AND group_id = $2`
		tag, err := tx.Exec(ctx, update, o.ID, id, o.Name, o.PriceDelta, o.IsDefault, o.SortOrder)
		if err != nil {
			return nil, fmt.Errorf("updateModifierGroup: update option: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return nil, ErrNotFound
		}
		kept = append(kept, o.ID)
	}

	const deactivate = `
		UPDATE modifier_options SET is_active = false, updated_at = now()
		WHERE group_id = $1 AND NOT (id = ANY($2::uuid[]))`
	if _, err := tx.Exec(ctx, deactivate, id, kept); err != nil {
		return nil, fmt.Errorf("updateModifierGroup: deactivate options: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("updateModifierGroup: commit: %w", err)
	}
	return r.ModifierGroup(ctx, businessID, id)
}

func (r *Repository) SetModifierGroupActive(ctx context.Context, businessID, id string, active bool) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE modifier_groups SET is_active = $3, updated_at = now() WHERE id = $1 AND business_id = $2`,
		id, businessID, active)
	if err != nil {
		return fmt.Errorf("setModifierGroupActive: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) SetProductModifierGroups(ctx context.Context, businessID, productID string, groupIDs []string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("setProductModifierGroups: begin: %w", err)
	}
	defer tx.Rollback(ctx)

	var exists bool
	if err := tx.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM products WHERE id = $1 AND business_id = $2)`,
		productID, businessID).Scan(&exists); err != nil {
		return fmt.Errorf("setProductModifierGroups: product: %w", err)
	}
	if !exists {
		return ErrNotFound
	}

	var valid int
	if err := tx.QueryRow(ctx,
		`SELECT count(*)::int FROM modifier_groups WHERE business_id = $1 AND id = ANY($2::uuid[])`,
		businessID, groupIDs).Scan(&valid); err != nil {
		return fmt.Errorf("setProductModifierGroups: groups: %w", err)
	}
	if valid != len(groupIDs) {
		return ErrNotFound
	}

	if _, err := tx.Exec(ctx, `DELETE FROM product_modifier_groups WHERE product_id = $1`, productID); err != nil {
		return fmt.Errorf("setProductModifierGroups: clear: %w", err)
	}
	for i, groupID := range groupIDs {
		const insert = `INSERT INTO product_modifier_groups (product_id, modifier_group_id, sort_order) VALUES ($1, $2, $3)`
		if _, err := tx.Exec(ctx, insert, productID, groupID, i); err != nil {
			return fmt.Errorf("setProductModifierGroups: insert: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("setProductModifierGroups: commit: %w", err)
	}
	return nil
}
