package opname

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("sesi opname tidak ditemukan")

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Begin(ctx context.Context) (pgx.Tx, error) {
	return r.pool.Begin(ctx)
}

// NextSequence menaikkan counter dokumen per outlet dan periode secara atomik.
// Nomor boleh melompat saat rollback, tetapi tidak pernah ganda.
func (r *Repository) NextSequence(ctx context.Context, tx pgx.Tx, outletID, docType, period string) (int, error) {
	const q = `
		INSERT INTO document_sequences (outlet_id, doc_type, period, last_number)
		VALUES ($1, $2, $3, 1)
		ON CONFLICT (outlet_id, doc_type, period)
		DO UPDATE SET last_number = document_sequences.last_number + 1
		RETURNING last_number`

	var n int
	if err := tx.QueryRow(ctx, q, outletID, docType, period).Scan(&n); err != nil {
		return 0, fmt.Errorf("nextSequence: %w", err)
	}
	return n, nil
}

// CreateOpname menyimpan sesi baru berstatus DRAFT.
func (r *Repository) CreateOpname(ctx context.Context, tx pgx.Tx, outletID, userID, code, scope string, note *string) (string, error) {
	const q = `
		INSERT INTO stock_opnames (outlet_id, code, scope, note, created_by)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id`

	var id string
	if err := tx.QueryRow(ctx, q, outletID, code, scope, note, userID).Scan(&id); err != nil {
		return "", fmt.Errorf("createOpname: %w", err)
	}
	return id, nil
}

// InsertItemsSnapshot mengisi item sesi (belum aktif) untuk seluruh bahan aktif
// bisnis atau hanya bahan terpilih.
func (r *Repository) InsertItemsSnapshot(ctx context.Context, tx pgx.Tx, outletID, opnameID, businessID string, ingredientIDs []string) (int, error) {
	const q = `
		INSERT INTO stock_opname_items (stock_opname_id, ingredient_id, system_quantity_snapshot, snapshot_at, is_active_session)
		SELECT $1, i.id, COALESCE(inv.quantity_on_hand, 0), now(), false
		FROM ingredients i
		LEFT JOIN inventory inv ON inv.ingredient_id = i.id AND inv.outlet_id = $2
		WHERE i.business_id = $3 AND i.is_active
		  AND ($4::uuid[] IS NULL OR i.id = ANY($4))`

	tag, err := tx.Exec(ctx, q, opnameID, outletID, businessID, ingredientIDs)
	if err != nil {
		return 0, fmt.Errorf("insertOpnameItems: %w", err)
	}
	return int(tag.RowsAffected()), nil
}

// SetStarted memindahkan sesi DRAFT ke IN_PROGRESS dan mengaktifkan itemnya.
func (r *Repository) SetStarted(ctx context.Context, tx pgx.Tx, outletID, id string) (bool, error) {
	const q = `
		UPDATE stock_opnames
		SET status = 'IN_PROGRESS', started_at = now(), version = version + 1, updated_at = now()
		WHERE id = $1 AND outlet_id = $2 AND status = 'DRAFT'`
	tag, err := tx.Exec(ctx, q, id, outletID)
	if err != nil {
		return false, fmt.Errorf("setOpnameStarted: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

// ActivateItems menyegarkan snapshot saldo dan menandai item sebagai sesi aktif.
func (r *Repository) ActivateItems(ctx context.Context, tx pgx.Tx, outletID, opnameID string) error {
	const q = `
		UPDATE stock_opname_items soi
		SET system_quantity_snapshot = COALESCE((
		        SELECT inv.quantity_on_hand FROM inventory inv
		        WHERE inv.outlet_id = $2 AND inv.ingredient_id = soi.ingredient_id), 0),
		    snapshot_at = now(), is_active_session = true
		WHERE soi.stock_opname_id = $1`

	if _, err := tx.Exec(ctx, q, opnameID, outletID); err != nil {
		return fmt.Errorf("activateOpnameItems: %w", err)
	}
	return nil
}

// CountItem mencatat hasil hitung fisik satu bahan.
func (r *Repository) CountItem(ctx context.Context, tx pgx.Tx, opnameID, ingredientID, qty, userID string, reason *string) (bool, error) {
	const q = `
		UPDATE stock_opname_items
		SET counted_quantity = $3, counted_at = now(), counted_by = $4, reason = $5
		WHERE stock_opname_id = $1 AND ingredient_id = $2 AND is_active_session`
	tag, err := tx.Exec(ctx, q, opnameID, ingredientID, qty, userID, reason)
	if err != nil {
		return false, fmt.Errorf("countOpnameItem: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

// Opname mengambil sesi beserta itemnya.
func (r *Repository) Opname(ctx context.Context, outletID, id string) (*Opname, error) {
	const q = `
		SELECT id, code, status, scope, note, started_at, submitted_at, approved_at, created_at
		FROM stock_opnames WHERE id = $1 AND outlet_id = $2`

	o := &Opname{}
	err := r.pool.QueryRow(ctx, q, id, outletID).Scan(&o.ID, &o.Code, &o.Status, &o.Scope,
		&o.Note, &o.StartedAt, &o.SubmittedAt, &o.ApprovedAt, &o.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("opname: %w", err)
	}
	items, counted, err := r.items(ctx, id)
	if err != nil {
		return nil, err
	}
	o.Items = items
	o.TotalItems = len(items)
	o.CountedItems = counted
	return o, nil
}

func (r *Repository) items(ctx context.Context, opnameID string) ([]Item, int, error) {
	const q = `
		SELECT soi.id, soi.ingredient_id, i.code, i.name, u.code,
		       soi.system_quantity_snapshot::text, soi.counted_quantity::text,
		       (soi.counted_quantity - soi.system_quantity_snapshot)::text,
		       soi.counted_at, soi.reason
		FROM stock_opname_items soi
		JOIN ingredients i ON i.id = soi.ingredient_id
		JOIN units u ON u.id = i.base_unit_id
		WHERE soi.stock_opname_id = $1
		ORDER BY i.name`

	rows, err := r.pool.Query(ctx, q, opnameID)
	if err != nil {
		return nil, 0, fmt.Errorf("opnameItems: %w", err)
	}
	defer rows.Close()

	items := []Item{}
	counted := 0
	for rows.Next() {
		var it Item
		if err := rows.Scan(&it.ID, &it.IngredientID, &it.IngredientCode, &it.IngredientName,
			&it.BaseUnitCode, &it.SystemQuantity, &it.CountedQuantity, &it.Difference,
			&it.CountedAt, &it.Reason); err != nil {
			return nil, 0, fmt.Errorf("opnameItems: scan: %w", err)
		}
		if it.CountedQuantity != nil {
			counted++
		}
		items = append(items, it)
	}
	return items, counted, rows.Err()
}

// StatusForUpdate mengunci sesi dan mengembalikan status terkini.
func (r *Repository) StatusForUpdate(ctx context.Context, tx pgx.Tx, outletID, id string) (string, bool, error) {
	const q = `SELECT status FROM stock_opnames WHERE id = $1 AND outlet_id = $2 FOR UPDATE`
	var status string
	err := tx.QueryRow(ctx, q, id, outletID).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("statusForUpdate: %w", err)
	}
	return status, true, nil
}

// IngredientsActive memastikan semua id bahan ada dan aktif pada bisnis.
func (r *Repository) IngredientsActive(ctx context.Context, businessID string, ids []string) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx,
		`SELECT count(*)::int FROM ingredients WHERE business_id = $1 AND is_active AND id = ANY($2)`,
		businessID, ids).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("ingredientsActive: %w", err)
	}
	return n, nil
}
