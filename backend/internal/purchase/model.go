// Package purchase mengelola purchase order dan penerimaan barang (F11).
// PO tidak mengubah stok; hanya penerimaan yang mengubah saldo (BR-INV-4).
package purchase

import "time"

type PurchaseOrder struct {
	ID             string      `json:"id"`
	Code           string      `json:"code"`
	SupplierID     string      `json:"supplier_id"`
	SupplierName   string      `json:"supplier_name"`
	Status         string      `json:"status"`
	OrderDate      string      `json:"order_date"`
	ExpectedDate   *string     `json:"expected_date,omitempty"`
	Subtotal       string      `json:"subtotal"`
	DiscountAmount string      `json:"discount_amount"`
	TotalAmount    string      `json:"total_amount"`
	Note           *string     `json:"note,omitempty"`
	Version        int         `json:"version"`
	CreatedAt      time.Time   `json:"created_at"`
	// SubmittedBy dipakai UI untuk menyembunyikan tombol Setujui dari pengaju;
	// backend tetap menolak self-approval bila setelan melarangnya.
	SubmittedBy *string     `json:"submitted_by,omitempty"`
	Items       []OrderItem `json:"items"`
}

type OrderItem struct {
	ID                   string `json:"id"`
	IngredientID         string `json:"ingredient_id"`
	IngredientCode       string `json:"ingredient_code"`
	IngredientName       string `json:"ingredient_name"`
	PurchaseUnitID       int    `json:"purchase_unit_id"`
	PurchaseUnitCode     string `json:"purchase_unit_code"`
	QuantityOrdered      string `json:"quantity_ordered"`
	UnitPrice            string `json:"unit_price"`
	DiscountAmount       string `json:"discount_amount"`
	LineTotal            string `json:"line_total"`
	BaseQuantityOrdered  string `json:"base_quantity_ordered"`
	QuantityReceivedBase string `json:"quantity_received_base"`
}

type CreateInput struct {
	SupplierID     string           `json:"supplier_id"`
	ExpectedDate   *string          `json:"expected_date"`
	Note           *string          `json:"note"`
	DiscountAmount string           `json:"discount_amount"`
	Items          []OrderItemInput `json:"items"`
}

type OrderItemInput struct {
	IngredientID    string `json:"ingredient_id"`
	PurchaseUnitID  int    `json:"purchase_unit_id"`
	QuantityOrdered string `json:"quantity_ordered"`
	UnitPrice       string `json:"unit_price"`
	DiscountAmount  string `json:"discount_amount"`
}

type UpdateInput struct {
	ExpectedDate   *string           `json:"expected_date"`
	Note           *string           `json:"note"`
	DiscountAmount *string           `json:"discount_amount"`
	Items          *[]OrderItemInput `json:"items"`
}

type CancelInput struct {
	Reason string `json:"reason"`
}

type Receiving struct {
	ID                string          `json:"id"`
	Code              string          `json:"code"`
	PurchaseOrderID   *string         `json:"purchase_order_id,omitempty"`
	PurchaseOrderCode *string         `json:"purchase_order_code,omitempty"`
	SupplierID        string          `json:"supplier_id"`
	SupplierName      string          `json:"supplier_name"`
	SupplierInvoiceNo *string         `json:"supplier_invoice_no,omitempty"`
	Note              *string         `json:"note,omitempty"`
	ReceivedAt        time.Time       `json:"received_at"`
	Items             []ReceivingItem `json:"items"`
}

type ReceivingItem struct {
	ID               string `json:"id"`
	IngredientID     string `json:"ingredient_id"`
	IngredientCode   string `json:"ingredient_code"`
	IngredientName   string `json:"ingredient_name"`
	QuantityReceived string `json:"quantity_received"`
	BaseQuantity     string `json:"base_quantity"`
	UnitPrice        string `json:"unit_price"`
	NetUnitCostBase  string `json:"net_unit_cost_base"`
	LineTotal        string `json:"line_total"`
}

type ReceivingInput struct {
	SupplierID        string               `json:"supplier_id"`
	SupplierInvoiceNo *string              `json:"supplier_invoice_no"`
	Note              *string              `json:"note"`
	Items             []ReceivingItemInput `json:"items"`
}

type ReceivingItemInput struct {
	POItemID          *string `json:"po_item_id"`
	IngredientID      string  `json:"ingredient_id"`
	PurchaseUnitID    int     `json:"purchase_unit_id"`
	QuantityReceived  string  `json:"quantity_received"`
	UnitPrice         string  `json:"unit_price"`
	DiscountAllocated string  `json:"discount_allocated"`
}

type Filter struct {
	Status     string
	SupplierID string
	Page       int
	PerPage    int
}
