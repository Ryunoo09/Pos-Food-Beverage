package dashboard

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// Timezone bisnis untuk batas hari operasional.
func (r *Repository) Timezone(ctx context.Context, businessID string) (string, error) {
	const q = `SELECT timezone FROM businesses WHERE id = $1`

	var tz string
	if err := r.pool.QueryRow(ctx, q, businessID).Scan(&tz); err != nil {
		return "", fmt.Errorf("dashboard timezone: %w", err)
	}
	return tz, nil
}

// totals mengabaikan pesanan batal. Gross = subtotal sebelum diskon,
// net = subtotal setelah diskon item dan transaksi (belum pajak/service).
func (r *Repository) totals(ctx context.Context, outletID, tz, from, to string) (totals, error) {
	const q = `
		SELECT COALESCE(SUM(subtotal_amount), 0)::text,
		       COALESCE(SUM(subtotal_amount - item_discount_amount - order_discount_amount), 0)::text,
		       COALESCE(SUM(total_cogs), 0)::text,
		       count(*)::int
		FROM orders
		WHERE outlet_id = $1 AND status = 'COMPLETED'
		  AND (completed_at AT TIME ZONE $2)::date BETWEEN $3::date AND $4::date`

	var t totals
	err := r.pool.QueryRow(ctx, q, outletID, tz, from, to).Scan(&t.GrossSales, &t.NetSales, &t.HPP, &t.Transactions)
	if err != nil {
		return totals{}, fmt.Errorf("dashboard totals: %w", err)
	}
	return t, nil
}

func (r *Repository) methods(ctx context.Context, outletID, tz, from, to string) ([]MethodTotal, error) {
	const q = `
		SELECT p.method_code, COALESCE(SUM(p.amount), 0)::text
		FROM payments p
		JOIN orders o ON o.id = p.order_id
		WHERE p.outlet_id = $1 AND p.status = 'SUCCESS'
		  AND (o.completed_at AT TIME ZONE $2)::date BETWEEN $3::date AND $4::date
		GROUP BY p.method_code
		ORDER BY SUM(p.amount) DESC`

	rows, err := r.pool.Query(ctx, q, outletID, tz, from, to)
	if err != nil {
		return nil, fmt.Errorf("dashboard methods: %w", err)
	}
	defer rows.Close()

	items := []MethodTotal{}
	for rows.Next() {
		var m MethodTotal
		if err := rows.Scan(&m.Method, &m.Amount); err != nil {
			return nil, fmt.Errorf("scan method: %w", err)
		}
		items = append(items, m)
	}
	return items, rows.Err()
}

// hourly selalu 24 titik supaya grafik tidak berlubang.
func (r *Repository) hourly(ctx context.Context, outletID, tz, from, to string) ([]HourPoint, error) {
	const q = `
		SELECT extract(hour FROM completed_at AT TIME ZONE $2)::int AS hour,
		       COALESCE(SUM(total_amount), 0)::text
		FROM orders
		WHERE outlet_id = $1 AND status = 'COMPLETED'
		  AND (completed_at AT TIME ZONE $2)::date BETWEEN $3::date AND $4::date
		GROUP BY hour
		ORDER BY hour`

	rows, err := r.pool.Query(ctx, q, outletID, tz, from, to)
	if err != nil {
		return nil, fmt.Errorf("dashboard hourly: %w", err)
	}
	defer rows.Close()

	found := map[int]string{}
	for rows.Next() {
		var hour int
		var sales string
		if err := rows.Scan(&hour, &sales); err != nil {
			return nil, fmt.Errorf("scan hourly: %w", err)
		}
		found[hour] = sales
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	items := make([]HourPoint, 24)
	for h := range items {
		sales, ok := found[h]
		if !ok {
			sales = "0"
		}
		items[h] = HourPoint{Hour: h, Sales: sales}
	}
	return items, nil
}

func (r *Repository) topProducts(ctx context.Context, outletID, tz, from, to string) ([]ProductTotal, error) {
	const q = `
		SELECT oi.product_id, oi.product_name_snapshot,
		       SUM(oi.quantity)::int, COALESCE(SUM(oi.line_total), 0)::text
		FROM order_items oi
		JOIN orders o ON o.id = oi.order_id
		WHERE o.outlet_id = $1 AND o.status = 'COMPLETED' AND oi.status = 'ACTIVE'
		  AND (o.completed_at AT TIME ZONE $2)::date BETWEEN $3::date AND $4::date
		GROUP BY oi.product_id, oi.product_name_snapshot
		ORDER BY SUM(oi.quantity) DESC
		LIMIT 5`

	rows, err := r.pool.Query(ctx, q, outletID, tz, from, to)
	if err != nil {
		return nil, fmt.Errorf("dashboard top products: %w", err)
	}
	defer rows.Close()

	items := []ProductTotal{}
	for rows.Next() {
		var p ProductTotal
		if err := rows.Scan(&p.ProductID, &p.Name, &p.Quantity, &p.Sales); err != nil {
			return nil, fmt.Errorf("scan top product: %w", err)
		}
		items = append(items, p)
	}
	return items, rows.Err()
}

func (r *Repository) criticalStock(ctx context.Context, outletID string) ([]StockItem, error) {
	const q = `
		SELECT i.id, i.name, inv.quantity_on_hand::text, i.min_stock::text
		FROM inventory inv
		JOIN ingredients i ON i.id = inv.ingredient_id
		WHERE inv.outlet_id = $1 AND i.is_active
		  AND inv.quantity_on_hand <= i.min_stock
		ORDER BY (inv.quantity_on_hand - i.min_stock) ASC
		LIMIT 5`

	rows, err := r.pool.Query(ctx, q, outletID)
	if err != nil {
		return nil, fmt.Errorf("dashboard critical stock: %w", err)
	}
	defer rows.Close()

	items := []StockItem{}
	for rows.Next() {
		var s StockItem
		if err := rows.Scan(&s.IngredientID, &s.Name, &s.Quantity, &s.MinStock); err != nil {
			return nil, fmt.Errorf("scan critical stock: %w", err)
		}
		items = append(items, s)
	}
	return items, rows.Err()
}

func (r *Repository) activeOrders(ctx context.Context, outletID string) ([]StatusCount, error) {
	const q = `
		SELECT status, count(*)::int
		FROM orders
		WHERE outlet_id = $1 AND status NOT IN ('COMPLETED', 'CANCELLED')
		GROUP BY status
		ORDER BY status`

	rows, err := r.pool.Query(ctx, q, outletID)
	if err != nil {
		return nil, fmt.Errorf("dashboard active orders: %w", err)
	}
	defer rows.Close()

	items := []StatusCount{}
	for rows.Next() {
		var s StatusCount
		if err := rows.Scan(&s.Status, &s.Count); err != nil {
			return nil, fmt.Errorf("scan active order: %w", err)
		}
		items = append(items, s)
	}
	return items, rows.Err()
}
