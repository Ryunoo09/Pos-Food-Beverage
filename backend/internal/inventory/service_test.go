package inventory

import (
	"testing"

	"github.com/ryunoo09/pos-food-beverage/backend/internal/pkg/money"
)

func d(t *testing.T, s string) money.Decimal {
	t.Helper()
	v, err := money.Parse(s)
	if err != nil {
		t.Fatalf("parse %q: %v", s, err)
	}
	return v
}

func TestNeedsAdjustmentApproval(t *testing.T) {
	limit := d(t, "500000.00")
	cases := []struct {
		name  string
		delta string
		qty   string
		value string
		want  bool
	}{
		{"kecil di bawah ambang", "1.5", "100", "75.00", false},
		{"nilai di atas ambang", "20000", "100", "1000000.00", true},
		{"persen di atas 20", "30", "100", "1500.00", true},
		{"persen tepat 20 tidak butuh", "20", "100", "1000.00", false},
		{"saldo nol hanya nilai", "1", "0", "50.00", false},
		{"saldo nol nilai besar", "20000", "0", "1000000.00", true},
		{"negatif ikut nilai absolut", "-30", "100", "1500.00", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := needsAdjustmentApproval(d(t, tc.delta), d(t, tc.qty), d(t, tc.value), limit)
			if got != tc.want {
				t.Fatalf("got %v want %v", got, tc.want)
			}
		})
	}
}
