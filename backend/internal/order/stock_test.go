package order

// Uji konkurensi dan idempotensi konfirmasi (plan 10.2). Butuh database nyata:
//
//	$env:ORDER_TEST_DATABASE_URL="postgres://postgres:postgres@localhost:5432/pos_food_test"
//	go test -count=1 ./internal/order/ -run 'TestConcurrent|TestConfirmIdempotent' -v
//
// Fixture memakai produk TRACK_AS_STOCK_ITEM (Air Mineral) yang stoknya langsung
// berkurang 1 per unit, jadi test tidak bergantung pada data resep.

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ryunoo09/pos-food-beverage/backend/internal/auth"
	"github.com/ryunoo09/pos-food-beverage/backend/internal/pkg/money"
)

type stockFixture struct {
	outletID     string
	userID       string
	productID    string
	variantID    string
	ingredientID string
	startQty     money.Decimal
}

// seedStockFixture menyiapkan stok 1.000.000 dan mengembalikan data produk
// TRACK_AS_STOCK_ITEM pertama.
func seedStockFixture(t *testing.T, pool *pgxpool.Pool) stockFixture {
	t.Helper()
	ctx := context.Background()
	f := stockFixture{}

	err := pool.QueryRow(ctx, `
		SELECT p.id, v.id, p.stock_ingredient_id, o.id, p.business_id
		FROM products p
		JOIN product_variants v ON v.product_id = p.id
		JOIN outlets o ON o.business_id = p.business_id
		WHERE p.stock_mode = 'TRACK_AS_STOCK_ITEM'
		  AND p.stock_ingredient_id IS NOT NULL
		LIMIT 1`).Scan(&f.productID, &f.variantID, &f.ingredientID, &f.outletID, new(string))
	if err != nil {
		t.Fatalf("ambil produk stok langsung: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		SELECT id FROM users WHERE business_id = (SELECT business_id FROM products WHERE id = $1)
		LIMIT 1`, f.productID).Scan(&f.userID); err != nil {
		t.Fatalf("ambil user: %v", err)
	}

	// Set stok besar agar 100 konfirmasi tidak menabrak stok negatif.
	err = pool.QueryRow(ctx, `
		INSERT INTO inventory (outlet_id, ingredient_id, quantity_on_hand, avg_unit_cost)
		VALUES ($1, $2, 1000000, 1000)
		ON CONFLICT (outlet_id, ingredient_id)
		DO UPDATE SET quantity_on_hand = 1000000, avg_unit_cost = 1000
		RETURNING quantity_on_hand::text`, f.outletID, f.ingredientID).Scan(&f.startQty)
	if err != nil {
		t.Fatalf("siapkan stok: %v", err)
	}
	return f
}

// newDraft membuat pesanan TAKEAWAY DRAFT satu item dengan jumlah qty.
func newDraft(t *testing.T, pool *pgxpool.Pool, f stockFixture, qty int) string {
	t.Helper()
	ctx := context.Background()

	var orderID string
	err := pool.QueryRow(ctx, `
		INSERT INTO orders (outlet_id, service_type, status, payment_timing,
		                    subtotal_amount, total_amount, pricing_snapshot)
		VALUES ($1, 'TAKEAWAY', 'DRAFT', 'PAY_FIRST', 0, 0, '{}'::jsonb)
		RETURNING id`, f.outletID).Scan(&orderID)
	if err != nil {
		t.Fatalf("buat draft: %v", err)
	}

	_, err = pool.Exec(ctx, `
		INSERT INTO order_items (order_id, product_id, variant_id, product_name_snapshot,
		                         variant_name_snapshot, sku_snapshot, quantity, unit_price,
		                         modifiers_total, line_total)
		VALUES ($1, $2, $3, 'uji', 'uji', 'UJI', $4, 10000, 0, $5)`,
		orderID, f.productID, f.variantID, qty, 10000*qty)
	if err != nil {
		t.Fatalf("tambah item: %v", err)
	}
	return orderID
}

// cleanupOrders menghapus pesanan uji. stock_movements append-only, jadi baris
// movement dibiarkan (tanpa FK) dan stok dikembalikan manual.
func cleanupOrders(t *testing.T, pool *pgxpool.Pool, f stockFixture, orderIDs []string, keyPrefix string) {
	t.Helper()
	ctx := context.Background()

	pool.Exec(ctx, `DELETE FROM order_item_consumptions
		WHERE order_item_id IN (SELECT id FROM order_items WHERE order_id = ANY($1::uuid[]))`, orderIDs)
	pool.Exec(ctx, `DELETE FROM kitchen_order_items
		WHERE order_item_id IN (SELECT id FROM order_items WHERE order_id = ANY($1::uuid[]))`, orderIDs)
	pool.Exec(ctx, `DELETE FROM kitchen_orders WHERE order_id = ANY($1::uuid[])`, orderIDs)
	pool.Exec(ctx, `DELETE FROM order_status_histories WHERE order_id = ANY($1::uuid[])`, orderIDs)
	pool.Exec(ctx, `DELETE FROM payments WHERE order_id = ANY($1::uuid[])`, orderIDs)
	pool.Exec(ctx, `DELETE FROM order_items WHERE order_id = ANY($1::uuid[])`, orderIDs)
	pool.Exec(ctx, `DELETE FROM orders WHERE id = ANY($1::uuid[])`, orderIDs)
	pool.Exec(ctx, `DELETE FROM idempotency_keys WHERE key LIKE $1`, keyPrefix+"%")
	pool.Exec(ctx, `UPDATE inventory SET quantity_on_hand = $3
		WHERE outlet_id = $1 AND ingredient_id = $2`,
		f.outletID, f.ingredientID, f.startQty.StringFixed(4))
}

// TestConcurrentConfirm_DeductsStockExactly menjalankan 100 konfirmasi paralel
// pada bahan yang sama: stok akhir harus tepat start-100 dan SUM(delta) = -100.
func TestConcurrentConfirm_DeductsStockExactly(t *testing.T) {
	pool := testPool(t)
	f := seedStockFixture(t, pool)
	svc := NewService(NewRepository(pool))
	actor := auth.Actor{OutletID: f.outletID, UserID: f.userID, BusinessID: ""}

	const n = 100
	orders := make([]string, n)
	for i := range orders {
		orders[i] = newDraft(t, pool, f, 1)
	}
	t.Cleanup(func() {
		cleanupOrders(t, pool, f, orders, "stress-")
	})

	var wg sync.WaitGroup
	errs := make([]error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = svc.Confirm(context.Background(), actor, orders[i],
				fmt.Sprintf("stress-%s-%d", orders[i], i))
		}(i)
	}
	wg.Wait()

	failed := 0
	for i, err := range errs {
		if err != nil {
			failed++
			if failed <= 3 {
				t.Errorf("konfirmasi %d gagal: %v", i, err)
			}
		}
	}
	if failed > 0 {
		t.Fatalf("%d dari %d konfirmasi gagal", failed, n)
	}

	ctx := context.Background()
	var finalQty string
	if err := pool.QueryRow(ctx, `SELECT quantity_on_hand::text FROM inventory
		WHERE outlet_id = $1 AND ingredient_id = $2`,
		f.outletID, f.ingredientID).Scan(&finalQty); err != nil {
		t.Fatalf("baca stok akhir: %v", err)
	}
	want := f.startQty.Sub(money.FromInt(n))
	got, err := money.Parse(finalQty)
	if err != nil {
		t.Fatalf("parse stok: %v", err)
	}
	if !got.Equal(want) {
		t.Fatalf("stok akhir = %s, mau %s (start %s - %d)", got, want, f.startQty, n)
	}

	var count int
	var sum string
	if err := pool.QueryRow(ctx, `
		SELECT count(*), COALESCE(SUM(quantity_delta), 0)::text
		FROM stock_movements
		WHERE reference_type = 'ORDER' AND reference_id = ANY($1::uuid[]) AND movement_type = 'SALE'`,
		orders).Scan(&count, &sum); err != nil {
		t.Fatalf("baca movement: %v", err)
	}
	if count != n {
		t.Fatalf("jumlah movement = %d, mau %d", count, n)
	}
	sumDec, err := money.Parse(sum)
	if err != nil {
		t.Fatalf("parse sum: %v", err)
	}
	if !sumDec.Equal(money.FromInt(-n)) {
		t.Fatalf("SUM(quantity_delta) = %s, mau -%d", sumDec, n)
	}
}

// TestConfirmIdempotent_OneMovement memastikan kunci yang sama mengembalikan
// hasil sama dan hanya memotong stok satu kali (AC-F04-2).
func TestConfirmIdempotent_OneMovement(t *testing.T) {
	pool := testPool(t)
	f := seedStockFixture(t, pool)
	svc := NewService(NewRepository(pool))
	actor := auth.Actor{OutletID: f.outletID, UserID: f.userID}

	orderID := newDraft(t, pool, f, 1)
	key := "idem-" + orderID
	t.Cleanup(func() { cleanupOrders(t, pool, f, []string{orderID}, "idem-") })

	first, err := svc.Confirm(context.Background(), actor, orderID, key)
	if err != nil {
		t.Fatalf("confirm pertama: %v", err)
	}
	second, err := svc.Confirm(context.Background(), actor, orderID, key)
	if err != nil {
		t.Fatalf("confirm kedua: %v", err)
	}
	if first.OrderNo == nil || second.OrderNo == nil || *first.OrderNo != *second.OrderNo {
		t.Fatalf("respons berbeda: %v vs %v", first.OrderNo, second.OrderNo)
	}
	if first.TotalAmount != second.TotalAmount {
		t.Fatalf("total berbeda: %s vs %s", first.TotalAmount, second.TotalAmount)
	}

	var movements int
	if err := pool.QueryRow(context.Background(), `
		SELECT count(*) FROM stock_movements
		WHERE reference_type = 'ORDER' AND reference_id = $1 AND movement_type = 'SALE'`,
		orderID).Scan(&movements); err != nil {
		t.Fatalf("hitung movement: %v", err)
	}
	if movements != 1 {
		t.Fatalf("movement = %d, mau 1", movements)
	}
}