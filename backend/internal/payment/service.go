package payment

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
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

// Void membatalkan pembayaran salah input selama shift masih terbuka.
func (s *Service) Void(ctx context.Context, actor auth.Actor, paymentID, key string, in VoidInput) (*CreateResult, error) {
	if actor.OutletID == "" {
		return nil, outletMissing()
	}
	if strings.TrimSpace(key) == "" {
		return nil, httpx.NewError(http.StatusBadRequest, "IDEMPOTENCY_KEY_REQUIRED",
			"Header Idempotency-Key wajib diisi.")
	}
	scope := actor.OutletID + ":" + actor.UserID + ":POST /payments/:id/void"
	hash := sha256.Sum256([]byte(paymentID + "|" + in.Reason))

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
		var result CreateResult
		if err := json.Unmarshal(body, &result); err != nil {
			return nil, err
		}
		return &result, nil
	}

	result, err := s.void(ctx, actor, paymentID, in)
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

func (s *Service) void(ctx context.Context, actor auth.Actor, paymentID string, in VoidInput) (*CreateResult, error) {
	reason := strings.TrimSpace(in.Reason)
	if reason == "" {
		return nil, validation("reason wajib diisi.")
	}

	// Void hanya boleh pada shift yang masih terbuka (FR-F14-4).
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

	pay, found, err := s.repo.LockPayment(ctx, tx, actor.OutletID, paymentID)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, httpx.NewError(http.StatusNotFound, "PAYMENT_NOT_FOUND", "Pembayaran tidak ditemukan.")
	}
	if pay.Status != "SUCCESS" {
		return nil, httpx.NewError(http.StatusUnprocessableEntity, "PAYMENT_NOT_VOIDABLE",
			"Hanya pembayaran berhasil yang dapat di-void.")
	}
	if pay.ShiftID != shiftID {
		return nil, httpx.NewError(http.StatusUnprocessableEntity, "SHIFT_MISMATCH",
			"Void hanya dapat dilakukan pada shift pembayaran tersebut.")
	}

	order, found, err := s.repo.LockOrder(ctx, tx, actor.OutletID, pay.OrderID)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, notFound()
	}

	if err := s.repo.VoidPayment(ctx, tx, actor.OutletID, paymentID, actor.UserID, reason); err != nil {
		return nil, err
	}

	// payment_status selalu diturunkan dari payments SUCCESS (BR-F14-1).
	// ponytail: refund belum dikurangkan; tambahkan saat commit refund.
	paidText, err := s.repo.SumSuccessfulPayments(ctx, tx, pay.OrderID)
	if err != nil {
		return nil, err
	}
	total, err := money.Parse(order.Total)
	if err != nil {
		return nil, err
	}
	paid, err := money.Parse(paidText)
	if err != nil {
		return nil, err
	}
	status := "UNPAID"
	if paid.IsPositive() {
		status = "PARTIALLY_PAID"
		if paid.GreaterThanOrEqual(total) {
			status = "PAID"
		}
	}
	if err := s.repo.SetOrderPaid(ctx, tx, actor.OutletID, pay.OrderID, money.Format(paid), status); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	payment, err := s.repo.Payment(ctx, actor.OutletID, paymentID)
	if err != nil {
		return nil, err
	}
	// ponytail: status pesanan (mis. COMPLETED) tidak dikembalikan; sesuaikan bila void pasca-selesai perlu dibuka lagi.
	updated, _, err := s.repo.Order(ctx, actor.OutletID, pay.OrderID)
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

