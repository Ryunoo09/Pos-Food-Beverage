package report

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Pelaporan bisa menyapu ratusan ribu baris; batas ini menjaga satu query
// laporan tidak menahan koneksi pool lebih lama dari wajar.
const statementTimeout = "10s"

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Timezone(ctx context.Context, businessID string) (string, error) {
	const q = `SELECT timezone FROM businesses WHERE id = $1`
	var tz string
	if err := r.pool.QueryRow(ctx, q, businessID).Scan(&tz); err != nil {
		return "", fmt.Errorf("report timezone: %w", err)
	}
	return tz, nil
}

// begin berjalan dalam transaksi read-only agar statement_timeout berlaku pada
// seluruh query laporan dan snapshot konsisten antar-query.
func (r *Repository) begin(ctx context.Context) (*txn, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("report begin: %w", err)
	}
	if _, err := tx.Exec(ctx, `SET LOCAL TRANSACTION READ ONLY`); err != nil {
		tx.Rollback(ctx)
		return nil, fmt.Errorf("report read only: %w", err)
	}
	if _, err := tx.Exec(ctx, `SET LOCAL statement_timeout = '`+statementTimeout+`'`); err != nil {
		tx.Rollback(ctx)
		return nil, fmt.Errorf("report timeout: %w", err)
	}
	return &txn{tx: tx}, nil
}

// salesTotals memakai definisi net yang sama dengan dashboard:
// subtotal - diskon item - diskon pesanan. Total = total_amount (sudah
// termasuk service, pajak, pembulatan).
func (r *Repository) salesTotals(ctx context.Context, t *txn, outletID, tz, from, to string) (SalesTotals, error) {
	const q = `
		SELECT count(*)::int,
		       COALESCE(SUM(subtotal_amount), 0)::text,
		       COALESCE(SUM(item_discount_amount + order_discount_amount), 0)::text,
		       COALESCE(SUM(service_charge_amount), 0)::text,
		       COALESCE(SUM(tax_amount), 0)::text,
		       COALESCE(SUM(subtotal_amount - item_discount_amount - order_discount_amount), 0)::text,
		       COALESCE(SUM(total_amount), 0)::text
		FROM orders
		WHERE outlet_id = $1 AND status = 'COMPLETED'
		  AND (completed_at AT TIME ZONE $2)::date BETWEEN $3::date AND $4::date`

	var s SalesTotals
	err := t.tx.QueryRow(ctx, q, outletID, tz, from, to).Scan(
		&s.OrderCount, &s.GrossSales, &s.Discount, &s.ServiceCharge, &s.Tax, &s.NetSales, &s.Total)
	if err != nil {
		return SalesTotals{}, fmt.Errorf("sales totals: %w", err)
	}
	return s, nil
}

