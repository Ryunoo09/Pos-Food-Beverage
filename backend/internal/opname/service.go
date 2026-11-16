package opname

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

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

// Create membuat sesi opname berstatus DRAFT beserta daftar bahannya.
func (s *Service) Create(ctx context.Context, actor auth.Actor, in CreateInput) (*Opname, error) {
	if actor.OutletID == "" {
		return nil, outletMissing()
	}
	scope := strings.ToUpper(strings.TrimSpace(in.Scope))
	if scope != "ALL" && scope != "SELECTED" {
		return nil, httpx.NewError(http.StatusBadRequest, "VALIDATION_ERROR",
			"scope harus ALL atau SELECTED.")
	}
	ids := dedupe(in.IngredientIDs)
	if scope == "SELECTED" && len(ids) == 0 {
		return nil, httpx.NewError(http.StatusBadRequest, "VALIDATION_ERROR",
			"ingredient_ids wajib untuk scope SELECTED.")
	}
	if scope == "SELECTED" {
		n, err := s.repo.IngredientsActive(ctx, actor.BusinessID, ids)
		if err != nil {
			return nil, err
		}
		if n != len(ids) {
			return nil, httpx.NewError(http.StatusUnprocessableEntity, "INGREDIENT_NOT_FOUND",
				"Ada bahan yang tidak ditemukan atau tidak aktif.")
		}
	}

	tx, err := s.repo.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	// ponytail: periode pakai tanggal server; pakai business_day_cutoff outlet bila perlu.
	period := time.Now().Format("060102")
	seq, err := s.repo.NextSequence(ctx, tx, actor.OutletID, "STOCK_OPNAME", period)
	if err != nil {
		return nil, err
	}
	code := opnameCode(period, seq)

	filterIDs := ids
	if scope == "ALL" {
		filterIDs = nil
	}
	id, err := s.repo.CreateOpname(ctx, tx, actor.OutletID, actor.UserID, code, scope, trimNote(in.Note))
	if err != nil {
		return nil, err
	}
	count, err := s.repo.InsertItemsSnapshot(ctx, tx, actor.OutletID, id, actor.BusinessID, filterIDs)
	if err != nil {
		return nil, err
	}
	if count == 0 {
		return nil, httpx.NewError(http.StatusUnprocessableEntity, "OPNAME_NO_INGREDIENT",
			"Tidak ada bahan aktif untuk diopname.")
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.repo.Opname(ctx, actor.OutletID, id)
}

func (s *Service) List(ctx context.Context, actor auth.Actor, status string, page, perPage int) ([]Opname, int, error) {
	if actor.OutletID == "" {
		return nil, 0, outletMissing()
	}
	return s.repo.List(ctx, actor.OutletID, strings.ToUpper(strings.TrimSpace(status)), page, perPage)
}

func (s *Service) Get(ctx context.Context, actor auth.Actor, id string) (*Opname, error) {
	if actor.OutletID == "" {
		return nil, outletMissing()
	}
	o, err := s.repo.Opname(ctx, actor.OutletID, id)
	if errors.Is(err, ErrNotFound) {
		return nil, notFound()
	}
	return o, err
}

// Start memindahkan sesi DRAFT ke IN_PROGRESS dan menyegarkan snapshot saldo.
func (s *Service) Start(ctx context.Context, actor auth.Actor, id string) (*Opname, error) {
	if actor.OutletID == "" {
		return nil, outletMissing()
	}
	tx, err := s.repo.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	status, ok, err := s.repo.StatusForUpdate(ctx, tx, actor.OutletID, id)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, notFound()
	}
	if status != "DRAFT" {
		return nil, httpx.NewError(http.StatusConflict, "OPNAME_NOT_STARTABLE",
			"Sesi opname tidak bisa dimulai pada status saat ini.")
	}

	started, err := s.repo.SetStarted(ctx, tx, actor.OutletID, id)
	if err != nil {
		return nil, err
	}
	if !started {
		return nil, httpx.NewError(http.StatusConflict, "OPNAME_NOT_STARTABLE",
			"Sesi opname tidak bisa dimulai pada status saat ini.")
	}
	if err := s.repo.ActivateItems(ctx, tx, actor.OutletID, id); err != nil {
		if isUniqueViolation(err) {
			return nil, httpx.NewError(http.StatusConflict, "INGREDIENT_IN_ACTIVE_OPNAME",
				"Ada bahan yang sudah masuk sesi opname aktif lain.")
		}
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.repo.Opname(ctx, actor.OutletID, id)
}

// Count mencatat hasil hitung fisik untuk satu atau beberapa bahan.
func (s *Service) Count(ctx context.Context, actor auth.Actor, id string, in CountInput) (*Opname, error) {
	if actor.OutletID == "" {
		return nil, outletMissing()
	}
	if len(in.Items) == 0 {
		return nil, httpx.NewError(http.StatusBadRequest, "VALIDATION_ERROR", "items wajib diisi.")
	}
	type parsed struct {
		ingredientID string
		qty          string
		reason       *string
	}
	rows := make([]parsed, 0, len(in.Items))
	for _, it := range in.Items {
		ingredientID := strings.TrimSpace(it.IngredientID)
		if ingredientID == "" {
			return nil, httpx.NewError(http.StatusBadRequest, "VALIDATION_ERROR", "ingredient_id wajib diisi.")
		}
		qty, err := money.Parse(strings.TrimSpace(it.CountedQuantity))
		if err != nil || qty.IsNegative() {
			return nil, httpx.NewError(http.StatusBadRequest, "VALIDATION_ERROR",
				"counted_quantity tidak valid.")
		}
		rows = append(rows, parsed{ingredientID: ingredientID, qty: qty.String(), reason: trimNote(it.Reason)})
	}

	tx, err := s.repo.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	status, ok, err := s.repo.StatusForUpdate(ctx, tx, actor.OutletID, id)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, notFound()
	}
	if status != "IN_PROGRESS" {
		return nil, httpx.NewError(http.StatusConflict, "OPNAME_NOT_COUNTABLE",
			"Sesi opname belum dimulai atau sudah dikunci.")
	}

	for _, it := range rows {
		found, err := s.repo.CountItem(ctx, tx, id, it.ingredientID, it.qty, actor.UserID, it.reason)
		if err != nil {
			return nil, err
		}
		if !found {
			return nil, httpx.NewError(http.StatusUnprocessableEntity, "OPNAME_ITEM_NOT_FOUND",
				"Bahan tidak termasuk dalam sesi opname ini.")
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.repo.Opname(ctx, actor.OutletID, id)
}

// Submit mengunci sesi setelah seluruh bahan selesai dihitung.
func (s *Service) Submit(ctx context.Context, actor auth.Actor, id string) (*Opname, error) {
	if actor.OutletID == "" {
		return nil, outletMissing()
	}
	tx, err := s.repo.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	status, _, found, err := s.repo.LockOpname(ctx, tx, actor.OutletID, id)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, notFound()
	}
	if status != "IN_PROGRESS" {
		return nil, httpx.NewError(http.StatusConflict, "OPNAME_NOT_SUBMITTABLE",
			"Sesi opname tidak bisa diajukan pada status saat ini.")
	}

	total, counted, err := s.repo.CountStats(ctx, tx, id)
	if err != nil {
		return nil, err
	}
	if counted < total {
		return nil, httpx.NewError(http.StatusUnprocessableEntity, "OPNAME_INCOMPLETE",
			"Semua bahan harus dihitung sebelum diajukan.")
	}

	ok, err := s.repo.SetSubmitted(ctx, tx, actor.OutletID, id, actor.UserID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, httpx.NewError(http.StatusConflict, "OPNAME_NOT_SUBMITTABLE",
			"Sesi opname tidak bisa diajukan pada status saat ini.")
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.repo.Opname(ctx, actor.OutletID, id)
}

// Approve menetapkan stok ke hasil hitung fisik dan menulis movement ADJUSTMENT.
func (s *Service) Approve(ctx context.Context, actor auth.Actor, id string) (*Opname, error) {
	if actor.OutletID == "" {
		return nil, outletMissing()
	}
	tx, err := s.repo.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	status, submittedBy, found, err := s.repo.LockOpname(ctx, tx, actor.OutletID, id)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, notFound()
	}
	if status != "SUBMITTED" {
		return nil, httpx.NewError(http.StatusConflict, "OPNAME_NOT_APPROVABLE",
			"Sesi opname tidak bisa disetujui pada status saat ini.")
	}

	allow, err := s.repo.AllowSelfApproval(ctx, actor.OutletID)
	if err != nil {
		return nil, err
	}
	if !allow && submittedBy != nil && *submittedBy == actor.UserID {
		return nil, httpx.NewError(http.StatusUnprocessableEntity, "OPNAME_SELF_APPROVAL",
			"Pengaju tidak boleh menyetujui opname miliknya sendiri.")
	}

	items, err := s.repo.CountedItems(ctx, tx, id)
	if err != nil {
		return nil, err
	}
	for _, it := range items {
		current, avg, err := s.repo.LockInventoryForOpname(ctx, tx, actor.OutletID, it.IngredientID)
		if err != nil {
			return nil, err
		}
		cur, err := money.Parse(current)
		if err != nil {
			return nil, err
		}
		counted, err := money.Parse(it.Quantity)
		if err != nil {
			return nil, err
		}
		cost, err := money.Parse(avg)
		if err != nil {
			return nil, err
		}

		// ponytail: expected = saldo saat approve (bukan snapshot + replay movement);
		// ganti ke replay bila butuh audit selisih per gerakan.
		delta := counted.Sub(cur)
		expected := money.Format(cur)
		diff := money.Format(delta)
		value := money.Format(delta.Mul(cost))

		if delta.IsZero() {
			if err := s.repo.UpdateItemResult(ctx, tx, it.ID, expected, diff, value, nil); err != nil {
				return nil, err
			}
			continue
		}

		balance := money.Format(counted)
		movementID, err := s.repo.InsertOpnameAdjustment(ctx, tx, actor.OutletID, id,
			it.IngredientID, diff, avg, balance, actor.UserID, it.Reason)
		if err != nil {
			if isUniqueViolation(err) {
				return nil, httpx.NewError(http.StatusConflict, "OPNAME_ALREADY_ADJUSTED",
					"Bahan pada sesi ini sudah pernah disesuaikan.")
			}
			return nil, err
		}
		if err := s.repo.UpdateInventoryAfterOpname(ctx, tx, actor.OutletID, it.IngredientID, balance); err != nil {
			return nil, err
		}
		if err := s.repo.UpdateItemResult(ctx, tx, it.ID, expected, diff, value, &movementID); err != nil {
			return nil, err
		}
	}

	ok, err := s.repo.SetApproved(ctx, tx, actor.OutletID, id, actor.UserID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, httpx.NewError(http.StatusConflict, "OPNAME_NOT_APPROVABLE",
			"Sesi opname tidak bisa disetujui pada status saat ini.")
	}
	if err := s.repo.DeactivateItems(ctx, tx, id); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.repo.Opname(ctx, actor.OutletID, id)
}

// Reject menolak sesi tanpa mengubah stok dan melepas itemnya.
func (s *Service) Reject(ctx context.Context, actor auth.Actor, id string, in RejectInput) (*Opname, error) {
	if actor.OutletID == "" {
		return nil, outletMissing()
	}
	reason := strings.TrimSpace(in.Reason)
	if reason == "" {
		return nil, httpx.NewError(http.StatusBadRequest, "VALIDATION_ERROR", "reason wajib diisi.")
	}
	tx, err := s.repo.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	status, _, found, err := s.repo.LockOpname(ctx, tx, actor.OutletID, id)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, notFound()
	}
	if status != "SUBMITTED" {
		return nil, httpx.NewError(http.StatusConflict, "OPNAME_NOT_REJECTABLE",
			"Sesi opname tidak bisa ditolak pada status saat ini.")
	}
	ok, err := s.repo.SetRejected(ctx, tx, actor.OutletID, id, reason)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, httpx.NewError(http.StatusConflict, "OPNAME_NOT_REJECTABLE",
			"Sesi opname tidak bisa ditolak pada status saat ini.")
	}
	if err := s.repo.DeactivateItems(ctx, tx, id); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.repo.Opname(ctx, actor.OutletID, id)
}