// CreateRefund mengajukan refund. Refund di atas ambang atau oleh kasir menunggu persetujuan.
// Status HTTP 201 (langsung selesai) atau 202 (menunggu persetujuan).
func (s *Service) CreateRefund(ctx context.Context, actor auth.Actor, orderID, key string, in RefundInput) (*RefundResult, int, error) {
	if actor.OutletID == "" {
		return nil, 0, outletMissing()
	}
	if strings.TrimSpace(key) == "" {
		return nil, 0, httpx.NewError(http.StatusBadRequest, "IDEMPOTENCY_KEY_REQUIRED",
			"Header Idempotency-Key wajib diisi.")
	}
	scope := actor.OutletID + ":" + actor.UserID + ":POST /orders/:id/refunds"
	hash := sha256.Sum256([]byte(refundFingerprint(orderID, in)))

	claimed, err := s.repo.ClaimIdempotency(ctx, scope, key, hash[:])
	if err != nil {
		return nil, 0, err
	}
	if !claimed {
		stored, status, body, found, err := s.repo.IdempotencyEntry(ctx, scope, key)
		if err != nil {
			return nil, 0, err
		}
		if !found || !bytes.Equal(stored, hash[:]) {
			return nil, 0, httpx.NewError(http.StatusConflict, "IDEMPOTENCY_CONFLICT",
				"Kunci idempotensi sudah dipakai untuk permintaan berbeda.")
		}
		if status == 0 {
			return nil, 0, httpx.NewError(http.StatusConflict, "IDEMPOTENCY_IN_PROGRESS",
				"Permintaan dengan kunci ini sedang diproses.")
		}
		var result RefundResult
		if err := json.Unmarshal(body, &result); err != nil {
			return nil, 0, err
		}
		return &result, status, nil
	}

	result, status, err := s.createRefund(ctx, actor, orderID, key, in)
	if err != nil {
		_ = s.repo.ReleaseIdempotency(ctx, scope, key)
		return nil, 0, err
	}
	body, err := json.Marshal(result)
	if err != nil {
		return nil, 0, err
	}
	if err := s.repo.SaveIdempotencyResponse(ctx, scope, key, status, body); err != nil {
		return nil, 0, err
	}
	return result, status, nil
}

