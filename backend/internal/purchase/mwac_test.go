package purchase

import (
	"testing"

	"github.com/ryunoo09/pos-food-beverage/backend/internal/pkg/money"
)

func mustParse(t *testing.T, s string) money.Decimal {
	t.Helper()
	d, err := money.Parse(s)
	if err != nil {
		t.Fatalf("parse %q: %v", s, err)
	}
	return d
}

func TestMwac_Scenarios(t *testing.T) {
	tests := []struct {
		name                      string
		oldQty, oldAvg, qty, cost string
		want                      string
	}{
		{"campuran dua biaya", "100", "50", "1000", "45", "45.454545"},
		{"saldo kosong pakai biaya baru", "0", "0", "1000", "45", "45.000000"},
		{"saldo negatif pakai biaya baru", "-10", "0", "1000", "45", "45.000000"},
		{"biaya sama tetap", "100", "50", "100", "50", "50.000000"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := mwac(mustParse(t, tc.oldQty), mustParse(t, tc.oldAvg),
				mustParse(t, tc.qty), mustParse(t, tc.cost))
			if got.StringFixed(6) != tc.want {
				t.Errorf("mwac = %s, mau %s", got.StringFixed(6), tc.want)
			}
		})
	}
}
