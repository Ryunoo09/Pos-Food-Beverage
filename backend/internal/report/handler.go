package report

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strings"

	"github.com/ryunoo09/pos-food-beverage/backend/internal/audit"
	"github.com/ryunoo09/pos-food-beverage/backend/internal/auth"
	"github.com/ryunoo09/pos-food-beverage/backend/internal/httpx"
)

type Handler struct {
	svc   *Service
	audit *audit.Writer
}

func NewHandler(svc *Service, writer *audit.Writer) *Handler {
	return &Handler{svc: svc, audit: writer}
}

func (h *Handler) Sales(w http.ResponseWriter, r *http.Request) {
	actor, requestID := actorOf(r)
	report, err := h.svc.Sales(r.Context(), actor, r.URL.Query().Get("from"), r.URL.Query().Get("to"))
	write(w, report, err, requestID)
}

func (h *Handler) Financial(w http.ResponseWriter, r *http.Request) {
	actor, requestID := actorOf(r)
	report, err := h.svc.Financial(r.Context(), actor, r.URL.Query().Get("from"), r.URL.Query().Get("to"))
	write(w, report, err, requestID)
}

func (h *Handler) Inventory(w http.ResponseWriter, r *http.Request) {
	actor, requestID := actorOf(r)
	report, err := h.svc.Inventory(r.Context(), actor, r.URL.Query().Get("from"), r.URL.Query().Get("to"))
	write(w, report, err, requestID)
}

func (h *Handler) MenuPerformance(w http.ResponseWriter, r *http.Request) {
	actor, requestID := actorOf(r)
	report, err := h.svc.MenuPerformance(r.Context(), actor, r.URL.Query().Get("from"), r.URL.Query().Get("to"))
	write(w, report, err, requestID)
}

func (h *Handler) Shifts(w http.ResponseWriter, r *http.Request) {
	actor, requestID := actorOf(r)
	report, err := h.svc.Shifts(r.Context(), actor, r.URL.Query().Get("from"), r.URL.Query().Get("to"))
	write(w, report, err, requestID)
}

func actorOf(r *http.Request) (auth.Actor, string) {
	actor, _ := auth.ActorFrom(r.Context())
	return actor, httpx.RequestIDFrom(r.Context())
}

func write(w http.ResponseWriter, data any, err error, requestID string) {
	if err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}
	httpx.WriteData(w, http.StatusOK, data, requestID)
}

// Export menulis CSV langsung ke response (tanpa file sementara) dan mencatat
// satu baris audit karena ekspor mengeluarkan data ke luar sistem.
func (h *Handler) Export(w http.ResponseWriter, r *http.Request) {
	actor, requestID := actorOf(r)

	kind := r.PathValue("type")
	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")

	headers, rows, err := h.exportRows(r, actor, kind, from, to)
	if err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}

	filename := fmt.Sprintf("%s_%s_%s.csv", kind, from, to)
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	w.WriteHeader(http.StatusOK)

	// BOM UTF-8 agar Excel edisi Indonesia membaca karakter dengan benar.
	if _, err := w.Write([]byte("\xEF\xBB\xBF")); err != nil {
		return
	}

	cw := csv.NewWriter(w)
	if err := cw.Write(headers); err != nil {
		return
	}
	for _, row := range rows {
		if err := cw.Write(row); err != nil {
			return
		}
	}
	cw.Flush()

	// Data sudah terkirim ke klien; kegagalan audit tidak boleh memotong
	// respons setengah jadi, cukup gagal diam-diam di sisi server.
	_ = h.audit.Record(r.Context(), audit.Entry{
		Actor: audit.Actor{
			BusinessID: actor.BusinessID,
			OutletID:   actor.OutletID,
			UserID:     actor.UserID,
			Roles:      actor.Roles,
			IPAddress:  actor.IPAddress,
			UserAgent:  actor.UserAgent,
			RequestID:  requestID,
		},
		Action:     "report.export",
		EntityType: "report",
		After:      map[string]any{"type": kind, "from": from, "to": to},
	})
}

