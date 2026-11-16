package order

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ryunoo09/pos-food-beverage/backend/internal/auth"
	"github.com/ryunoo09/pos-food-beverage/backend/internal/pkg/money"
)

var ErrNotFound = errors.New("pesanan tidak ditemukan")

// ErrNotDraft muncul saat operasi tulis kalah balapan dengan confirm: baris
// pesanan sudah bukan DRAFT ketika INSERT/UPDATE dijalankan.
var ErrNotDraft = errors.New("pesanan sudah dikonfirmasi")

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// Pricing mengumpulkan pengaturan penagihan outlet: timing bayar, tarif pajak
// dan service yang aktif, pembulatan, serta sifat pajak.
func (r *Repository) Pricing(ctx context.Context, outletID, serviceType string) (pricingSettings, error) {
	const q = `
		SELECT s.payment_timing_dine_in, s.payment_timing_takeaway,
		       s.payment_timing_pickup, s.payment_timing_delivery,
		       s.tax_inclusive, s.tax_on_service, s.rounding_mode,
		       COALESCE((SELECT t.rate_percent::text FROM tax_configs t
		                 WHERE t.outlet_id = s.outlet_id AND t.type = 'TAX' AND t.is_active
		                   AND (t.applies_to_service_types IS NULL OR t.applies_to_service_types = '{}'
		                        OR $2 = ANY(t.applies_to_service_types))
		                 ORDER BY t.effective_from DESC LIMIT 1), '0'),
		       COALESCE((SELECT t.rate_percent::text FROM tax_configs t
		                 WHERE t.outlet_id = s.outlet_id AND t.type = 'SERVICE' AND t.is_active
		                   AND (t.applies_to_service_types IS NULL OR t.applies_to_service_types = '{}'
		                        OR $2 = ANY(t.applies_to_service_types))
		                 ORDER BY t.effective_from DESC LIMIT 1), '0')
		FROM outlet_settings s
		WHERE s.outlet_id = $1`

	var dineIn, takeaway, pickup, delivery string
	p := pricingSettings{}
	err := r.pool.QueryRow(ctx, q, outletID, serviceType).Scan(&dineIn, &takeaway, &pickup,
		&delivery, &p.TaxInclusive, &p.TaxOnService, &p.RoundingMode, &p.TaxRate, &p.ServiceRate)
	if errors.Is(err, pgx.ErrNoRows) {
		return pricingSettings{}, ErrNotFound
	}
	if err != nil {
		return pricingSettings{}, fmt.Errorf("pricing: %w", err)
	}

	switch serviceType {
	case "DINE_IN":
		p.PaymentTiming = dineIn
	case "TAKEAWAY":
		p.PaymentTiming = takeaway
	case "PICKUP":
		p.PaymentTiming = pickup
	case "DELIVERY":
		p.PaymentTiming = delivery
	default:
		return pricingSettings{}, fmt.Errorf("pricing: service type tidak dikenal %q", serviceType)
	}
	return p, nil
}

func (r *Repository) CreateDraft(ctx context.Context, outletID, userID string, in DraftInput, p pricingSettings) (*Order, error) {
	snapshot, err := json.Marshal(pricingSnapshot{
		PaymentTiming: p.PaymentTiming,
		TaxOnService:  p.TaxOnService,
		RoundingMode:  p.RoundingMode,
	})
	if err != nil {
		return nil, fmt.Errorf("createDraft: snapshot: %w", err)
	}

	const q = `
		INSERT INTO orders (outlet_id, service_type, payment_timing, table_id, customer_name,
		                    customer_phone, delivery_address, note, priority, created_by,
		                    service_charge_rate, tax_rate, tax_inclusive, pricing_snapshot)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		RETURNING id`

	var id string
	if err := r.pool.QueryRow(ctx, q, outletID, in.ServiceType, p.PaymentTiming, in.TableID,
		in.CustomerName, in.CustomerPhone, in.DeliveryAddress, in.Note, in.Priority, userID,
		p.ServiceRate, p.TaxRate, p.TaxInclusive, snapshot).Scan(&id); err != nil {
		return nil, fmt.Errorf("createDraft: %w", err)
	}
	return r.Order(ctx, outletID, id)
}

