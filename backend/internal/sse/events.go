// Package sse — payload event SSE per event type.
//
// KITCHEN_STAFF tidak menerima field harga (unit_price, line_total, cost,
// payment_status, customer_phone). Payload di sini memang sudah tanpa harga
// karena DTO kitchen hanya snapshot nama/kuantitas/modifier.
package sse

import "time"

// KitchenOrderCreatedPayload dikirim saat pesanan dikonfirmasi (confirm) dan
// tiket dapur dibuat.
type KitchenOrderCreatedPayload struct {
	OrderID     string         `json:"order_id"`
	OrderNo     *string        `json:"order_no,omitempty"`
	QueueNo     *int           `json:"queue_no,omitempty"`
	ServiceType string         `json:"service_type"`
	TableName   *string        `json:"table_name,omitempty"`
	RoundNo     int            `json:"round_no"`
	Priority    string         `json:"priority"`
	Items       []KitchenItem  `json:"items"`
	CreatedAt   time.Time      `json:"created_at"`
}

// KitchenItem adalah snapshot item dapur tanpa field harga.
type KitchenItem struct {
	Name          string `json:"name"`
	Variant       string `json:"variant,omitempty"`
	ModifiersText string `json:"modifiers_text,omitempty"`
	Note          string `json:"note,omitempty"`
	Quantity      int    `json:"quantity"`
	Station       string `json:"station,omitempty"`
}

// ItemStatusChangedPayload dikirim saat status item dapur berubah.
type ItemStatusChangedPayload struct {
	OrderID   string `json:"order_id"`
	ItemID    string `json:"item_id"`
	OldStatus string `json:"old_status"`
	NewStatus string `json:"new_status"`
	UpdatedBy string `json:"updated_by"`
	Timestamp time.Time `json:"timestamp"`
}

// ItemVoidedPayload dikirim saat item dapur di-void.
type ItemVoidedPayload struct {
	OrderID   string    `json:"order_id"`
	ItemID    string    `json:"item_id"`
	Reason    string    `json:"reason"`
	VoidedBy  string    `json:"voided_by"`
	Timestamp time.Time `json:"timestamp"`
}

// OrderStatusChangedPayload dikirim tiap perubahan status order.
type OrderStatusChangedPayload struct {
	OrderID   string    `json:"order_id"`
	OldStatus string    `json:"old_status"`
	NewStatus string    `json:"new_status"`
	Timestamp time.Time `json:"timestamp"`
}

// OrderPaymentStatusChangedPayload dikirim saat pembayaran sukses / refund.
type OrderPaymentStatusChangedPayload struct {
	OrderID        string `json:"order_id"`
	PaymentStatus  string `json:"payment_status"`
	OrderStatus    string `json:"order_status,omitempty"`
	Timestamp      time.Time `json:"timestamp"`
}

// TableStatusChangedPayload dikirim saat status meja berubah.
type TableStatusChangedPayload struct {
	TableID    string `json:"table_id"`
	TableNo    string `json:"table_no"`
	OldStatus  string `json:"old_status"`
	NewStatus  string `json:"new_status"`
	Timestamp  time.Time `json:"timestamp"`
}

// Event type constant.
const (
	EventTypeKitchenOrderCreated     = "kitchen.order.created"
	EventTypeKitchenItemStatusChanged = "kitchen.item.status_changed"
	EventTypeKitchenItemVoided     = "kitchen.item.voided"
	EventTypeOrderStatusChanged    = "order.status_changed"
	EventTypeOrderPaymentStatusChanged = "order.payment_status_changed"
	EventTypeTableStatusChanged    = "table.status_changed"
)
