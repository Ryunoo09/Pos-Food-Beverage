package waste

import (
	"testing"

	"github.com/ryunoo09/pos-food-beverage/backend/internal/pkg/money"
)

func mustDec(t *testing.T, s string) money.Decimal {
	t.Helper()
	d, err := money.Parse(s)
	if err != nil {
		t.Fatalf("parse %q: %v", s, err)
	}
	return d
}

func TestRecipeWasteLines_ScaleAndMerge(t *testing.T) {
	components := []recipeComponent{
		{IngredientID: "a", Quantity: "100", WasteFactor: "10"},
		{IngredientID: "b", Quantity: "5", WasteFactor: "0"},
		{IngredientID: "a", Quantity: "50", WasteFactor: "0"},
	}
	lines, err := recipeWasteLines(components, mustDec(t, "2"))
	if err != nil {
		t.Fatalf("recipeWasteLines: %v", err)
	}
	if len(lines) != 2 {
		t.Fatalf("mau 2 baris, dapat %d", len(lines))
	}
	// a: (100*1.10 + 50) * 2 = 320; b: 5*2 = 10
	if got := lines[0]; got.IngredientID != "a" || got.Quantity.StringFixed(4) != "320.0000" {
		t.Errorf("baris a = %s %s, mau a 320.0000", got.IngredientID, got.Quantity.StringFixed(4))
	}
	if got := lines[1]; got.IngredientID != "b" || got.Quantity.StringFixed(4) != "10.0000" {
		t.Errorf("baris b = %s %s, mau b 10.0000", got.IngredientID, got.Quantity.StringFixed(4))
	}
}

func TestRecipeWasteLines_ZeroUnits(t *testing.T) {
	lines, err := recipeWasteLines([]recipeComponent{{IngredientID: "a", Quantity: "100", WasteFactor: "0"}},
		mustDec(t, "0"))
	if err != nil {
		t.Fatalf("recipeWasteLines: %v", err)
	}
	if len(lines) != 0 {
		t.Fatalf("mau tanpa baris untuk 0 unit, dapat %d", len(lines))
	}
}
