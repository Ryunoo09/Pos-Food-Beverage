package settings

import "time"

type OutletSettings struct {
	TaxInclusive              bool   `json:"tax_inclusive"`
	TaxOnService              bool   `json:"tax_on_service"`
	RoundingMode              string `json:"rounding_mode"`
	DiscountCashierMaxPercent string `json:"discount_cashier_max_percent"`
	DiscountApprovalThreshold string `json:"discount_approval_threshold_amount"`
	RefundApprovalThreshold   string `json:"refund_approval_threshold"`
}

type OutletSettingsInput struct {
	TaxInclusive              *bool   `json:"tax_inclusive"`
	TaxOnService              *bool   `json:"tax_on_service"`
	RoundingMode              *string `json:"rounding_mode"`
	DiscountCashierMaxPercent *string `json:"discount_cashier_max_percent"`
	DiscountApprovalThreshold *string `json:"discount_approval_threshold_amount"`
	RefundApprovalThreshold   *string `json:"refund_approval_threshold"`
}

type TaxConfig struct {
	ID            string    `json:"id"`
	Type          string    `json:"type"`
	Name          string    `json:"name"`
	RatePercent   string    `json:"rate_percent"`
	AppliesTo     []string  `json:"applies_to"`
	IsActive      bool      `json:"is_active"`
	EffectiveFrom time.Time `json:"effective_from"`
}

type TaxConfigInput struct {
	Type          string   `json:"type"`
	Name          string   `json:"name"`
	RatePercent   string   `json:"rate_percent"`
	AppliesTo     []string `json:"applies_to"`
	EffectiveFrom *string  `json:"effective_from"`
}

// PaymentMethod adalah cara bayar yang tersedia di kasir. Kode dipakai
// transaksi pembayaran (payments.method_code), jadi kode tidak boleh diubah
// setelah dipakai; yang bisa diubah hanya nama, urutan, dan status aktif.
type PaymentMethod struct {
	ID                string `json:"id"`
	Code              string `json:"code"`
	Name              string `json:"name"`
	IsActive          bool   `json:"is_active"`
	RequiresReference bool   `json:"requires_reference"`
	IsCash            bool   `json:"is_cash"`
	Provider          string `json:"provider"`
	SortOrder         int    `json:"sort_order"`
}

type PaymentMethodInput struct {
	Code              string  `json:"code"`
	Name              string  `json:"name"`
	RequiresReference *bool   `json:"requires_reference"`
	Provider          *string `json:"provider"`
	SortOrder         *int    `json:"sort_order"`
}
