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
