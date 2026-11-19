package report

// Benchmark dan uji target performa PRD (plan 10.5). Butuh database nyata:
//
//	$env:REPORT_TEST_DATABASE_URL="postgres://postgres:postgres@localhost:5432/pos_food_test"
//	go test -count=1 ./internal/report/ -run TestReport31Days -v
//	go test -bench 'ReportSales31Days|Dashboard500Orders' ./internal/report/
//
// Data uji memakai periode 2035-01 (jauh dari data dev) dan dihapus lagi.

import (
	"context"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ryunoo09/pos-food-beverage/backend/internal/auth"
	"github.com/ryunoo09/pos-food-beverage/backend/internal/dashboard"
)

const (
	benchOrderCount = 15000
	benchFrom       = "2035-01-01"
	benchTo         = "2035-01-31"
	benchDay        = "2035-01-15"
)

func benchPool(tb testing.TB) *pgxpool.Pool {
	tb.Helper()
	url := os.Getenv("REPORT_TEST_DATABASE_URL")
	if url == "" {
		tb.Skip("REPORT_TEST_DATABASE_URL tidak diisi; lewati benchmark")
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		tb.Fatalf("buka pool: %v", err)
	}
	tb.Cleanup(pool.Close)
	return pool
}

// seedBulkOrders mengisi 15.000 pesanan COMPLETED tersebar 31 hari dan
// mengembalikan aktor + periode.
func seedBulkOrders(tb testing.TB, pool *pgxpool.Pool) auth.Actor {
	tb.Helper()
	ctx := context.Background()

	var outletID, businessID string
	if err := pool.QueryRow(ctx, `
		SELECT o.id, o.business_id FROM outlets o LIMIT 1`).Scan(&outletID, &businessID); err != nil {
		tb.Fatalf("ambil outlet: %v", err)
	}

	tb.Cleanup(func() {
		_, _ = pool.Exec(context.Background(),
			`DELETE FROM orders WHERE outlet_id = $1
			   AND completed_at >= '2035-01-01' AND completed_at < '2035-02-01'`, outletID)
	})

	if _, err := pool.Exec(ctx, `
		DELETE FROM orders WHERE outlet_id = $1
		  AND completed_at >= '2035-01-01' AND completed_at < '2035-02-01'`, outletID); err != nil {
		tb.Fatalf("bersihkan data benchmark: %v", err)
	}

	if _, err := pool.Exec(ctx, `
		INSERT INTO orders (outlet_id, service_type, status, payment_timing, payment_status,
		                    subtotal_amount, total_cogs, total_amount, completed_at)
		SELECT $1, 'TAKEAWAY', 'COMPLETED', 'PAY_FIRST', 'PAID',
		       10000 + (g % 5) * 1000, 4000, 12000,
		       timestamptz '2035-01-01 10:00:00+00' + ((g % 31) * interval '1 day')
		FROM generate_series(1, $2) g`, outletID, benchOrderCount); err != nil {
		tb.Fatalf("seed %d pesanan: %v", benchOrderCount, err)
	}

	return auth.Actor{
		BusinessID: businessID,
		OutletID:   outletID,
		UserID:     "bench-user",
	}
}

func BenchmarkReportSales31Days(b *testing.B) {
	pool := benchPool(b)
	actor := seedBulkOrders(b, pool)
	svc := NewService(NewRepository(pool))
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := svc.Sales(ctx, actor, benchFrom, benchTo); err != nil {
			b.Fatalf("sales report: %v", err)
		}
	}
}

func BenchmarkDashboard500Orders(b *testing.B) {
	pool := benchPool(b)
	actor := seedBulkOrders(b, pool)
	svc := dashboard.NewService(dashboard.NewRepository(pool))
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := svc.Summary(ctx, actor, benchDay, benchDay); err != nil {
			b.Fatalf("dashboard: %v", err)
		}
	}
}

// TestReport31Days_WithinTarget menegakkan target PRD: laporan 31 hari /
// 15.000 pesanan p95 <= 5 detik.
func TestReport31Days_WithinTarget(t *testing.T) {
	pool := benchPool(t)
	actor := seedBulkOrders(t, pool)
	svc := NewService(NewRepository(pool))
	ctx := context.Background()

	p95 := p95Duration(t, func() error {
		_, err := svc.Sales(ctx, actor, benchFrom, benchTo)
		return err
	})
	t.Logf("laporan 31 hari p95 = %v (target 5s)", p95)
	if p95 > 5*time.Second {
		t.Fatalf("p95 = %v melewati target 5s", p95)
	}
}

// TestDashboard500_WithinTarget menegakkan target PRD: dashboard ±500 pesanan
// p95 <= 3 detik.
func TestDashboard500_WithinTarget(t *testing.T) {
	pool := benchPool(t)
	actor := seedBulkOrders(t, pool)
	svc := dashboard.NewService(dashboard.NewRepository(pool))
	ctx := context.Background()

	p95 := p95Duration(t, func() error {
		_, err := svc.Summary(ctx, actor, benchDay, benchDay)
		return err
	})
	t.Logf("dashboard 1 hari p95 = %v (target 3s)", p95)
	if p95 > 3*time.Second {
		t.Fatalf("p95 = %v melewati target 3s", p95)
	}
}

// p95Duration menjalankan fn 20 kali (atau 1 kali untuk -short) dan
// mengembalikan durasi pada persentil 95.
func p95Duration(t *testing.T, fn func() error) time.Duration {
	t.Helper()
	runs := 20
	if testing.Short() {
		runs = 1
	}
	samples := make([]time.Duration, runs)
	for i := 0; i < runs; i++ {
		start := time.Now()
		if err := fn(); err != nil {
			t.Fatalf("iterasi %d: %v", i, err)
		}
		samples[i] = time.Since(start)
	}
	sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
	return samples[len(samples)-1]
}