func (s *Service) createRefund(ctx context.Context, actor auth.Actor, orderID, key string, in RefundInput) (*RefundResult, int, error) {
	amount, err := money.Parse(strings.TrimSpace(in.Amount))
	if err != nil || !amount.IsPositive() {
		return nil, 0, validation("amount refund harus lebih dari 0.")
	}

	shiftID, ok, err := s.repo.OpenShift(ctx, actor.UserID)
	if err != nil {
		return nil, 0, err
	}
	if !ok {
		return nil, 0, httpx.NewError(http.StatusUnprocessableEntity, "SHIFT_NOT_OPEN",
			"Kasir belum membuka shift.")
	}

	threshold, err := s.repo.RefundSettings(ctx, actor.OutletID)
	if err != nil {
		return nil, 0, err
	}
	limit, err := money.Parse(threshold)
	if err != nil {
		return nil, 0, err
	}

	tx, err := s.repo.Begin(ctx)
	if err != nil {
		return nil, 0, err
	}
	defer tx.Rollback(ctx)

	order, found, err := s.repo.LockOrder(ctx, tx, actor.OutletID, orderID)
	if err != nil {
		return nil, 0, err
	}
	if !found {
		return nil, 0, notFound()
	}
	if order.Status == "CANCELLED" || order.Status == "DRAFT" {
		return nil, 0, httpx.NewError(http.StatusUnprocessableEntity, "REFUND_NOT_ALLOWED",
			"Pesanan belum dikonfirmasi atau sudah dibatalkan.")
	}

	paid, err := money.Parse(order.Paid)
	if err != nil {
		return nil, 0, err
	}
	refunded, err := money.Parse(order.Refunded)
	if err != nil {
		return nil, 0, err
	}
	refundable := paid.Sub(refunded)
	if !refundable.IsPositive() {
		return nil, 0, httpx.NewError(http.StatusConflict, "NOTHING_TO_REFUND",
			"Tidak ada pembayaran yang dapat direfund.")
	}
	if amount.GreaterThan(refundable) {
		return nil, 0, httpx.NewError(http.StatusUnprocessableEntity, "REFUND_EXCEEDS_PAID",
			"Jumlah refund melebihi sisa yang dapat direfund.")
	}

	// Validasi item refund: milik pesanan ini dan belum melebihi kuantitas terjual.
	var restock bool
	sum, _ := money.Parse("0")
	for _, it := range in.Items {
		if it.Quantity <= 0 {
			return nil, 0, validation("quantity item refund harus lebih dari 0.")
		}
		ordered, found, err := s.repo.OrderItemQuantity(ctx, tx, orderID, it.OrderItemID)
		if err != nil {
			return nil, 0, err
		}
		if !found {
			return nil, 0, httpx.NewError(http.StatusUnprocessableEntity, "REFUND_ITEM_INVALID",
				"Item refund bukan bagian dari pesanan ini.")
		}
		done, err := s.repo.RefundedItemQuantity(ctx, tx, it.OrderItemID)
		if err != nil {
			return nil, 0, err
		}
		if it.Quantity > ordered-done {
			return nil, 0, httpx.NewError(http.StatusUnprocessableEntity, "REFUND_ITEM_EXCEEDS_QTY",
				"Kuantitas item refund melebihi sisa item yang dapat direfund.")
		}
		itemAmount, err := money.Parse(strings.TrimSpace(it.Amount))
		if err != nil || !itemAmount.IsPositive() {
			return nil, 0, validation("amount tiap item refund harus lebih dari 0.")
		}
		sum = sum.Add(itemAmount)
		restock = restock || it.Restock
	}
	if len(in.Items) > 0 {
		if sum.GreaterThan(amount) {
			return nil, 0, httpx.NewError(http.StatusUnprocessableEntity, "REFUND_ITEM_MISMATCH",
				"Total amount item melebihi amount refund.")
		}
	}

	cashier := false
	for _, r := range actor.Roles {
		if r == "CASHIER" {
			cashier = true
			break
		}
	}
	needsApproval := cashier || amount.GreaterThan(limit)

	refundType := "PARTIAL"
	if !amount.LessThan(refundable) {
		refundType = "FULL"
	}

	status := "COMPLETED"
	var approvedBy *string
	selfApproved := !needsApproval
	httpStatus := http.StatusCreated
	if needsApproval {
		status = "PENDING_APPROVAL"
		httpStatus = http.StatusAccepted
	} else {
		approvedBy = &actor.UserID
	}

	refundID, err := s.repo.InsertRefund(ctx, tx, refundInsert{
		OutletID:       actor.OutletID,
		OrderID:        orderID,
		PaymentID:      in.PaymentID,
		ShiftID:        shiftID,
		RefundType:     refundType,
		MethodCode:     trimNote(in.MethodCode),
		Amount:         money.Format(amount),
		ReasonCode:     trimNote(in.ReasonCode),
		ReasonNote:     trimNote(in.ReasonNote),
		Status:         status,
		RequestedBy:    actor.UserID,
		ApprovedBy:     approvedBy,
		SelfApproved:   selfApproved,
		IdempotencyKey: key,
		Restock:        restock,
		Completed:      !needsApproval,
	})
	if err != nil {
		return nil, 0, err
	}
	for _, it := range in.Items {
		itemAmount, err := money.Parse(strings.TrimSpace(it.Amount))
		if err != nil {
			return nil, 0, err
		}
		if err := s.repo.InsertRefundItem(ctx, tx, refundID, it.OrderItemID, it.Quantity,
			money.Format(itemAmount), it.Restock); err != nil {
			return nil, 0, err
		}
	}

	if !needsApproval {
		newRefunded := refunded.Add(amount)
		payStatus := "PARTIALLY_REFUNDED"
		if newRefunded.GreaterThanOrEqual(paid) {
			payStatus = "REFUNDED"
		}
		if err := s.repo.ApplyRefund(ctx, tx, actor.OutletID, orderID, money.Format(newRefunded), payStatus); err != nil {
			return nil, 0, err
		}
	}
	// ponytail: restock bahan dari refund belum menggerakkan stok; tambahkan bila kebijakan restock ditetapkan.

	if err := tx.Commit(ctx); err != nil {
		return nil, 0, err
	}

	refund, err := s.repo.RefundByID(ctx, actor.OutletID, refundID)
	if err != nil {
		return nil, 0, err
	}
	updated, _, err := s.repo.Order(ctx, actor.OutletID, orderID)
	if err != nil {
		return nil, 0, err
	}
	return &RefundResult{Refund: *refund, Order: stateOf(updated)}, httpStatus, nil
}

