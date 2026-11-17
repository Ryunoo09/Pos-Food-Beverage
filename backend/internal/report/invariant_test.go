package report

// Uji regresi konsistensi data (plan 08 §8.8). Membutuhkan database nyata,
// jadi dilewati bila REPORT_TEST_DATABASE_URL kosong:
//
//	$env:REPORT_TEST_DATABASE_URL="postgres://postgres:postgres@localhost:5432/pos_food_db"
//	go test -count=1 ./internal/report/ -run TestInvariant -v
//
// Invariant yang dicek:
//  1. dashboard.net_sales(periode) == salesReport.net_sales(periode)
//  2. SUM(salesReport.daily.net_sales) == salesReport.total.net_sales
//  3. financialReport.net_sales == salesReport.net_sales
//  4. financialReport.HPP == SUM(orders.total_cogs) periode
//  5. user tanpa reports.profit.view tidak menerima field HPP/laba

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ryunoo09/pos-food-beverage/backend/internal/auth"
)

func reportTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("REPORT_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("REPORT_TEST_DATABASE_URL tidak diisi; lewati uji invariant")
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatalf("buka pool: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// seedCompleted membuat pesanan COMPLETED dengan subtotal/total_cogs dikenal
// pada outlet seed. Tanggal selesai diambil hari ini di zona waktu bisnis,
// sehingga filter laporan (completed_at AT TIME ZONE tz)::date = day konsisten.
// Mengembalikan outletID, businessID, tz (zona bisnis), dan day (YYYY-MM-DD
// di zona bisnis) agar test bisa membangun aktor dan query bandingkan yang
// tepat.
func seedCompleted(t *testing.T, pool *pgxpool.Pool, subtotal, cogs string) (outletID, businessID, tz, day string) {
	t.Helper()
	ctx := context.Background()
	if err := pool.QueryRow(ctx, `
		SELECT o.id, o.business_id, b.timezone
		FROM outlets o JOIN businesses b ON b.id = o.business_id
		LIMIT 1`).Scan(&outletID, &businessID, &tz); err != nil {
		t.Fatalf("ambil outlet: %v", err)
	}

	// Titik waktu acuan: sekarang di zona bisnis.
	ref := time.Now().UTC()
	if tz != "" {
		if zone, err := time.LoadLocation(tz); err == nil && zone != time.UTC {
			ref = time.Now().In(zone)
		}
	}
	day = ref.Format("2006-01-02")

	// Set completed_at eksplisit ke tengah hari UTC dari `day` agar deterministik
	// dan tetap jatuh di tanggal `day` untuk zona bisnis (rentang wajar).
	at, _ := time.Parse(time.RFC3339, day+"T12:00:00Z")
	err := pool.QueryRow(ctx, `
		INSERT INTO orders (outlet_id, service_type, status, payment_timing, payment_status,
		                     subtotal_amount, total_cogs, total_amount, completed_at)
		VALUES ($1, 'TAKEAWAY', 'COMPLETED', 'PAY_FIRST', 'PAID',
		        $2, $3, $4, $5)
		RETURNING 1`,
		outletID, subtotal, cogs, subtotal, at).Scan(new(int))
	if err != nil {
		t.Fatalf("buat order COMPLETED: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(),
			`DELETE FROM orders WHERE outlet_id = $1
			   AND completed_at = $2
			   AND service_type = 'TAKEAWAY' AND status = 'COMPLETED'`,
			outletID, at)
	})
	return outletID, businessID, tz, day
}

// TestInvariantNetSalesDashboardEqualsReport memastikan net sales laporan
// sama dengan dashboard untuk periode yang sama.
func TestInvariantNetSalesDashboardEqualsReport(t *testing.T) {
	pool := reportTestPool(t)
	ctx := context.Background()

	outletID, businessID, tz, day := seedCompleted(t, pool, "10000.00", "4000.00")
	seedCompleted(t, pool, "5000.00", "2000.00")

	// Net yang diharapkan dari dua pesanan: subtotal 15000 - diskon 0.
	rep := NewService(NewRepository(pool))
	sales, err := rep.Sales(ctx, actorWith(outletID, businessID, "reports.profit.view"), day, day)
	if err != nil {
		t.Fatalf("sales report: %v", err)
	}
	if sales.Total.NetSales != "15000.00" {
		t.Fatalf("sales net = %s, want 15000.00 (2 pesanan)", sales.Total.NetSales)
	}

	// Dashboard memakai definisi net yang sama (zona bisnis untuk filter hari).
	var dashNet string
	if err := pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(subtotal_amount - item_discount_amount - order_discount_amount), 0)::text
		FROM orders
		WHERE outlet_id = $1 AND status = 'COMPLETED'
		  AND (completed_at AT TIME ZONE $2)::date = $3::date`, outletID, tz, day).Scan(&dashNet); err != nil {
		t.Fatalf("dashboard net query: %v", err)
	}
	if dashNet != sales.Total.NetSales {
		t.Fatalf("dashboard net=%s berbeda dengan sales report net=%s", dashNet, sales.Total.NetSales)
	}
}

// TestInvariantDailySumEqualsTotal memastikan jumlah harian sama dengan total.
func TestInvariantDailySumEqualsTotal(t *testing.T) {
	pool := reportTestPool(t)
	ctx := context.Background()

	outletID, businessID, _, day := seedCompleted(t, pool, "3000.00", "1000.00")
	seedCompleted(t, pool, "2000.00", "800.00")

	rep := NewService(NewRepository(pool))
	sales, err := rep.Sales(ctx, actorWith(outletID, businessID, "reports.profit.view"), day, day)
	if err != nil {
		t.Fatalf("sales report: %v", err)
	}

	sum := "0"
	for _, d := range sales.Daily {
		sum, err = addMoney(sum, d.NetSales)
		if err != nil {
			t.Fatalf("jumlah harian: %v", err)
		}
	}
	if sum != sales.Total.NetSales {
		t.Fatalf("SUM(daily.net)=%s berbeda dengan total.net=%s", sum, sales.Total.NetSales)
	}
}

// TestInvariantFinancialMatchesSales memastikan net sales dan HPP laporan
// keuangan konsisten dengan laporan penjualan.
func TestInvariantFinancialMatchesSales(t *testing.T) {
	pool := reportTestPool(t)
	ctx := context.Background()

	outletID, businessID, tz, day := seedCompleted(t, pool, "8000.00", "3000.00")

	rep := NewService(NewRepository(pool))
	sales, err := rep.Sales(ctx, actorWith(outletID, businessID, "reports.profit.view"), day, day)
	if err != nil {
		t.Fatalf("sales report: %v", err)
	}
	fin, err := rep.Financial(ctx, actorWith(outletID, businessID, "reports.profit.view"), day, day)
	if err != nil {
		t.Fatalf("financial report: %v", err)
	}

	if fin.NetSales != sales.Total.NetSales {
		t.Fatalf("financial net=%s berbeda dengan sales net=%s", fin.NetSales, sales.Total.NetSales)
	}
	if fin.HPP == nil {
		t.Fatal("HPP tidak diisi padahal punya reports.profit.view")
	}
	// HPP = SUM(total_cogs) periode (zona bisnis).
	var cogs string
	if err := pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(total_cogs), 0)::text
		FROM orders
		WHERE outlet_id = $1 AND status = 'COMPLETED'
		  AND (completed_at AT TIME ZONE $2)::date = $3::date`, outletID, tz, day).Scan(&cogs); err != nil {
		t.Fatalf("jumlah cogs: %v", err)
	}
	if *fin.HPP != cogs {
		t.Fatalf("financial HPP=%s berbeda dengan SUM(total_cogs)=%s", *fin.HPP, cogs)
	}
}

// TestInvariantProfitGate memastikan user tanpa reports.profit.view tidak
// menerima field HPP/laba.
func TestInvariantProfitGate(t *testing.T) {
	pool := reportTestPool(t)
	ctx := context.Background()

	outletID, businessID, _, day := seedCompleted(t, pool, "6000.00", "2500.00")
	rep := NewService(NewRepository(pool))

	// Tanpa permission profit: field biaya harus kosong.
	fin, err := rep.Financial(ctx, actorWith(outletID, businessID), day, day)
	if err != nil {
		t.Fatalf("financial report: %v", err)
	}
	if fin.HPP != nil || fin.GrossProfit != nil || fin.NetProfit != nil {
		t.Fatalf("field biaya terisi tanpa reports.profit.view: hpp=%v", fin.HPP)
	}

	// Net sales tetap dikirim (bukan rahasia).
	if fin.NetSales == "" {
		t.Fatal("net sales harus tetap dikirim")
	}
}

// actorWith membuat aktor outlet dengan businessID dan permission opsional.
func actorWith(outletID, businessID string, perms ...string) auth.Actor {
	return auth.Actor{
		BusinessID:  businessID,
		OutletID:    outletID,
		UserID:      "test-user",
		Permissions: perms,
	}
}