func (r *Repository) Order(ctx context.Context, outletID, id string) (*Order, error) {
	const q = `
		SELECT id, order_no, queue_no, service_type, status, payment_status, payment_timing,
		       table_id, customer_name, customer_phone, delivery_address, note, priority,
		       subtotal_amount::text, item_discount_amount::text, order_discount_amount::text,
		       service_charge_rate::text, service_charge_amount::text, tax_rate::text,
		       tax_amount::text, tax_inclusive, rounding_amount::text, total_amount::text,
		       paid_amount::text, total_cogs::text, created_at, confirmed_at, pricing_snapshot
		FROM orders WHERE id = $1 AND outlet_id = $2`

	o := &Order{}
	var snapshot []byte
	err := r.pool.QueryRow(ctx, q, id, outletID).Scan(&o.ID, &o.OrderNo, &o.QueueNo,
		&o.ServiceType, &o.Status, &o.PaymentStatus, &o.PaymentTiming, &o.TableID,
		&o.CustomerName, &o.CustomerPhone, &o.DelivAddress, &o.Note, &o.Priority,
		&o.SubtotalAmount, &o.ItemDiscount,
		&o.OrderDiscount, &o.ServiceRate, &o.ServiceAmount, &o.TaxRate, &o.TaxAmount,
		&o.TaxInclusive, &o.RoundingAmount, &o.TotalAmount, &o.PaidAmount, &o.TotalCogs,
		&o.CreatedAt, &o.ConfirmedAt, &snapshot)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("order: %w", err)
	}
	if err := json.Unmarshal(snapshot, &o.pricing); err != nil {
		return nil, fmt.Errorf("order: pricing snapshot: %w", err)
	}

	items, err := r.items(ctx, id)
	if err != nil {
		return nil, err
	}
	o.Items = items
	return o, nil
}

