package payment

// Uji idempotensi pembayaran + kembalian tunai (plan 10.1 F14). Butuh DB:
//
//	$env:PAYMENT_TEST_DATABASE_URL="postgres://postgres:postgres@localhost:5432/pos_food_test"
//	go test -count=1 ./internal/payment/ -run 'TestPayment' -v

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ryunoo09/pos-food-beverage/backend/internal/audit"
	"github.com/ryunoo09/pos-food-beverage/backend/internal/auth"
)

func paymentPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("PAYMENT_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("PAYMENT_TEST_DATABASE_URL tidak diisi; lewati uji pembayaran")
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatalf("buka pool: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// seedPayableOrder membuat shift OPEN, memastikan metode CASH ada, lalu membuat
// pesanan CONFIRMED dengan total 20.000.
func seedPayableOrder(t *testing.T, pool *pgxpool.Pool) (orderID, shiftID, outletID, userID string) {
	t.Helper()
	ctx := context.Background()

	err := pool.QueryRow(ctx, `
		SELECT u.id, o.id
		FROM users u
		JOIN outlets o ON o.business_id = u.business_id
		WHERE NOT EXISTS (
			SELECT 1 FROM shifts s WHERE s.user_id = u.id AND s.status = 'OPEN')
		LIMIT 1`).Scan(&userID, &outletID)
	if err != nil {
		t.Fatalf("cari user: %v", err)
	}

	if err := pool.QueryRow(ctx, `
		INSERT INTO shifts (outlet_id, user_id, status, opening_cash)
		VALUES ($1, $2, 'OPEN', 0) RETURNING id`, outletID, userID).Scan(&shiftID); err != nil {
		t.Fatalf("buat shift: %v", err)
	}

	if _, err := pool.Exec(ctx, `
		INSERT INTO payment_methods (outlet_id, code, name, is_cash)
		VALUES ($1, 'CASH', 'Tunai', true)
		ON CONFLICT (outlet_id, code) DO NOTHING`, outletID); err != nil {
		t.Fatalf("siapkan metode bayar: %v", err)
	}

	err = pool.QueryRow(ctx, `
		INSERT INTO orders (outlet_id, service_type, status, payment_timing, payment_status,
		                    subtotal_amount, total_amount)
		VALUES ($1, 'TAKEAWAY', 'CONFIRMED', 'PAY_FIRST', 'UNPAID', 20000, 20000)
		RETURNING id`, outletID).Scan(&orderID)
	if err != nil {
		t.Fatalf("buat pesanan: %v", err)
	}

	t.Cleanup(func() {
		ctx := context.Background()
		pool.Exec(ctx, `DELETE FROM payments WHERE order_id = $1`, orderID)
		pool.Exec(ctx, `DELETE FROM orders WHERE id = $1`, orderID)
		pool.Exec(ctx, `DELETE FROM idempotency_keys WHERE key LIKE $1`, "payment-idem-%")
		pool.Exec(ctx, `DELETE FROM shifts WHERE id = $1`, shiftID)
	})
	return orderID, shiftID, outletID, userID
}

// TestPaymentIdempotent_AndCashChange memastikan kunci yang sama tidak mencatat
// pembayaran ganda, dan kembalian tunai dihitung server (AC-F14-1, AC-F14-2).
func TestPaymentIdempotent_AndCashChange(t *testing.T) {
	pool := paymentPool(t)
	orderID, _, outletID, userID := seedPayableOrder(t, pool)
	svc := NewService(NewRepository(pool), audit.NewWriter(pool))
	actor := auth.Actor{OutletID: outletID, UserID: userID}

	cash := "50000"
	in := CreateInput{MethodCode: "CASH", Amount: "20000", CashReceived: &cash}
	key := "payment-idem-" + orderID

	first, err := svc.Create(context.Background(), actor, orderID, key, in)
	if err != nil {
		t.Fatalf("bayar pertama: %v", err)
	}
	second, err := svc.Create(context.Background(), actor, orderID, key, in)
	if err != nil {
		t.Fatalf("bayar kedua: %v", err)
	}

	if first.Payment.ID != second.Payment.ID {
		t.Fatalf("payment id berbeda: %s vs %s", first.Payment.ID, second.Payment.ID)
	}
	if first.Payment.ChangeAmount == nil || *first.Payment.ChangeAmount != "30000.00" {
		t.Fatalf("kembalian = %v, mau 30000.00", first.Payment.ChangeAmount)
	}
	if first.Order.PaymentStatus != "PAID" {
		t.Fatalf("status bayar = %s, mau PAID", first.Order.PaymentStatus)
	}

	var count int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM payments WHERE order_id = $1`, orderID).Scan(&count); err != nil {
		t.Fatalf("hitung pembayaran: %v", err)
	}
	if count != 1 {
		t.Fatalf("baris pembayaran = %d, mau 1", count)
	}
}

// TestPayment_SameKeyDifferentPayload_Conflict memastikan kunci sama dengan
// nominal berbeda ditolak 409, bukan memproses pembayaran kedua.
func TestPayment_SameKeyDifferentPayload_Conflict(t *testing.T) {
	pool := paymentPool(t)
	orderID, _, outletID, userID := seedPayableOrder(t, pool)
	svc := NewService(NewRepository(pool), audit.NewWriter(pool))
	actor := auth.Actor{OutletID: outletID, UserID: userID}
	key := "payment-idem-conflict-" + orderID

	if _, err := svc.Create(context.Background(), actor, orderID, key,
		CreateInput{MethodCode: "CASH", Amount: "10000"}); err != nil {
		t.Fatalf("bayar pertama: %v", err)
	}
	_, err := svc.Create(context.Background(), actor, orderID, key,
		CreateInput{MethodCode: "CASH", Amount: "11000"})
	if err == nil {
		t.Fatal("payload berbeda dengan kunci sama diterima")
	}
}