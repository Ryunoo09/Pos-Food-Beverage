package recipe

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("resep tidak ditemukan")

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) VariantExists(ctx context.Context, businessID, variantID string) (bool, error) {
	var ok bool
	err := r.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM product_variants v
			JOIN products p ON p.id = v.product_id
			WHERE v.id = $1 AND p.business_id = $2
		)`, variantID, businessID).Scan(&ok)
	if err != nil {
		return false, fmt.Errorf("variantExists: %w", err)
	}
	return ok, nil
}

// Active mengembalikan resep ACTIVE milik varian beserta komponen dan biaya
// per komponen berdasarkan rata-rata biaya bahan di outlet.
func (r *Repository) Active(ctx context.Context, businessID, outletID, variantID string) (*Recipe, error) {
	const q = `
		SELECT r.id, r.variant_id, r.version_no, r.status, r.yield_note
		FROM recipes r
		JOIN product_variants v ON v.id = r.variant_id
		JOIN products p ON p.id = v.product_id
		WHERE r.variant_id = $1 AND r.status = 'ACTIVE' AND p.business_id = $2`

	rc := &Recipe{}
	err := r.pool.QueryRow(ctx, q, variantID, businessID).Scan(&rc.ID, &rc.VariantID,
		&rc.VersionNo, &rc.Status, &rc.YieldNote)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("activeRecipe: %w", err)
	}

	items, err := r.items(ctx, rc.ID, outletID)
	if err != nil {
		return nil, err
	}
	rc.Items = items
	return rc, nil
}

func (r *Repository) items(ctx context.Context, recipeID, outletID string) ([]RecipeItem, error) {
	const q = `
		SELECT ri.ingredient_id, i.name, u.code, ri.quantity::text,
		       ri.waste_factor_percent::text, COALESCE(inv.avg_unit_cost, 0)::text
		FROM recipe_items ri
		JOIN ingredients i ON i.id = ri.ingredient_id
		JOIN units u ON u.id = i.base_unit_id
		LEFT JOIN inventory inv ON inv.ingredient_id = ri.ingredient_id AND inv.outlet_id = $2
		WHERE ri.recipe_id = $1
		ORDER BY i.name`

	rows, err := r.pool.Query(ctx, q, recipeID, outletID)
	if err != nil {
		return nil, fmt.Errorf("recipeItems: %w", err)
	}
	defer rows.Close()

	items := []RecipeItem{}
	for rows.Next() {
		var item RecipeItem
		var avgCost string
		if err := rows.Scan(&item.IngredientID, &item.IngredientName, &item.UnitCode,
			&item.Quantity, &item.WasteFactorPercent, &avgCost); err != nil {
			return nil, fmt.Errorf("scan recipe item: %w", err)
		}
		item.avgCost = avgCost
		items = append(items, item)
	}
	return items, rows.Err()
}

// Versions mengembalikan riwayat versi resep varian (tanpa komponen).
func (r *Repository) Versions(ctx context.Context, businessID, variantID string) ([]Recipe, error) {
	const q = `
		SELECT r.id, r.variant_id, r.version_no, r.status, r.yield_note
		FROM recipes r
		JOIN product_variants v ON v.id = r.variant_id
		JOIN products p ON p.id = v.product_id
		WHERE r.variant_id = $1 AND p.business_id = $2
		ORDER BY r.version_no DESC`

	rows, err := r.pool.Query(ctx, q, variantID, businessID)
	if err != nil {
		return nil, fmt.Errorf("recipeVersions: %w", err)
	}
	defer rows.Close()

	items := []Recipe{}
	for rows.Next() {
		var rc Recipe
		if err := rows.Scan(&rc.ID, &rc.VariantID, &rc.VersionNo, &rc.Status, &rc.YieldNote); err != nil {
			return nil, fmt.Errorf("scan recipe version: %w", err)
		}
		items = append(items, rc)
	}
	return items, rows.Err()
}

func (r *Repository) IngredientsExist(ctx context.Context, businessID string, ids []string) (bool, error) {
	var count int
	err := r.pool.QueryRow(ctx,
		`SELECT count(*)::int FROM ingredients WHERE business_id = $1 AND id = ANY($2::uuid[])`,
		businessID, ids).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("ingredientsExist: %w", err)
	}
	return count == len(ids), nil
}

// CreateVersion menonaktifkan versi aktif lama lalu menyimpan versi baru dalam
// satu transaksi, sehingga selalu ada tepat satu resep ACTIVE per varian.
func (r *Repository) CreateVersion(ctx context.Context, userID, variantID string, in RecipeInput) (*Recipe, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("createRecipeVersion: begin: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx,
		`UPDATE recipes SET status = 'INACTIVE', updated_at = now()
		 WHERE variant_id = $1 AND status = 'ACTIVE'`, variantID); err != nil {
		return nil, fmt.Errorf("createRecipeVersion: deactivate: %w", err)
	}

	const insertRecipe = `
		INSERT INTO recipes (variant_id, version_no, yield_note, created_by)
		VALUES ($1, COALESCE((SELECT max(version_no) FROM recipes WHERE variant_id = $1), 0) + 1, $2, $3)
		RETURNING id`

	var id string
	if err := tx.QueryRow(ctx, insertRecipe, variantID, in.YieldNote, userID).Scan(&id); err != nil {
		return nil, fmt.Errorf("createRecipeVersion: recipe: %w", err)
	}

	for _, item := range in.Items {
		const insert = `
			INSERT INTO recipe_items (recipe_id, ingredient_id, quantity, waste_factor_percent)
			VALUES ($1, $2, $3, $4)`
		if _, err := tx.Exec(ctx, insert, id, item.IngredientID, item.Quantity,
			item.WasteFactorPercent); err != nil {
			return nil, fmt.Errorf("createRecipeVersion: item: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("createRecipeVersion: commit: %w", err)
	}

	const q = `
		SELECT id, variant_id, version_no, status, yield_note
		FROM recipes WHERE id = $1`
	rc := &Recipe{}
	if err := r.pool.QueryRow(ctx, q, id).Scan(&rc.ID, &rc.VariantID, &rc.VersionNo,
		&rc.Status, &rc.YieldNote); err != nil {
		return nil, fmt.Errorf("createRecipeVersion: reload: %w", err)
	}
	return rc, nil
}
