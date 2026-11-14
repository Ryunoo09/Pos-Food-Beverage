package shift

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound    = errors.New("shift tidak ditemukan")
	ErrAlreadyOpen = errors.New("shift sudah terbuka")
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Begin(ctx context.Context) (pgx.Tx, error) {
	return r.pool.Begin(ctx)
}

const shiftCols = `id, user_id, status, opened_at, closed_at, opening_cash::text,
	expected_cash::text, actual_cash::text, difference::text, difference_reason, version`

func scanShift(row pgx.Row) (*Shift, error) {
	s := &Shift{}
	if err := row.Scan(&s.ID, &s.UserID, &s.Status, &s.OpenedAt, &s.ClosedAt, &s.OpeningCash,
		&s.ExpectedCash, &s.ActualCash, &s.Difference, &s.DifferenceReason, &s.Version); err != nil {
		return nil, err
	}
	return s, nil
}

func (r *Repository) Current(ctx context.Context, userID string) (*Shift, bool, error) {
	s, err := scanShift(r.pool.QueryRow(ctx,
		`SELECT `+shiftCols+` FROM shifts WHERE user_id = $1 AND status = 'OPEN'`, userID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("current: %w", err)
	}
	return s, true, nil
}

func (r *Repository) ByID(ctx context.Context, outletID, id string) (*Shift, bool, error) {
	s, err := scanShift(r.pool.QueryRow(ctx,
		`SELECT `+shiftCols+` FROM shifts WHERE id = $1 AND outlet_id = $2`, id, outletID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("byID: %w", err)
	}
	return s, true, nil
}

func (r *Repository) LockShift(ctx context.Context, tx pgx.Tx, outletID, id string) (*Shift, bool, error) {
	s, err := scanShift(tx.QueryRow(ctx,
		`SELECT `+shiftCols+` FROM shifts WHERE id = $1 AND outlet_id = $2 FOR UPDATE`, id, outletID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("lockShift: %w", err)
	}
	return s, true, nil
}

func (r *Repository) Insert(ctx context.Context, outletID, userID, openingCash string) (*Shift, error) {
	s, err := scanShift(r.pool.QueryRow(ctx, `
		INSERT INTO shifts (outlet_id, user_id, opening_cash)
		VALUES ($1, $2, $3)
		RETURNING `+shiftCols, outletID, userID, openingCash))
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrAlreadyOpen
		}
		return nil, fmt.Errorf("insert: %w", err)
	}
	return s, nil
}

func (r *Repository) Close(ctx context.Context, tx pgx.Tx, id, expectedCash, actualCash, difference string,
	reason *string, summary json.RawMessage) error {
	if _, err := tx.Exec(ctx, `
		UPDATE shifts SET status = 'CLOSED', closed_at = now(), expected_cash = $2, actual_cash = $3,
		    difference = $4, difference_reason = $5, summary = $6,
		    version = version + 1, updated_at = now()
		WHERE id = $1 AND status = 'OPEN'`,
		id, expectedCash, actualCash, difference, reason, summary); err != nil {
		return fmt.Errorf("close: %w", err)
	}
	return nil
}

// totals adalah agregat kas pada satu shift.
type totals struct {
	byMethod    []MethodTotal
	refundsCash string
	cashIn      string
	cashOut     string
	voidCount   int
	voidAmount  string
	pending     int
}

func (r *Repository) Totals(ctx context.Context, q interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}, shiftID string) (*totals, error) {
	t := &totals{refundsCash: "0", cashIn: "0", cashOut: "0", voidAmount: "0"}

	rows, err := q.Query(ctx, `
		SELECT method_code, COALESCE(sum(amount), 0)::text, count(*)
		FROM payments WHERE shift_id = $1 AND status = 'SUCCESS'
		GROUP BY method_code ORDER BY method_code`, shiftID)
	if err != nil {
		return nil, fmt.Errorf("totals: methods: %w", err)
	}
	for rows.Next() {
		var m MethodTotal
		if err := rows.Scan(&m.MethodCode, &m.Amount, &m.Count); err != nil {
			rows.Close()
			return nil, fmt.Errorf("totals: scan method: %w", err)
		}
		t.byMethod = append(t.byMethod, m)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("totals: methods: %w", err)
	}

	if err := q.QueryRow(ctx, `
		SELECT COALESCE(sum(amount), 0)::text FROM refunds
		WHERE shift_id = $1 AND status = 'COMPLETED' AND COALESCE(method_code, 'CASH') = 'CASH'`,
		shiftID).Scan(&t.refundsCash); err != nil {
		return nil, fmt.Errorf("totals: refunds: %w", err)
	}

	if err := q.QueryRow(ctx, `
		SELECT COALESCE(sum(amount) FILTER (WHERE type = 'IN'), 0)::text,
		       COALESCE(sum(amount) FILTER (WHERE type = 'OUT'), 0)::text
		FROM cash_movements WHERE shift_id = $1`, shiftID).Scan(&t.cashIn, &t.cashOut); err != nil {
		return nil, fmt.Errorf("totals: cash movements: %w", err)
	}

	if err := q.QueryRow(ctx, `
		SELECT count(*) FILTER (WHERE status = 'VOIDED'),
		       COALESCE(sum(amount) FILTER (WHERE status = 'VOIDED'), 0)::text,
		       count(*) FILTER (WHERE status = 'PENDING')
		FROM payments WHERE shift_id = $1`, shiftID).Scan(&t.voidCount, &t.voidAmount, &t.pending); err != nil {
		return nil, fmt.Errorf("totals: payment states: %w", err)
	}
	return t, nil
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

// InsertCashMovement mencatat kas masuk/keluar pada shift.
func (r *Repository) InsertCashMovement(ctx context.Context, outletID, shiftID, userID, movementType,
	amount string, category, reason *string) (*CashMovement, error) {
	const q = `
		INSERT INTO cash_movements (outlet_id, shift_id, type, amount, category, reason, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, shift_id, type, amount::text, category, reason, created_by::text, created_at`

	m := &CashMovement{}
	if err := r.pool.QueryRow(ctx, q, outletID, shiftID, movementType, amount, category, reason, userID).
		Scan(&m.ID, &m.ShiftID, &m.Type, &m.Amount, &m.Category, &m.Reason, &m.CreatedBy, &m.CreatedAt); err != nil {
		return nil, fmt.Errorf("insertCashMovement: %w", err)
	}
	return m, nil
}

func (r *Repository) CashMovements(ctx context.Context, outletID, shiftID string) ([]CashMovement, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, shift_id, type, amount::text, category, reason, created_by::text, created_at
		FROM cash_movements WHERE outlet_id = $1 AND shift_id = $2
		ORDER BY created_at`, outletID, shiftID)
	if err != nil {
		return nil, fmt.Errorf("cashMovements: %w", err)
	}
	defer rows.Close()

	items := []CashMovement{}
	for rows.Next() {
		var m CashMovement
		if err := rows.Scan(&m.ID, &m.ShiftID, &m.Type, &m.Amount, &m.Category, &m.Reason,
			&m.CreatedBy, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("cashMovements: scan: %w", err)
		}
		items = append(items, m)
	}
	return items, rows.Err()
}

func (r *Repository) List(ctx context.Context, outletID string, f Filter) ([]Shift, int, error) {
	where := ` WHERE outlet_id = $1`
	args := []any{outletID}
	if f.Status != "" {
		args = append(args, f.Status)
		where += fmt.Sprintf(" AND status = $%d", len(args))
	}

	var total int
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM shifts`+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("list: count: %w", err)
	}

	args = append(args, f.PerPage, (f.Page-1)*f.PerPage)
	rows, err := r.pool.Query(ctx, `SELECT `+shiftCols+` FROM shifts`+where+
		fmt.Sprintf(" ORDER BY opened_at DESC LIMIT $%d OFFSET $%d", len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list: %w", err)
	}
	defer rows.Close()

	items := []Shift{}
	for rows.Next() {
		s, err := scanShift(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("list: scan: %w", err)
		}
		items = append(items, *s)
	}
	return items, total, rows.Err()
}
