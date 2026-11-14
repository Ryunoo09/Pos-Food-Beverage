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

// paymentRow adalah data minimal pembayaran saat akan di-void.
type paymentRow struct {
	ID      string
	OrderID string
	ShiftID string
	Status  string
}

func (r *Repository) LockPayment(ctx context.Context, tx pgx.Tx, outletID, id string) (*paymentRow, bool, error) {
	const q = `
		SELECT order_id, shift_id, status FROM payments
		WHERE id = $1 AND outlet_id = $2 FOR UPDATE`

	p := &paymentRow{ID: id}
	err := tx.QueryRow(ctx, q, id, outletID).Scan(&p.OrderID, &p.ShiftID, &p.Status)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("lockPayment: %w", err)
	}
	return p, true, nil
}

func (r *Repository) VoidPayment(ctx context.Context, tx pgx.Tx, outletID, id, userID, reason string) error {
	tag, err := tx.Exec(ctx, `
		UPDATE payments SET status = 'VOIDED', voided_by = $3, voided_at = now(), void_reason = $4
		WHERE id = $1 AND outlet_id = $2 AND status = 'SUCCESS'`, id, outletID, userID, reason)
	if err != nil {
		return fmt.Errorf("voidPayment: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// SumSuccessfulPayments menghitung ulang total pembayaran yang masih berlaku.
func (r *Repository) SumSuccessfulPayments(ctx context.Context, tx pgx.Tx, orderID string) (string, error) {
	var sum string
	err := tx.QueryRow(ctx, `
		SELECT COALESCE(sum(amount), 0)::text FROM payments
		WHERE order_id = $1 AND status = 'SUCCESS'`, orderID).Scan(&sum)
	if err != nil {
		return "", fmt.Errorf("sumSuccessfulPayments: %w", err)
	}
	return sum, nil
}

func (r *Repository) SetOrderPaid(ctx context.Context, tx pgx.Tx, outletID, orderID, paid, paymentStatus string) error {
	if _, err := tx.Exec(ctx, `
		UPDATE orders SET paid_amount = $3, payment_status = $4,
		       version = version + 1, updated_at = now()
		WHERE id = $1 AND outlet_id = $2`, orderID, outletID, paid, paymentStatus); err != nil {
		return fmt.Errorf("setOrderPaid: %w", err)
	}
	return nil
}

func (r *Repository) Payment(ctx context.Context, outletID, id string) (*Payment, error) {
	const q = `
		SELECT id, order_id, shift_id, method_code, amount::text, cash_received::text,
		       change_amount::text, status, reference_no, paid_at, voided_by::text, voided_at,
		       void_reason, received_by, created_at
		FROM payments WHERE id = $1 AND outlet_id = $2`

	p := &Payment{}
	err := r.pool.QueryRow(ctx, q, id, outletID).Scan(&p.ID, &p.OrderID, &p.ShiftID, &p.MethodCode,
		&p.Amount, &p.CashReceived, &p.ChangeAmount, &p.Status, &p.ReferenceNo, &p.PaidAt,
		&p.VoidedBy, &p.VoidedAt, &p.VoidReason, &p.ReceivedBy, &p.CreatedAt)
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
		       change_amount::text, status, reference_no, paid_at, voided_by::text, voided_at,
		       void_reason, received_by, created_at
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
			&p.ChangeAmount, &p.Status, &p.ReferenceNo, &p.PaidAt, &p.VoidedBy, &p.VoidedAt,
			&p.VoidReason, &p.ReceivedBy, &p.CreatedAt); err != nil {
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

// refundRow adalah status refund saat approval/reject.
type refundRow struct {
	ID          string
	OrderID     string
	PaymentID   *string
	ShiftID     string
	Amount      string
	Status      string
	RequestedBy string
}

// RefundSettings membaca ambang nominal refund yang wajib persetujuan terpisah.
func (r *Repository) RefundSettings(ctx context.Context, outletID string) (string, error) {
	var threshold string
	err := r.pool.QueryRow(ctx,
		`SELECT refund_approval_threshold::text FROM outlet_settings WHERE outlet_id = $1`,
		outletID).Scan(&threshold)
	if errors.Is(err, pgx.ErrNoRows) {
		return "100000", nil
	}
	if err != nil {
		return "", fmt.Errorf("refundSettings: %w", err)
	}
	return threshold, nil
}

func (r *Repository) LockRefund(ctx context.Context, tx pgx.Tx, outletID, id string) (*refundRow, bool, error) {
	const q = `
		SELECT order_id, payment_id::text, shift_id, amount::text, status, requested_by
		FROM refunds WHERE id = $1 AND outlet_id = $2 FOR UPDATE`

	f := &refundRow{ID: id}
	err := tx.QueryRow(ctx, q, id, outletID).Scan(&f.OrderID, &f.PaymentID, &f.ShiftID,
		&f.Amount, &f.Status, &f.RequestedBy)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("lockRefund: %w", err)
	}
	return f, true, nil
}

type refundInsert struct {
	OutletID       string
	OrderID        string
	PaymentID      *string
	ShiftID        string
	RefundType     string
	MethodCode     *string
	Amount         string
	ReasonCode     *string
	ReasonNote     *string
	Status         string
	RequestedBy    string
	ApprovedBy     *string
	SelfApproved   bool
	IdempotencyKey string
	Restock        bool
	Completed      bool
}

func (r *Repository) InsertRefund(ctx context.Context, tx pgx.Tx, in refundInsert) (string, error) {
	const q = `
		INSERT INTO refunds (outlet_id, order_id, payment_id, shift_id, refund_type, method_code,
		    amount, reason_code, reason_note, status, requested_by, approved_by, self_approved,
		    idempotency_key, restock_decision, completed_at)
		VALUES ($1, $2, $3::uuid, $4, $5, $6, $7, $8, $9, $10, $11, $12::uuid, $13, $14,
		    CASE WHEN $15 THEN '{"restock":true}'::jsonb ELSE NULL END,
		    CASE WHEN $16 THEN now() ELSE NULL END)
		RETURNING id`

	var id string
	if err := tx.QueryRow(ctx, q, in.OutletID, in.OrderID, in.PaymentID, in.ShiftID, in.RefundType,
		in.MethodCode, in.Amount, in.ReasonCode, in.ReasonNote, in.Status, in.RequestedBy,
		in.ApprovedBy, in.SelfApproved, in.IdempotencyKey, in.Restock, in.Completed).Scan(&id); err != nil {
		return "", fmt.Errorf("insertRefund: %w", err)
	}
	return id, nil
}

func (r *Repository) InsertRefundItem(ctx context.Context, tx pgx.Tx, refundID, orderItemID string,
	quantity int, amount string, restock bool) error {
	const q = `
		INSERT INTO refund_items (refund_id, order_item_id, quantity, amount, restock)
		VALUES ($1, $2, $3, $4, $5)`
	if _, err := tx.Exec(ctx, q, refundID, orderItemID, quantity, amount, restock); err != nil {
		return fmt.Errorf("insertRefundItem: %w", err)
	}
	return nil
}

func (r *Repository) SetRefundStatus(ctx context.Context, tx pgx.Tx, outletID, id, status, approvedBy string, note *string) error {
	const q = `
		UPDATE refunds SET status = $3, approved_by = $4::uuid,
		    reason_note = COALESCE($5, reason_note),
		    completed_at = CASE WHEN $3::text IN ('COMPLETED', 'REJECTED') THEN now() ELSE completed_at END
		WHERE id = $1 AND outlet_id = $2`
	if _, err := tx.Exec(ctx, q, id, outletID, status, approvedBy, note); err != nil {
		return fmt.Errorf("setRefundStatus: %w", err)
	}
	return nil
}

// ApplyRefund memperbarui akumulasi refund dan status bayar pesanan.
func (r *Repository) ApplyRefund(ctx context.Context, tx pgx.Tx, outletID, orderID, refunded, paymentStatus string) error {
	if _, err := tx.Exec(ctx, `
		UPDATE orders SET refunded_amount = $3, payment_status = $4,
		       version = version + 1, updated_at = now()
		WHERE id = $1 AND outlet_id = $2`, orderID, outletID, refunded, paymentStatus); err != nil {
		return fmt.Errorf("applyRefund: %w", err)
	}
	return nil
}

// OrderItemQuantity mengambil jumlah item terjual untuk validasi refund item.
func (r *Repository) OrderItemQuantity(ctx context.Context, tx pgx.Tx, orderID, orderItemID string) (int, bool, error) {
	var qty int
	err := tx.QueryRow(ctx,
		`SELECT quantity FROM order_items WHERE id = $1 AND order_id = $2`,
		orderItemID, orderID).Scan(&qty)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("orderItemQuantity: %w", err)
	}
	return qty, true, nil
}

// RefundedItemQuantity menjumlah kuantitas item yang sudah direfund (selain yang ditolak).
func (r *Repository) RefundedItemQuantity(ctx context.Context, tx pgx.Tx, orderItemID string) (int, error) {
	var qty int
	err := tx.QueryRow(ctx, `
		SELECT COALESCE(sum(ri.quantity), 0)
		FROM refund_items ri JOIN refunds rf ON rf.id = ri.refund_id
		WHERE ri.order_item_id = $1 AND rf.status <> 'REJECTED'`, orderItemID).Scan(&qty)
	if err != nil {
		return 0, fmt.Errorf("refundedItemQuantity: %w", err)
	}
	return qty, nil
}

func (r *Repository) RefundByID(ctx context.Context, outletID, id string) (*Refund, error) {
	const q = `
		SELECT id, order_id, payment_id::text, shift_id, refund_type, method_code, amount::text,
		       reason_code, reason_note, status, self_approved, created_at, completed_at
		FROM refunds WHERE id = $1 AND outlet_id = $2`

	f := &Refund{}
	err := r.pool.QueryRow(ctx, q, id, outletID).Scan(&f.ID, &f.OrderID, &f.PaymentID, &f.ShiftID,
		&f.Type, &f.MethodCode, &f.Amount, &f.ReasonCode, &f.ReasonNote, &f.Status,
		&f.SelfApproved, &f.CreatedAt, &f.CompletedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("refundByID: %w", err)
	}

	rows, err := r.pool.Query(ctx, `
		SELECT ri.order_item_id::text, oi.product_name_snapshot, ri.quantity, ri.amount::text, ri.restock
		FROM refund_items ri JOIN order_items oi ON oi.id = ri.order_item_id
		WHERE ri.refund_id = $1 ORDER BY ri.id`, id)
	if err != nil {
		return nil, fmt.Errorf("refundByID: items: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var line RefundLine
		if err := rows.Scan(&line.OrderItemID, &line.Name, &line.Quantity, &line.Amount, &line.Restock); err != nil {
			return nil, fmt.Errorf("refundByID: scan item: %w", err)
		}
		f.Items = append(f.Items, line)
	}
	return f, rows.Err()
}

func (r *Repository) Refunds(ctx context.Context, outletID string, f RefundFilter) ([]Refund, int, error) {
	where := ` WHERE outlet_id = $1`
	args := []any{outletID}
	if f.OrderID != "" {
		args = append(args, f.OrderID)
		where += fmt.Sprintf(" AND order_id = $%d", len(args))
	}
	if f.Status != "" {
		args = append(args, f.Status)
		where += fmt.Sprintf(" AND status = $%d", len(args))
	}

	var total int
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM refunds`+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("refunds: count: %w", err)
	}

	args = append(args, f.PerPage, (f.Page-1)*f.PerPage)
	q := `SELECT id, order_id, payment_id::text, shift_id, refund_type, method_code, amount::text,
	             reason_code, reason_note, status, self_approved, created_at, completed_at
	      FROM refunds` + where + fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d",
		len(args)-1, len(args))

	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("refunds: %w", err)
	}
	defer rows.Close()

	items := []Refund{}
	for rows.Next() {
		var x Refund
		if err := rows.Scan(&x.ID, &x.OrderID, &x.PaymentID, &x.ShiftID, &x.Type, &x.MethodCode,
			&x.Amount, &x.ReasonCode, &x.ReasonNote, &x.Status, &x.SelfApproved,
			&x.CreatedAt, &x.CompletedAt); err != nil {
			return nil, 0, fmt.Errorf("refunds: scan: %w", err)
		}
		items = append(items, x)
	}
	return items, total, rows.Err()
}
