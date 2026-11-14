// Package waste mengelola pencatatan bahan atau menu terbuang (F13).
// Setiap pencatatan menulis movement WASTE; pembatalan menulis WASTE_REVERSAL.
package waste

import "time"

type Record struct {
	ID             string    `json:"id"`
	Type           string    `json:"waste_type"`
	IngredientID   *string   `json:"ingredient_id,omitempty"`
	IngredientCode *string   `json:"ingredient_code,omitempty"`
	IngredientName *string   `json:"ingredient_name,omitempty"`
	VariantID      *string   `json:"variant_id,omitempty"`
	VariantName    *string   `json:"variant_name,omitempty"`
	Quantity       string    `json:"quantity"`
	UnitCode       *string   `json:"unit_code,omitempty"`
	CostValue      string    `json:"cost_value"`
	ReasonCode     *string   `json:"reason_code,omitempty"`
	Note           *string   `json:"note,omitempty"`
	OrderID        *string   `json:"order_id,omitempty"`
	Status         string    `json:"status"`
	RecordedAt     time.Time `json:"recorded_at"`
}

type Input struct {
	WasteType    string  `json:"waste_type"`
	IngredientID *string `json:"ingredient_id"`
	VariantID    *string `json:"variant_id"`
	Quantity     string  `json:"quantity"`
	UnitID       *int    `json:"unit_id"`
	ReasonCode   *string `json:"reason_code"`
	Note         *string `json:"note"`
	OrderID      *string `json:"order_id"`
}

type Filter struct {
	Type       string
	ReasonCode string
	From       string
	To         string
	Page       int
	PerPage    int
}