// Cancel membatalkan sesi yang belum final dan melepas itemnya.
func (s *Service) Cancel(ctx context.Context, actor auth.Actor, id string) (*Opname, error) {
	if actor.OutletID == "" {
		return nil, outletMissing()
	}
	tx, err := s.repo.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	status, _, found, err := s.repo.LockOpname(ctx, tx, actor.OutletID, id)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, notFound()
	}
	if status != "DRAFT" && status != "IN_PROGRESS" && status != "SUBMITTED" {
		return nil, httpx.NewError(http.StatusConflict, "OPNAME_NOT_CANCELLABLE",
			"Sesi opname tidak bisa dibatalkan pada status saat ini.")
	}
	ok, err := s.repo.SetCancelled(ctx, tx, actor.OutletID, id)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, httpx.NewError(http.StatusConflict, "OPNAME_NOT_CANCELLABLE",
			"Sesi opname tidak bisa dibatalkan pada status saat ini.")
	}
	if err := s.repo.DeactivateItems(ctx, tx, id); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.repo.Opname(ctx, actor.OutletID, id)
}

func opnameCode(period string, n int) string {
	return fmt.Sprintf("OPN-%s-%04d", period, n)
}

func dedupe(ids []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
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

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func notFound() error {
	return httpx.NewError(http.StatusNotFound, "OPNAME_NOT_FOUND", "Sesi opname tidak ditemukan.")
}

func outletMissing() error {
	return httpx.NewError(http.StatusUnprocessableEntity, "OUTLET_NOT_CONFIGURED",
		"Outlet belum dikonfigurasi.")
}
