package inventory

import "time"

type Unit struct {
	ID           int16  `json:"id"`
	Code         string `json:"code"`
	Name         string `json:"name"`
	Dimension    string `json:"dimension"`
	ToBaseFactor string `json:"to_base_factor"`
	IsBase       bool   `json:"is_base"`
}

type IngredientCategory struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	IsActive bool   `json:"is_active"`
}

type IngredientCategoryInput struct {
	Name     string `json:"name"`
	IsActive *bool  `json:"is_active"`
}

type Ingredient struct {
	ID                  string  `json:"id"`
	Code                string  `json:"code"`
	Name                string  `json:"name"`
	CategoryID          *string `json:"category_id,omitempty"`
	CategoryName        *string `json:"category_name,omitempty"`
	BaseUnitID          int16   `json:"base_unit_id"`
	BaseUnitCode        string  `json:"base_unit_code"`
	DefaultPurchaseUnit *int16  `json:"default_purchase_unit_id,omitempty"`
	MinStock            string  `json:"min_stock"`
	IsActive            bool    `json:"is_active"`
	IsStockProduct      bool    `json:"is_stock_product"`
	QuantityOnHand      string  `json:"quantity_on_hand"`
	AvgUnitCost         string  `json:"avg_unit_cost"`
}

type IngredientInput struct {
	Code                string  `json:"code"`
	Name                string  `json:"name"`
	CategoryID          *string `json:"category_id"`
	BaseUnitID          int16   `json:"base_unit_id"`
	DefaultPurchaseUnit *int16  `json:"default_purchase_unit_id"`
	MinStock            string  `json:"min_stock"`
	IsStockProduct      bool    `json:"is_stock_product"`
}

type IngredientFilter struct {
	Query   string
	Active  *bool
	Page    int
	PerPage int
}

type StockMovement struct {
	ID            string    `json:"id"`
	MovementType  string    `json:"movement_type"`
	QuantityDelta string    `json:"quantity_delta"`
	UnitCost      string    `json:"unit_cost"`
	TotalCost     string    `json:"total_cost"`
	BalanceAfter  string    `json:"balance_after"`
	AvgCostAfter  string    `json:"avg_cost_after"`
	ReferenceType string    `json:"reference_type"`
	ReferenceID   *string   `json:"reference_id,omitempty"`
	IsShortage    bool      `json:"is_shortage"`
	Reason        *string   `json:"reason,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}
