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
// Baris pesanan tidak dipakai untuk stok, cukup untuk menguji serialisasi.
func seedDraft(t *testing.T, pool *pgxpool.Pool) (orderID string) {
	t.Helper()
	ctx := context.Background()

	var outletID, productID, variantID string
	err := pool.QueryRow(ctx, `
		SELECT o.id, p.id, v.id
		FROM outlets o
		JOIN products p ON p.business_id = o.business_id AND p.is_active
		JOIN product_variants v ON v.product_id = p.id
		LIMIT 1`).Scan(&outletID, &productID, &variantID)
	if err != nil {
		t.Fatalf("ambil data seed: %v", err)
	}

	err = pool.QueryRow(ctx, `
		INSERT INTO orders (outlet_id, service_type, status, payment_timing, subtotal_amount, total_amount)
		VALUES ($1, 'TAKEAWAY', 'DRAFT', 'PAY_FIRST', 0, 0)
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
		_, _ = pool.Exec(context.Background(), `DELETE FROM order_items WHERE order_id = $1`, orderID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM orders WHERE id = $1`, orderID)
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
