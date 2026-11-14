// Package opname mengelola sesi stock opname: pembuatan, mulai, input hitung
// fisik, dan (tahap berikut) submit/approve.
package opname

import "time"

type Opname struct {
	ID           string     `json:"id"`
	Code         string     `json:"code"`
	Status       string     `json:"status"`
	Scope        string     `json:"scope"`
	Note         *string    `json:"note,omitempty"`
	RejectReason *string    `json:"rejected_reason,omitempty"`
	StartedAt    *time.Time `json:"started_at,omitempty"`
	SubmittedAt  *time.Time `json:"submitted_at,omitempty"`
	ApprovedAt   *time.Time `json:"approved_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	Items        []Item     `json:"items"`
	TotalItems   int        `json:"total_items"`
	CountedItems int        `json:"counted_items"`
}

type Item struct {
	ID              string     `json:"id"`
	IngredientID    string     `json:"ingredient_id"`
	IngredientCode  string     `json:"ingredient_code"`
	IngredientName  string     `json:"ingredient_name"`
	BaseUnitCode    string     `json:"base_unit_code"`
	SystemQuantity  string     `json:"system_quantity"`
	CountedQuantity *string    `json:"counted_quantity,omitempty"`
	Difference      *string    `json:"difference_quantity,omitempty"`
	DifferenceValue *string    `json:"difference_value,omitempty"`
	CountedAt       *time.Time `json:"counted_at,omitempty"`
	Reason          *string    `json:"reason,omitempty"`
}

type CreateInput struct {
	Scope         string   `json:"scope"`
	IngredientIDs []string `json:"ingredient_ids"`
	Note          *string  `json:"note"`
}

type CountInput struct {
	Items []CountItem `json:"items"`
}

type CountItem struct {
	IngredientID    string  `json:"ingredient_id"`
	CountedQuantity string  `json:"counted_quantity"`
	Reason          *string `json:"reason"`
}

type RejectInput struct {
	Reason string `json:"reason"`
}
