package waste

import (
	"github.com/ryunoo09/pos-food-beverage/backend/internal/pkg/money"
)

// wasteLine adalah kebutuhan satu bahan yang terbuang, dalam satuan dasar.
type wasteLine struct {
	IngredientID string
	Quantity     money.Decimal
}

var hundred = money.FromInt(100)

// recipeWasteLines mengalikan komponen resep dengan jumlah unit produk dan
// waste factor tiap bahan (BR-F08), lalu menjumlahkan bahan yang sama.
func recipeWasteLines(components []recipeComponent, units money.Decimal) ([]wasteLine, error) {
	perIngredient := map[string]money.Decimal{}
	order := []string{}
	for _, c := range components {
		qty, err := money.Parse(c.Quantity)
		if err != nil {
			return nil, err
		}
		waste, err := money.Parse(c.WasteFactor)
		if err != nil {
			return nil, err
		}
		scaled := qty.Mul(hundred.Add(waste)).Div(hundred).Mul(units)
		if _, ok := perIngredient[c.IngredientID]; !ok {
			order = append(order, c.IngredientID)
		}
		perIngredient[c.IngredientID] = perIngredient[c.IngredientID].Add(scaled)
	}

	lines := make([]wasteLine, 0, len(order))
	for _, id := range order {
		if qty := perIngredient[id]; qty.IsPositive() {
			lines = append(lines, wasteLine{IngredientID: id, Quantity: qty})
		}
	}
	return lines, nil
}
