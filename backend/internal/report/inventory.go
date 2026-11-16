package report

import (
	"context"
	"fmt"

	"github.com/ryunoo09/pos-food-beverage/backend/internal/pkg/money"
)

// stock mengambil posisi stok saat ini (bukan per periode) beserta valuasi.
// Bahan bernaung di bawah bisnis, sedangkan saldo stok per outlet.
func (r *Repository) stock(ctx context.Context, t *txn, businessID, outletID string) ([]StockLine, string, error) {
	const q = `
		SELECT i.id::text, i.code, i.name, u.code,
		       COALESCE(inv.quantity_on_hand, 0)::text,
		       i.min_stock::text,
		       COALESCE(inv.avg_unit_cost, 0)::text,
		       (COALESCE(inv.quantity_on_hand, 0) * COALESCE(inv.avg_unit_cost, 0))::numeric(14,2)::text
		FROM ingredients i
		JOIN units u ON u.id = i.base_unit_id
		LEFT JOIN inventory inv ON inv.ingredient_id = i.id AND inv.outlet_id = $2
		WHERE i.business_id = $1 AND i.is_active
		ORDER BY i.name`

	rows, err := t.tx.Query(ctx, q, businessID, outletID)
	if err != nil {
		return nil, "", fmt.Errorf("stock: %w", err)
	}
	defer rows.Close()

	items := []StockLine{}
	for rows.Next() {
		var s StockLine
		if err := rows.Scan(&s.IngredientID, &s.Code, &s.Name, &s.Unit,
			&s.OnHand, &s.MinStock, &s.AvgCost, &s.Value); err != nil {
			return nil, "", fmt.Errorf("stock scan: %w", err)
		}
		s.Status = stockStatus(s.OnHand, s.MinStock)
		items = append(items, s)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}

	total := "0"
	for _, item := range items {
		sum, err := addMoney(total, item.Value)
		if err != nil {
			return nil, "", err
		}
		total = sum
	}
	return items, total, nil
}

// movements meringkas mutasi stok per bahan dan tipe dalam periode.
// quantity_delta bisa negatif (SALE, WASTE), jadi dijumlah apa adanya.
func (r *Repository) movements(ctx context.Context, t *txn, outletID, tz, from, to string) ([]MovementLine, error) {
	const q = `
		SELECT i.id::text, i.name, m.movement_type, SUM(m.quantity_delta)::text
		FROM stock_movements m
		JOIN ingredients i ON i.id = m.ingredient_id
		WHERE m.outlet_id = $1
		  AND (m.created_at AT TIME ZONE $2)::date BETWEEN $3::date AND $4::date
		GROUP BY i.id, i.name, m.movement_type
		ORDER BY i.name, m.movement_type`

	rows, err := t.tx.Query(ctx, q, outletID, tz, from, to)
	if err != nil {
		return nil, fmt.Errorf("movements: %w", err)
	}
	defer rows.Close()

	items := []MovementLine{}
	for rows.Next() {
		var m MovementLine
		if err := rows.Scan(&m.IngredientID, &m.Name, &m.Type, &m.Quantity); err != nil {
			return nil, fmt.Errorf("movements scan: %w", err)
		}
		items = append(items, m)
	}
	return items, rows.Err()
}

// wasteSummary mengelompokkan waste POSTED per alasan dalam periode.
func (r *Repository) wasteSummary(ctx context.Context, t *txn, outletID, tz, from, to string) ([]WasteLine, error) {
	const q = `
		SELECT COALESCE(NULLIF(reason_code, ''), 'TANPA_ALASAN'), count(*)::int,
		       COALESCE(SUM(cost_value), 0)::text
		FROM waste_records
		WHERE outlet_id = $1 AND status = 'POSTED'
		  AND (recorded_at AT TIME ZONE $2)::date BETWEEN $3::date AND $4::date
		GROUP BY 1
		ORDER BY SUM(cost_value) DESC`

	rows, err := t.tx.Query(ctx, q, outletID, tz, from, to)
	if err != nil {
		return nil, fmt.Errorf("waste summary: %w", err)
	}
	defer rows.Close()

	items := []WasteLine{}
	for rows.Next() {
		var w WasteLine
		if err := rows.Scan(&w.ReasonCode, &w.Count, &w.Cost); err != nil {
			return nil, fmt.Errorf("waste summary scan: %w", err)
		}
		items = append(items, w)
	}
	return items, rows.Err()
}

// opnames mendaftar opname yang sudah selesai/approve beserta total selisihnya.
func (r *Repository) opnames(ctx context.Context, t *txn, outletID, from, to string) ([]OpnameLine, error) {
	const q = `
		SELECT o.id::text, o.code, o.status,
		       to_char(COALESCE(o.approved_at, o.submitted_at, o.updated_at),
		               'YYYY-MM-DD"T"HH24:MI:SSOF'),
		       COALESCE(SUM(i.difference_quantity), 0)::text,
		       COALESCE(SUM(i.difference_value), 0)::text
		FROM stock_opnames o
		LEFT JOIN stock_opname_items i ON i.stock_opname_id = o.id
		WHERE o.outlet_id = $1 AND o.status IN ('APPROVED', 'COMPLETED')
		  AND COALESCE(o.approved_at, o.submitted_at, o.updated_at)::date
		      BETWEEN $2::date AND $3::date
		GROUP BY o.id, o.code, o.status, o.approved_at, o.submitted_at, o.updated_at
		ORDER BY COALESCE(o.approved_at, o.submitted_at, o.updated_at) DESC`

	rows, err := t.tx.Query(ctx, q, outletID, from, to)
	if err != nil {
		return nil, fmt.Errorf("opnames: %w", err)
	}
	defer rows.Close()

	items := []OpnameLine{}
	for rows.Next() {
		var o OpnameLine
		if err := rows.Scan(&o.ID, &o.OpnameNo, &o.Status, &o.FinishedAt,
			&o.TotalDiffQty, &o.TotalDiffVal); err != nil {
			return nil, fmt.Errorf("opnames scan: %w", err)
		}
		items = append(items, o)
	}
	return items, rows.Err()
}

func stockStatus(onHand, minStock string) string {
	hand, err := money.Parse(onHand)
	if err != nil {
		return "OK"
	}
	min, err := money.Parse(minStock)
	if err != nil {
		return "OK"
	}
	switch {
	case !min.IsPositive():
		return "OK"
	case hand.IsZero() || hand.LessThan(min.Mul(oneHalf())):
		return "CRITICAL"
	case hand.LessThan(min):
		return "LOW"
	default:
		return "OK"
	}
}
