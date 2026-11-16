package report

import (
	"context"
	"net/http"
	"time"

	"github.com/ryunoo09/pos-food-beverage/backend/internal/auth"
	"github.com/ryunoo09/pos-food-beverage/backend/internal/httpx"
	"github.com/ryunoo09/pos-food-beverage/backend/internal/pkg/money"
)

const maxRangeDays = 366

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// Sales membangun laporan penjualan periode.
func (s *Service) Sales(ctx context.Context, actor auth.Actor, from, to string) (*SalesReport, error) {
	if err := requireOutlet(actor); err != nil {
		return nil, err
	}
	fromDate, toDate, err := parsePeriod(from, to)
	if err != nil {
		return nil, err
	}
	tz, err := s.repo.Timezone(ctx, actor.BusinessID)
	if err != nil {
		return nil, err
	}

	t, err := s.repo.begin(ctx)
	if err != nil {
		return nil, err
	}
	defer t.rollback(ctx)

	totals, err := s.repo.salesTotals(ctx, t, actor.OutletID, tz, fromDate, toDate)
	if err != nil {
		return nil, err
	}
	totals.AOV = average(totals.NetSales, totals.OrderCount)

	daily, err := s.repo.daily(ctx, t, actor.OutletID, tz, fromDate, toDate)
	if err != nil {
		return nil, err
	}
	byService, err := s.repo.byService(ctx, t, actor.OutletID, tz, fromDate, toDate)
	if err != nil {
		return nil, err
	}
	byMethod, err := s.repo.byMethod(ctx, t, actor.OutletID, tz, fromDate, toDate)
	if err != nil {
		return nil, err
	}
	byCashier, err := s.repo.byCashier(ctx, t, actor.OutletID, tz, fromDate, toDate)
	if err != nil {
		return nil, err
	}
	top, err := s.repo.topProducts(ctx, t, actor.OutletID, tz, fromDate, toDate)
	if err != nil {
		return nil, err
	}
	for i := range top {
		top[i].MarginPct, _ = percent(top[i].GrossProfit, top[i].Revenue)
	}

	if err := t.commit(ctx); err != nil {
		return nil, err
	}

	return &SalesReport{
		From: fromDate, To: toDate,
		Total: totals, Daily: daily, ByService: byService,
		ByMethod: byMethod, ByCashier: byCashier, TopProducts: top,
	}, nil
}

// Financial membangun estimasi laba rugi. Angka biaya hanya diisi bila
// pengguna punya reports.profit.view; field-nya dibiarkan nil supaya tidak
// ikut terserialisasi ke JSON.
func (s *Service) Financial(ctx context.Context, actor auth.Actor, from, to string) (*FinancialReport, error) {
	if err := requireOutlet(actor); err != nil {
		return nil, err
	}
	fromDate, toDate, err := parsePeriod(from, to)
	if err != nil {
		return nil, err
	}
	tz, err := s.repo.Timezone(ctx, actor.BusinessID)
	if err != nil {
		return nil, err
	}

	t, err := s.repo.begin(ctx)
	if err != nil {
		return nil, err
	}
	defer t.rollback(ctx)

	totals, err := s.repo.salesTotals(ctx, t, actor.OutletID, tz, fromDate, toDate)
	if err != nil {
		return nil, err
	}

	report := &FinancialReport{
		From: fromDate, To: toDate,
		GrossSales: totals.GrossSales, Discount: totals.Discount,
		ServiceCharge: totals.ServiceCharge, Tax: totals.Tax, NetSales: totals.NetSales,
	}

	if contains(actor.Permissions, "reports.profit.view") {
		hpp, waste, err := s.repo.financialCosts(ctx, t, actor.OutletID, tz, fromDate, toDate)
		if err != nil {
			return nil, err
		}
		expense, err := s.repo.operatingExpense(ctx, t, actor.OutletID, fromDate, toDate)
		if err != nil {
			return nil, err
		}
		byCategory, err := s.repo.expensesByCategory(ctx, t, actor.OutletID, fromDate, toDate)
		if err != nil {
			return nil, err
		}

		grossProfit, err := subMoney(totals.NetSales, hpp)
		if err != nil {
			return nil, err
		}
		// Laba bersih = laba kotor - waste - pengeluaran operasional.
		afterWaste, err := subMoney(grossProfit, waste)
		if err != nil {
			return nil, err
		}
		netProfit, err := subMoney(afterWaste, expense)
		if err != nil {
			return nil, err
		}
		margin, err := percent(grossProfit, totals.NetSales)
		if err != nil {
			return nil, err
		}

		report.HPP = &hpp
		report.GrossProfit = &grossProfit
		report.Waste = &waste
		report.OperatingCost = &expense
		report.NetProfit = &netProfit
		report.GrossMarginPct = &margin
		report.ExpensesByCategory = byCategory
	}

	if err := t.commit(ctx); err != nil {
		return nil, err
	}
	return report, nil
}

