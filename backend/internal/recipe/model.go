package recipe

type Recipe struct {
	ID        string       `json:"id"`
	VariantID string       `json:"variant_id"`
	VersionNo int          `json:"version_no"`
	Status    string       `json:"status"`
	YieldNote *string      `json:"yield_note,omitempty"`
	Items     []RecipeItem `json:"items"`
	TotalCost *string      `json:"total_cost,omitempty"`
}

type RecipeItem struct {
	IngredientID       string  `json:"ingredient_id"`
	IngredientName     string  `json:"ingredient_name"`
	UnitCode           string  `json:"unit_code"`
	Quantity           string  `json:"quantity"`
	WasteFactorPercent string  `json:"waste_factor_percent"`
	LineCost           *string `json:"line_cost,omitempty"`
	avgCost            string
}

type RecipeInput struct {
	YieldNote *string           `json:"yield_note"`
	Items     []RecipeItemInput `json:"items"`
}

type RecipeItemInput struct {
	IngredientID       string `json:"ingredient_id"`
	Quantity           string `json:"quantity"`
	WasteFactorPercent string `json:"waste_factor_percent"`
}
