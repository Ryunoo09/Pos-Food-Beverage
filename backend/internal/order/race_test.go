package order

// Uji regresi balapan transaksi. Butuh database nyata, jadi test ini dilewati
// bila ORDER_TEST_DATABASE_URL kosong. Contoh menjalankan:
//
//	$env:ORDER_TEST_DATABASE_URL="postgres://postgres:postgres@localhost:5432/pos_food_db"
//	go test -count=1 ./internal/order/ -run TestRace -v
//
// Kedua test meniru bug yang pernah terjadi: dua request paralel yang
// seharusnya saling menunggu lewat kunci baris orders.

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ryunoo09/pos-food-beverage/backend/internal/auth"
)

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("ORDER_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("ORDER_TEST_DATABASE_URL tidak diisi; lewati uji balapan")
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatalf("buka pool: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// seedDraft membuat pesanan DRAFT minimal satu item dan mengembalikan id-nya.
// Produk dipilih yang tidak punya resep supaya confirm tidak menyentuh stok.
func seedDraft(t *testing.T, pool *pgxpool.Pool) (orderID string) {
	t.Helper()
	ctx := context.Background()

	var outletID, productID, variantID string
	err := pool.QueryRow(ctx, `
		SELECT o.id, p.id, v.id
		FROM outlets o
		JOIN products p ON p.business_id = o.business_id AND p.is_active
		JOIN product_variants v ON v.product_id = p.id
		WHERE p.stock_mode = 'TRACK_BY_RECIPE'
		  AND NOT EXISTS (
			SELECT 1 FROM recipes r
			JOIN recipe_items ri ON ri.recipe_id = r.id
			WHERE r.variant_id = v.id AND r.status = 'ACTIVE')
		LIMIT 1`).Scan(&outletID, &productID, &variantID)
	if err != nil {
		t.Fatalf("ambil data seed: %v", err)
	}

	err = pool.QueryRow(ctx, `
		INSERT INTO orders (outlet_id, service_type, status, payment_timing, subtotal_amount, total_amount, pricing_snapshot)
		VALUES ($1, 'TAKEAWAY', 'DRAFT', 'PAY_FIRST', 0, 0, '{}'::jsonb)
		RETURNING id`, outletID).Scan(&orderID)
	if err != nil {
		t.Fatalf("buat draft: %v", err)
	}

	_, err = pool.Exec(ctx, `
		INSERT INTO order_items (order_id, product_id, variant_id, product_name_snapshot,
		                         variant_name_snapshot, sku_snapshot, quantity, unit_price,
		                         modifiers_total, line_total)
		VALUES ($1, $2, $3, 'uji', 'uji', 'UJI', 1, 10000, 0, 10000)`,
		orderID, productID, variantID)
	if err != nil {
		t.Fatalf("tambah item: %v", err)
	}

	t.Cleanup(func() {
		ctx := context.Background()
		// kitchen_orders memakai ON DELETE RESTRICT, jadi dibersihkan lebih dulu.
		_, _ = pool.Exec(ctx, `DELETE FROM kitchen_order_items
			WHERE order_item_id IN (SELECT id FROM order_items WHERE order_id = $1)`, orderID)
		_, _ = pool.Exec(ctx, `DELETE FROM kitchen_orders WHERE order_id = $1`, orderID)
		_, _ = pool.Exec(ctx, `DELETE FROM order_status_histories WHERE order_id = $1`, orderID)
		_, _ = pool.Exec(ctx, `DELETE FROM order_items WHERE order_id = $1`, orderID)
		_, _ = pool.Exec(ctx, `DELETE FROM orders WHERE id = $1`, orderID)
	})
	return orderID
}

// confirmOnce meniru langkah awal Service.confirm: kunci baris, periksa DRAFT,
// lalu ubah status. Transaksi kedua harus melihat status terbaru dan gagal.
func confirmOnce(pool *pgxpool.Pool, orderID string) error {
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var status string
	if err := tx.QueryRow(ctx,
		`SELECT status FROM orders WHERE id = $1 FOR UPDATE`, orderID).Scan(&status); err != nil {
		return err
	}
	if status != "DRAFT" {
		return ErrNotDraft
	}
	if _, err := tx.Exec(ctx,
		`UPDATE orders SET status = 'CONFIRMED' WHERE id = $1`, orderID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// TestRaceConfirmTwice memastikan dua confirm paralel tidak sama-sama lolos.
func TestRaceConfirmTwice(t *testing.T) {
	pool := testPool(t)
	orderID := seedDraft(t, pool)

	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs[i] = confirmOnce(pool, orderID)
		}(i)
	}
	wg.Wait()

	okCount := 0
	for _, err := range errs {
		if err == nil {
			okCount++
		} else if !errors.Is(err, ErrNotDraft) {
			t.Fatalf("error tak terduga: %v", err)
		}
	}
	if okCount != 1 {
		t.Fatalf("confirm berhasil %d kali, harus tepat 1", okCount)
	}
}

// TestRaceAddItemAfterConfirm memastikan item tidak bisa masuk setelah confirm.
func TestRaceAddItemAfterConfirm(t *testing.T) {
	pool := testPool(t)
	orderID := seedDraft(t, pool)

	if err := confirmOnce(pool, orderID); err != nil {
		t.Fatalf("confirm: %v", err)
	}

	repo := NewRepository(pool)
	_, err := repo.AddItem(context.Background(), orderID, saleVariant{
		ProductID: "00000000-0000-0000-0000-000000000000",
		VariantID: "00000000-0000-0000-0000-000000000000",
		UnitPrice: "10000",
	}, ItemInput{Quantity: 1}, nil, "0", "10000")

	if !errors.Is(err, ErrNotDraft) {
		t.Fatalf("AddItem pada pesanan CONFIRMED = %v, harus ErrNotDraft", err)
	}
}

// TestRaceDeleteItemAfterConfirm memastikan item tidak bisa dihapus setelah
// confirm. Sebelum diperbaiki, DELETE hanya memakai guard status di klausa
// USING, tanpa kunci baris, sehingga bisa berjalan bersamaan dengan confirm dan
// meninggalkan potongan stok tanpa baris konsumsi (tidak bisa dibalik).
func TestRaceDeleteItemAfterConfirm(t *testing.T) {
	pool := testPool(t)
	orderID := seedDraft(t, pool)

	if err := confirmOnce(pool, orderID); err != nil {
		t.Fatalf("confirm: %v", err)
	}

	repo := NewRepository(pool)
	var itemID string
	if err := pool.QueryRow(context.Background(),
		`SELECT id FROM order_items WHERE order_id = $1 LIMIT 1`, orderID).Scan(&itemID); err != nil {
		t.Fatalf("ambil item: %v", err)
	}

	ok, err := repo.DeleteItem(context.Background(), orderID, itemID)
	if !errors.Is(err, ErrNotDraft) {
		t.Fatalf("DeleteItem pada pesanan CONFIRMED = %v, harus ErrNotDraft", err)
	}
	if ok {
		t.Fatal("DeleteItem melaporkan berhasil padahal pesanan sudah CONFIRMED")
	}

	var still int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM order_items WHERE id = $1`, itemID).Scan(&still); err != nil {
		t.Fatalf("hitung item: %v", err)
	}
	if still != 1 {
		t.Fatal("item terhapus dari pesanan yang sudah CONFIRMED")
	}
}

// TestSaveTotalsAfterConfirm memastikan recalc yang kalah balapan tidak bisa
// menimpa total pesanan yang sudah CONFIRMED (dan mungkin sudah dibayar).
func TestSaveTotalsAfterConfirm(t *testing.T) {
	pool := testPool(t)
	orderID := seedDraft(t, pool)
	ctx := context.Background()

	if err := confirmOnce(pool, orderID); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`UPDATE orders SET total_amount = 11111 WHERE id = $1`, orderID); err != nil {
		t.Fatalf("set total: %v", err)
	}

	repo := NewRepository(pool)
	if err := repo.SaveTotals(ctx, orderID, orderTotals{
		Subtotal: "99999", ItemDiscount: "0", OrderDiscount: "0",
		Service: "0", Tax: "0", Rounding: "0", Total: "99999",
	}); err != nil {
		t.Fatalf("SaveTotals: %v", err)
	}

	var total string
	if err := pool.QueryRow(ctx,
		`SELECT total_amount::text FROM orders WHERE id = $1`, orderID).Scan(&total); err != nil {
		t.Fatalf("baca total: %v", err)
	}
	if total != "11111.00" {
		t.Fatalf("total pesanan CONFIRMED berubah jadi %s, harus tetap 11111.00", total)
	}
}

// TestConfirmStoresTotalsForCurrentItems memastikan confirm menghitung total dari
// item yang benar-benar ada saat transaksi berjalan, bukan dari angka yang
// tersimpan sebelumnya. Sebelumnya confirm hanya menyalin total lama, sehingga
// perubahan item yang belum sempat menyimpan total membuat struk tidak cocok
// dengan isi pesanan.
func TestConfirmStoresTotalsForCurrentItems(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	orderID := seedDraft(t, pool)

	var itemID, outletID, userID string
	if err := pool.QueryRow(ctx,
		`SELECT id FROM order_items WHERE order_id = $1 LIMIT 1`, orderID).Scan(&itemID); err != nil {
		t.Fatalf("ambil item: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`SELECT outlet_id FROM orders WHERE id = $1`, orderID).Scan(&outletID); err != nil {
		t.Fatalf("ambil outlet: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT id FROM users LIMIT 1`).Scan(&userID); err != nil {
		t.Fatalf("ambil user: %v", err)
	}

	// Ubah jumlah tanpa menghitung ulang total: meniru permintaan ubah item yang
	// belum sampai menyimpan total saat confirm berjalan.
	repo := NewRepository(pool)
	ok, err := repo.UpdateItemQuantity(ctx, orderID, itemID, 3, nil)
	if err != nil || !ok {
		t.Fatalf("ubah jumlah: ok=%v err=%v", ok, err)
	}

	svc := NewService(repo)
	o, err := svc.Confirm(ctx, auth.Actor{OutletID: outletID, UserID: userID}, orderID, "uji-confirm-"+orderID)
	if err != nil {
		t.Fatalf("confirm: %v", err)
	}
	if o.Status != "CONFIRMED" {
		t.Fatalf("status = %s, harus CONFIRMED", o.Status)
	}

	// Harga satuan 10000 x 3 item.
	if o.SubtotalAmount != "30000.00" {
		t.Fatalf("subtotal = %s, harus 30000.00 (3 x 10000)", o.SubtotalAmount)
	}

	var snapshotTotal string
	if err := pool.QueryRow(ctx,
		`SELECT pricing_snapshot->>'total' FROM orders WHERE id = $1`, orderID).Scan(&snapshotTotal); err != nil {
		t.Fatalf("baca snapshot: %v", err)
	}
	if o.TotalAmount != snapshotTotal {
		t.Fatalf("total tersimpan %s berbeda dengan snapshot %s", o.TotalAmount, snapshotTotal)
	}
}
