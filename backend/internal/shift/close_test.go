package shift

// Uji balapan tutup shift (plan 10.2): dua request paralel tidak boleh sama-sama
// menutup shift. Butuh database nyata:
//
//	$env:SHIFT_TEST_DATABASE_URL="postgres://postgres:postgres@localhost:5432/pos_food_test"
//	go test -count=1 ./internal/shift/ -run TestConcurrentClose -v

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ryunoo09/pos-food-beverage/backend/internal/audit"
	"github.com/ryunoo09/pos-food-beverage/backend/internal/auth"
	"github.com/ryunoo09/pos-food-beverage/backend/internal/httpx"
)

func shiftPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("SHIFT_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("SHIFT_TEST_DATABASE_URL tidak diisi; lewati uji balapan shift")
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatalf("buka pool: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// seedOpenShift membuat shift OPEN untuk user yang belum punya shift OPEN.
func seedOpenShift(t *testing.T, pool *pgxpool.Pool) (shiftID, outletID, userID string) {
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
		t.Fatalf("cari user tanpa shift OPEN: %v", err)
	}

	err = pool.QueryRow(ctx, `
		INSERT INTO shifts (outlet_id, user_id, status, opening_cash)
		VALUES ($1, $2, 'OPEN', 0)
		RETURNING id`, outletID, userID).Scan(&shiftID)
	if err != nil {
		t.Fatalf("buat shift: %v", err)
	}

	t.Cleanup(func() {
		ctx := context.Background()
		pool.Exec(ctx, `DELETE FROM cash_movements WHERE shift_id = $1`, shiftID)
		pool.Exec(ctx, `DELETE FROM idempotency_keys WHERE key LIKE $1`, "shift-close-%")
		pool.Exec(ctx, `DELETE FROM shifts WHERE id = $1`, shiftID)
	})
	return shiftID, outletID, userID
}

// TestConcurrentClose_OnlyOneSucceeds menjalankan dua tutup shift dengan kunci
// berbeda. Lock baris shifts harus membuat tepat satu berhasil; sisanya
// SHIFT_ALREADY_CLOSED, bukan dua penutupan.
func TestConcurrentClose_OnlyOneSucceeds(t *testing.T) {
	pool := shiftPool(t)
	shiftID, outletID, userID := seedOpenShift(t, pool)

	svc := NewService(NewRepository(pool), audit.NewWriter(pool))
	actor := auth.Actor{OutletID: outletID, UserID: userID}

	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = svc.Close(context.Background(), actor, shiftID,
				"shift-close-"+shiftID+"-"+string(rune('a'+i)),
				CloseInput{ActualCash: "0"})
		}(i)
	}
	wg.Wait()

	success := 0
	for _, err := range errs {
		if err == nil {
			success++
			continue
		}
		var appErr *httpx.AppError
		if !errors.As(err, &appErr) || appErr.Code != "SHIFT_ALREADY_CLOSED" {
			t.Fatalf("error tak terduga: %v", err)
		}
	}
	if success != 1 {
		t.Fatalf("shift tertutup %d kali, harus tepat 1", success)
	}

	var status string
	if err := pool.QueryRow(context.Background(),
		`SELECT status FROM shifts WHERE id = $1`, shiftID).Scan(&status); err != nil {
		t.Fatalf("baca shift: %v", err)
	}
	if status != "CLOSED" {
		t.Fatalf("status = %s, mau CLOSED", status)
	}
}