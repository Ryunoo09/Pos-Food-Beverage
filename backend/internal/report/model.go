// Package report menyediakan laporan penjualan, keuangan, inventaris, performa
// menu, dan shift beserta ekspor CSV (F19).
//
// Semua laporan hanya membaca snapshot yang tersimpan di orders/order_items
// (harga & HPP saat transaksi), bukan menghitung ulang dengan resep terkini,
// sehingga laporan bulan lalu tidak berubah ketika resep diedit.
package report

// SalesReport adalah laporan penjualan untuk satu periode.
//
// NetSales memakai definisi yang sama persis dengan dashboard
// (subtotal - diskon item - diskon pesanan) supaya angkanya konsisten.
type SalesReport struct {
	From        string        `json:"from"`
	To          string        `json:"to"`
	Total       SalesTotals   `json:"total"`
	Daily       []SalesBucket `json:"daily"`
	ByService   []SalesBucket `json:"by_service_type"`
	ByMethod    []SalesBucket `json:"by_payment_method"`
	ByCashier   []SalesBucket `json:"by_cashier"`
	TopProducts []ProductLine `json:"top_products"`
}

type SalesTotals struct {
	OrderCount    int    `json:"order_count"`
	GrossSales    string `json:"gross_sales"`
	Discount      string `json:"discount"`
	ServiceCharge string `json:"service_charge"`
	Tax           string `json:"tax"`
	NetSales      string `json:"net_sales"`
	Total         string `json:"total"`
	AOV           string `json:"aov"`
}

// SalesBucket adalah satu baris pengelompokan. Label berisi nilai mentah
// (tanggal, tipe layanan, kode metode, user id) dan nama tampilan bila ada.
type SalesBucket struct {
	Key       string `json:"key"`
	Label     string `json:"label"`
	OrderCount int   `json:"order_count"`
	GrossSales string `json:"gross_sales"`
	NetSales   string `json:"net_sales"`
	Total      string `json:"total"`
}

type ProductLine struct {
	ProductID   string `json:"product_id"`
	Name        string `json:"name"`
	Quantity    int    `json:"quantity"`
	Revenue     string `json:"revenue"`
	GrossProfit string `json:"gross_profit"`
	MarginPct   string `json:"margin_percent"`
}

// FinancialReport adalah estimasi laba rugi operasional. Field HPP/laba hanya
// diisi bila pengguna punya reports.profit.view; untuk peran lain struct ini
// tetap dikirim tetapi tanpa angka biaya.
//
// Bukan laporan akuntansi resmi: HPP berasal dari snapshot konsumsi bahan dan
// pengeluaran operasional dicatat manual oleh staf.
type FinancialReport struct {
	From    string `json:"from"`
	To      string `json:"to"`
	GrossSales    string `json:"gross_sales"`
	Discount      string `json:"discount"`
	ServiceCharge string `json:"service_charge"`
	Tax           string `json:"tax"`
	NetSales      string `json:"net_sales"`

	// Kelompok biaya — hanya untuk reports.profit.view.
	HPP            *string `json:"hpp,omitempty"`
	GrossProfit    *string `json:"gross_profit,omitempty"`
	Waste          *string `json:"waste,omitempty"`
	OperatingCost  *string `json:"operating_expense,omitempty"`
	NetProfit      *string `json:"net_profit,omitempty"`
	GrossMarginPct *string `json:"gross_margin_percent,omitempty"`

	// Pengeluaran per kategori berguna saat menelusuri biaya terbesar.
	ExpensesByCategory []ExpenseLine `json:"expenses_by_category,omitempty"`
}

type ExpenseLine struct {
	CategoryName string `json:"category_name"`
	Amount       string `json:"amount"`
}

// InventoryReport: stok saat ini + valuasi, ringkasan movement, waste, dan
// hasil opname pada periode.
type InventoryReport struct {
	From       string           `json:"from"`
	To         string           `json:"to"`
	StockValue string           `json:"stock_value"`
	Stock      []StockLine      `json:"stock"`
	Movements  []MovementLine   `json:"movements"`
	Waste      []WasteLine      `json:"waste"`
	Opnames    []OpnameLine     `json:"opnames"`
}

type StockLine struct {
	IngredientID string `json:"ingredient_id"`
	Code         string `json:"code"`
	Name         string `json:"name"`
	Unit         string `json:"unit"`
	OnHand       string `json:"quantity_on_hand"`
	MinStock     string `json:"min_stock"`
	AvgCost      string `json:"avg_unit_cost"`
	Value        string `json:"value"`
	Status       string `json:"status"`
}

type MovementLine struct {
	IngredientID string `json:"ingredient_id"`
	Name         string `json:"name"`
	Type         string `json:"movement_type"`
	Quantity     string `json:"quantity"`
}

type WasteLine struct {
	ReasonCode string `json:"reason_code"`
	Count      int    `json:"count"`
	Cost       string `json:"cost_value"`
}

type OpnameLine struct {
	ID           string `json:"id"`
	OpnameNo     string `json:"opname_no"`
	Status       string `json:"status"`
	FinishedAt   string `json:"finished_at"`
	TotalDiffQty string `json:"total_difference_quantity"`
	TotalDiffVal string `json:"total_difference_value"`
}

// MenuPerformanceReport: margin per menu, urut margin terendah supaya menu
// yang merugi muncul lebih dulu.
type MenuPerformanceReport struct {
	From  string           `json:"from"`
	To    string           `json:"to"`
	Items []MenuPerformance `json:"items"`
}

type MenuPerformance struct {
	ProductID   string `json:"product_id"`
	Name        string `json:"name"`
	VariantName string `json:"variant_name"`
	Quantity    int    `json:"quantity"`
	Revenue     string `json:"revenue"`
	Cost        string `json:"total_cost"`
	GrossProfit string `json:"gross_profit"`
	MarginPct   *string `json:"margin_percent"`
}

// ShiftReport: satu baris per shift pada periode.
type ShiftReport struct {
	From   string      `json:"from"`
	To     string      `json:"to"`
	Shifts []ShiftLine `json:"shifts"`
}

type ShiftLine struct {
	ID           string         `json:"id"`
	CashierName  string         `json:"cashier_name"`
	Status       string         `json:"status"`
	OpenedAt     string         `json:"opened_at"`
	ClosedAt     *string        `json:"closed_at,omitempty"`
	OpeningCash  string         `json:"opening_cash"`
	ExpectedCash *string        `json:"expected_cash,omitempty"`
	ActualCash   *string        `json:"actual_cash,omitempty"`
	Difference   *string        `json:"difference,omitempty"`
	Sales        string         `json:"sales"`
	Methods      []MethodAmount `json:"payment_methods"`
}

type MethodAmount struct {
	Method string `json:"method"`
	Amount string `json:"amount"`
}