func (r *Repository) items(ctx context.Context, orderID string) ([]OrderItem, error) {
	const q = `
		SELECT id, round_no, product_id, variant_id, product_name_snapshot,
		       variant_name_snapshot, sku_snapshot, quantity, unit_price::text,
		       modifiers_total::text, discount_amount::text, line_total::text,
		       unit_cogs::text, line_cogs::text, cost_unknown, status, note
		FROM order_items
		WHERE order_id = $1
		ORDER BY created_at`

	rows, err := r.pool.Query(ctx, q, orderID)
	if err != nil {
		return nil, fmt.Errorf("orderItems: %w", err)
	}
	defer rows.Close()

	items := []OrderItem{}
	ids := []string{}
	for rows.Next() {
		var item OrderItem
		if err := rows.Scan(&item.ID, &item.RoundNo, &item.ProductID, &item.VariantID,
			&item.ProductName, &item.VariantName, &item.SKU, &item.Quantity, &item.UnitPrice,
			&item.ModifiersTotal, &item.DiscountAmount, &item.LineTotal,
			&item.UnitCogs, &item.LineCogs, &item.CostUnknown, &item.Status,
			&item.Note); err != nil {
			return nil, fmt.Errorf("scan order item: %w", err)
		}
		items = append(items, item)
		ids = append(ids, item.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if len(ids) == 0 {
		return items, nil
	}

	const mq = `
		SELECT id, order_item_id, option_id, group_name_snapshot, option_name_snapshot,
		       price_delta::text, quantity
		FROM order_item_modifiers
		WHERE order_item_id = ANY($1::uuid[])
		ORDER BY option_name_snapshot`

	mrows, err := r.pool.Query(ctx, mq, ids)
	if err != nil {
		return nil, fmt.Errorf("orderItemModifiers: %w", err)
	}
	defer mrows.Close()

	byItem := map[string][]ItemModifier{}
	for mrows.Next() {
		var itemID string
		var m ItemModifier
		if err := mrows.Scan(&m.ID, &itemID, &m.OptionID, &m.GroupName, &m.OptionName,
			&m.PriceDelta, &m.Quantity); err != nil {
			return nil, fmt.Errorf("scan order item modifier: %w", err)
		}
		byItem[itemID] = append(byItem[itemID], m)
	}
	if err := mrows.Err(); err != nil {
		return nil, err
	}

	for i := range items {
		items[i].Modifiers = byItem[items[i].ID]
	}
	return items, nil
}

// SaleVariant memuat harga jual varian beserta snapshot nama dan SKU.
func (r *Repository) SaleVariant(ctx context.Context, businessID, productID, variantID string) (saleVariant, error) {
	const q = `
		SELECT p.id, v.id, p.name, v.name,
		       p.sku || COALESCE('-' || v.sku_suffix, ''), v.price::text
		FROM product_variants v
		JOIN products p ON p.id = v.product_id
		WHERE p.id = $1 AND v.id = $2 AND p.business_id = $3
		  AND p.is_active AND p.is_available AND v.is_active`

	var sv saleVariant
	err := r.pool.QueryRow(ctx, q, productID, variantID, businessID).Scan(&sv.ProductID,
		&sv.VariantID, &sv.ProductName, &sv.VariantName, &sv.SKU, &sv.UnitPrice)
	if errors.Is(err, pgx.ErrNoRows) {
		return saleVariant{}, ErrNotFound
	}
	if err != nil {
		return saleVariant{}, fmt.Errorf("saleVariant: %w", err)
	}
	return sv, nil
}

// TableOwned memastikan meja milik outlet dan aktif.
func (r *Repository) TableOwned(ctx context.Context, outletID, tableID string) (bool, error) {
	const q = `SELECT EXISTS (SELECT 1 FROM tables WHERE id = $1 AND outlet_id = $2 AND is_active)`

	var ok bool
	if err := r.pool.QueryRow(ctx, q, tableID, outletID).Scan(&ok); err != nil {
		return false, fmt.Errorf("tableOwned: %w", err)
	}
	return ok, nil
}

type modifierChoice struct {
	ID         string
	GroupName  string
	OptionName string
	PriceDelta string
}

func (r *Repository) ModifierOptions(ctx context.Context, businessID string, ids []string) ([]modifierChoice, error) {
	const q = `
		SELECT o.id, g.name, o.name, o.price_delta::text
		FROM modifier_options o
		JOIN modifier_groups g ON g.id = o.group_id
		WHERE o.id = ANY($1::uuid[]) AND g.business_id = $2 AND o.is_active AND g.is_active`

	rows, err := r.pool.Query(ctx, q, ids, businessID)
	if err != nil {
		return nil, fmt.Errorf("modifierOptions: %w", err)
	}
	defer rows.Close()

	choices := []modifierChoice{}
	for rows.Next() {
		var c modifierChoice
		if err := rows.Scan(&c.ID, &c.GroupName, &c.OptionName, &c.PriceDelta); err != nil {
			return nil, fmt.Errorf("scan modifier choice: %w", err)
		}
		choices = append(choices, c)
	}
	return choices, rows.Err()
}

func (r *Repository) AddItem(ctx context.Context, orderID string, sv saleVariant, in ItemInput, choices []modifierChoice, modifiersTotal, lineTotal string) (string, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("addItem: begin: %w", err)
	}
	defer tx.Rollback(ctx)

	// Kunci baris pesanan lebih dulu agar bersaing langsung dengan confirm:
	// confirm memegang FOR UPDATE selama memotong stok, jadi penambahan item
	// akan menunggu lalu melihat status terbaru, bukan DRAFT yang sudah basi.
	var status string
	lockErr := tx.QueryRow(ctx,
		`SELECT status FROM orders WHERE id = $1 FOR UPDATE`, orderID).Scan(&status)
	if errors.Is(lockErr, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	if lockErr != nil {
		return "", fmt.Errorf("addItem: lock order: %w", lockErr)
	}
	if status != "DRAFT" {
		return "", ErrNotDraft
	}

	const q = `
		INSERT INTO order_items (order_id, product_id, variant_id, product_name_snapshot,
		                         variant_name_snapshot, sku_snapshot, quantity, unit_price,
		                         modifiers_total, line_total, note)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id`

	var id string
	if err := tx.QueryRow(ctx, q, orderID, sv.ProductID, sv.VariantID, sv.ProductName,
		sv.VariantName, sv.SKU, in.Quantity, sv.UnitPrice, modifiersTotal, lineTotal,
		in.Note).Scan(&id); err != nil {
		return "", fmt.Errorf("addItem: %w", err)
	}

	for _, c := range choices {
		const mq = `
			INSERT INTO order_item_modifiers (order_item_id, option_id, group_name_snapshot,
			                                 option_name_snapshot, price_delta)
			VALUES ($1, $2, $3, $4, $5)`
		if _, err := tx.Exec(ctx, mq, id, c.ID, c.GroupName, c.OptionName, c.PriceDelta); err != nil {
			return "", fmt.Errorf("addItem: modifier: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("addItem: commit: %w", err)
	}
	return id, nil
}

func (r *Repository) UpdateItemQuantity(ctx context.Context, orderID, itemID string, quantity int, note *string) (bool, error) {
	const q = `
		UPDATE order_items oi
		SET quantity = $3::int,
		    note = COALESCE($4::text, oi.note),
		    line_total = (unit_price + modifiers_total) * $3::int,
		    updated_at = now()
		FROM orders o
		WHERE oi.id = $1 AND o.id = oi.order_id AND o.id = $2 AND o.status = 'DRAFT'
		RETURNING oi.id`

	var id string
	err := r.pool.QueryRow(ctx, q, itemID, orderID, quantity, note).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("updateItemQuantity: %w", err)
	}
	return true, nil
}

func (r *Repository) UpdateItemNote(ctx context.Context, orderID, itemID string, note *string) (bool, error) {
	const q = `
		UPDATE order_items oi
		SET note = $3::text, updated_at = now()
		FROM orders o
		WHERE oi.id = $1 AND o.id = oi.order_id AND o.id = $2 AND o.status = 'DRAFT'
		RETURNING oi.id`

	var id string
	err := r.pool.QueryRow(ctx, q, itemID, orderID, note).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("updateItemNote: %w", err)
	}
	return true, nil
}

func (r *Repository) DeleteItem(ctx context.Context, orderID, itemID string) (bool, error) {
	const q = `
		DELETE FROM order_items oi
		USING orders o
		WHERE oi.id = $1 AND o.id = oi.order_id AND o.id = $2 AND o.status = 'DRAFT'`

	tag, err := r.pool.Exec(ctx, q, itemID, orderID)
	if err != nil {
		return false, fmt.Errorf("deleteItem: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

// SaveTotals menyimpan hasil kalkulasi server-side ke pesanan draft.
func (r *Repository) SaveTotals(ctx context.Context, orderID string, t orderTotals) error {
	const q = `
		UPDATE orders
		SET subtotal_amount = $2, item_discount_amount = $3, order_discount_amount = $4,
		    service_charge_amount = $5, tax_amount = $6, rounding_amount = $7,
		    total_amount = $8, updated_at = now()
		WHERE id = $1`

	if _, err := r.pool.Exec(ctx, q, orderID, t.Subtotal, t.ItemDiscount, t.OrderDiscount,
		t.Service, t.Tax, t.Rounding, t.Total); err != nil {
		return fmt.Errorf("saveTotals: %w", err)
	}
	return nil
}

type orderTotals struct {
	Subtotal      string
	ItemDiscount  string
	OrderDiscount string
	Service       string
	Tax           string
	Rounding      string
	Total         string
}

// querier adalah subset pgxpool.Pool atau pgx.Tx yang dipakai helper repository.
type querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// LockOrder mengunci pesanan untuk pembaruan dan mengembalikan status serta meja.
func (r *Repository) LockOrder(ctx context.Context, tx pgx.Tx, outletID, orderID string) (string, *string, bool, error) {
	const q = `SELECT status, table_id FROM orders WHERE id = $1 AND outlet_id = $2 FOR UPDATE`
	var status string
	var tableID *string
	err := tx.QueryRow(ctx, q, orderID, outletID).Scan(&status, &tableID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil, false, nil
	}
	if err != nil {
		return "", nil, false, fmt.Errorf("lockOrder: %w", err)
	}
	return status, tableID, true, nil
}

// UpdateOrderTable memindahkan pesanan ke meja lain.
func (r *Repository) UpdateOrderTable(ctx context.Context, tx pgx.Tx, orderID, tableID string) error {
	const q = `UPDATE orders SET table_id = $2, version = version + 1, updated_at = now() WHERE id = $1`
	if _, err := tx.Exec(ctx, q, orderID, tableID); err != nil {
		return fmt.Errorf("updateOrderTable: %w", err)
	}
	return nil
}

// SetTableStatus mengubah status meja milik outlet.
func (r *Repository) SetTableStatus(ctx context.Context, q querier, outletID, tableID, status string) error {
	const sql = `UPDATE tables SET status = $3, updated_at = now() WHERE id = $1 AND outlet_id = $2`
	if _, err := q.Exec(ctx, sql, tableID, outletID, status); err != nil {
		return fmt.Errorf("setTableStatus: %w", err)
	}
	return nil
}

// OutletInfo mengambil identitas outlet untuk header struk.
func (r *Repository) OutletInfo(ctx context.Context, outletID string) (*ReceiptOutlet, error) {
	const q = `SELECT id, code, name, address, phone FROM outlets WHERE id = $1`
	o := &ReceiptOutlet{}
	err := r.pool.QueryRow(ctx, q, outletID).Scan(&o.ID, &o.Code, &o.Name, &o.Address, &o.Phone)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("outletInfo: %w", err)
	}
	return o, nil
}

// Payments mengambil pembayaran berhasil pesanan untuk struk.
func (r *Repository) Payments(ctx context.Context, orderID string) ([]Payment, error) {
	const q = `
		SELECT id, method_code, amount::text, status, reference_no, paid_at
		FROM payments WHERE order_id = $1 AND status = 'SUCCESS'
		ORDER BY created_at`

	rows, err := r.pool.Query(ctx, q, orderID)
	if err != nil {
		return nil, fmt.Errorf("payments: %w", err)
	}
	defer rows.Close()

	out := []Payment{}
	for rows.Next() {
		var p Payment
		if err := rows.Scan(&p.ID, &p.MethodCode, &p.Amount, &p.Status, &p.ReferenceNo, &p.PaidAt); err != nil {
			return nil, fmt.Errorf("payments: scan: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// InsertAudit menulis audit log; kegagalan dicatat pemanggil tanpa membatalkan aksi.
func (r *Repository) InsertAudit(ctx context.Context, action, entityType, entityID string, a auth.Actor, after any) error {
	const q = `
		INSERT INTO audit_logs (business_id, user_id, action, entity_type, entity_id,
		                        after_data, ip_address, user_agent, request_id)
		VALUES ($1, NULLIF($2, '')::uuid, $3, NULLIF($4, ''), NULLIF($5, ''),
		        $6, NULLIF($7, '')::inet, NULLIF($8, ''), NULLIF($9, ''))`

	_, err := r.pool.Exec(ctx, q, a.BusinessID, a.UserID, action, entityType, entityID,
		after, a.IPAddress, a.UserAgent, a.RequestID)
	if err != nil {
		return fmt.Errorf("insertAudit: %w", err)
	}
	return nil
}

// NextSequence menaikkan counter dokumen secara atomik per outlet dan periode.
// Nomor boleh melompat saat rollback, tetapi tidak pernah ganda.
func (r *Repository) NextSequence(ctx context.Context, q querier, outletID, docType, period string) (int, error) {
	const sql = `
		INSERT INTO document_sequences (outlet_id, doc_type, period, last_number)
		VALUES ($1, $2, $3, 1)
		ON CONFLICT (outlet_id, doc_type, period)
		DO UPDATE SET last_number = document_sequences.last_number + 1
		RETURNING last_number`

	var n int
	if err := q.QueryRow(ctx, sql, outletID, docType, period).Scan(&n); err != nil {
		return 0, fmt.Errorf("nextSequence: %w", err)
	}
	return n, nil
}

// Begin membuka transaksi supaya penomoran dan perubahan status atomik.
func (r *Repository) Begin(ctx context.Context) (pgx.Tx, error) {
	return r.pool.Begin(ctx)
}

// SetConfirmed mengunci draft menjadi CONFIRMED dengan nomor resmi dan snapshot harga.
func (r *Repository) SetConfirmed(ctx context.Context, tx pgx.Tx, orderID, userID, no string, queueNo *int, snapshot []byte) error {
	const sql = `
		UPDATE orders
		SET status = 'CONFIRMED', order_no = $2, queue_no = $3, confirmed_by = $4,
		    confirmed_at = now(), pricing_snapshot = $5, version = version + 1, updated_at = now()
		WHERE id = $1 AND status = 'DRAFT'`

	if _, err := tx.Exec(ctx, sql, orderID, no, queueNo, userID, snapshot); err != nil {
		return fmt.Errorf("setConfirmed: %w", err)
	}
	return nil
}

// InsertStatusHistory mencatat perpindahan status (append-only).
func (r *Repository) InsertStatusHistory(ctx context.Context, tx pgx.Tx, orderID, from, to, userID string) error {
	const sql = `
		INSERT INTO order_status_histories (order_id, from_status, to_status, changed_by)
		VALUES ($1, $2, $3, $4)`
	if _, err := tx.Exec(ctx, sql, orderID, from, to, userID); err != nil {
		return fmt.Errorf("insertStatusHistory: %w", err)
	}
	return nil
}

// CreateKitchenOrder membuat tiket dapur round 1 berisi item ACTIVE pesanan.
func (r *Repository) CreateKitchenOrder(ctx context.Context, tx pgx.Tx, outletID, orderID, priority string) error {
	var kitchenID string
	const insOrder = `
		INSERT INTO kitchen_orders (outlet_id, order_id, round_no, priority)
		VALUES ($1, $2, 1, $3)
		ON CONFLICT (order_id, round_no) DO NOTHING
		RETURNING id`
	err := tx.QueryRow(ctx, insOrder, outletID, orderID, priority).Scan(&kitchenID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil // tiket sudah ada, idempoten
	}
	if err != nil {
		return fmt.Errorf("createKitchenOrder: %w", err)
	}

	const insItems = `
		INSERT INTO kitchen_order_items (kitchen_order_id, order_item_id, name_snapshot,
		                                 variant_snapshot, modifiers_text, note, quantity, station)
		SELECT $1, oi.id, oi.product_name_snapshot, oi.variant_name_snapshot,
		       (SELECT string_agg(m.option_name_snapshot, ', ' ORDER BY m.option_name_snapshot)
		          FROM order_item_modifiers m WHERE m.order_item_id = oi.id),
		       oi.note, oi.quantity, p.kitchen_station
		FROM order_items oi
		JOIN products p ON p.id = oi.product_id
		WHERE oi.order_id = $2 AND oi.status = 'ACTIVE'`
	if _, err := tx.Exec(ctx, insItems, kitchenID, orderID); err != nil {
		return fmt.Errorf("createKitchenOrder: items: %w", err)
	}
	return nil
}

// ClaimIdempotency menyisipkan kunci baru; false berarti kunci sudah ada.
func (r *Repository) ClaimIdempotency(ctx context.Context, scope, key string, hash []byte) (bool, error) {
	tag, err := r.pool.Exec(ctx, `
		INSERT INTO idempotency_keys (scope, key, request_hash, expires_at)
		VALUES ($1, $2, $3, now() + interval '24 hours')
		ON CONFLICT (scope, key) DO NOTHING`, scope, key, hash)
	if err != nil {
		return false, fmt.Errorf("claimIdempotency: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

// IdempotencyEntry mengambil hasil tersimpan; status 0 berarti masih diproses.
func (r *Repository) IdempotencyEntry(ctx context.Context, scope, key string) ([]byte, int, []byte, bool, error) {
	var hash, body []byte
	var status *int
	err := r.pool.QueryRow(ctx, `
		SELECT request_hash, response_status, response_body
		FROM idempotency_keys WHERE scope = $1 AND key = $2`, scope, key).Scan(&hash, &status, &body)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, 0, nil, false, nil
	}
	if err != nil {
		return nil, 0, nil, false, fmt.Errorf("idempotencyEntry: %w", err)
	}
	if status == nil {
		return hash, 0, nil, true, nil
	}
	return hash, *status, body, true, nil
}

// SaveIdempotencyResponse menyimpan hasil akhir agar permintaan ulang mengembalikan yang sama.
func (r *Repository) SaveIdempotencyResponse(ctx context.Context, scope, key string, status int, body []byte) error {
	const q = `UPDATE idempotency_keys SET response_status = $3, response_body = $4
	           WHERE scope = $1 AND key = $2`
	if _, err := r.pool.Exec(ctx, q, scope, key, status, body); err != nil {
		return fmt.Errorf("saveIdempotencyResponse: %w", err)
	}
	return nil
}

// ReleaseIdempotency menghapus klaim supaya percobaan berikutnya bisa memproses ulang.
func (r *Repository) ReleaseIdempotency(ctx context.Context, scope, key string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM idempotency_keys WHERE scope = $1 AND key = $2`, scope, key)
	if err != nil {
		return fmt.Errorf("releaseIdempotency: %w", err)
	}
	return nil
}

// KitchenStarted melaporkan apakah ada item pesanan yang sudah mulai dimasak.
func (r *Repository) KitchenStarted(ctx context.Context, tx pgx.Tx, orderID string) (bool, error) {
	var started bool
	err := tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM kitchen_order_items koi
			JOIN kitchen_orders ko ON ko.id = koi.kitchen_order_id
			WHERE ko.order_id = $1 AND koi.status IN ('COOKING', 'READY', 'SERVED')
		)`, orderID).Scan(&started)
	if err != nil {
		return false, fmt.Errorf("kitchenStarted: %w", err)
	}
	return started, nil
}

// ReleaseConsumption melepas konsumsi bahan jadi stok lewat movement
// SALE_REVERSAL. itemID nil berarti seluruh pesanan. mode WASTE menambah
// movement WASTE setelah pengembalian sehingga saldo bersih nol tetapi susut
// tetap tercatat (butuh reason).
func (r *Repository) ReleaseConsumption(ctx context.Context, tx pgx.Tx, outletID, orderID, userID string, itemID *string, waste bool, reason *string) error {
	const q = `
		SELECT oic.id, oic.ingredient_id::text,
		       (oic.quantity_base - oic.reversed_quantity_base)::text, oic.unit_cost::text
		FROM order_item_consumptions oic
		JOIN order_items oi ON oi.id = oic.order_item_id
		WHERE oi.order_id = $1 AND oic.quantity_base > oic.reversed_quantity_base
		  AND ($2::uuid IS NULL OR oi.id = $2)`

	rows, err := tx.Query(ctx, q, orderID, itemID)
	if err != nil {
		return fmt.Errorf("releaseConsumption: query: %w", err)
	}
	type row struct {
		id       string
		ingID    string
		qty      money.Decimal
		unitCost money.Decimal
	}
	pending := []row{}
	perIngredient := map[string]money.Decimal{}
	for rows.Next() {
		var rr row
		var qtyStr, costStr string
		if err := rows.Scan(&rr.id, &rr.ingID, &qtyStr, &costStr); err != nil {
			rows.Close()
			return fmt.Errorf("releaseConsumption: scan: %w", err)
		}
		if rr.qty, err = money.Parse(qtyStr); err != nil {
			rows.Close()
			return err
		}
		if rr.unitCost, err = money.Parse(costStr); err != nil {
			rows.Close()
			return err
		}
		pending = append(pending, rr)
		perIngredient[rr.ingID] = perIngredient[rr.ingID].Add(rr.qty)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	ingredients := make([]string, 0, len(perIngredient))
	for id := range perIngredient {
		ingredients = append(ingredients, id)
	}
	sort.Strings(ingredients)

	for _, ingID := range ingredients {
		back := perIngredient[ingID]
		if !back.IsPositive() {
			continue
		}
		var onHandStr, avgStr string
		const lock = `SELECT quantity_on_hand::text, avg_unit_cost::text
		              FROM inventory WHERE outlet_id = $1 AND ingredient_id = $2 FOR UPDATE`
		if err := tx.QueryRow(ctx, lock, outletID, ingID).Scan(&onHandStr, &avgStr); err != nil {
			return fmt.Errorf("releaseConsumption: lock: %w", err)
		}
		onHand, err := money.Parse(onHandStr)
		if err != nil {
			return err
		}
		avg, err := money.Parse(avgStr)
		if err != nil {
			return err
		}
		balance := onHand.Add(back)
		totalCost := money.RoundHalfUp(back.Mul(avg), 2)

		const movement = `
			INSERT INTO stock_movements (outlet_id, ingredient_id, movement_type, quantity_delta,
			                             unit_cost, total_cost, balance_after, avg_cost_after,
			                             reference_type, reference_id, created_by)
			VALUES ($1, $2, 'SALE_REVERSAL', $3, $4, $5, $6, $4, 'ORDER', $7, $8)`
		if _, err := tx.Exec(ctx, movement, outletID, ingID, back.StringFixed(4),
			avg.StringFixed(6), money.Format(totalCost), balance.StringFixed(4),
			orderID, userID); err != nil {
			return fmt.Errorf("releaseConsumption: movement: %w", err)
		}

		if waste {
			// Barang kembali ke stok lalu langsung dicatat susut: saldo bersih tetap.
			balance = balance.Sub(back)
			const wasteQ = `
				INSERT INTO stock_movements (outlet_id, ingredient_id, movement_type, quantity_delta,
				                             unit_cost, total_cost, balance_after, avg_cost_after,
				                             reference_type, reference_id, reason, created_by)
				VALUES ($1, $2, 'WASTE', $3, $4, $5, $6, $4, 'ORDER', $7, $8, $9)`
			if _, err := tx.Exec(ctx, wasteQ, outletID, ingID, back.Neg().StringFixed(4),
				avg.StringFixed(6), money.Format(totalCost), balance.StringFixed(4),
				orderID, reason, userID); err != nil {
				return fmt.Errorf("releaseConsumption: waste: %w", err)
			}
		}

		const update = `
			UPDATE inventory SET quantity_on_hand = $3, last_movement_at = now(),
			       version = version + 1, updated_at = now()
			WHERE outlet_id = $1 AND ingredient_id = $2`
		if _, err := tx.Exec(ctx, update, outletID, ingID, balance.StringFixed(4)); err != nil {
			return fmt.Errorf("releaseConsumption: update: %w", err)
		}
	}

	for _, rr := range pending {
		const mark = `UPDATE order_item_consumptions SET reversed_quantity_base = quantity_base WHERE id = $1`
		if _, err := tx.Exec(ctx, mark, rr.id); err != nil {
			return fmt.Errorf("releaseConsumption: mark: %w", err)
		}
	}
	return nil
}

// KitchenItemStatus mengembalikan status tiket dapur satu item; kosong bila item
// belum pernah dikirim ke dapur.
func (r *Repository) KitchenItemStatus(ctx context.Context, tx pgx.Tx, orderID, itemID string) (string, error) {
	const q = `
		SELECT koi.status
		FROM kitchen_order_items koi
		JOIN kitchen_orders ko ON ko.id = koi.kitchen_order_id
		WHERE ko.order_id = $1 AND koi.order_item_id = $2`
	var status string
	err := tx.QueryRow(ctx, q, orderID, itemID).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("kitchenItemStatus: %w", err)
	}
	return status, nil
}

// SetItemVoid menandai item pesanan VOID bila masih ACTIVE.
func (r *Repository) SetItemVoid(ctx context.Context, tx pgx.Tx, orderID, itemID, userID string, reason *string) (bool, error) {
	const q = `
		UPDATE order_items
		SET status = 'VOID', void_reason = $4, voided_by = $3, voided_at = now(), updated_at = now()
		WHERE id = $2 AND order_id = $1 AND status = 'ACTIVE'
		RETURNING id`
	var id string
	err := tx.QueryRow(ctx, q, orderID, itemID, userID, reason).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("setItemVoid: %w", err)
	}
	return true, nil
}

// InsertItemEvent mencatat event per item (append-only).
func (r *Repository) InsertItemEvent(ctx context.Context, tx pgx.Tx, itemID, eventType, userID string, data []byte) error {
	const q = `INSERT INTO order_item_events (order_item_id, event_type, data, created_by)
	           VALUES ($1, $2, $3, $4)`
	if _, err := tx.Exec(ctx, q, itemID, eventType, data, userID); err != nil {
		return fmt.Errorf("insertItemEvent: %w", err)
	}
	return nil
}

// VoidKitchenItem menandai tiket dapur satu item sebagai VOID.
func (r *Repository) VoidKitchenItem(ctx context.Context, tx pgx.Tx, orderID, itemID string) error {
	const q = `
		UPDATE kitchen_order_items koi SET status = 'VOID'
		FROM kitchen_orders ko
		WHERE ko.id = koi.kitchen_order_id AND ko.order_id = $1 AND koi.order_item_id = $2`
	if _, err := tx.Exec(ctx, q, orderID, itemID); err != nil {
		return fmt.Errorf("voidKitchenItem: %w", err)
	}
	return nil
}

// VoidKitchen menandai tiket dapur terkait pesanan sebagai VOID.
func (r *Repository) VoidKitchen(ctx context.Context, tx pgx.Tx, orderID string) error {
	if _, err := tx.Exec(ctx, `UPDATE kitchen_orders SET status = 'VOID' WHERE order_id = $1`, orderID); err != nil {
		return fmt.Errorf("voidKitchen: orders: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE kitchen_order_items koi SET status = 'VOID'
		FROM kitchen_orders ko
		WHERE ko.id = koi.kitchen_order_id AND ko.order_id = $1`, orderID); err != nil {
		return fmt.Errorf("voidKitchen: items: %w", err)
	}
	return nil
}

// SetCancelled mengubah pesanan menjadi CANCELLED bila belum final.
func (r *Repository) SetCancelled(ctx context.Context, tx pgx.Tx, orderID, userID, code string, note *string) (bool, error) {
	const q = `
		UPDATE orders
		SET status = 'CANCELLED', cancelled_by = $2, cancelled_at = now(),
		    cancel_reason_code = $3, cancel_reason_note = $4, version = version + 1, updated_at = now()
		WHERE id = $1 AND status NOT IN ('COMPLETED', 'CANCELLED')
		RETURNING id`
	var id string
	err := tx.QueryRow(ctx, q, orderID, userID, code, note).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("setCancelled: %w", err)
	}
	return true, nil
}

// AllowNegativeStock membaca setelan outlet; default true bila baris tidak ada.
func (r *Repository) AllowNegativeStock(ctx context.Context, q querier, outletID string) (bool, error) {
	var ok bool
	err := q.QueryRow(ctx,
		`SELECT allow_negative_stock FROM outlet_settings WHERE outlet_id = $1`, outletID).Scan(&ok)
	if errors.Is(err, pgx.ErrNoRows) {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("allowNegativeStock: %w", err)
	}
	return ok, nil
}

func (r *Repository) List(ctx context.Context, outletID string, f Filter) ([]Order, int, error) {
	const q = `
		SELECT o.id, o.order_no, o.queue_no, o.service_type, o.status, o.payment_status,
		       o.payment_timing, o.table_id, o.customer_name, o.customer_phone,
		       o.delivery_address, o.note, o.priority,
		       o.subtotal_amount::text, o.item_discount_amount::text,
		       o.order_discount_amount::text, o.service_charge_rate::text,
		       o.service_charge_amount::text, o.tax_rate::text, o.tax_amount::text,
		       o.tax_inclusive, o.rounding_amount::text, o.total_amount::text,
		       o.paid_amount::text, o.total_cogs::text, o.created_at, o.confirmed_at,
		       count(*) OVER ()::int
		FROM orders o
		WHERE o.outlet_id = $1
		  AND ($2::text IS NULL OR o.status = $2)
		  AND ($3 = '' OR o.customer_name ILIKE '%' || $3 || '%'
		       OR o.order_no ILIKE '%' || $3 || '%')
		ORDER BY o.created_at DESC
		LIMIT $4 OFFSET $5`

	rows, err := r.pool.Query(ctx, q, outletID, f.Status, f.Query, f.PerPage, (f.Page-1)*f.PerPage)
	if err != nil {
		return nil, 0, fmt.Errorf("listOrders: %w", err)
	}
	defer rows.Close()

	items := []Order{}
	total := 0
	for rows.Next() {
		var o Order
		if err := rows.Scan(&o.ID, &o.OrderNo, &o.QueueNo, &o.ServiceType, &o.Status,
			&o.PaymentStatus, &o.PaymentTiming, &o.TableID, &o.CustomerName,
			&o.CustomerPhone, &o.DelivAddress, &o.Note,
			&o.Priority, &o.SubtotalAmount, &o.ItemDiscount, &o.OrderDiscount,
			&o.ServiceRate, &o.ServiceAmount, &o.TaxRate, &o.TaxAmount, &o.TaxInclusive,
			&o.RoundingAmount, &o.TotalAmount, &o.PaidAmount, &o.TotalCogs, &o.CreatedAt,
			&o.ConfirmedAt, &total); err != nil {
			return nil, 0, fmt.Errorf("scan order: %w", err)
		}
		items = append(items, o)
	}
	return items, total, rows.Err()
}