// daily mengisi hari tanpa transaksi dengan nol agar tabel tidak berlubang.
func (r *Repository) daily(ctx context.Context, t *txn, outletID, tz, from, to string) ([]SalesBucket, error) {
	const q = `
		SELECT (completed_at AT TIME ZONE $2)::date::text AS period,
		       count(*)::int,
		       COALESCE(SUM(subtotal_amount), 0)::text,
		       COALESCE(SUM(subtotal_amount - item_discount_amount - order_discount_amount), 0)::text,
		       COALESCE(SUM(total_amount), 0)::text
		FROM orders
		WHERE outlet_id = $1 AND status = 'COMPLETED'
		  AND (completed_at AT TIME ZONE $2)::date BETWEEN $3::date AND $4::date
		GROUP BY period
		ORDER BY period`

	rows, err := t.tx.Query(ctx, q, outletID, tz, from, to)
	if err != nil {
		return nil, fmt.Errorf("sales daily: %w", err)
	}
	defer rows.Close()

	found := map[string]SalesBucket{}
	for rows.Next() {
		var b SalesBucket
		if err := rows.Scan(&b.Key, &b.OrderCount, &b.GrossSales, &b.NetSales, &b.Total); err != nil {
			return nil, fmt.Errorf("sales daily scan: %w", err)
		}
		b.Label = b.Key
		found[b.Key] = b
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	items := []SalesBucket{}
	for _, day := range days(from, to) {
		b, ok := found[day]
		if !ok {
			b = SalesBucket{Key: day, Label: day, GrossSales: "0", NetSales: "0", Total: "0"}
		}
		items = append(items, b)
	}
	return items, nil
}

func (r *Repository) byService(ctx context.Context, t *txn, outletID, tz, from, to string) ([]SalesBucket, error) {
	const q = `
		SELECT service_type, count(*)::int,
		       COALESCE(SUM(subtotal_amount), 0)::text,
		       COALESCE(SUM(subtotal_amount - item_discount_amount - order_discount_amount), 0)::text,
		       COALESCE(SUM(total_amount), 0)::text
		FROM orders
		WHERE outlet_id = $1 AND status = 'COMPLETED'
		  AND (completed_at AT TIME ZONE $2)::date BETWEEN $3::date AND $4::date
		GROUP BY service_type
		ORDER BY SUM(total_amount) DESC`
	return r.buckets(ctx, t, q, outletID, tz, from, to, serviceLabel)
}

// byMethod menjumlah pembayaran SUCCESS; satu pesanan bisa punya beberapa
// pembayaran (split bill), jadi angka di sini tidak harus sama dengan omzet.
func (r *Repository) byMethod(ctx context.Context, t *txn, outletID, tz, from, to string) ([]SalesBucket, error) {
	const q = `
		SELECT p.method_code, count(DISTINCT o.id)::int,
		       COALESCE(SUM(o.subtotal_amount), 0)::text,
		       COALESCE(SUM(o.subtotal_amount - o.item_discount_amount - o.order_discount_amount), 0)::text,
		       COALESCE(SUM(p.amount), 0)::text
		FROM payments p
		JOIN orders o ON o.id = p.order_id
		WHERE p.outlet_id = $1 AND p.status = 'SUCCESS'
		  AND (o.completed_at AT TIME ZONE $2)::date BETWEEN $3::date AND $4::date
		GROUP BY p.method_code
		ORDER BY SUM(p.amount) DESC`
	return r.buckets(ctx, t, q, outletID, tz, from, to, methodLabel)
}

func (r *Repository) byCashier(ctx context.Context, t *txn, outletID, tz, from, to string) ([]SalesBucket, error) {
	const q = `
		SELECT COALESCE(u.full_name, 'Tanpa kasir'), count(*)::int,
		       COALESCE(SUM(o.subtotal_amount), 0)::text,
		       COALESCE(SUM(o.subtotal_amount - o.item_discount_amount - o.order_discount_amount), 0)::text,
		       COALESCE(SUM(o.total_amount), 0)::text
		FROM orders o
		LEFT JOIN users u ON u.id = o.created_by
		WHERE o.outlet_id = $1 AND o.status = 'COMPLETED'
		  AND (o.completed_at AT TIME ZONE $2)::date BETWEEN $3::date AND $4::date
		GROUP BY u.full_name
		ORDER BY SUM(o.total_amount) DESC`
	return r.buckets(ctx, t, q, outletID, tz, from, to, func(key string) string { return key })
}

// buckets menjalankan query berkolom (key, count, gross, net, total).
func (r *Repository) buckets(ctx context.Context, t *txn, q, outletID, tz, from, to string,
	label func(string) string) ([]SalesBucket, error) {
	rows, err := t.tx.Query(ctx, q, outletID, tz, from, to)
	if err != nil {
		return nil, fmt.Errorf("sales buckets: %w", err)
	}
	defer rows.Close()

	items := []SalesBucket{}
	for rows.Next() {
		var b SalesBucket
		if err := rows.Scan(&b.Key, &b.OrderCount, &b.GrossSales, &b.NetSales, &b.Total); err != nil {
			return nil, fmt.Errorf("sales buckets scan: %w", err)
		}
		b.Label = label(b.Key)
		items = append(items, b)
	}
	return items, rows.Err()
}

func (r *Repository) topProducts(ctx context.Context, t *txn, outletID, tz, from, to string) ([]ProductLine, error) {
	const q = `
		SELECT oi.product_id::text, oi.product_name_snapshot,
		       SUM(oi.quantity)::int,
		       COALESCE(SUM(oi.line_total), 0)::text,
		       COALESCE(SUM(oi.line_total - oi.line_cogs), 0)::text
		FROM order_items oi
		JOIN orders o ON o.id = oi.order_id
		WHERE o.outlet_id = $1 AND o.status = 'COMPLETED' AND oi.status = 'ACTIVE'
		  AND (o.completed_at AT TIME ZONE $2)::date BETWEEN $3::date AND $4::date
		GROUP BY oi.product_id, oi.product_name_snapshot
		ORDER BY SUM(oi.line_total) DESC
		LIMIT 10`

	rows, err := t.tx.Query(ctx, q, outletID, tz, from, to)
	if err != nil {
		return nil, fmt.Errorf("top products: %w", err)
	}
	defer rows.Close()

	items := []ProductLine{}
	for rows.Next() {
		var p ProductLine
		if err := rows.Scan(&p.ProductID, &p.Name, &p.Quantity, &p.Revenue, &p.GrossProfit); err != nil {
			return nil, fmt.Errorf("top products scan: %w", err)
		}
		items = append(items, p)
	}
	return items, rows.Err()
}

// financialCosts mengambil biaya periode untuk laporan laba rugi. HPP memakai
// SUM(total_cogs) pesanan agar identik dengan dashboard, sedangkan waste
// menghitung catatan POSTED saja (record REVERSED adalah pembaliknya).
func (r *Repository) financialCosts(ctx context.Context, t *txn, outletID, tz, from, to string) (hpp, waste string, err error) {
	const q = `
		SELECT
		  (SELECT COALESCE(SUM(total_cogs), 0)
		     FROM orders
		    WHERE outlet_id = $1 AND status = 'COMPLETED'
		      AND (completed_at AT TIME ZONE $2)::date BETWEEN $3::date AND $4::date)::text,
		  (SELECT COALESCE(SUM(cost_value), 0)
		     FROM waste_records
		    WHERE outlet_id = $1 AND status = 'POSTED'
		      AND (recorded_at AT TIME ZONE $2)::date BETWEEN $3::date AND $4::date)::text`

	if err := t.tx.QueryRow(ctx, q, outletID, tz, from, to).Scan(&hpp, &waste); err != nil {
		return "", "", fmt.Errorf("financial costs: %w", err)
	}
	return hpp, waste, nil
}

func (r *Repository) operatingExpense(ctx context.Context, t *txn, outletID, from, to string) (string, error) {
	const q = `
		SELECT COALESCE(SUM(amount), 0)::text
		FROM expenses
		WHERE outlet_id = $1 AND NOT is_voided
		  AND expense_date BETWEEN $2::date AND $3::date`
	var total string
	if err := t.tx.QueryRow(ctx, q, outletID, from, to).Scan(&total); err != nil {
		return "", fmt.Errorf("operating expense: %w", err)
	}
	return total, nil
}

func (r *Repository) expensesByCategory(ctx context.Context, t *txn, outletID, from, to string) ([]ExpenseLine, error) {
	const q = `
		SELECT c.name, SUM(e.amount)::text
		FROM expenses e
		JOIN expense_categories c ON c.id = e.category_id
		WHERE e.outlet_id = $1 AND NOT e.is_voided
		  AND e.expense_date BETWEEN $2::date AND $3::date
		GROUP BY c.name
		ORDER BY SUM(e.amount) DESC`

	rows, err := t.tx.Query(ctx, q, outletID, from, to)
	if err != nil {
		return nil, fmt.Errorf("expenses by category: %w", err)
	}
	defer rows.Close()

	items := []ExpenseLine{}
	for rows.Next() {
		var line ExpenseLine
		if err := rows.Scan(&line.CategoryName, &line.Amount); err != nil {
			return nil, fmt.Errorf("expenses by category scan: %w", err)
		}
		items = append(items, line)
	}
	return items, rows.Err()
}

func (r *Repository) menuPerformance(ctx context.Context, t *txn, outletID, tz, from, to string) ([]MenuPerformance, error) {
	// cost_unknown = true berarti HPP item tidak diketahui; menyertakannya
	// akan membuat margin tampak lebih tinggi dari kenyataan.
	const q = `
		SELECT oi.product_id::text, oi.product_name_snapshot, oi.variant_name_snapshot,
		       SUM(oi.quantity)::int,
		       COALESCE(SUM(oi.line_total), 0)::text,
		       COALESCE(SUM(oi.line_cogs), 0)::text,
		       COALESCE(SUM(oi.line_total - oi.line_cogs), 0)::text
		FROM order_items oi
		JOIN orders o ON o.id = oi.order_id
		WHERE o.outlet_id = $1 AND o.status = 'COMPLETED' AND oi.status = 'ACTIVE'
		  AND NOT oi.cost_unknown
		  AND (o.completed_at AT TIME ZONE $2)::date BETWEEN $3::date AND $4::date
		GROUP BY oi.product_id, oi.product_name_snapshot, oi.variant_name_snapshot
		ORDER BY SUM(oi.line_total) DESC`

	rows, err := t.tx.Query(ctx, q, outletID, tz, from, to)
	if err != nil {
		return nil, fmt.Errorf("menu performance: %w", err)
	}
	defer rows.Close()

	items := []MenuPerformance{}
	for rows.Next() {
		var m MenuPerformance
		if err := rows.Scan(&m.ProductID, &m.Name, &m.VariantName, &m.Quantity,
			&m.Revenue, &m.Cost, &m.GrossProfit); err != nil {
			return nil, fmt.Errorf("menu performance scan: %w", err)
		}
		items = append(items, m)
	}
	return items, rows.Err()
}

func (r *Repository) shifts(ctx context.Context, t *txn, outletID, from, to string) ([]ShiftLine, error) {
	const q = `
		SELECT s.id::text, COALESCE(u.full_name, 'Tanpa nama'), s.status,
		       to_char(s.opened_at, 'YYYY-MM-DD"T"HH24:MI:SSOF'),
		       to_char(s.closed_at, 'YYYY-MM-DD"T"HH24:MI:SSOF'),
		       s.opening_cash::text, s.expected_cash::text, s.actual_cash::text, s.difference::text
		FROM shifts s
		LEFT JOIN users u ON u.id = s.user_id
		WHERE s.outlet_id = $1
		  AND s.opened_at::date BETWEEN $2::date AND $3::date
		ORDER BY s.opened_at DESC`

	rows, err := t.tx.Query(ctx, q, outletID, from, to)
	if err != nil {
		return nil, fmt.Errorf("shifts: %w", err)
	}
	defer rows.Close()

	type raw struct {
		line ShiftLine
	}
	items := []raw{}
	for rows.Next() {
		var rw raw
		if err := rows.Scan(&rw.line.ID, &rw.line.CashierName, &rw.line.Status,
			&rw.line.OpenedAt, &rw.line.ClosedAt, &rw.line.OpeningCash,
			&rw.line.ExpectedCash, &rw.line.ActualCash, &rw.line.Difference); err != nil {
			return nil, fmt.Errorf("shifts scan: %w", err)
		}
		rw.line.Methods = []MethodAmount{}
		items = append(items, rw)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Satu query agregat untuk semua shift, bukan N query per baris.
	const qm = `
		SELECT shift_id::text, method_code, SUM(amount)::text
		FROM payments
		WHERE outlet_id = $1 AND status = 'SUCCESS'
		  AND shift_id = ANY($2::uuid[])
		GROUP BY shift_id, method_code
		ORDER BY SUM(amount) DESC`

	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.line.ID)
	}

	result := make([]ShiftLine, 0, len(items))
	if len(ids) == 0 {
		for _, item := range items {
			result = append(result, item.line)
		}
		return result, nil
	}

	byID := map[string][]MethodAmount{}
	prows, err := t.tx.Query(ctx, qm, outletID, ids)
	if err != nil {
		return nil, fmt.Errorf("shift methods: %w", err)
	}
	defer prows.Close()
	for prows.Next() {
		var shiftID string
		var m MethodAmount
		if err := prows.Scan(&shiftID, &m.Method, &m.Amount); err != nil {
			return nil, fmt.Errorf("shift methods scan: %w", err)
		}
		byID[shiftID] = append(byID[shiftID], m)
	}
	if err := prows.Err(); err != nil {
		return nil, err
	}

	for _, item := range items {
		line := item.line
		if methods, ok := byID[line.ID]; ok {
			line.Methods = methods
		}
		line.Sales = sumAmounts(line.Methods)
		result = append(result, line)
	}
	return result, nil
}