func (h *Handler) exportRows(r *http.Request, actor auth.Actor, kind, from, to string) (
	[]string, [][]string, error,
) {
	ctx := r.Context()
	switch kind {
	case "sales":
		report, err := h.svc.Sales(ctx, actor, from, to)
		if err != nil {
			return nil, nil, err
		}
		headers := []string{"tanggal", "pesanan", "penjualan_kotor", "diskon", "penjualan_bersih", "total"}
		rows := make([][]string, 0, len(report.Daily))
		for _, d := range report.Daily {
			rows = append(rows, []string{
				d.Key, fmt.Sprint(d.OrderCount), d.GrossSales, "", d.NetSales, d.Total,
			})
		}
		return headers, rows, nil

	case "financial":
		report, err := h.svc.Financial(ctx, actor, from, to)
		if err != nil {
			return nil, nil, err
		}
		headers := []string{"keterangan", "nilai"}
		rows := [][]string{
			{"Penjualan kotor", report.GrossSales},
			{"Diskon", report.Discount},
			{"Service charge", report.ServiceCharge},
			{"Pajak", report.Tax},
			{"Penjualan bersih", report.NetSales},
		}
		if report.HPP != nil {
			rows = append(rows,
				[]string{"HPP", *report.HPP},
				[]string{"Laba kotor", *report.GrossProfit},
				[]string{"Kerugian waste", *report.Waste},
				[]string{"Pengeluaran operasional", *report.OperatingCost},
				[]string{"Estimasi laba bersih", *report.NetProfit},
			)
		}
		return headers, rows, nil

	case "menu-performance":
		report, err := h.svc.MenuPerformance(ctx, actor, from, to)
		if err != nil {
			return nil, nil, err
		}
		headers := []string{"produk", "varian", "jumlah_terjual", "omzet", "hpp", "laba_kotor", "margin_persen"}
		rows := make([][]string, 0, len(report.Items))
		for _, item := range report.Items {
			margin := ""
			if item.MarginPct != nil {
				margin = *item.MarginPct
			}
			rows = append(rows, []string{
				item.Name, item.VariantName, fmt.Sprint(item.Quantity),
				item.Revenue, item.Cost, item.GrossProfit, margin,
			})
		}
		return headers, rows, nil

	case "inventory":
		report, err := h.svc.Inventory(ctx, actor, from, to)
		if err != nil {
			return nil, nil, err
		}
		headers := []string{"kode", "bahan", "satuan", "stok", "min_stok", "hpp_rata_rata", "nilai", "status"}
		rows := make([][]string, 0, len(report.Stock))
		for _, item := range report.Stock {
			rows = append(rows, []string{
				item.Code, item.Name, item.Unit, item.OnHand,
				item.MinStock, item.AvgCost, item.Value, item.Status,
			})
		}
		return headers, rows, nil

	case "shifts":
		report, err := h.svc.Shifts(ctx, actor, from, to)
		if err != nil {
			return nil, nil, err
		}
		headers := []string{"kasir", "status", "dibuka", "ditutup", "kas_awal", "kas_seharusnya", "kas_aktual", "selisih", "penjualan"}
		rows := make([][]string, 0, len(report.Shifts))
		for _, s := range report.Shifts {
			rows = append(rows, []string{
				s.CashierName, s.Status, s.OpenedAt, deref(s.ClosedAt),
				s.OpeningCash, deref(s.ExpectedCash), deref(s.ActualCash),
				deref(s.Difference), s.Sales,
			})
		}
		return headers, rows, nil

	default:
		return nil, nil, httpx.NewError(http.StatusNotFound, "NOT_FOUND",
			"Jenis laporan tidak dikenal: "+strings.TrimSpace(kind))
	}
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
