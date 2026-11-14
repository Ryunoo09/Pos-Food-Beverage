// Package shift mengelola buka/tutup shift kasir dan rekonsiliasi kas (F16).
package shift

import "time"

type Shift struct {
	ID               string     `json:"id"`
	UserID           string     `json:"user_id"`
	Status           string     `json:"status"`
	OpenedAt         time.Time  `json:"opened_at"`
	ClosedAt         *time.Time `json:"closed_at,omitempty"`
	OpeningCash      string     `json:"opening_cash"`
	ExpectedCash     *string    `json:"expected_cash,omitempty"`
	ActualCash       *string    `json:"actual_cash,omitempty"`
	Difference       *string    `json:"difference,omitempty"`
	DifferenceReason *string    `json:"difference_reason,omitempty"`
	Version          int        `json:"version"`
}

type OpenInput struct {
	OpeningCash string `json:"opening_cash"`
}

type CloseInput struct {
	ActualCash       string  `json:"actual_cash"`
	DifferenceReason *string `json:"difference_reason"`
}

type MethodTotal struct {
	MethodCode string `json:"method_code"`
	Amount     string `json:"amount"`
	Count      int    `json:"count"`
}

// Summary adalah laporan kas shift (FR-F16-4/5).
type Summary struct {
	Shift               Shift         `json:"shift"`
	CashSales           string        `json:"cash_sales"`
	CashRefunds         string        `json:"cash_refunds"`
	CashIn              string        `json:"cash_in"`
	CashOut             string        `json:"cash_out"`
	ExpectedCash        string        `json:"expected_cash"`
	ByMethod            []MethodTotal `json:"by_method"`
	VoidedPaymentCount  int           `json:"voided_payment_count"`
	VoidedPaymentAmount string        `json:"voided_payment_amount"`
	PendingPaymentCount int           `json:"pending_payment_count"`
	Difference          string        `json:"difference"`
}

type Filter struct {
	Status  string
	Page    int
	PerPage int
}
