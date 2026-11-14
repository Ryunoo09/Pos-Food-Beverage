package purchase

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("dokumen pembelian tidak ditemukan")

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

func (r *Repository) SupplierActive(ctx context.Context, businessID, supplierID string) (bool, error) {
	var ok bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM suppliers WHERE id = $1 AND business_id = $2 AND is_active)`,
		supplierID, businessID).Scan(&ok)
	if err != nil {
		return false, fmt.Errorf("supplierActive: %w", err)
	}
	return ok, nil
}

// IngredientActive memastikan bahan ada dan aktif pada bisnis.
func (r *Repository) IngredientActive(ctx context.Context, businessID, ingredientID string) (bool, error) {
	var ok bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM ingredients WHERE id = $1 AND business_id = $2 AND is_active)`,
		ingredientID, businessID).Scan(&ok)
	if err != nil {
		return false, fmt.Errorf("ingredientActive: %w", err)
	}
	return ok, nil
}

// UnitToBase menghitung faktor konversi satuan beli ke satuan dasar bahan.
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

func (r *Repository) CreatePO(ctx context.Context, tx pgx.Tx, outletID, code, userID, supplierID string,
	expectedDate, note *string, discount, subtotal, total string) (string, error) {
	const q = `
		INSERT INTO purchase_orders (outlet_id, code, supplier_id, expected_date, note,
		                             discount_amount, subtotal, total_amount, created_by)
		VALUES ($1, $2, $3, NULLIF($4, '')::date, $5, $6, $7, $8, $9)
		RETURNING id`

	var id string
	if err := tx.QueryRow(ctx, q, outletID, code, supplierID, expectedDate, note,
		discount, subtotal, total, userID).Scan(&id); err != nil {
		return "", fmt.Errorf("createPurchaseOrder: %w", err)
	}
	return id, nil
}

func (r *Repository) InsertPOItem(ctx context.Context, tx pgx.Tx, poID, ingredientID string, unitID int,
	qty, price, discount, lineTotal, baseQty string) error {
	const q = `
		INSERT INTO purchase_order_items (purchase_order_id, ingredient_id, purchase_unit_id,
		                                  quantity_ordered, unit_price, discount_amount, line_total,
		                                  base_quantity_ordered)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`

	if _, err := tx.Exec(ctx, q, poID, ingredientID, unitID, qty, price, discount, lineTotal, baseQty); err != nil {
		return fmt.Errorf("insertPurchaseOrderItem: %w", err)
	}
	return nil
}

func (r *Repository) DeletePOItems(ctx context.Context, tx pgx.Tx, poID string) error {
	if _, err := tx.Exec(ctx, `DELETE FROM purchase_order_items WHERE purchase_order_id = $1`, poID); err != nil {
		return fmt.Errorf("deletePurchaseOrderItems: %w", err)
	}
	return nil
}

func (r *Repository) UpdatePOHeader(ctx context.Context, tx pgx.Tx, outletID, id string,
	expectedDate, note *string, discount, subtotal, total string) error {
	const q = `
		UPDATE purchase_orders
		SET expected_date = NULLIF($3, '')::date, note = $4,
		    discount_amount = $5, subtotal = $6, total_amount = $7,
		    version = version + 1, updated_at = now()
		WHERE id = $1 AND outlet_id = $2 AND status = 'DRAFT'`

	if _, err := tx.Exec(ctx, q, id, outletID, expectedDate, note, discount, subtotal, total); err != nil {
		return fmt.Errorf("updatePurchaseOrder: %w", err)
	}
	return nil
}

