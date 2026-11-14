package menu

import "time"

type Category struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	SortOrder int       `json:"sort_order"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
}

type CategoryInput struct {
	Name      string `json:"name"`
	SortOrder int    `json:"sort_order"`
	IsActive  *bool  `json:"is_active"`
}

type Product struct {
	ID              string    `json:"id"`
	CategoryID      string    `json:"category_id"`
	CategoryName    string    `json:"category_name,omitempty"`
	SKU             string    `json:"sku"`
	Name            string    `json:"name"`
	Description     *string   `json:"description,omitempty"`
	ProductType     string    `json:"product_type"`
	StockMode       string    `json:"stock_mode"`
	StockIngredient *string   `json:"stock_ingredient_id,omitempty"`
	KitchenStation  *string   `json:"kitchen_station,omitempty"`
	AllowZeroPrice  bool      `json:"allow_zero_price"`
	IsActive        bool      `json:"is_active"`
	IsAvailable     bool      `json:"is_available"`
	SortOrder       int       `json:"sort_order"`
	Price           string    `json:"price"`
	Variants        []Variant `json:"variants,omitempty"`
}

type Variant struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	SKU       string `json:"sku,omitempty"`
	Price     string `json:"price"`
	IsDefault bool   `json:"is_default"`
	IsActive  bool   `json:"is_active"`
	SortOrder int    `json:"sort_order"`
}

type ProductFilter struct {
	Query      string
	CategoryID string
	IsActive   *bool
	Available  *bool
	Page       int
	PerPage    int
}

type ProductInput struct {
	CategoryID      string         `json:"category_id"`
	SKU             string         `json:"sku"`
	Name            string         `json:"name"`
	Description     *string        `json:"description"`
	ProductType     string         `json:"product_type"`
	StockMode       string         `json:"stock_mode"`
	StockIngredient *string        `json:"stock_ingredient_id"`
	KitchenStation  *string        `json:"kitchen_station"`
	AllowZeroPrice  bool           `json:"allow_zero_price"`
	SortOrder       int            `json:"sort_order"`
	Variants        []VariantInput `json:"variants"`
}

type VariantInput struct {
	Name      string `json:"name"`
	SKU       string `json:"sku"`
	Price     string `json:"price"`
	IsDefault bool   `json:"is_default"`
	SortOrder int    `json:"sort_order"`
}
