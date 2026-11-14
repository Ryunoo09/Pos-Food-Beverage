package payment

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("data pembayaran tidak ditemukan")

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Begin(ctx context.Context) (pgx.Tx, error) {
	return r.pool.Begin(ctx)
}

// orderRow adalah status pesanan yang dibutuhkan saat menerima pembayaran.
type orderRow struct {
	ID            string
	OrderNo       string
	Status        string
	PaymentStatus string
	Total         string
	Paid          string
	Refunded      string
	ServiceType   string
	TableID       *string
}

func (r *Repository) LockOrder(ctx context.Context, tx pgx.Tx, outletID, orderID string) (*orderRow, bool, error) {
	const q = `
		SELECT COALESCE(order_no, '') AS order_no, status, payment_status, total_amount::text, paid_amount::text,
		       refunded_amount::text, service_type, table_id::text
		FROM orders WHERE id = $1 AND outlet_id = $2 FOR UPDATE`

	o := &orderRow{ID: orderID}
	err := tx.QueryRow(ctx, q, orderID, outletID).Scan(&o.OrderNo, &o.Status, &o.PaymentStatus,
		&o.Total, &o.Paid, &o.Refunded, &o.ServiceType, &o.TableID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("lockOrder: %w", err)
	}
	return o, true, nil
}

func (r *Repository) Order(ctx context.Context, outletID, orderID string) (*orderRow, bool, error) {
	const q = `
		SELECT COALESCE(order_no, '') AS order_no, status, payment_status, total_amount::text, paid_amount::text,
		       refunded_amount::text, service_type, table_id::text
		FROM orders WHERE id = $1 AND outlet_id = $2`

	o := &orderRow{ID: orderID}
	err := r.pool.QueryRow(ctx, q, orderID, outletID).Scan(&o.OrderNo, &o.Status, &o.PaymentStatus,
		&o.Total, &o.Paid, &o.Refunded, &o.ServiceType, &o.TableID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("order: %w", err)
	}
	return o, true, nil
}

// OpenShift mengambil shift OPEN milik kasir.
func (r *Repository) OpenShift(ctx context.Context, userID string) (string, bool, error) {
	var id string
	err := r.pool.QueryRow(ctx,
		`SELECT id FROM shifts WHERE user_id = $1 AND status = 'OPEN'`, userID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("openShift: %w", err)
	}
	return id, true, nil
}

func (r *Repository) InsertPayment(ctx context.Context, tx pgx.Tx, outletID, orderID, shiftID, userID, methodCode,
	amount string, cashReceived, change, referenceNo *string, key string) (string, error) {
	const q = `
		INSERT INTO payments (outlet_id, order_id, shift_id, method_code, amount, cash_received,
		                      change_amount, status, reference_no, idempotency_key, paid_at,
		                      confirmation_type, received_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, 'SUCCESS', $8, $9, now(), 'MANUAL', $10)
		RETURNING id`

	var id string
	if err := tx.QueryRow(ctx, q, outletID, orderID, shiftID, methodCode, amount, cashReceived,
		change, referenceNo, key, userID).Scan(&id); err != nil {
		return "", fmt.Errorf("insertPayment: %w", err)
	}
	return id, nil
}

func (r *Repository) Payment(ctx context.Context, outletID, id string) (*Payment, error) {
	const q = `
		SELECT id, order_id, shift_id, method_code, amount::text, cash_received::text,
		       change_amount::text, status, reference_no, paid_at, received_by, created_at
		FROM payments WHERE id = $1 AND outlet_id = $2`

	p := &Payment{}
	err := r.pool.QueryRow(ctx, q, id, outletID).Scan(&p.ID, &p.OrderID, &p.ShiftID, &p.MethodCode,
		&p.Amount, &p.CashReceived, &p.ChangeAmount, &p.Status, &p.ReferenceNo, &p.PaidAt,
		&p.ReceivedBy, &p.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("payment: %w", err)
	}
	return p, nil
}

func (r *Repository) Payments(ctx context.Context, outletID, orderID string) ([]Payment, error) {
	const q = `
		SELECT id, order_id, shift_id, method_code, amount::text, cash_received::text,
		       change_amount::text, status, reference_no, paid_at, received_by, created_at
		FROM payments WHERE outlet_id = $1 AND order_id = $2
		ORDER BY created_at`

	rows, err := r.pool.Query(ctx, q, outletID, orderID)
	if err != nil {
		return nil, fmt.Errorf("payments: %w", err)
	}
	defer rows.Close()

	items := []Payment{}
	for rows.Next() {
		var p Payment
		if err := rows.Scan(&p.ID, &p.OrderID, &p.ShiftID, &p.MethodCode, &p.Amount, &p.CashReceived,
			&p.ChangeAmount, &p.Status, &p.ReferenceNo, &p.PaidAt, &p.ReceivedBy, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("payments: scan: %w", err)
		}
		items = append(items, p)
	}
	return items, rows.Err()
}

func (r *Repository) UpdateOrderPayment(ctx context.Context, tx pgx.Tx, outletID, orderID, shiftID, paid, paymentStatus string) error {
	if _, err := tx.Exec(ctx, `
		UPDATE orders SET paid_amount = $3, payment_status = $4, shift_id = COALESCE(shift_id, $5),
		       version = version + 1, updated_at = now()
		WHERE id = $1 AND outlet_id = $2`,
		orderID, outletID, paid, paymentStatus, shiftID); err != nil {
		return fmt.Errorf("updateOrderPayment: %w", err)
	}
	return nil
}

// AllItemsServed melaporkan apakah seluruh item dapur sudah disajikan.
func (r *Repository) AllItemsServed(ctx context.Context, tx pgx.Tx, orderID string) (bool, error) {
	var served bool
	err := tx.QueryRow(ctx, `
		SELECT COALESCE(bool_and(koi.status IN ('SERVED', 'VOID')), true)
		FROM kitchen_order_items koi
		JOIN kitchen_orders ko ON ko.id = koi.kitchen_order_id
		WHERE ko.order_id = $1`, orderID).Scan(&served)
	if err != nil {
		return false, fmt.Errorf("allItemsServed: %w", err)
	}
	return served, nil
}

func (r *Repository) CompleteOrder(ctx context.Context, tx pgx.Tx, outletID, orderID string) error {
	if _, err := tx.Exec(ctx, `
		UPDATE orders SET status = 'COMPLETED', completed_at = now(),
		       version = version + 1, updated_at = now()
		WHERE id = $1 AND outlet_id = $2 AND status <> 'COMPLETED'`, orderID, outletID); err != nil {
		return fmt.Errorf("completeOrder: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE tables SET status = 'AVAILABLE', updated_at = now()
		WHERE id = (SELECT table_id FROM orders WHERE id = $1) AND status = 'OCCUPIED'`, orderID); err != nil {
		return fmt.Errorf("completeOrder: free table: %w", err)
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

func (r *Repository) SaveIdempotencyResponse(ctx context.Context, scope, key string, status int, body []byte) error {
	if _, err := r.pool.Exec(ctx, `
		UPDATE idempotency_keys SET response_status = $3, response_body = $4
		WHERE scope = $1 AND key = $2`, scope, key, status, body); err != nil {
		return fmt.Errorf("saveIdempotencyResponse: %w", err)
	}
	return nil
}

func (r *Repository) ReleaseIdempotency(ctx context.Context, scope, key string) error {
	if _, err := r.pool.Exec(ctx, `DELETE FROM idempotency_keys WHERE scope = $1 AND key = $2`, scope, key); err != nil {
		return fmt.Errorf("releaseIdempotency: %w", err)
	}
	return nil
}