func (s *Service) Inventory(ctx context.Context, actor auth.Actor, from, to string) (*InventoryReport, error) {
	if err := requireOutlet(actor); err != nil {
		return nil, err
	}
	fromDate, toDate, err := parsePeriod(from, to)
	if err != nil {
		return nil, err
	}
	tz, err := s.repo.Timezone(ctx, actor.BusinessID)
	if err != nil {
		return nil, err
	}

	t, err := s.repo.begin(ctx)
	if err != nil {
		return nil, err
	}
	defer t.rollback(ctx)

	stock, value, err := s.repo.stock(ctx, t, actor.BusinessID, actor.OutletID)
	if err != nil {
		return nil, err
	}
	movements, err := s.repo.movements(ctx, t, actor.OutletID, tz, fromDate, toDate)
	if err != nil {
		return nil, err
	}
	waste, err := s.repo.wasteSummary(ctx, t, actor.OutletID, tz, fromDate, toDate)
	if err != nil {
		return nil, err
	}
	opnames, err := s.repo.opnames(ctx, t, actor.OutletID, fromDate, toDate)
	if err != nil {
		return nil, err
	}

	// Total waste dijumlah di server supaya klien tidak menjumlahkan uang.
	wasteTotal := "0"
	for _, w := range waste {
		wasteTotal, err = addMoney(wasteTotal, w.Cost)
		if err != nil {
			return nil, err
		}
	}

	if err := t.commit(ctx); err != nil {
		return nil, err
	}
	return &InventoryReport{
		From: fromDate, To: toDate, StockValue: value, WasteValue: wasteTotal,
		Stock: stock, Movements: movements, Waste: waste, Opnames: opnames,
	}, nil
}

func (s *Service) MenuPerformance(ctx context.Context, actor auth.Actor, from, to string) (*MenuPerformanceReport, error) {
	if err := requireOutlet(actor); err != nil {
		return nil, err
	}
	fromDate, toDate, err := parsePeriod(from, to)
	if err != nil {
		return nil, err
	}
	tz, err := s.repo.Timezone(ctx, actor.BusinessID)
	if err != nil {
		return nil, err
	}

	t, err := s.repo.begin(ctx)
	if err != nil {
		return nil, err
	}
	defer t.rollback(ctx)

	items, err := s.repo.menuPerformance(ctx, t, actor.OutletID, tz, fromDate, toDate)
	if err != nil {
		return nil, err
	}
	for i := range items {
		if items[i].Revenue == "0.00" || items[i].Revenue == "0" {
			continue
		}
		margin, err := percent(items[i].GrossProfit, items[i].Revenue)
		if err != nil {
			continue
		}
		m := margin
		items[i].MarginPct = &m
	}

	if err := t.commit(ctx); err != nil {
		return nil, err
	}
	return &MenuPerformanceReport{From: fromDate, To: toDate, Items: items}, nil
}

func (s *Service) Shifts(ctx context.Context, actor auth.Actor, from, to string) (*ShiftReport, error) {
	if err := requireOutlet(actor); err != nil {
		return nil, err
	}
	fromDate, toDate, err := parsePeriod(from, to)
	if err != nil {
		return nil, err
	}

	t, err := s.repo.begin(ctx)
	if err != nil {
		return nil, err
	}
	defer t.rollback(ctx)

	shifts, err := s.repo.shifts(ctx, t, actor.OutletID, fromDate, toDate)
	if err != nil {
		return nil, err
	}

	if err := t.commit(ctx); err != nil {
		return nil, err
	}
	return &ShiftReport{From: fromDate, To: toDate, Shifts: shifts}, nil
}

func requireOutlet(actor auth.Actor) error {
	if actor.OutletID == "" {
		return httpx.NewError(http.StatusUnprocessableEntity, "OUTLET_NOT_CONFIGURED",
			"Outlet belum dikonfigurasi. Hubungi pemilik usaha.")
	}
	return nil
}

func parsePeriod(from, to string) (string, string, error) {
	if from == "" || to == "" {
		return "", "", httpx.NewError(http.StatusBadRequest, "VALIDATION_ERROR",
			"Parameter from dan to wajib diisi (YYYY-MM-DD).")
	}
	fromDate, err := time.Parse("2006-01-02", from)
	if err != nil {
		return "", "", httpx.NewError(http.StatusBadRequest, "VALIDATION_ERROR",
			"Parameter from tidak valid (YYYY-MM-DD).")
	}
	toDate, err := time.Parse("2006-01-02", to)
	if err != nil {
		return "", "", httpx.NewError(http.StatusBadRequest, "VALIDATION_ERROR",
			"Parameter to tidak valid (YYYY-MM-DD).")
	}
	if toDate.Before(fromDate) {
		return "", "", httpx.NewError(http.StatusBadRequest, "VALIDATION_ERROR",
			"Tanggal akhir harus setelah tanggal awal.")
	}
	if toDate.Sub(fromDate) > maxRangeDays*24*time.Hour {
		return "", "", httpx.NewError(http.StatusBadRequest, "VALIDATION_ERROR",
			"Rentang maksimal 366 hari.")
	}
	return from, to, nil
}

func average(total string, count int) string {
	if count == 0 {
		return "0.00"
	}
	d, err := money.Parse(total)
	if err != nil {
		return "0.00"
	}
	return money.Format(d.Div(money.FromInt(int64(count))))
}

func contains(list []string, want string) bool {
	for _, item := range list {
		if item == want {
			return true
		}
	}
	return false
}
