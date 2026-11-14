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
		SELECT id, code, status, scope, note, rejected_reason, started_at, submitted_at, approved_at, created_at
		FROM stock_opnames WHERE id = $1 AND outlet_id = $2`

	o := &Opname{}
	err := r.pool.QueryRow(ctx, q, id, outletID).Scan(&o.ID, &o.Code, &o.Status, &o.Scope,
		&o.Note, &o.RejectReason, &o.StartedAt, &o.SubmittedAt, &o.ApprovedAt, &o.CreatedAt)
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
		       soi.difference_value::text, soi.counted_at, soi.reason
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
			&it.DifferenceValue, &it.CountedAt, &it.Reason); err != nil {
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

// LockOpname mengunci sesi dan mengembalikan status serta pengaju.
func (r *Repository) LockOpname(ctx context.Context, tx pgx.Tx, outletID, id string) (status string, submittedBy *string, found bool, err error) {
	const q = `SELECT status, submitted_by FROM stock_opnames WHERE id = $1 AND outlet_id = $2 FOR UPDATE`
	err = tx.QueryRow(ctx, q, id, outletID).Scan(&status, &submittedBy)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil, false, nil
	}
	if err != nil {
		return "", nil, false, fmt.Errorf("lockOpname: %w", err)
	}
	return status, submittedBy, true, nil
}

// SetSubmitted mengunci sesi IN_PROGRESS menjadi SUBMITTED.
func (r *Repository) SetSubmitted(ctx context.Context, tx pgx.Tx, outletID, id, userID string) (bool, error) {
	const q = `
		UPDATE stock_opnames
		SET status = 'SUBMITTED', submitted_at = now(), submitted_by = $3,
		    version = version + 1, updated_at = now()
		WHERE id = $1 AND outlet_id = $2 AND status = 'IN_PROGRESS'`
	tag, err := tx.Exec(ctx, q, id, outletID, userID)
	if err != nil {
		return false, fmt.Errorf("setOpnameSubmitted: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

// SetApproved mengunci sesi SUBMITTED menjadi APPROVED.
func (r *Repository) SetApproved(ctx context.Context, tx pgx.Tx, outletID, id, userID string) (bool, error) {
	const q = `
		UPDATE stock_opnames
		SET status = 'APPROVED', approved_at = now(), approved_by = $3,
		    version = version + 1, updated_at = now()
		WHERE id = $1 AND outlet_id = $2 AND status = 'SUBMITTED'`
	tag, err := tx.Exec(ctx, q, id, outletID, userID)
	if err != nil {
		return false, fmt.Errorf("setOpnameApproved: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

// SetRejected menolak sesi SUBMITTED dan melepas itemnya.
func (r *Repository) SetRejected(ctx context.Context, tx pgx.Tx, outletID, id, reason string) (bool, error) {
	const q = `
		UPDATE stock_opnames
		SET status = 'REJECTED', rejected_reason = $3, version = version + 1, updated_at = now()
		WHERE id = $1 AND outlet_id = $2 AND status = 'SUBMITTED'`
	tag, err := tx.Exec(ctx, q, id, outletID, reason)
	if err != nil {
		return false, fmt.Errorf("setOpnameRejected: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

// SetCancelled membatalkan sesi yang belum final.
func (r *Repository) SetCancelled(ctx context.Context, tx pgx.Tx, outletID, id string) (bool, error) {
	const q = `
		UPDATE stock_opnames
		SET status = 'CANCELLED', version = version + 1, updated_at = now()
		WHERE id = $1 AND outlet_id = $2 AND status IN ('DRAFT', 'IN_PROGRESS', 'SUBMITTED')`
	tag, err := tx.Exec(ctx, q, id, outletID)
	if err != nil {
		return false, fmt.Errorf("setOpnameCancelled: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

// CountStats mengembalikan jumlah item dan yang sudah dihitung.
func (r *Repository) CountStats(ctx context.Context, tx pgx.Tx, opnameID string) (total, counted int, err error) {
	const q = `SELECT count(*)::int, count(counted_quantity)::int
	           FROM stock_opname_items WHERE stock_opname_id = $1`
	if err := tx.QueryRow(ctx, q, opnameID).Scan(&total, &counted); err != nil {
		return 0, 0, fmt.Errorf("opnameCountStats: %w", err)
	}
	return total, counted, nil
}

// CountedItem adalah baris yang dipakai saat approve untuk menyesuaikan stok.
type CountedItem struct {
	ID           string
	IngredientID string
	Quantity     string
	Reason       *string
}

// CountedItems mengambil item yang sudah dihitung, urut ingredient_id.
func (r *Repository) CountedItems(ctx context.Context, tx pgx.Tx, opnameID string) ([]CountedItem, error) {
	const q = `
		SELECT id, ingredient_id::text, counted_quantity::text, reason
		FROM stock_opname_items
		WHERE stock_opname_id = $1 AND counted_quantity IS NOT NULL
		ORDER BY ingredient_id`
	rows, err := tx.Query(ctx, q, opnameID)
	if err != nil {
		return nil, fmt.Errorf("countedItems: %w", err)
	}
	defer rows.Close()

	items := []CountedItem{}
	for rows.Next() {
		var it CountedItem
		if err := rows.Scan(&it.ID, &it.IngredientID, &it.Quantity, &it.Reason); err != nil {
			return nil, fmt.Errorf("countedItems: scan: %w", err)
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

// LockInventoryForOpname memastikan baris inventaris ada dan menguncinya.
func (r *Repository) LockInventoryForOpname(ctx context.Context, tx pgx.Tx, outletID, ingredientID string) (qty, avg string, err error) {
	if _, err := tx.Exec(ctx, `
		INSERT INTO inventory (outlet_id, ingredient_id) VALUES ($1, $2)
		ON CONFLICT (outlet_id, ingredient_id) DO NOTHING`, outletID, ingredientID); err != nil {
		return "", "", fmt.Errorf("lockInventoryForOpname: ensure: %w", err)
	}
	if err := tx.QueryRow(ctx, `
		SELECT quantity_on_hand::text, avg_unit_cost::text
		FROM inventory WHERE outlet_id = $1 AND ingredient_id = $2 FOR UPDATE`,
		outletID, ingredientID).Scan(&qty, &avg); err != nil {
		return "", "", fmt.Errorf("lockInventoryForOpname: lock: %w", err)
	}
	return qty, avg, nil
}

// InsertOpnameAdjustment menulis movement ADJUSTMENT opname dan mengembalikan id-nya.
func (r *Repository) InsertOpnameAdjustment(ctx context.Context, tx pgx.Tx, outletID, opnameID, ingredientID, delta, avg, balance, userID string, reason *string) (string, error) {
	const q = `
		INSERT INTO stock_movements (outlet_id, ingredient_id, movement_type, quantity_delta,
		                             unit_cost, total_cost, balance_after, avg_cost_after,
		                             reference_type, reference_id, reason, created_by)
		VALUES ($1, $2, 'ADJUSTMENT', $3, $4, ROUND($3::numeric * $4::numeric, 2),
		        $5, $4, 'STOCK_OPNAME', $6, COALESCE($7, 'Stock opname'), $8)
		RETURNING id`

	var id string
	if err := tx.QueryRow(ctx, q, outletID, ingredientID, delta, avg, balance, opnameID, reason, userID).Scan(&id); err != nil {
		return "", fmt.Errorf("insertOpnameAdjustment: %w", err)
	}
	return id, nil
}

// UpdateInventoryAfterOpname menetapkan saldo inventaris ke hasil hitung fisik.
func (r *Repository) UpdateInventoryAfterOpname(ctx context.Context, tx pgx.Tx, outletID, ingredientID, balance string) error {
	if _, err := tx.Exec(ctx, `
		UPDATE inventory SET quantity_on_hand = $3, last_movement_at = now(),
		       version = version + 1, updated_at = now()
		WHERE outlet_id = $1 AND ingredient_id = $2`,
		outletID, ingredientID, balance); err != nil {
		return fmt.Errorf("updateInventoryAfterOpname: %w", err)
	}
	return nil
}

// UpdateItemResult menyimpan hasil hitung approve pada item.
func (r *Repository) UpdateItemResult(ctx context.Context, tx pgx.Tx, itemID, expected, difference, value string, movementID *string) error {
	if _, err := tx.Exec(ctx, `
		UPDATE stock_opname_items
		SET expected_quantity_at_approval = $2, difference_quantity = $3,
		    difference_value = $4, adjustment_movement_id = $5
		WHERE id = $1`,
		itemID, expected, difference, value, movementID); err != nil {
		return fmt.Errorf("updateOpnameItemResult: %w", err)
	}
	return nil
}

// DeactivateItems melepas seluruh item sesi agar bahan bisa diopname ulang.
func (r *Repository) DeactivateItems(ctx context.Context, tx pgx.Tx, opnameID string) error {
	if _, err := tx.Exec(ctx,
		`UPDATE stock_opname_items SET is_active_session = false WHERE stock_opname_id = $1`,
		opnameID); err != nil {
		return fmt.Errorf("deactivateOpnameItems: %w", err)
	}
	return nil
}

// AllowSelfApproval membaca setelan SoD opname outlet.
func (r *Repository) AllowSelfApproval(ctx context.Context, outletID string) (bool, error) {
	var ok bool
	err := r.pool.QueryRow(ctx,
		`SELECT allow_self_approval_opname FROM outlet_settings WHERE outlet_id = $1`, outletID).Scan(&ok)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("allowSelfApproval: %w", err)
	}
	return ok, nil
}
