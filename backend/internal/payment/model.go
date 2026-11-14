// Package payment mengelola pembayaran pesanan (F14).
// Semua nominal dihitung server; klien hanya mengirim jumlah bayar.
package payment

import "time"

type Payment struct {
	ID           string     `json:"id"`
	OrderID      string     `json:"order_id"`
	ShiftID      string     `json:"shift_id"`
	MethodCode   string     `json:"method_code"`
	Amount       string     `json:"amount"`
	CashReceived *string    `json:"cash_received,omitempty"`
	ChangeAmount *string    `json:"change_amount,omitempty"`
	Status       string     `json:"status"`
	ReferenceNo  *string    `json:"reference_no,omitempty"`
	PaidAt       *time.Time `json:"paid_at,omitempty"`
	VoidedBy     *string    `json:"voided_by,omitempty"`
	VoidedAt     *time.Time `json:"voided_at,omitempty"`
	VoidReason   *string    `json:"void_reason,omitempty"`
	ReceivedBy   string     `json:"received_by"`
	CreatedAt    time.Time  `json:"created_at"`
}

type CreateInput struct {
	MethodCode   string  `json:"method_code"`
	Amount       string  `json:"amount"`
	CashReceived *string `json:"cash_received"`
	ReferenceNo  *string `json:"reference_no"`
}

// OrderState adalah ringkasan status pembayaran pesanan setelah aksi.
type OrderState struct {
	ID             string `json:"id"`
	OrderNo        string `json:"order_no"`
	Status         string `json:"status"`
	PaymentStatus  string `json:"payment_status"`
	TotalAmount    string `json:"total_amount"`
	PaidAmount     string `json:"paid_amount"`
	RefundedAmount string `json:"refunded_amount"`
	Remaining      string `json:"remaining"`
}

type CreateResult struct {
	Payment Payment    `json:"payment"`
	Order   OrderState `json:"order"`
}

type VoidInput struct {
	Reason string `json:"reason"`
}

type RefundItemInput struct {
	OrderItemID string `json:"order_item_id"`
	Quantity    int    `json:"quantity"`
}

type RefundInput struct {
	Amount     string            `json:"amount"`
	PaymentID  *string           `json:"payment_id"`
	MethodCode *string           `json:"method_code"`
	ReasonCode *string           `json:"reason_code"`
	ReasonNote *string           `json:"reason_note"`
	Items      []RefundItemInput `json:"items"`
	Restock    bool              `json:"restock"`
}

type RefundResult struct {
	Refund Refund     `json:"refund"`
	Order  OrderState `json:"order"`
}

type Refund struct {
	ID           string       `json:"id"`
	OrderID      string       `json:"order_id"`
	PaymentID    *string      `json:"payment_id,omitempty"`
	ShiftID      string       `json:"shift_id"`
	Type         string       `json:"refund_type"`
	MethodCode   *string      `json:"method_code,omitempty"`
	Amount       string       `json:"amount"`
	ReasonCode   *string      `json:"reason_code,omitempty"`
	ReasonNote   *string      `json:"reason_note,omitempty"`
	Status       string       `json:"status"`
	SelfApproved bool         `json:"self_approved"`
	CreatedAt    time.Time    `json:"created_at"`
	CompletedAt  *time.Time   `json:"completed_at,omitempty"`
	Items        []RefundLine `json:"items"`
}

type RefundLine struct {
	OrderItemID string `json:"order_item_id"`
	Name        string `json:"name"`
	Quantity    int    `json:"quantity"`
	Amount      string `json:"amount"`
}
