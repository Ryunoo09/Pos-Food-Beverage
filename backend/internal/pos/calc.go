// Package pos berisi kalkulasi total pesanan yang otoritatif di server.
// Frontend hanya menampilkan hasil; semua pembulatan dan pajak dihitung di sini.
package pos

import (
	"net/http"
	"strings"

	"github.com/ryunoo09/pos-food-beverage/backend/internal/httpx"
	"github.com/ryunoo09/pos-food-beverage/backend/internal/pkg/money"
)

type Item struct {
	UnitPrice string `json:"unit_price"`
	Quantity  int    `json:"quantity"`
}

// LineDiscount menunjuk baris item lewat Index untuk diskon per item.
type LineDiscount struct {
	Index        int    `json:"index"`
	DiscountType string `json:"discount_type"` // PERCENT | AMOUNT
	Value        string `json:"value"`
	MaxDiscount  string `json:"max_discount"`
}

type Input struct {
	Items         []Item         `json:"items"`
	ItemDiscounts []LineDiscount `json:"item_discounts"`
	OrderDiscount *LineDiscount  `json:"order_discount"`
	TaxRate       string         `json:"tax_rate"`
	ServiceRate   string         `json:"service_rate"`
	TaxInclusive  bool           `json:"tax_inclusive"`
	TaxOnService  bool           `json:"tax_on_service"`
	RoundingMode  string         `json:"rounding_mode"`
}

type Line struct {
	Gross    string `json:"gross"`
	Discount string `json:"discount"`
	Net      string `json:"net"`
}

type Result struct {
	Subtotal           string `json:"subtotal"`
	ItemDiscount       string `json:"item_discount"`
	OrderDiscount      string `json:"order_discount"`
	NetSales           string `json:"net_sales"`
	Service            string `json:"service"`
	Tax                string `json:"tax"`
	RoundingAdjustment string `json:"rounding_adjustment"`
	Total              string `json:"total"`
	Lines              []Line `json:"lines"`
}

var zero = parse("0")

// Calculate menghitung total dari daftar item, diskon, pajak, service, dan
// pembulatan. Uang selalu decimal; hasil dikembalikan sebagai string NUMERIC.
func Calculate(in Input) (Result, error) {
	if len(in.Items) == 0 {
		return Result{}, httpx.NewError(http.StatusBadRequest, "VALIDATION_ERROR", "Pesanan minimal satu item.")
	}

	taxRate, err := optionalRate(in.TaxRate, "tax_rate")
	if err != nil {
		return Result{}, err
	}
	serviceRate, err := optionalRate(in.ServiceRate, "service_rate")
	if err != nil {
		return Result{}, err
	}

	lines := make([]Line, len(in.Items))
	subtotal := zero
	for i, item := range in.Items {
		if item.Quantity <= 0 {
			return Result{}, httpx.NewError(http.StatusBadRequest, "VALIDATION_ERROR", "Quantity item harus lebih dari nol.")
		}
		price, err := nonNegative(item.UnitPrice, "unit_price")
		if err != nil {
			return Result{}, err
		}
		gross := money.RoundHalfUp(price.Mul(intToDecimal(item.Quantity)), 2)
		subtotal = subtotal.Add(gross)
		lines[i] = Line{Gross: money.Format(gross), Discount: money.Format(zero), Net: money.Format(gross)}
	}

	itemDiscountTotal := zero
	for _, d := range in.ItemDiscounts {
		if d.Index < 0 || d.Index >= len(lines) {
			return Result{}, httpx.NewError(http.StatusBadRequest, "VALIDATION_ERROR", "Index diskon item di luar rentang.")
		}
		gross, _ := money.Parse(lines[d.Index].Gross)
		amount, err := discountAmount(gross, d)
		if err != nil {
			return Result{}, err
		}
		lineDisc, _ := money.Parse(lines[d.Index].Discount)
		lineDisc = lineDisc.Add(amount)
		if lineDisc.GreaterThan(gross) {
			lineDisc = gross
		}
		lines[d.Index].Discount = money.Format(lineDisc)
		lines[d.Index].Net = money.Format(gross.Sub(lineDisc))
		itemDiscountTotal = itemDiscountTotal.Add(amount)
	}
	if itemDiscountTotal.GreaterThan(subtotal) {
		itemDiscountTotal = subtotal
	}

	afterItems := subtotal.Sub(itemDiscountTotal)

	orderDiscount := zero
	if in.OrderDiscount != nil {
		orderDiscount, err = discountAmount(afterItems, *in.OrderDiscount)
		if err != nil {
			return Result{}, err
		}
	}

	netSales := afterItems.Sub(orderDiscount)
	service := money.RoundHalfUp(netSales.Mul(serviceRate).Div(hundred()), 2)

	taxBase := netSales
	if in.TaxOnService {
		taxBase = taxBase.Add(service)
	}

	var tax, total money.Decimal
	if in.TaxInclusive {
		divisor := hundred().Add(taxRate)
		tax = money.RoundHalfUp(taxBase.Sub(taxBase.Mul(hundred()).Div(divisor)), 2)
		total = netSales.Add(service)
	} else {
		tax = money.RoundHalfUp(taxBase.Mul(taxRate).Div(hundred()), 2)
		total = netSales.Add(service).Add(tax)
	}

	rounded, err := round(total, in.RoundingMode)
	if err != nil {
		return Result{}, err
	}

	return Result{
		Subtotal:           money.Format(subtotal),
		ItemDiscount:       money.Format(itemDiscountTotal),
		OrderDiscount:      money.Format(orderDiscount),
		NetSales:           money.Format(netSales),
		Service:            money.Format(service),
		Tax:                money.Format(tax),
		RoundingAdjustment: money.Format(rounded.Sub(total)),
		Total:              money.Format(rounded),
		Lines:              lines,
	}, nil
}

