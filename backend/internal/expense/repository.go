package expense

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound       = errors.New("data tidak ditemukan")
	ErrCategoryExists = errors.New("kategori sudah ada")
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

func (r *Repository) Categories(ctx context.Context, businessID string) ([]Category, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, name, is_active, created_at FROM expense_categories
		WHERE business_id = $1 ORDER BY name`, businessID)
	if err != nil {
		return nil, fmt.Errorf("categories: %w", err)
	}
	defer rows.Close()

	items := []Category{}
	for rows.Next() {
		var c Category
		if err := rows.Scan(&c.ID, &c.Name, &c.IsActive, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("categories: scan: %w", err)
		}
		items = append(items, c)
	}
	return items, rows.Err()
}

func (r *Repository) CategoryByID(ctx context.Context, businessID, id string) (*Category, bool, error) {
	var c Category
	err := r.pool.QueryRow(ctx, `
		SELECT id, name, is_active, created_at FROM expense_categories
		WHERE id = $1 AND business_id = $2`, id, businessID).
		Scan(&c.ID, &c.Name, &c.IsActive, &c.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("categoryByID: %w", err)
	}
	return &c, true, nil
}

func (r *Repository) CreateCategory(ctx context.Context, businessID, name string) (*Category, error) {
	var c Category
	err := r.pool.QueryRow(ctx, `
		INSERT INTO expense_categories (business_id, name) VALUES ($1, $2)
		RETURNING id, name, is_active, created_at`, businessID, name).
		Scan(&c.ID, &c.Name, &c.IsActive, &c.CreatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrCategoryExists
		}
		return nil, fmt.Errorf("createCategory: %w", err)
	}
	return &c, nil
}

func (r *Repository) UpdateCategory(ctx context.Context, businessID, id string, name *string, isActive *bool) (*Category, error) {
	var c Category
	err := r.pool.QueryRow(ctx, `
		UPDATE expense_categories SET name = COALESCE($3, name), is_active = COALESCE($4, is_active),
		    updated_at = now()
		WHERE id = $1 AND business_id = $2
		RETURNING id, name, is_active, created_at`, id, businessID, name, isActive).
		Scan(&c.ID, &c.Name, &c.IsActive, &c.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrCategoryExists
		}
		return nil, fmt.Errorf("updateCategory: %w", err)
	}
	return &c, nil
}

const expenseCols = `e.id, e.category_id, c.name, e.name, e.amount::text, e.expense_date::text,
	e.payment_method_code, e.paid_from_drawer, e.shift_id::text, e.note, e.attachment_url,
	e.is_voided, e.void_reason, e.recorded_by::text, e.created_at`

func scanExpense(row pgx.Row) (*Expense, error) {
	x := &Expense{}
	if err := row.Scan(&x.ID, &x.CategoryID, &x.CategoryName, &x.Name, &x.Amount, &x.ExpenseDate,
		&x.PaymentMethodCode, &x.PaidFromDrawer, &x.ShiftID, &x.Note, &x.AttachmentURL,
		&x.IsVoided, &x.VoidReason, &x.RecordedBy, &x.CreatedAt); err != nil {
		return nil, err
	}
	return x, nil
}

func (r *Repository) ByID(ctx context.Context, outletID, id string) (*Expense, bool, error) {
	x, err := scanExpense(r.pool.QueryRow(ctx, `
		SELECT `+expenseCols+` FROM expenses e
		JOIN expense_categories c ON c.id = e.category_id
		WHERE e.id = $1 AND e.outlet_id = $2`, id, outletID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("byID: %w", err)
	}
	return x, true, nil
}

func (r *Repository) LockExpense(ctx context.Context, tx pgx.Tx, outletID, id string) (*Expense, bool, error) {
	x, err := scanExpense(tx.QueryRow(ctx, `
		SELECT `+expenseCols+` FROM expenses e
		JOIN expense_categories c ON c.id = e.category_id
		WHERE e.id = $1 AND e.outlet_id = $2 FOR UPDATE OF e`, id, outletID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("lockExpense: %w", err)
	}
	return x, true, nil
}

func (r *Repository) Create(ctx context.Context, tx pgx.Tx, outletID, userID, categoryID, name, amount,
	expenseDate string, methodCode *string, paidFromDrawer bool, shiftID, note, attachment *string) (string, error) {
	const q = `
		INSERT INTO expenses (outlet_id, category_id, name, amount, expense_date, payment_method_code,
		    paid_from_drawer, shift_id, note, attachment_url, recorded_by)
		VALUES ($1, $2, $3, $4, $5::date, $6, $7, $8::uuid, $9, $10, $11)
		RETURNING id`

	var id string
	if err := tx.QueryRow(ctx, q, outletID, categoryID, name, amount, expenseDate, methodCode,
		paidFromDrawer, shiftID, note, attachment, userID).Scan(&id); err != nil {
		return "", fmt.Errorf("create: %w", err)
	}
	return id, nil
}

func (r *Repository) Update(ctx context.Context, tx pgx.Tx, outletID, id string, categoryID *string,
	name, amount, expenseDate, methodCode *string, paidFromDrawer *bool, shiftID, note, attachment *string) error {
	const q = `
		UPDATE expenses SET category_id = COALESCE($3::uuid, category_id), name = COALESCE($4, name),
		    amount = COALESCE($5::numeric, amount), expense_date = COALESCE($6::date, expense_date),
		    payment_method_code = COALESCE($7, payment_method_code),
		    paid_from_drawer = COALESCE($8::boolean, paid_from_drawer),
		    shift_id = COALESCE($9::uuid, shift_id), note = COALESCE($10, note),
		    attachment_url = COALESCE($11, attachment_url), updated_at = now()
		WHERE id = $1 AND outlet_id = $2 AND is_voided = false`

	tag, err := tx.Exec(ctx, q, id, outletID, categoryID, name, amount, expenseDate, methodCode,
		paidFromDrawer, shiftID, note, attachment)
	if err != nil {
		return fmt.Errorf("update: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) Void(ctx context.Context, tx pgx.Tx, outletID, id, reason string) error {
	tag, err := tx.Exec(ctx, `
		UPDATE expenses SET is_voided = true, void_reason = $3, updated_at = now()
		WHERE id = $1 AND outlet_id = $2 AND is_voided = false`, id, outletID, reason)
	if err != nil {
		return fmt.Errorf("void: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// SyncDrawerMovement menjaga tepat satu cash_movement OUT per pengeluaran dari laci (BR-F17-3).
func (r *Repository) SyncDrawerMovement(ctx context.Context, tx pgx.Tx, outletID, expenseID, shiftID, userID, amount string) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO cash_movements (outlet_id, shift_id, type, amount, category, reason, expense_id, created_by)
		VALUES ($1, $2, 'OUT', $3, 'EXPENSE', 'Pengeluaran operasional', $4, $5)
		ON CONFLICT (expense_id) DO UPDATE SET amount = EXCLUDED.amount, shift_id = EXCLUDED.shift_id`,
		outletID, shiftID, amount, expenseID, userID)
	if err != nil {
		return fmt.Errorf("syncDrawerMovement: %w", err)
	}
	return nil
}

