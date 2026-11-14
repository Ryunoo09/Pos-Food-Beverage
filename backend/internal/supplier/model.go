package supplier

type Supplier struct {
	ID            string               `json:"id"`
	Name          string               `json:"name"`
	ContactPerson *string              `json:"contact_person,omitempty"`
	Phone         *string              `json:"phone,omitempty"`
	Email         *string              `json:"email,omitempty"`
	Address       *string              `json:"address,omitempty"`
	Note          *string              `json:"note,omitempty"`
	IsActive      bool                 `json:"is_active"`
	Ingredients   []SupplierIngredient `json:"ingredients"`
}

type SupplierInput struct {
	Name          string  `json:"name"`
	ContactPerson *string `json:"contact_person"`
	Phone         *string `json:"phone"`
	Email         *string `json:"email"`
	Address       *string `json:"address"`
	Note          *string `json:"note"`
}

type SupplierIngredient struct {
	IngredientID   string  `json:"ingredient_id"`
	IngredientName string  `json:"ingredient_name,omitempty"`
	PurchaseUnitID *int16  `json:"purchase_unit_id,omitempty"`
	LastPrice      *string `json:"last_price,omitempty"`
	LeadTimeDays   *int    `json:"lead_time_days,omitempty"`
	IsPreferred    bool    `json:"is_preferred"`
}

type SupplierIngredientInput struct {
	IngredientID   string  `json:"ingredient_id"`
	PurchaseUnitID *int16  `json:"purchase_unit_id"`
	LastPrice      *string `json:"last_price"`
	LeadTimeDays   *int    `json:"lead_time_days"`
	IsPreferred    bool    `json:"is_preferred"`
}
