package discount

import "time"

type Discount struct {
	ID               string     `json:"id"`
	Code             string     `json:"code"`
	Name             string     `json:"name"`
	DiscountType     string     `json:"discount_type"`
	Value            string     `json:"value"`
	Scope            string     `json:"scope"`
	MinSpend         string     `json:"min_spend"`
	MaxDiscount      *string    `json:"max_discount,omitempty"`
	StartsAt         *time.Time `json:"starts_at,omitempty"`
	EndsAt           *time.Time `json:"ends_at,omitempty"`
	DaysOfWeek       []int      `json:"days_of_week,omitempty"`
	RequiresApproval bool       `json:"requires_approval"`
	IsActive         bool       `json:"is_active"`
}

type Input struct {
	Code             string  `json:"code"`
	Name             string  `json:"name"`
	DiscountType     string  `json:"discount_type"`
	Value            string  `json:"value"`
	Scope            string  `json:"scope"`
	MinSpend         string  `json:"min_spend"`
	MaxDiscount      *string `json:"max_discount"`
	StartsAt         *string `json:"starts_at"`
	EndsAt           *string `json:"ends_at"`
	DaysOfWeek       []int   `json:"days_of_week"`
	RequiresApproval bool    `json:"requires_approval"`
}
