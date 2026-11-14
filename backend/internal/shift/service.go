package shift

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/ryunoo09/pos-food-beverage/backend/internal/audit"
	"github.com/ryunoo09/pos-food-beverage/backend/internal/auth"
	"github.com/ryunoo09/pos-food-beverage/backend/internal/httpx"
	"github.com/ryunoo09/pos-food-beverage/backend/internal/pkg/money"
)

type Service struct {
	repo  *Repository
	audit *audit.Writer
}

func NewService(repo *Repository, writer *audit.Writer) *Service {
	return &Service{repo: repo, audit: writer}
}

// Open membuka shift baru untuk kasir; satu shift OPEN per kasir (BR-F16-1).
func (s *Service) Open(ctx context.Context, actor auth.Actor, in OpenInput) (*Shift, error) {
	if actor.OutletID == "" {
		return nil, outletMissing()
	}
	opening, err := money.Parse(strings.TrimSpace(in.OpeningCash))
	if err != nil || opening.IsNegative() {
		return nil, validation("opening_cash tidak valid.")
	}
	if _, found, err := s.repo.Current(ctx, actor.UserID); err != nil {
		return nil, err
	} else if found {
		return nil, alreadyOpen()
	}
	shift, err := s.repo.Insert(ctx, actor.OutletID, actor.UserID, money.Format(opening))
	if err != nil {
		if errors.Is(err, ErrAlreadyOpen) {
			return nil, alreadyOpen()
		}
		return nil, err
	}
	return shift, nil
}

// Current mengambil shift OPEN milik pengguna.
func (s *Service) Current(ctx context.Context, actor auth.Actor) (*Shift, error) {
	shift, found, err := s.repo.Current(ctx, actor.UserID)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, httpx.NewError(http.StatusNotFound, "SHIFT_NOT_OPEN", "Belum ada shift terbuka.")
	}
	return shift, nil
}

// Summary menghitung laporan kas shift (FR-F16-4/5).
func (s *Service) Summary(ctx context.Context, actor auth.Actor, id string) (*Summary, error) {
	if actor.OutletID == "" {
		return nil, outletMissing()
	}
	shift, found, err := s.repo.ByID(ctx, actor.OutletID, id)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, notFound()
	}
	t, err := s.repo.Totals(ctx, s.repo.pool, id)
	if err != nil {
		return nil, err
	}
	return buildSummary(shift, t)
}

// Close menutup shift: hitung expected cash, kunci selisih, simpan snapshot (FR-F16-4).
func (s *Service) Close(ctx context.Context, actor auth.Actor, id, key string, in CloseInput) (*Summary, error) {
	if actor.OutletID == "" {
		return nil, outletMissing()
	}
	if strings.TrimSpace(key) == "" {
		return nil, httpx.NewError(http.StatusBadRequest, "IDEMPOTENCY_KEY_REQUIRED",
			"Header Idempotency-Key wajib diisi.")
	}
	scope := actor.OutletID + ":" + actor.UserID + ":POST /shifts/:id/close"
	hash := sha256.Sum256([]byte(id + "|" + in.ActualCash + "|" + deref(in.DifferenceReason)))

	claimed, err := s.repo.ClaimIdempotency(ctx, scope, key, hash[:])
	if err != nil {
		return nil, err
	}
	if !claimed {
		stored, status, body, found, err := s.repo.IdempotencyEntry(ctx, scope, key)
		if err != nil {
			return nil, err
		}
		if !found || !bytes.Equal(stored, hash[:]) {
			return nil, httpx.NewError(http.StatusConflict, "IDEMPOTENCY_CONFLICT",
				"Kunci idempotensi sudah dipakai untuk permintaan berbeda.")
		}
		if status == 0 {
			return nil, httpx.NewError(http.StatusConflict, "IDEMPOTENCY_IN_PROGRESS",
				"Permintaan dengan kunci ini sedang diproses.")
		}
		var summary Summary
		if err := json.Unmarshal(body, &summary); err != nil {
			return nil, err
		}
		return &summary, nil
	}

	summary, err := s.close(ctx, actor, id, in)
	if err != nil {
		_ = s.repo.ReleaseIdempotency(ctx, scope, key)
		return nil, err
	}
	body, err := json.Marshal(summary)
	if err != nil {
		return nil, err
	}
	if err := s.repo.SaveIdempotencyResponse(ctx, scope, key, http.StatusOK, body); err != nil {
		return nil, err
	}
	return summary, nil
}

