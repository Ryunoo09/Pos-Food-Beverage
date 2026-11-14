package dashboard

import (
	"context"
	"net/http"
	"time"

	"github.com/shopspring/decimal"

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

func (s *Service) Summary(ctx context.Context, actor auth.Actor, from, to string) (*Summary, error) {
	if actor.OutletID == "" {
		return nil, httpx.NewError(http.StatusUnprocessableEntity, "OUTLET_NOT_CONFIGURED",
			"Outlet belum dikonfigurasi. Hubungi pemilik usaha.")
	}

	fromDate, toDate, err := parsePeriod(from, to)
	if err != nil {
		return nil, err
	}

	tz, err := s.repo.Timezone(ctx, actor.BusinessID)
	if err != nil {
		return nil, err
	}

	totals, err := s.repo.totals(ctx, actor.OutletID, tz, fromDate, toDate)
	if err != nil {
		return nil, err
	}
	methods, err := s.repo.methods(ctx, actor.OutletID, tz, fromDate, toDate)
	if err != nil {
		return nil, err
	}
	hourly, err := s.repo.hourly(ctx, actor.OutletID, tz, fromDate, toDate)
	if err != nil {
		return nil, err
	}
	top, err := s.repo.topProducts(ctx, actor.OutletID, tz, fromDate, toDate)
	if err != nil {
		return nil, err
	}
	critical, err := s.repo.criticalStock(ctx, actor.OutletID)
	if err != nil {
		return nil, err
	}
	active, err := s.repo.activeOrders(ctx, actor.OutletID)
	if err != nil {
		return nil, err
	}

	out := &Summary{
		From: fromDate, To: toDate,
		GrossSales: totals.GrossSales, NetSales: totals.NetSales,
		Transactions: totals.Transactions,
		AOV:          average(totals.NetSales, totals.Transactions),
		Methods:      methods, Hourly: hourly, TopProducts: top,
		Critical: critical, Active: active,
	}

	if contains(actor.Permissions, "reports.profit.view") {
		hpp := totals.HPP
		net, _ := money.Parse(totals.NetSales)
		cogs, _ := money.Parse(totals.HPP)
		profit := money.Format(net.Sub(cogs))
		out.HPP = &hpp
		out.GrossProfit = &profit
	}

	return out, nil
}

func parsePeriod(from, to string) (string, string, error) {
	today := time.Now().Format("2006-01-02")
	if from == "" {
		from = today
	}
	if to == "" {
		to = from
	}

	fromDate, err := time.Parse("2006-01-02", from)
	if err != nil {
		return "", "", httpx.NewError(http.StatusBadRequest, "VALIDATION_ERROR", "Parameter from tidak valid (YYYY-MM-DD).")
	}
	toDate, err := time.Parse("2006-01-02", to)
	if err != nil {
		return "", "", httpx.NewError(http.StatusBadRequest, "VALIDATION_ERROR", "Parameter to tidak valid (YYYY-MM-DD).")
	}
	if toDate.Before(fromDate) {
		return "", "", httpx.NewError(http.StatusBadRequest, "VALIDATION_ERROR", "Tanggal akhir harus setelah tanggal awal.")
	}
	if toDate.Sub(fromDate) > maxRangeDays*24*time.Hour {
		return "", "", httpx.NewError(http.StatusBadRequest, "VALIDATION_ERROR", "Rentang maksimal 366 hari.")
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
	return money.Format(d.Div(decimal.NewFromInt(int64(count))))
}

func contains(list []string, want string) bool {
	for _, item := range list {
		if item == want {
			return true
		}
	}
	return false
}