func (r *Repository) PO(ctx context.Context, outletID, id string) (*PurchaseOrder, error) {
	const q = `
		SELECT po.id, po.code, po.supplier_id, s.name, po.status, po.order_date::text,
		       po.expected_date::text, po.subtotal::text, po.discount_amount::text,
		       po.total_amount::text, po.note, po.version, po.created_at
		FROM purchase_orders po
		JOIN suppliers s ON s.id = po.supplier_id
		WHERE po.id = $1 AND po.outlet_id = $2`

	o := &PurchaseOrder{}
	err := r.pool.QueryRow(ctx, q, id, outletID).Scan(&o.ID, &o.Code, &o.SupplierID, &o.SupplierName,
		&o.Status, &o.OrderDate, &o.ExpectedDate, &o.Subtotal, &o.DiscountAmount,
		&o.TotalAmount, &o.Note, &o.Version, &o.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("purchaseOrder: %w", err)
	}
	items, err := r.poItems(ctx, id)
	if err != nil {
		return nil, err
	}
	o.Items = items
	return o, nil
}

func (r *Repository) poItems(ctx context.Context, poID string) ([]OrderItem, error) {
	const q = `
		SELECT poi.id, poi.ingredient_id, i.code, i.name, poi.purchase_unit_id, u.code,
		       poi.quantity_ordered::text, poi.unit_price::text, poi.discount_amount::text,
		       poi.line_total::text, poi.base_quantity_ordered::text, poi.quantity_received_base::text
		FROM purchase_order_items poi
		JOIN ingredients i ON i.id = poi.ingredient_id
		JOIN units u ON u.id = poi.purchase_unit_id
		WHERE poi.purchase_order_id = $1
		ORDER BY i.name`

	rows, err := r.pool.Query(ctx, q, poID)
	if err != nil {
		return nil, fmt.Errorf("purchaseOrderItems: %w", err)
	}
	defer rows.Close()

	items := []OrderItem{}
	for rows.Next() {
		var it OrderItem
		if err := rows.Scan(&it.ID, &it.IngredientID, &it.IngredientCode, &it.IngredientName,
			&it.PurchaseUnitID, &it.PurchaseUnitCode, &it.QuantityOrdered, &it.UnitPrice,
			&it.DiscountAmount, &it.LineTotal, &it.BaseQuantityOrdered, &it.QuantityReceivedBase); err != nil {
			return nil, fmt.Errorf("purchaseOrderItems: scan: %w", err)
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

func (r *Repository) ListPO(ctx context.Context, outletID string, f Filter) ([]PurchaseOrder, int, error) {
	const q = `
		SELECT po.id, po.code, po.supplier_id, s.name, po.status, po.order_date::text,
		       po.expected_date::text, po.subtotal::text, po.discount_amount::text,
		       po.total_amount::text, po.note, po.version, po.created_at, count(*) OVER ()::int
		FROM purchase_orders po
		JOIN suppliers s ON s.id = po.supplier_id
		WHERE po.outlet_id = $1
		  AND ($2 = '' OR po.status = $2)
		  AND ($3 = '' OR po.supplier_id = $3::uuid)
		ORDER BY po.created_at DESC
		LIMIT $4 OFFSET $5`

	rows, err := r.pool.Query(ctx, q, outletID, f.Status, f.SupplierID, f.PerPage, (f.Page-1)*f.PerPage)
	if err != nil {
		return nil, 0, fmt.Errorf("listPurchaseOrders: %w", err)
	}
	defer rows.Close()

	items := []PurchaseOrder{}
	total := 0
	for rows.Next() {
		var o PurchaseOrder
		if err := rows.Scan(&o.ID, &o.Code, &o.SupplierID, &o.SupplierName, &o.Status, &o.OrderDate,
			&o.ExpectedDate, &o.Subtotal, &o.DiscountAmount, &o.TotalAmount, &o.Note, &o.Version,
			&o.CreatedAt, &total); err != nil {
			return nil, 0, fmt.Errorf("listPurchaseOrders: scan: %w", err)
		}
		o.Items = []OrderItem{}
		items = append(items, o)
	}
	return items, total, rows.Err()
}

// LockPO mengunci PO dan mengembalikan status, pengaju, dan totalnya.
func (r *Repository) LockPO(ctx context.Context, tx pgx.Tx, outletID, id string) (status string, submittedBy *string, total string, found bool, err error) {
	const q = `SELECT status, submitted_by, total_amount::text
	           FROM purchase_orders WHERE id = $1 AND outlet_id = $2 FOR UPDATE`
	err = tx.QueryRow(ctx, q, id, outletID).Scan(&status, &submittedBy, &total)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil, "", false, nil
	}
	if err != nil {
		return "", nil, "", false, fmt.Errorf("lockPurchaseOrder: %w", err)
	}
	return status, submittedBy, total, true, nil
}

func (r *Repository) SetPOSubmitted(ctx context.Context, tx pgx.Tx, outletID, id, userID string) (bool, error) {
	const q = `
		UPDATE purchase_orders
		SET status = 'SUBMITTED', submitted_by = $3, version = version + 1, updated_at = now()
		WHERE id = $1 AND outlet_id = $2 AND status = 'DRAFT'`
	tag, err := tx.Exec(ctx, q, id, outletID, userID)
	if err != nil {
		return false, fmt.Errorf("setPurchaseOrderSubmitted: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

func (r *Repository) SetPOApproved(ctx context.Context, tx pgx.Tx, outletID, id, userID string) (bool, error) {
	const q = `
		UPDATE purchase_orders
		SET status = 'APPROVED', approved_by = $3, approved_at = now(),
		    version = version + 1, updated_at = now()
		WHERE id = $1 AND outlet_id = $2 AND status = 'SUBMITTED'`
	tag, err := tx.Exec(ctx, q, id, outletID, userID)
	if err != nil {
		return false, fmt.Errorf("setPurchaseOrderApproved: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

func (r *Repository) SetPOCancelled(ctx context.Context, tx pgx.Tx, outletID, id, reason string) (bool, error) {
	const q = `
		UPDATE purchase_orders
		SET status = 'CANCELLED', cancelled_reason = $3, version = version + 1, updated_at = now()
		WHERE id = $1 AND outlet_id = $2 AND status IN ('DRAFT', 'SUBMITTED', 'APPROVED')`
	tag, err := tx.Exec(ctx, q, id, outletID, reason)
	if err != nil {
		return false, fmt.Errorf("setPurchaseOrderCancelled: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

// LockPOItem mengunci baris PO item dan mengembalikan sisa kuantitasnya.
func (r *Repository) LockPOItem(ctx context.Context, tx pgx.Tx, poItemID string) (ingredientID, baseOrdered, received string, found bool, err error) {
	const q = `
		SELECT ingredient_id::text, base_quantity_ordered::text, quantity_received_base::text
		FROM purchase_order_items WHERE id = $1 FOR UPDATE`
	err = tx.QueryRow(ctx, q, poItemID).Scan(&ingredientID, &baseOrdered, &received)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", "", false, nil
	}
	if err != nil {
		return "", "", "", false, fmt.Errorf("lockPurchaseOrderItem: %w", err)
	}
	return ingredientID, baseOrdered, received, true, nil
}

func (r *Repository) AddReceived(ctx context.Context, tx pgx.Tx, poItemID, baseQty string) error {
	if _, err := tx.Exec(ctx,
		`UPDATE purchase_order_items SET quantity_received_base = quantity_received_base + $2 WHERE id = $1`,
		poItemID, baseQty); err != nil {
		return fmt.Errorf("addReceivedQuantity: %w", err)
	}
	return nil
}

// POReceiptProgress menghitung apakah PO sudah diterima seluruhnya.
func (r *Repository) POReceiptProgress(ctx context.Context, tx pgx.Tx, poID string) (allReceived bool, err error) {
	const q = `
		SELECT COALESCE(bool_and(quantity_received_base >= base_quantity_ordered), false)
		FROM purchase_order_items WHERE purchase_order_id = $1`
	if err := tx.QueryRow(ctx, q, poID).Scan(&allReceived); err != nil {
		return false, fmt.Errorf("purchaseOrderReceiptProgress: %w", err)
	}
	return allReceived, nil
}

func (r *Repository) SetPOReceivedStatus(ctx context.Context, tx pgx.Tx, outletID, id string, allReceived bool) error {
	status := "PARTIALLY_RECEIVED"
	if allReceived {
		status = "RECEIVED"
	}
	if _, err := tx.Exec(ctx, `
		UPDATE purchase_orders
		SET status = $3, version = version + 1, updated_at = now()
		WHERE id = $1 AND outlet_id = $2 AND status IN ('APPROVED', 'PARTIALLY_RECEIVED')`,
		id, outletID, status); err != nil {
		return fmt.Errorf("setPurchaseOrderReceivedStatus: %w", err)
	}
	return nil
}

func (r *Repository) ReceivingByKey(ctx context.Context, key string) (string, bool, error) {
	var id string
	err := r.pool.QueryRow(ctx,
		`SELECT id FROM purchase_receivings WHERE idempotency_key = $1`, key).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("receivingByKey: %w", err)
	}
	return id, true, nil
}

func (r *Repository) CreateReceiving(ctx context.Context, tx pgx.Tx, outletID, code string, poID *string,
	supplierID, userID, key string, invoice, note *string) (string, error) {
	const q = `
		INSERT INTO purchase_receivings (outlet_id, code, purchase_order_id, supplier_id,
		                                 received_by, idempotency_key, supplier_invoice_no, note)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id`

	var id string
	if err := tx.QueryRow(ctx, q, outletID, code, poID, supplierID, userID, key, invoice, note).Scan(&id); err != nil {
		return "", fmt.Errorf("createReceiving: %w", err)
	}
	return id, nil
}

func (r *Repository) InsertReceivingItem(ctx context.Context, tx pgx.Tx, receivingID string, poItemID *string,
	ingredientID string, unitID int, qtyReceived, baseQty, unitPrice, discount, netUnitCost, lineTotal, movementID string) error {
	const q = `
		INSERT INTO purchase_receiving_items (receiving_id, po_item_id, ingredient_id, purchase_unit_id,
		                                      quantity_received, base_quantity, unit_price, discount_allocated,
		                                      net_unit_cost_base, line_total, stock_movement_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`

	if _, err := tx.Exec(ctx, q, receivingID, poItemID, ingredientID, unitID, qtyReceived, baseQty,
		unitPrice, discount, netUnitCost, lineTotal, movementID); err != nil {
		return fmt.Errorf("insertReceivingItem: %w", err)
	}
	return nil
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

func (r *Repository) InsertPurchaseMovement(ctx context.Context, tx pgx.Tx, outletID, ingredientID, receivingID,
	delta, unitCost, totalCost, balance, avgAfter, userID string) (string, error) {
	const q = `
		INSERT INTO stock_movements (outlet_id, ingredient_id, movement_type, quantity_delta,
		                             unit_cost, total_cost, balance_after, avg_cost_after,
		                             reference_type, reference_id, created_by)
		VALUES ($1, $2, 'PURCHASE', $3, $4, $5, $6, $7, 'PURCHASE', $8, $9)
		RETURNING id`

	var id string
	if err := tx.QueryRow(ctx, q, outletID, ingredientID, delta, unitCost, totalCost, balance, avgAfter,
		receivingID, userID).Scan(&id); err != nil {
		return "", fmt.Errorf("insertPurchaseMovement: %w", err)
	}
	return id, nil
}

func (r *Repository) UpdateInventoryMWAC(ctx context.Context, tx pgx.Tx, outletID, ingredientID,
	balance, avg, lastCost string) error {
	if _, err := tx.Exec(ctx, `
		UPDATE inventory
		SET quantity_on_hand = $3, avg_unit_cost = $4, last_purchase_cost = $5,
		    last_movement_at = now(), version = version + 1, updated_at = now()
		WHERE outlet_id = $1 AND ingredient_id = $2`,
		outletID, ingredientID, balance, avg, lastCost); err != nil {
		return fmt.Errorf("updateInventoryMWAC: %w", err)
	}
	return nil
}

func (r *Repository) InsertCostHistory(ctx context.Context, tx pgx.Tx, outletID, ingredientID, receivingID,
	unitCost, avgAfter, qtyReceived string) error {
	if _, err := tx.Exec(ctx, `
		INSERT INTO ingredient_cost_history (ingredient_id, outlet_id, source, reference_id,
		                                     unit_cost, avg_cost_after, quantity_received)
		VALUES ($1, $2, 'PURCHASE', $3, $4, $5, $6)`,
		ingredientID, outletID, receivingID, unitCost, avgAfter, qtyReceived); err != nil {
		return fmt.Errorf("insertCostHistory: %w", err)
	}
	return nil
}

func (r *Repository) Receiving(ctx context.Context, outletID, id string) (*Receiving, error) {
	const q = `
		SELECT pr.id, pr.code, pr.purchase_order_id, po.code, pr.supplier_id, s.name,
		       pr.supplier_invoice_no, pr.note, pr.received_at
		FROM purchase_receivings pr
		JOIN suppliers s ON s.id = pr.supplier_id
		LEFT JOIN purchase_orders po ON po.id = pr.purchase_order_id
		WHERE pr.id = $1 AND pr.outlet_id = $2`

	o := &Receiving{}
	err := r.pool.QueryRow(ctx, q, id, outletID).Scan(&o.ID, &o.Code, &o.PurchaseOrderID,
		&o.PurchaseOrderCode, &o.SupplierID, &o.SupplierName, &o.SupplierInvoiceNo, &o.Note, &o.ReceivedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("receiving: %w", err)
	}
	items, err := r.receivingItems(ctx, id)
	if err != nil {
		return nil, err
	}
	o.Items = items
	return o, nil
}

func (r *Repository) receivingItems(ctx context.Context, receivingID string) ([]ReceivingItem, error) {
	const q = `
		SELECT pri.id, pri.ingredient_id, i.code, i.name, pri.quantity_received::text,
		       pri.base_quantity::text, pri.unit_price::text, pri.net_unit_cost_base::text,
		       pri.line_total::text
		FROM purchase_receiving_items pri
		JOIN ingredients i ON i.id = pri.ingredient_id
		WHERE pri.receiving_id = $1
		ORDER BY i.name`

	rows, err := r.pool.Query(ctx, q, receivingID)
	if err != nil {
		return nil, fmt.Errorf("receivingItems: %w", err)
	}
	defer rows.Close()

	items := []ReceivingItem{}
	for rows.Next() {
		var it ReceivingItem
		if err := rows.Scan(&it.ID, &it.IngredientID, &it.IngredientCode, &it.IngredientName,
			&it.QuantityReceived, &it.BaseQuantity, &it.UnitPrice, &it.NetUnitCostBase, &it.LineTotal); err != nil {
			return nil, fmt.Errorf("receivingItems: scan: %w", err)
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

// ApproveSettings membaca ambang nilai PO yang wajib persetujuan terpisah.
func (r *Repository) ApproveSettings(ctx context.Context, outletID string) (threshold string, allowSelf bool, err error) {
	const q = `SELECT po_approval_threshold::text, allow_self_approval_po
	           FROM outlet_settings WHERE outlet_id = $1`
	err = r.pool.QueryRow(ctx, q, outletID).Scan(&threshold, &allowSelf)
	if errors.Is(err, pgx.ErrNoRows) {
		return "0", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("approveSettings: %w", err)
	}
	return threshold, allowSelf, nil
}
