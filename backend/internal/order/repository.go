package order

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("pesanan tidak ditemukan")

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
		                    note, priority, created_by, service_charge_rate, tax_rate,
		                    tax_inclusive, pricing_snapshot)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING id`

	var id string
	if err := r.pool.QueryRow(ctx, q, outletID, in.ServiceType, p.PaymentTiming, in.TableID,
		in.CustomerName, in.Note, in.Priority, userID, p.ServiceRate, p.TaxRate,
		p.TaxInclusive, snapshot).Scan(&id); err != nil {
		return nil, fmt.Errorf("createDraft: %w", err)
	}
	return r.Order(ctx, outletID, id)
}

func (r *Repository) Order(ctx context.Context, outletID, id string) (*Order, error) {
	const q = `
		SELECT id, order_no, queue_no, service_type, status, payment_status, payment_timing,
		       table_id, customer_name, note, priority, subtotal_amount::text,
		       item_discount_amount::text, order_discount_amount::text,
		       service_charge_rate::text, service_charge_amount::text, tax_rate::text,
		       tax_amount::text, tax_inclusive, rounding_amount::text, total_amount::text,
		       paid_amount::text, total_cogs::text, created_at, confirmed_at, pricing_snapshot
		FROM orders WHERE id = $1 AND outlet_id = $2`

	o := &Order{}
	var snapshot []byte
	err := r.pool.QueryRow(ctx, q, id, outletID).Scan(&o.ID, &o.OrderNo, &o.QueueNo,
		&o.ServiceType, &o.Status, &o.PaymentStatus, &o.PaymentTiming, &o.TableID,
		&o.CustomerName, &o.Note, &o.Priority, &o.SubtotalAmount, &o.ItemDiscount,
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
		       modifiers_total::text, discount_amount::text, line_total::text, status, note
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
			&item.ModifiersTotal, &item.DiscountAmount, &item.LineTotal, &item.Status,
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

func (r *Repository) List(ctx context.Context, outletID string, f Filter) ([]Order, int, error) {
	const q = `
		SELECT o.id, o.order_no, o.queue_no, o.service_type, o.status, o.payment_status,
		       o.payment_timing, o.table_id, o.customer_name, o.note, o.priority,
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
			&o.PaymentStatus, &o.PaymentTiming, &o.TableID, &o.CustomerName, &o.Note,
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
