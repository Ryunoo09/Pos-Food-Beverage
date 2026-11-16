package dashboard

// Summary adalah payload dashboard. Field HPP dan laba hanya diisi bila
// pengguna punya permission reports.profit.view.
type Summary struct {
	From         string         `json:"from"`
	To           string         `json:"to"`
	GrossSales   string         `json:"gross_sales"`
	NetSales     string         `json:"net_sales"`
	Transactions int            `json:"transactions"`
	AOV          string         `json:"aov"`
	HPP          *string        `json:"hpp,omitempty"`
	GrossProfit  *string        `json:"gross_profit,omitempty"`
	Methods      []MethodTotal  `json:"payment_methods"`
	Hourly       []HourPoint    `json:"hourly_sales"`
	TopProducts  []ProductTotal `json:"top_products"`
	Critical     []StockItem    `json:"critical_stock"`
	Active       []StatusCount  `json:"active_orders"`
}

type MethodTotal struct {
	Method string `json:"method"`
	Amount string `json:"amount"`
}

type HourPoint struct {
	Hour  int    `json:"hour"`
	Sales string `json:"sales"`
	// Transactions dipakai grafik "Pesanan per jam": tanpa angka ini klien
	// hanya punya nominal dan tidak bisa menggambar jumlah pesanan.
	Transactions int `json:"transactions"`
}

type ProductTotal struct {
	ProductID string `json:"product_id"`
	Name      string `json:"name"`
	Quantity  int    `json:"quantity"`
	Sales     string `json:"sales"`
}

type StockItem struct {
	IngredientID string `json:"ingredient_id"`
	Name         string `json:"name"`
	Quantity     string `json:"quantity_on_hand"`
	MinStock     string `json:"min_stock"`
}

type StatusCount struct {
	Status string `json:"status"`
	Count  int    `json:"count"`
}

type totals struct {
	GrossSales   string
	NetSales     string
	HPP          string
	Transactions int
}