func (r *Repository) DeleteDrawerMovement(ctx context.Context, tx pgx.Tx, expenseID string) error {
	if _, err := tx.Exec(ctx, `DELETE FROM cash_movements WHERE expense_id = $1`, expenseID); err != nil {
		return fmt.Errorf("deleteDrawerMovement: %w", err)
	}
	return nil
}

func (r *Repository) List(ctx context.Context, outletID string, f Filter) ([]Expense, int, error) {
	where := ` WHERE e.outlet_id = $1`
	args := []any{outletID}
	if f.From != "" {
		args = append(args, f.From)
		where += fmt.Sprintf(" AND e.expense_date >= $%d::date", len(args))
	}
	if f.To != "" {
		args = append(args, f.To)
		where += fmt.Sprintf(" AND e.expense_date <= $%d::date", len(args))
	}
	if f.CategoryID != "" {
		args = append(args, f.CategoryID)
		where += fmt.Sprintf(" AND e.category_id = $%d", len(args))
	}
	if !f.IncludeVoided {
		where += ` AND e.is_voided = false`
	}

	var total int
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM expenses e`+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("list: count: %w", err)
	}

	args = append(args, f.PerPage, (f.Page-1)*f.PerPage)
	rows, err := r.pool.Query(ctx, `SELECT `+expenseCols+` FROM expenses e
		JOIN expense_categories c ON c.id = e.category_id`+where+
		fmt.Sprintf(" ORDER BY e.expense_date DESC, e.created_at DESC LIMIT $%d OFFSET $%d",
			len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list: %w", err)
	}
	defer rows.Close()

	items := []Expense{}
	for rows.Next() {
		x, err := scanExpense(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("list: scan: %w", err)
		}
		items = append(items, *x)
	}
	return items, total, rows.Err()
}

// OpenShift mengambil shift OPEN milik pengguna untuk pengeluaran dari laci.
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
