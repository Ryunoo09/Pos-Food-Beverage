// Package money menyediakan tipe dan operasi uang berbasis decimal.
// Uang tidak pernah memakai float; nilai dari API/DB adalah string NUMERIC.
package money

import (
	"fmt"

	"github.com/shopspring/decimal"
)

// Decimal adalah alias agar pemanggil tidak mengimpor dua paket.
type Decimal = decimal.Decimal

// Parse membaca string NUMERIC dari API atau DB.
func Parse(s string) (Decimal, error) {
	d, err := decimal.NewFromString(s)
	if err != nil {
		return Decimal{}, fmt.Errorf("money: parse %q: %w", s, err)
	}
	return d, nil
}

// RoundHalfUp membulatkan ke jumlah desimal tertentu dengan aturan setengah ke atas.
func RoundHalfUp(d Decimal, places int32) Decimal {
	return d.Round(places)
}

// Format mengembalikan string 2 desimal, format yang dipakai kolom NUMERIC(14,2).
func Format(d Decimal) string {
	return d.StringFixed(2)
}

// Percent menghitung rate persen (mis. 11 untuk PPN 11%) dari base.
func Percent(base, rate Decimal, places int32) Decimal {
	return base.Mul(rate).Div(decimal.NewFromInt(100)).Round(places)
}