// ApproveRefund menyetujui refund menunggu; pengaju tidak boleh menyetujui sendiri.
func (s *Service) ApproveRefund(ctx context.Context, actor auth.Actor, refundID string, in RefundDecisionInput) (*RefundResult, error) {
	if actor.OutletID == "" {
		return nil, outletMissing()
	}
	tx, err := s.repo.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	refund, found, err := s.repo.LockRefund(ctx, tx, actor.OutletID, refundID)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, refundNotFound()
	}
	if refund.Status != "PENDING_APPROVAL" {
		return nil, httpx.NewError(http.StatusUnprocessableEntity, "REFUND_NOT_PENDING",
			"Refund ini tidak menunggu persetujuan.")
	}
	if refund.RequestedBy == actor.UserID {
		return nil, httpx.NewError(http.StatusUnprocessableEntity, "REFUND_SELF_APPROVAL",
			"Pengaju refund tidak boleh menyetujui refundnya sendiri.")
	}

	order, found, err := s.repo.LockOrder(ctx, tx, actor.OutletID, refund.OrderID)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, notFound()
	}
	amount, err := money.Parse(refund.Amount)
	if err != nil {
		return nil, err
	}
	paid, err := money.Parse(order.Paid)
	if err != nil {
		return nil, err
	}
	refunded, err := money.Parse(order.Refunded)
	if err != nil {
		return nil, err
	}
	newRefunded := refunded.Add(amount)
	if newRefunded.GreaterThan(paid) {
		return nil, httpx.NewError(http.StatusUnprocessableEntity, "REFUND_EXCEEDS_PAID",
			"Jumlah refund melebihi sisa yang dapat direfund.")
	}

	if err := s.repo.SetRefundStatus(ctx, tx, actor.OutletID, refundID, "COMPLETED", actor.UserID,
		trimNote(in.Note)); err != nil {
		return nil, err
	}
	payStatus := "PARTIALLY_REFUNDED"
	if newRefunded.GreaterThanOrEqual(paid) {
		payStatus = "REFUNDED"
	}
	if err := s.repo.ApplyRefund(ctx, tx, actor.OutletID, refund.OrderID, money.Format(newRefunded), payStatus); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.refundResult(ctx, actor.OutletID, refundID, refund.OrderID)
}

// RejectRefund menolak refund menunggu tanpa mengubah saldo pesanan.
func (s *Service) RejectRefund(ctx context.Context, actor auth.Actor, refundID string, in RefundDecisionInput) (*RefundResult, error) {
	if actor.OutletID == "" {
		return nil, outletMissing()
	}
	tx, err := s.repo.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	refund, found, err := s.repo.LockRefund(ctx, tx, actor.OutletID, refundID)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, refundNotFound()
	}
	if refund.Status != "PENDING_APPROVAL" {
		return nil, httpx.NewError(http.StatusUnprocessableEntity, "REFUND_NOT_PENDING",
			"Refund ini tidak menunggu persetujuan.")
	}
	if refund.RequestedBy == actor.UserID {
		return nil, httpx.NewError(http.StatusUnprocessableEntity, "REFUND_SELF_APPROVAL",
			"Pengaju refund tidak boleh menolak refundnya sendiri.")
	}
	if err := s.repo.SetRefundStatus(ctx, tx, actor.OutletID, refundID, "REJECTED", actor.UserID,
		trimNote(in.Note)); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.refundResult(ctx, actor.OutletID, refundID, refund.OrderID)
}

func (s *Service) ListRefunds(ctx context.Context, actor auth.Actor, f RefundFilter) ([]Refund, httpx.PageMeta, error) {
	if actor.OutletID == "" {
		return nil, httpx.PageMeta{}, outletMissing()
	}
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PerPage < 1 || f.PerPage > 100 {
		f.PerPage = 20
	}
	items, total, err := s.repo.Refunds(ctx, actor.OutletID, f)
	if err != nil {
		return nil, httpx.PageMeta{}, err
	}
	return items, httpx.PageMeta{Page: f.Page, PerPage: f.PerPage, Total: total}, nil
}

func (s *Service) refundResult(ctx context.Context, outletID, refundID, orderID string) (*RefundResult, error) {
	refund, err := s.repo.RefundByID(ctx, outletID, refundID)
	if err != nil {
		return nil, err
	}
	order, _, err := s.repo.Order(ctx, outletID, orderID)
	if err != nil {
		return nil, err
	}
	return &RefundResult{Refund: *refund, Order: stateOf(order)}, nil
}

func refundFingerprint(orderID string, in RefundInput) string {
	parts := []string{orderID, in.Amount, deref(in.MethodCode), deref(in.ReasonCode), deref(in.ReasonNote)}
	for _, it := range in.Items {
		parts = append(parts, fmt.Sprintf("%s:%d:%s:%t", it.OrderItemID, it.Quantity, it.Amount, it.Restock))
	}
	return strings.Join(parts, "|")
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

func refundNotFound() error {
	return httpx.NewError(http.StatusNotFound, "REFUND_NOT_FOUND", "Refund tidak ditemukan.")
}

func outletMissing() error {
	return httpx.NewError(http.StatusUnprocessableEntity, "OUTLET_NOT_CONFIGURED",
		"Outlet belum dikonfigurasi.")
}