func discountAmount(base money.Decimal, d LineDiscount) (money.Decimal, error) {
	value, err := nonNegative(d.Value, "value")
	if err != nil {
		return zero, err
	}

	var amount money.Decimal
	switch d.DiscountType {
	case "PERCENT":
		if value.GreaterThan(hundred()) {
			return zero, httpx.NewError(http.StatusBadRequest, "VALIDATION_ERROR", "Diskon persen maksimal 100.")
		}
		amount = money.RoundHalfUp(base.Mul(value).Div(hundred()), 2)
	case "AMOUNT":
		amount = money.RoundHalfUp(value, 2)
	default:
		return zero, httpx.NewError(http.StatusBadRequest, "VALIDATION_ERROR",
			"discount_type harus PERCENT atau AMOUNT.")
	}

	if d.MaxDiscount != "" {
		max, err := nonNegative(d.MaxDiscount, "max_discount")
		if err != nil {
			return zero, err
		}
		if amount.GreaterThan(max) {
			amount = max
		}
	}
	if amount.GreaterThan(base) {
		amount = base
	}
	return amount, nil
}

func round(total money.Decimal, mode string) (money.Decimal, error) {
	switch mode {
	case "", "NONE":
		return total, nil
	case "UP_100":
		return total.Div(hundred()).Ceil().Mul(hundred()), nil
	case "NEAREST_100":
		return total.Div(hundred()).Round(0).Mul(hundred()), nil
	case "NEAREST_500":
		step := parse("500")
		return total.Div(step).Round(0).Mul(step), nil
	default:
		return zero, httpx.NewError(http.StatusBadRequest, "VALIDATION_ERROR", "rounding_mode tidak valid.")
	}
}

func optionalRate(raw, field string) (money.Decimal, error) {
	if strings.TrimSpace(raw) == "" {
		return zero, nil
	}
	rate, err := nonNegative(raw, field)
	if err != nil {
		return zero, err
	}
	if rate.GreaterThan(hundred()) {
		return zero, httpx.NewError(http.StatusBadRequest, "VALIDATION_ERROR", field+" maksimal 100.")
	}
	return rate, nil
}

func nonNegative(raw, field string) (money.Decimal, error) {
	value, err := money.Parse(raw)
	if err != nil {
		return zero, httpx.NewError(http.StatusBadRequest, "VALIDATION_ERROR", field+" tidak valid.")
	}
	if value.IsNegative() {
		return zero, httpx.NewError(http.StatusBadRequest, "VALIDATION_ERROR", field+" tidak boleh negatif.")
	}
	return value, nil
}

func hundred() money.Decimal { return parse("100") }

func intToDecimal(n int) money.Decimal { return money.FromInt(int64(n)) }

func parse(raw string) money.Decimal {
	d, _ := money.Parse(raw)
	return d
}
