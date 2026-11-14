// Package expense mengelola pengeluaran operasional dan kategorinya (F17).
package expense

import "time"

type Category struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
}

type CategoryInput struct {
	Name     string `json:"name"`
	IsActive *bool  `json:"is_active"`
}

type Expense struct {
	ID                string    `json:"id"`
	CategoryID        string    `json:"category_id"`
	CategoryName      string    `json:"category_name"`
	Name              string    `json:"name"`
	Amount            string    `json:"amount"`
	ExpenseDate       string    `json:"expense_date"`
	PaymentMethodCode *string   `json:"payment_method_code,omitempty"`
	PaidFromDrawer    bool      `json:"paid_from_drawer"`
	ShiftID           *string   `json:"shift_id,omitempty"`
	Note              *string   `json:"note,omitempty"`
	AttachmentURL     *string   `json:"attachment_url,omitempty"`
	IsVoided          bool      `json:"is_voided"`
	VoidReason        *string   `json:"void_reason,omitempty"`
	RecordedBy        *string   `json:"recorded_by,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
}

type CreateInput struct {
	CategoryID        string  `json:"category_id"`
	Name              string  `json:"name"`
	Amount            string  `json:"amount"`
	ExpenseDate       *string `json:"expense_date"`
	PaymentMethodCode *string `json:"payment_method_code"`
	PaidFromDrawer    bool    `json:"paid_from_drawer"`
	Note              *string `json:"note"`
	AttachmentURL     *string `json:"attachment_url"`
}

type UpdateInput struct {
	CategoryID        *string `json:"category_id"`
	Name              *string `json:"name"`
	Amount            *string `json:"amount"`
	ExpenseDate       *string `json:"expense_date"`
	PaymentMethodCode *string `json:"payment_method_code"`
	PaidFromDrawer    *bool   `json:"paid_from_drawer"`
	Note              *string `json:"note"`
	AttachmentURL     *string `json:"attachment_url"`
}

type VoidInput struct {
	Reason string `json:"reason"`
}

type Filter struct {
	From          string
	To            string
	CategoryID    string
	IncludeVoided bool
	Page          int
	PerPage       int
}
