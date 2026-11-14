package audit

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Writer struct {
	pool *pgxpool.Pool
}

func NewWriter(pool *pgxpool.Pool) *Writer {
	return &Writer{pool: pool}
}

// Insert menulis satu baris audit dalam transaksi bisnis pemanggil (fail closed).
func (w *Writer) Insert(ctx context.Context, tx pgx.Tx, e Entry) error {
	const q = `
		INSERT INTO audit_logs (business_id, outlet_id, user_id, user_roles_snapshot, action,
		    entity_type, entity_id, before_data, after_data, reason, ip_address, user_agent,
		    request_id, self_approved)
		VALUES ($1::uuid, $2::uuid, $3::uuid, $4, $5, $6, $7, $8, $9, $10, $11::inet, $12, $13, $14)`

	if _, err := tx.Exec(ctx, q, nullUUID(e.Actor.BusinessID), nullUUID(e.Actor.OutletID),
		nullUUID(e.Actor.UserID), rolesOrNil(e.Actor.Roles), e.Action,
		nullText(e.EntityType), nullText(e.EntityID), Redact(e.Before), Redact(e.After),
		nullText(e.Reason), nullText(e.Actor.IPAddress), nullText(e.Actor.UserAgent),
		nullText(e.Actor.RequestID), e.SelfApproved); err != nil {
		return fmt.Errorf("audit insert: %w", err)
	}
	return nil
}

// Record menulis audit di luar transaksi bisnis; hanya untuk aksi yang tidak mengubah data inti.
func (w *Writer) Record(ctx context.Context, e Entry) error {
	tx, err := w.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := w.Insert(ctx, tx, e); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (w *Writer) List(ctx context.Context, businessID string, f Filter) ([]Log, int, error) {
	where := ` WHERE business_id = $1`
	args := []any{businessID}
	add := func(v, clause string) {
		args = append(args, v)
		where += fmt.Sprintf(clause, len(args))
	}
	if f.Action != "" {
		add(f.Action, " AND action = $%d")
	}
	if f.EntityType != "" {
		add(f.EntityType, " AND entity_type = $%d")
	}
	if f.EntityID != "" {
		add(f.EntityID, " AND entity_id = $%d")
	}
	if f.UserID != "" {
		add(f.UserID, " AND user_id = $%d")
	}
	if f.From != "" {
		add(f.From, " AND occurred_at >= $%d::date")
	}
	if f.To != "" {
		add(f.To, " AND occurred_at < $%d::date + interval '1 day'")
	}

	var total int
	if err := w.pool.QueryRow(ctx, `SELECT count(*) FROM audit_logs`+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("audit list: count: %w", err)
	}

	args = append(args, f.PerPage, (f.Page-1)*f.PerPage)
	rows, err := w.pool.Query(ctx, `SELECT id, occurred_at, user_id::text, user_roles_snapshot, action,
			entity_type, entity_id, outlet_id::text, before_data, after_data, reason, ip_address::text,
			user_agent, request_id
		FROM audit_logs`+where+fmt.Sprintf(
		" ORDER BY occurred_at DESC, id DESC LIMIT $%d OFFSET $%d", len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, fmt.Errorf("audit list: %w", err)
	}
	defer rows.Close()

	items := []Log{}
	for rows.Next() {
		var l Log
		if err := rows.Scan(&l.ID, &l.OccurredAt, &l.UserID, &l.Roles, &l.Action, &l.EntityType,
			&l.EntityID, &l.OutletID, &l.Before, &l.After, &l.Reason, &l.IPAddress,
			&l.UserAgent, &l.RequestID); err != nil {
			return nil, 0, fmt.Errorf("audit list: scan: %w", err)
		}
		items = append(items, l)
	}
	return items, total, rows.Err()
}

func (w *Writer) All(ctx context.Context, businessID string, f Filter) ([]Log, error) {
	f.Page = 1
	f.PerPage = 10000
	items, _, err := w.List(ctx, businessID, f)
	return items, err
}

func nullUUID(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nullText(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func rolesOrNil(roles []string) any {
	if len(roles) == 0 {
		return nil
	}
	return roles
}