func (s *Service) close(ctx context.Context, actor auth.Actor, id string, in CloseInput) (*Summary, error) {
	actual, err := money.Parse(strings.TrimSpace(in.ActualCash))
	if err != nil || actual.IsNegative() {
		return nil, validation("actual_cash tidak valid.")
	}
	reason := trimNote(in.DifferenceReason)

	tx, err := s.repo.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	shift, found, err := s.repo.LockShift(ctx, tx, actor.OutletID, id)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, notFound()
	}
	if shift.Status != "OPEN" {
		return nil, httpx.NewError(http.StatusConflict, "SHIFT_ALREADY_CLOSED", "Shift sudah ditutup.")
	}

	t, err := s.repo.Totals(ctx, tx, id)
	if err != nil {
		return nil, err
	}
	if t.pending > 0 {
		return nil, httpx.NewError(http.StatusUnprocessableEntity, "SHIFT_HAS_PENDING_PAYMENTS",
			"Masih ada pembayaran menggantung; selesaikan atau batalkan dahulu.")
	}

	summary, err := buildSummary(shift, t)
	if err != nil {
		return nil, err
	}
	expected, err := money.Parse(summary.ExpectedCash)
	if err != nil {
		return nil, err
	}
	difference := actual.Sub(expected)
	if !difference.IsZero() && reason == nil {
		return nil, httpx.NewError(http.StatusUnprocessableEntity, "SHIFT_DIFFERENCE_REASON_REQUIRED",
			"Alasan wajib diisi bila terdapat selisih kas.")
	}
	summary.Difference = money.Format(difference)

	snapshot, err := json.Marshal(summary)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Close(ctx, tx, id, summary.ExpectedCash, money.Format(actual),
		summary.Difference, reason, snapshot); err != nil {
		return nil, err
	}
	if err := s.audit.Insert(ctx, tx, audit.Entry{
		Actor:      actor.Audit(),
		Action:     "shift.close",
		EntityType: "shift",
		EntityID:   id,
		Before:     map[string]any{"status": "OPEN"},
		After: map[string]any{
			"status":        "CLOSED",
			"expected_cash": summary.ExpectedCash,
			"actual_cash":   money.Format(actual),
			"difference":    summary.Difference,
		},
		Reason: deref(reason),
	}); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	closed, found, err := s.repo.ByID(ctx, actor.OutletID, id)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, notFound()
	}
	summary.Shift = *closed
	return summary, nil
}

func (s *Service) List(ctx context.Context, actor auth.Actor, f Filter) ([]Shift, httpx.PageMeta, error) {
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

// buildSummary menerapkan rumus expected_cash = modal + tunai - refund tunai + kas masuk - kas keluar.
func buildSummary(shift *Shift, t *totals) (*Summary, error) {
	opening, err := money.Parse(shift.OpeningCash)
	if err != nil {
		return nil, err
	}
	cash, err := money.Parse("0")
	if err != nil {
		return nil, err
	}
	for _, m := range t.byMethod {
		if m.MethodCode != "CASH" {
			continue
		}
		v, err := money.Parse(m.Amount)
		if err != nil {
			return nil, err
		}
		cash = v
	}
	refunds, err := money.Parse(t.refundsCash)
	if err != nil {
		return nil, err
	}
	in, err := money.Parse(t.cashIn)
	if err != nil {
		return nil, err
	}
	out, err := money.Parse(t.cashOut)
	if err != nil {
		return nil, err
	}
	expected := opening.Add(cash).Sub(refunds).Add(in).Sub(out)

	methods := t.byMethod
	if methods == nil {
		methods = []MethodTotal{}
	}
	return &Summary{
		Shift:               *shift,
		CashSales:           money.Format(cash),
		CashRefunds:         money.Format(refunds),
		CashIn:              money.Format(in),
		CashOut:             money.Format(out),
		ExpectedCash:        money.Format(expected),
		ByMethod:            methods,
		VoidedPaymentCount:  t.voidCount,
		VoidedPaymentAmount: t.voidAmount,
		PendingPaymentCount: t.pending,
		Difference:          "0.00",
	}, nil
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

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func validation(msg string) error {
	return httpx.NewError(http.StatusBadRequest, "VALIDATION_ERROR", msg)
}

func notFound() error {
	return httpx.NewError(http.StatusNotFound, "SHIFT_NOT_FOUND", "Shift tidak ditemukan.")
}

func alreadyOpen() error {
	return httpx.NewError(http.StatusConflict, "SHIFT_ALREADY_OPEN", "Kasir masih memiliki shift terbuka.")
}

func outletMissing() error {
	return httpx.NewError(http.StatusUnprocessableEntity, "OUTLET_NOT_CONFIGURED",
		"Outlet belum dikonfigurasi.")
}

// AddCashMovement mencatat kas masuk/keluar pada shift yang masih terbuka (FR-F16-3).
func (s *Service) AddCashMovement(ctx context.Context, actor auth.Actor, shiftID string, in CashMovementInput) (*CashMovement, error) {
	if actor.OutletID == "" {
		return nil, outletMissing()
	}
	if in.Type != "IN" && in.Type != "OUT" {
		return nil, validation("type harus IN atau OUT.")
	}
	amount, err := money.Parse(strings.TrimSpace(in.Amount))
	if err != nil || !amount.IsPositive() {
		return nil, validation("amount harus lebih dari 0.")
	}
	reason := trimNote(in.Reason)
	if reason == nil {
		return nil, validation("reason wajib diisi.")
	}

	shift, found, err := s.repo.ByID(ctx, actor.OutletID, shiftID)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, notFound()
	}
	if shift.Status != "OPEN" {
		return nil, httpx.NewError(http.StatusUnprocessableEntity, "SHIFT_ALREADY_CLOSED",
			"Shift sudah ditutup; kas tidak dapat diubah.")
	}
	return s.repo.InsertCashMovement(ctx, actor.OutletID, shiftID, actor.UserID, in.Type,
		money.Format(amount), trimNote(in.Category), reason)
}

// CashMovements menampilkan kas masuk/keluar sebuah shift.
func (s *Service) CashMovements(ctx context.Context, actor auth.Actor, shiftID string) ([]CashMovement, error) {
	if actor.OutletID == "" {
		return nil, outletMissing()
	}
	if _, found, err := s.repo.ByID(ctx, actor.OutletID, shiftID); err != nil {
		return nil, err
	} else if !found {
		return nil, notFound()
	}
	return s.repo.CashMovements(ctx, actor.OutletID, shiftID)
}
