package expense

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/ryunoo09/pos-food-beverage/backend/internal/auth"
	"github.com/ryunoo09/pos-food-beverage/backend/internal/httpx"
	"github.com/ryunoo09/pos-food-beverage/backend/internal/pkg/money"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) ListCategories(ctx context.Context, actor auth.Actor) ([]Category, error) {
	return s.repo.Categories(ctx, actor.BusinessID)
}

func (s *Service) CreateCategory(ctx context.Context, actor auth.Actor, in CategoryInput) (*Category, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return nil, validation("name wajib diisi.")
	}
	c, err := s.repo.CreateCategory(ctx, actor.BusinessID, name)
	if errors.Is(err, ErrCategoryExists) {
		return nil, conflict("EXPENSE_CATEGORY_EXISTS", "Kategori pengeluaran sudah ada.")
	}
	return c, err
}

func (s *Service) UpdateCategory(ctx context.Context, actor auth.Actor, id string, in CategoryInput) (*Category, error) {
	var name *string
	if v := strings.TrimSpace(in.Name); v != "" {
		name = &v
	}
	c, err := s.repo.UpdateCategory(ctx, actor.BusinessID, id, name, in.IsActive)
	if errors.Is(err, ErrNotFound) {
		return nil, notFound()
	}
	if errors.Is(err, ErrCategoryExists) {
		return nil, conflict("EXPENSE_CATEGORY_EXISTS", "Kategori pengeluaran sudah ada.")
	}
	return c, err
}

func (s *Service) List(ctx context.Context, actor auth.Actor, f Filter) ([]Expense, httpx.PageMeta, error) {
	if actor.OutletID == "" {
		return nil, httpx.PageMeta{}, outletMissing()
	}
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PerPage < 1 || f.PerPage > 100 {
		f.PerPage = 20
	}
	items, total, err := s.repo.List(ctx, actor.OutletID, f)
	if err != nil {
		return nil, httpx.PageMeta{}, err
	}
	return items, httpx.PageMeta{Page: f.Page, PerPage: f.PerPage, Total: total}, nil
}

func (s *Service) Create(ctx context.Context, actor auth.Actor, in CreateInput) (*Expense, error) {
	if actor.OutletID == "" {
		return nil, outletMissing()
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return nil, validation("name wajib diisi.")
	}
	amount, err := money.Parse(strings.TrimSpace(in.Amount))
	if err != nil || !amount.IsPositive() {
		return nil, validation("amount harus lebih dari 0.")
	}
	date, err := expenseDate(in.ExpenseDate)
	if err != nil {
		return nil, err
	}
	category, found, err := s.repo.CategoryByID(ctx, actor.BusinessID, in.CategoryID)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, conflict("EXPENSE_CATEGORY_INVALID", "Kategori pengeluaran tidak valid.")
	}
	if !category.IsActive {
		return nil, conflict("EXPENSE_CATEGORY_INVALID", "Kategori pengeluaran tidak aktif.")
	}

	var shiftID *string
	if in.PaidFromDrawer {
		id, ok, err := s.repo.OpenShift(ctx, actor.UserID)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, httpx.NewError(http.StatusUnprocessableEntity, "SHIFT_NOT_OPEN",
				"Pengeluaran dari laci memerlukan shift terbuka.")
		}
		shiftID = &id
	}

	tx, err := s.repo.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	id, err := s.repo.Create(ctx, tx, actor.OutletID, actor.UserID, in.CategoryID, name,
		money.Format(amount), date, trimNote(in.PaymentMethodCode), in.PaidFromDrawer, shiftID,
		trimNote(in.Note), trimNote(in.AttachmentURL))
	if err != nil {
		return nil, err
	}
	if in.PaidFromDrawer {
		if err := s.repo.SyncDrawerMovement(ctx, tx, actor.OutletID, id, *shiftID, actor.UserID,
			money.Format(amount)); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.load(ctx, actor.OutletID, id)
}

