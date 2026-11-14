package pos

import "testing"

// Kasus kalkulasi inti: setiap baris memverifikasi satu perilaku total.
func TestCalculate(t *testing.T) {
	cases := []struct {
		name         string
		in           Input
		wantTotal    string
		wantNetSales string
		wantTax      string
		wantRounding string
	}{
		{
			name:         "tanpa pajak satu item",
			in:           Input{Items: []Item{{"25000", 2}}},
			wantTotal:    "50000.00",
			wantNetSales: "50000.00",
			wantTax:      "0.00",
			wantRounding: "0.00",
		},
		{
			name:         "pajak eksklusif 11 persen",
			in:           Input{Items: []Item{{"47300", 1}}, TaxRate: "11"},
			wantTotal:    "52503.00",
			wantNetSales: "47300.00",
			wantTax:      "5203.00",
			wantRounding: "0.00",
		},
		{
			name:      "pajak inklusif 11 persen",
			in:        Input{Items: []Item{{"55500", 1}}, TaxRate: "11", TaxInclusive: true},
			wantTotal: "55500.00",
			wantTax:   "5500.00",
		},
		{
			name:      "diskon order persen",
			in:        Input{Items: []Item{{"50000", 1}}, OrderDiscount: &LineDiscount{DiscountType: "PERCENT", Value: "10"}},
			wantTotal: "45000.00",
		},
		{
			name:      "diskon order nominal dibatasi subtotal",
			in:        Input{Items: []Item{{"10000", 1}}, OrderDiscount: &LineDiscount{DiscountType: "AMOUNT", Value: "999999"}},
			wantTotal: "0.00",
		},
		{
			name: "diskon item per indeks",
			in: Input{
				Items:         []Item{{"20000", 1}, {"30000", 1}},
				ItemDiscounts: []LineDiscount{{Index: 0, DiscountType: "PERCENT", Value: "50"}},
			},
			wantTotal: "40000.00",
		},
		{
			name: "diskon item melebihi harga dibatasi",
			in: Input{
				Items:         []Item{{"5000", 1}},
				ItemDiscounts: []LineDiscount{{Index: 0, DiscountType: "AMOUNT", Value: "99999"}},
			},
			wantTotal: "0.00",
		},
		{
			name: "max_discount membatasi diskon persen",
			in: Input{
				Items:         []Item{{"100000", 1}},
				OrderDiscount: &LineDiscount{DiscountType: "PERCENT", Value: "50", MaxDiscount: "10000"},
			},
			wantTotal: "90000.00",
		},
		{
			name:      "service charge saja",
			in:        Input{Items: []Item{{"100000", 1}}, ServiceRate: "5"},
			wantTotal: "105000.00",
		},
		{
			name:      "pajak atas service",
			in:        Input{Items: []Item{{"100000", 1}}, ServiceRate: "10", TaxRate: "11", TaxOnService: true},
			wantTotal: "122100.00",
		},
		{
			name:      "pajak tanpa dasar service",
			in:        Input{Items: []Item{{"100000", 1}}, ServiceRate: "10", TaxRate: "11"},
			wantTotal: "121000.00",
		},
		{
			name:         "pembulatan up 100",
			in:           Input{Items: []Item{{"10123", 1}}, RoundingMode: "UP_100"},
			wantTotal:    "10200.00",
			wantRounding: "77.00",
		},
		{
			name:      "pembulatan nearest 100 ke atas",
			in:        Input{Items: []Item{{"10150", 1}}, RoundingMode: "NEAREST_100"},
			wantTotal: "10200.00",
		},
		{
			name:      "pembulatan nearest 100 ke bawah",
			in:        Input{Items: []Item{{"10149", 1}}, RoundingMode: "NEAREST_100"},
			wantTotal: "10100.00",
		},
		{
			name:      "pembulatan nearest 500",
			in:        Input{Items: []Item{{"10249", 1}}, RoundingMode: "NEAREST_500"},
			wantTotal: "10000.00",
		},
		{
			name:      "pembulatan tepat kelipatan tidak berubah",
			in:        Input{Items: []Item{{"10000", 1}}, RoundingMode: "UP_100"},
			wantTotal: "10000.00",
		},
		{
			name: "kombinasi lengkap",
			in: Input{
				Items:         []Item{{"47300", 2}, {"18000", 1}},
				ItemDiscounts: []LineDiscount{{Index: 1, DiscountType: "AMOUNT", Value: "3000"}},
				OrderDiscount: &LineDiscount{DiscountType: "PERCENT", Value: "10"},
				TaxRate:       "11",
				ServiceRate:   "5",
				RoundingMode:  "NEAREST_100",
			},
			wantTotal: "114400.00",
			wantTax:   "10850.40",
		},
		{
			name:      "quantity besar tanpa kehilangan presisi",
			in:        Input{Items: []Item{{"3333", 300}}},
			wantTotal: "999900.00",
		},
		{
			name:      "harga desimal dua angka",
			in:        Input{Items: []Item{{"15000.55", 3}}},
			wantTotal: "45001.65",
		},
		{
			name:      "diskon menghabiskan subtotal tidak menghasilkan total negatif",
			in:        Input{Items: []Item{{"10000", 1}}, OrderDiscount: &LineDiscount{DiscountType: "PERCENT", Value: "100"}, ServiceRate: "5", TaxRate: "11"},
			wantTotal: "0.00",
		},
	}

	for _, c := range cases {
		got, err := Calculate(c.in)
		if err != nil {
			t.Fatalf("%s: error %v", c.name, err)
		}
		if got.Total != c.wantTotal {
			t.Errorf("%s: total = %s, want %s", c.name, got.Total, c.wantTotal)
		}
		if c.wantNetSales != "" && got.NetSales != c.wantNetSales {
			t.Errorf("%s: net_sales = %s, want %s", c.name, got.NetSales, c.wantNetSales)
		}
		if c.wantTax != "" && got.Tax != c.wantTax {
			t.Errorf("%s: tax = %s, want %s", c.name, got.Tax, c.wantTax)
		}
		if c.wantRounding != "" && got.RoundingAdjustment != c.wantRounding {
			t.Errorf("%s: rounding = %s, want %s", c.name, got.RoundingAdjustment, c.wantRounding)
		}
	}
}

func TestCalculate_Errors(t *testing.T) {
	if _, err := Calculate(Input{}); err == nil {
		t.Error("pesanan tanpa item diterima")
	}
	if _, err := Calculate(Input{Items: []Item{{"1000", 0}}}); err == nil {
		t.Error("quantity nol diterima")
	}
	if _, err := Calculate(Input{Items: []Item{{"1000", 1}}, TaxRate: "abc"}); err == nil {
		t.Error("tarif bukan angka diterima")
	}
	if _, err := Calculate(Input{Items: []Item{{"1000", 1}}, TaxRate: "150"}); err == nil {
		t.Error("tarif lebih dari 100 diterima")
	}
	if _, err := Calculate(Input{Items: []Item{{"1000", 1}}, RoundingMode: "BULAT_SEMBARANG"}); err == nil {
		t.Error("rounding_mode tidak dikenal diterima")
	}
	if _, err := Calculate(Input{
		Items:         []Item{{"1000", 1}},
		ItemDiscounts: []LineDiscount{{Index: 5, DiscountType: "PERCENT", Value: "10"}},
	}); err == nil {
		t.Error("index diskon di luar rentang diterima")
	}
	if _, err := Calculate(Input{Items: []Item{{"-1000", 1}}}); err == nil {
		t.Error("harga negatif diterima")
	}
}
