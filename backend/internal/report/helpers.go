package report

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"

	"github.com/ryunoo09/pos-food-beverage/backend/internal/pkg/money"
)

// txn membungkus transaksi read-only pelaporan.
type txn struct {
	tx pgx.Tx
}

func (t *txn) commit(ctx context.Context) error {
	return t.tx.Commit(ctx)
}

func (t *txn) rollback(ctx context.Context) {
	_ = t.tx.Rollback(ctx)
}

// days menghasilkan tanggal (YYYY-MM-DD) dari..to inklusif untuk mengisi hari
// kosong pada laporan harian.
func days(from, to string) []string {
	start, err := time.Parse("2006-01-02", from)
	if err != nil {
		return nil
	}
	end, err := time.Parse("2006-01-02", to)
	if err != nil {
		return nil
	}
	out := []string{}
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		out = append(out, d.Format("2006-01-02"))
	}
	return out
}

var methodLabels = map[string]string{
	"CASH":     "Tunai",
	"QRIS":     "QRIS",
	"TRANSFER": "Transfer bank",
	"E_WALLET": "E-wallet",
}

func methodLabel(code string) string {
	if label, ok := methodLabels[code]; ok {
		return label
	}
	return code
}

var serviceLabels = map[string]string{
	"DINE_IN":  "Dine-in",
	"TAKEAWAY": "Takeaway",
	"PICKUP":   "Ambil sendiri",
	"DELIVERY": "Delivery",
}

func serviceLabel(code string) string {
	if label, ok := serviceLabels[code]; ok {
		return label
	}
	return code
}

// addMoney menjumlah dua nominal string lewat decimal agar tidak ada
// kehilangan presisi; hasilnya tetap string terformat.
func addMoney(a, b string) (string, error) {
	x, err := money.Parse(a)
	if err != nil {
		return "", fmt.Errorf("add money %q: %w", a, err)
	}
	y, err := money.Parse(b)
	if err != nil {
		return "", fmt.Errorf("add money %q: %w", b, err)
	}
	return money.Format(x.Add(y)), nil
}

func subMoney(a, b string) (string, error) {
	x, err := money.Parse(a)
	if err != nil {
		return "", fmt.Errorf("sub money %q: %w", a, err)
	}
	y, err := money.Parse(b)
	if err != nil {
		return "", fmt.Errorf("sub money %q: %w", b, err)
	}
	return money.Format(x.Sub(y)), nil
}

// percent mengembalikan (part/total)*100 dengan 2 desimal.
func percent(part, total string) (string, error) {
	p, err := money.Parse(part)
	if err != nil {
		return "", err
	}
	t, err := money.Parse(total)
	if err != nil {
		return "", err
	}
	if t.IsZero() {
		return "0.00", nil
	}
	return p.Div(t).Mul(decimal.NewFromInt(100)).StringFixed(2), nil
}

func sumAmounts(items []MethodAmount) string {
	total := "0"
	for _, item := range items {
		next, err := addMoney(total, item.Amount)
		if err != nil {
			return total
		}
		total = next
	}
	return total
}

func oneHalf() decimal.Decimal {
	return decimal.New(5, -1)
}
