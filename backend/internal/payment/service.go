package payment

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"net/http"
	"strings"

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

// Create mencatat pembayaran dengan proteksi idempotensi.
func (s *Service) Create(ctx context.Context, actor auth.Actor, orderID, key string, in CreateInput) (*CreateResult, error) {
	if actor.OutletID == "" {
		return nil, outletMissing()
	}
	if strings.TrimSpace(key) == "" {
		return nil, httpx.NewError(http.StatusBadRequest, "IDEMPOTENCY_KEY_REQUIRED",
			"Header Idempotency-Key wajib diisi.")
	}
	scope := actor.OutletID + ":" + actor.UserID + ":POST /orders/:id/payments"
	raw := strings.Join([]string{orderID, in.MethodCode, in.Amount, deref(in.CashReceived), deref(in.ReferenceNo)}, "|")
	hash := sha256.Sum256([]byte(raw))

	claimed, err := s.repo.ClaimIdempotency(ctx, scope, key, hash[:])
	if err != nil {
		return nil, err
	}
	if !claimed {
		stored, status, body, found, err := s.repo.IdempotencyEntry(ctx, scope, key)
		if err != nil {
			return nil, err
		}
		if !found {
			return nil, httpx.NewError(http.StatusConflict, "IDEMPOTENCY_CONFLICT",
				"Kunci idempotensi bertabrakan.")
		}
		if !bytes.Equal(stored, hash[:]) {
			return nil, httpx.NewError(http.StatusConflict, "IDEMPOTENCY_CONFLICT",
				"Kunci idempotensi sudah dipakai untuk permintaan berbeda.")
		}
		if status == 0 {
			return nil, httpx.NewError(http.StatusConflict, "IDEMPOTENCY_IN_PROGRESS",
				"Permintaan dengan kunci ini sedang diproses.")
		}
		var result CreateResult
		if err := json.Unmarshal(body, &result); err != nil {
			return nil, err
		}
		return &result, nil
	}

	result, err := s.create(ctx, actor, orderID, key, in)
	if err != nil {
		_ = s.repo.ReleaseIdempotency(ctx, scope, key)
		return nil, err
	}
	body, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	if err := s.repo.SaveIdempotencyResponse(ctx, scope, key, http.StatusCreated, body); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *Service) create(ctx context.Context, actor auth.Actor, orderID, key string, in CreateInput) (*CreateResult, error) {
	if strings.TrimSpace(in.MethodCode) == "" {
		return nil, validation("method_code wajib diisi.")
	}
	amount, err := money.Parse(strings.TrimSpace(in.Amount))
	if err != nil || !amount.IsPositive() {
		return nil, validation("amount harus lebih dari 0.")
	}

	shiftID, ok, err := s.repo.OpenShift(ctx, actor.UserID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, httpx.NewError(http.StatusUnprocessableEntity, "SHIFT_NOT_OPEN",
			"Kasir belum membuka shift.")
	}

	tx, err := s.repo.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	order, found, err := s.repo.LockOrder(ctx, tx, actor.OutletID, orderID)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, notFound()
	}
	if order.Status == "CANCELLED" || order.Status == "DRAFT" {
		return nil, httpx.NewError(http.StatusUnprocessableEntity, "ORDER_NOT_PAYABLE",
			"Pesanan belum dikonfirmasi atau sudah dibatalkan.")
	}

	total, err := money.Parse(order.Total)
	if err != nil {
		return nil, err
	}
	paid, err := money.Parse(order.Paid)
	if err != nil {
		return nil, err
	}
	remaining := total.Sub(paid)
	if !remaining.IsPositive() {
		return nil, httpx.NewError(http.StatusConflict, "ORDER_ALREADY_PAID",
			"Pesanan sudah lunas.")
	}

	applied := amount
	var cashReceived, change *string
	if in.CashReceived != nil && strings.TrimSpace(*in.CashReceived) != "" {
		cash, err := money.Parse(strings.TrimSpace(*in.CashReceived))
		if err != nil || !cash.IsPositive() {
			return nil, validation("cash_received tidak valid.")
		}
		if cash.LessThan(amount) {
			return nil, httpx.NewError(http.StatusUnprocessableEntity, "CASH_INSUFFICIENT",
				"Uang diterima kurang dari jumlah bayar.")
		}
		cashText := money.Format(cash)
		changeText := money.Format(cash.Sub(amount))
		cashReceived, change = &cashText, &changeText
	}
	if applied.GreaterThan(remaining) {
		return nil, httpx.NewError(http.StatusUnprocessableEntity, "PAYMENT_EXCEEDS_TOTAL",
			"Jumlah bayar melebihi sisa tagihan.")
	}

	paymentID, err := s.repo.InsertPayment(ctx, tx, actor.OutletID, orderID, shiftID, actor.UserID,
		in.MethodCode, money.Format(applied), cashReceived, change, trimNote(in.ReferenceNo), key)
	if err != nil {
		return nil, err
	}

	newPaid := paid.Add(applied)
	status := "PARTIALLY_PAID"
	if newPaid.GreaterThanOrEqual(total) {
		status = "PAID"
	}
	if err := s.repo.UpdateOrderPayment(ctx, tx, actor.OutletID, orderID, shiftID, money.Format(newPaid), status); err != nil {
		return nil, err
	}

	if status == "PAID" {
		served, err := s.repo.AllItemsServed(ctx, tx, orderID)
		if err != nil {
			return nil, err
		}
		if served {
			if err := s.repo.CompleteOrder(ctx, tx, actor.OutletID, orderID); err != nil {
				return nil, err
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	payment, err := s.repo.Payment(ctx, actor.OutletID, paymentID)
	if err != nil {
		return nil, err
	}
	updated, _, err := s.repo.Order(ctx, actor.OutletID, orderID)
	if err != nil {
		return nil, err
	}
	return &CreateResult{Payment: *payment, Order: stateOf(updated)}, nil
}

func (s *Service) List(ctx context.Context, actor auth.Actor, orderID string) ([]Payment, error) {
	if actor.OutletID == "" {
		return nil, outletMissing()
	}
	if _, found, err := s.repo.Order(ctx, actor.OutletID, orderID); err != nil {
		return nil, err
	} else if !found {
		return nil, notFound()
	}
	return s.repo.Payments(ctx, actor.OutletID, orderID)
}

func stateOf(o *orderRow) OrderState {
	remaining := "0.00"
	total, err1 := money.Parse(o.Total)
	paid, err2 := money.Parse(o.Paid)
	if err1 == nil && err2 == nil {
		if diff := total.Sub(paid); diff.IsPositive() {
			remaining = money.Format(diff)
		}
	}
	return OrderState{
		ID: o.ID, OrderNo: o.OrderNo, Status: o.Status, PaymentStatus: o.PaymentStatus,
		TotalAmount: o.Total, PaidAmount: o.Paid, RefundedAmount: o.Refunded, Remaining: remaining,
	}
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
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

func notFound() error {
	return httpx.NewError(http.StatusNotFound, "ORDER_NOT_FOUND", "Pesanan tidak ditemukan.")
}

func outletMissing() error {
	return httpx.NewError(http.StatusUnprocessableEntity, "OUTLET_NOT_CONFIGURED",
		"Outlet belum dikonfigurasi.")
}