func (s *Service) Update(ctx context.Context, actor auth.Actor, id string, in UpdateInput) (*Expense, error) {
	if actor.OutletID == "" {
		return nil, outletMissing()
	}
	var amount *string
	if in.Amount != nil {
		v, err := money.Parse(strings.TrimSpace(*in.Amount))
		if err != nil || !v.IsPositive() {
			return nil, validation("amount harus lebih dari 0.")
		}
		formatted := money.Format(v)
		amount = &formatted
	}
	var date *string
	if in.ExpenseDate != nil {
		v, err := expenseDate(in.ExpenseDate)
		if err != nil {
			return nil, err
		}
		date = &v
	}
	if in.CategoryID != nil {
		category, found, err := s.repo.CategoryByID(ctx, actor.BusinessID, *in.CategoryID)
		if err != nil {
			return nil, err
		}
		if !found || !category.IsActive {
			return nil, conflict("EXPENSE_CATEGORY_INVALID", "Kategori pengeluaran tidak valid.")
		}
	}

	tx, err := s.repo.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	current, found, err := s.repo.LockExpense(ctx, tx, actor.OutletID, id)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, notFound()
	}
	if current.IsVoided {
		return nil, conflict("EXPENSE_ALREADY_VOIDED", "Pengeluaran sudah dibatalkan.")
	}

	var shiftID *string
	paidFromDrawer := current.PaidFromDrawer
	if in.PaidFromDrawer != nil {
		paidFromDrawer = *in.PaidFromDrawer
	}
	if paidFromDrawer {
		sid, ok, err := s.repo.OpenShift(ctx, actor.UserID)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, httpx.NewError(http.StatusUnprocessableEntity, "SHIFT_NOT_OPEN",
				"Pengeluaran dari laci memerlukan shift terbuka.")
		}
		shiftID = &sid
	}

	if err := s.repo.Update(ctx, tx, actor.OutletID, id, in.CategoryID, trimNote(in.Name), amount, date,
		trimNote(in.PaymentMethodCode), in.PaidFromDrawer, shiftID, trimNote(in.Note),
		trimNote(in.AttachmentURL)); err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, notFound()
		}
		return nil, err
	}

	switch {
	case paidFromDrawer:
		newAmount := current.Amount
		if amount != nil {
			newAmount = *amount
		}
		if err := s.repo.SyncDrawerMovement(ctx, tx, actor.OutletID, id, *shiftID, actor.UserID, newAmount); err != nil {
			return nil, err
		}
	default:
		if err := s.repo.DeleteDrawerMovement(ctx, tx, id); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.load(ctx, actor.OutletID, id)
}

func (s *Service) Void(ctx context.Context, actor auth.Actor, id string, in VoidInput) (*Expense, error) {
	if actor.OutletID == "" {
		return nil, outletMissing()
	}
	reason := strings.TrimSpace(in.Reason)
	if reason == "" {
		return nil, validation("reason wajib diisi.")
	}

	tx, err := s.repo.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	current, found, err := s.repo.LockExpense(ctx, tx, actor.OutletID, id)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, notFound()
	}
	if current.IsVoided {
		return nil, conflict("EXPENSE_ALREADY_VOIDED", "Pengeluaran sudah dibatalkan.")
	}
	if err := s.repo.Void(ctx, tx, actor.OutletID, id, reason); err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, notFound()
		}
		return nil, err
	}
	// Mutasi kas ikut dibatalkan agar expected_cash shift kembali konsisten.
	if err := s.repo.DeleteDrawerMovement(ctx, tx, id); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.load(ctx, actor.OutletID, id)
}

func (s *Service) load(ctx context.Context, outletID, id string) (*Expense, error) {
	x, found, err := s.repo.ByID(ctx, outletID, id)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, notFound()
	}
	return x, nil
}

// expenseDate memvalidasi tanggal (YYYY-MM-DD) dan menolak lebih dari 1 tahun ke belakang.
func expenseDate(in *string) (string, error) {
	if in == nil || strings.TrimSpace(*in) == "" {
		return time.Now().Format("2006-01-02"), nil
	}
	d, err := time.Parse("2006-01-02", strings.TrimSpace(*in))
	if err != nil {
		return "", validation("expense_date harus berformat YYYY-MM-DD.")
	}
	if d.Before(time.Now().AddDate(-1, 0, 0)) {
		return "", validation("expense_date tidak boleh lebih dari 1 tahun ke belakang.")
	}
	return d.Format("2006-01-02"), nil
}

func trimNote(s *string) *string {
	if s == nil {
		return nil
	}
	v := strings.TrimSpace(*s)
	if v == "" {
		return nil
	}
	return &v
}

func validation(msg string) error {
	return httpx.NewError(http.StatusBadRequest, "VALIDATION_ERROR", msg)
}

func conflict(code, msg string) error {
	return httpx.NewError(http.StatusUnprocessableEntity, code, msg)
}

func notFound() error {
	return httpx.NewError(http.StatusNotFound, "EXPENSE_NOT_FOUND", "Pengeluaran tidak ditemukan.")
}

func outletMissing() error {
	return httpx.NewError(http.StatusUnprocessableEntity, "OUTLET_NOT_CONFIGURED",
		"Outlet belum dikonfigurasi.")
}
