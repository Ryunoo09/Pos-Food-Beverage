package purchase

import "github.com/ryunoo09/pos-food-beverage/backend/internal/pkg/money"

// mwac menghitung biaya rata-rata bergerak setelah penerimaan (BR-F11-6).
// Bila saldo lama <= 0, biaya rata-rata menjadi biaya penerimaan.
func mwac(oldQty, oldAvg, recvQty, recvCost money.Decimal) money.Decimal {
	total := oldQty.Add(recvQty)
	if !oldQty.IsPositive() || total.IsZero() {
		return recvCost.Round(6)
	}
	return oldQty.Mul(oldAvg).Add(recvQty.Mul(recvCost)).Div(total).Round(6)
}
