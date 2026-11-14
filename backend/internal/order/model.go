package order

import "time"

type Order struct {
	ID             string      `json:"id"`
	OrderNo        *string     `json:"order_no,omitempty"`
	QueueNo        *int        `json:"queue_no,omitempty"`
	ServiceType    string      `json:"service_type"`
	Status         string      `json:"status"`
	PaymentStatus  string      `json:"payment_status"`
	PaymentTiming  string      `json:"payment_timing"`
	TableID        *string     `json:"table_id,omitempty"`
	CustomerName   *string     `json:"customer_name,omitempty"`
	CustomerPhone  *string     `json:"customer_phone,omitempty"`
	DelivAddress   *string     `json:"delivery_address,omitempty"`
	Note           *string     `json:"note,omitempty"`
	Priority       string      `json:"priority"`
	SubtotalAmount string      `json:"subtotal_amount"`
	ItemDiscount   string      `json:"item_discount_amount"`
	OrderDiscount  string      `json:"order_discount_amount"`
	ServiceRate    string      `json:"service_charge_rate"`
	ServiceAmount  string      `json:"service_charge_amount"`
	TaxRate        string      `json:"tax_rate"`
	TaxAmount      string      `json:"tax_amount"`
	TaxInclusive   bool        `json:"tax_inclusive"`
	RoundingAmount string      `json:"rounding_amount"`
	TotalAmount    string      `json:"total_amount"`
	PaidAmount     string      `json:"paid_amount"`
	TotalCogs      string      `json:"total_cogs"`
	CreatedAt      time.Time   `json:"created_at"`
	ConfirmedAt    *time.Time  `json:"confirmed_at,omitempty"`
	Items          []OrderItem `json:"items,omitempty"`

	// pricing diambil dari pricing_snapshot agar recalc memakai setelan saat draft dibuat.
	pricing pricingSnapshot
}

// pricingSnapshot menyimpan setelan penagihan yang tidak punya kolom sendiri di
// orders (tax_on_service, rounding_mode) supaya kalkulasi ulang tetap konsisten.
type pricingSnapshot struct {
	PaymentTiming string `json:"payment_timing"`
	TaxOnService  bool   `json:"tax_on_service"`
	RoundingMode  string `json:"rounding_mode"`
}

type OrderItem struct {
	ID             string         `json:"id"`
	RoundNo        int            `json:"round_no"`
	ProductID      string         `json:"product_id"`
	VariantID      string         `json:"variant_id"`
	ProductName    string         `json:"product_name"`
	VariantName    string         `json:"variant_name"`
	SKU            *string        `json:"sku,omitempty"`
	Quantity       int            `json:"quantity"`
	UnitPrice      string         `json:"unit_price"`
	ModifiersTotal string         `json:"modifiers_total"`
	DiscountAmount string         `json:"discount_amount"`
	LineTotal      string         `json:"line_total"`
	UnitCogs       string         `json:"unit_cogs"`
	LineCogs       string         `json:"line_cogs"`
	CostUnknown    bool           `json:"cost_unknown"`
	Status         string         `json:"status"`
	Note           *string        `json:"note,omitempty"`
	Modifiers      []ItemModifier `json:"modifiers,omitempty"`
}

type ItemModifier struct {
	ID         string  `json:"id"`
	OptionID   *string `json:"option_id,omitempty"`
	GroupName  *string `json:"group_name,omitempty"`
	OptionName string  `json:"option_name"`
	PriceDelta string  `json:"price_delta"`
	Quantity   int     `json:"quantity"`
}

type DraftInput struct {
	ServiceType     string  `json:"service_type"`
	TableID         *string `json:"table_id"`
	CustomerName    *string `json:"customer_name"`
	CustomerPhone   *string `json:"customer_phone"`
	DeliveryAddress *string `json:"delivery_address"`
	Note            *string `json:"note"`
	Priority        string  `json:"priority"`
}

type ItemInput struct {
	ProductID         string   `json:"product_id"`
	VariantID         string   `json:"variant_id"`
	Quantity          int      `json:"quantity"`
	ModifierOptionIDs []string `json:"modifier_option_ids"`
	Note              *string  `json:"note"`
}

type UpdateItemInput struct {
	Quantity *int    `json:"quantity"`
	Note     *string `json:"note"`
}

type CancelInput struct {
	ReasonCode string  `json:"reason_code"`
	Note       *string `json:"note"`
}

type Filter struct {
	Status  *string
	Query   string
	Page    int
	PerPage int
}

// saleVariant adalah harga jual yang sudah divalidasi milik bisnis.
type saleVariant struct {
	ProductID   string
	VariantID   string
	ProductName string
	VariantName string
	SKU         *string
	UnitPrice   string
}

type pricingSettings struct {
	PaymentTiming string
	ServiceRate   string
	TaxRate       string
	TaxInclusive  bool
	TaxOnService  bool
	RoundingMode  string
}
