package opname

// Uji persetujuan opname (plan 10.1 F10): pengaju tidak boleh menyetujui
// sesinya sendiri, dan dua approve paralel hanya menghasilkan satu adjustment.
// Butuh database nyata:
//
//	$env:OPNAME_TEST_DATABASE_URL="postgres://postgres:postgres@localhost:5432/pos_food_test"
//	go test -count=1 ./internal/opname/ -run TestApprove -v

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ryunoo09/pos-food-beverage/backend/internal/auth"
	"github.com/ryunoo09/pos-food-beverage/backend/internal/httpx"
	"github.com/ryunoo09/pos-food-beverage/backend/internal/pkg/money"
)

func opnamePool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("OPNAME_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("OPNAME_TEST_DATABASE_URL tidak diisi; lewati uji opname")
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatalf("buka pool: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// submittedOpname menyiapkan sesi opname satu bahan sampai status SUBMITTED.
// Mengembalikan id sesi, data aktor, dan jumlah sistem bahan.
func submittedOpname(t *testing.T, pool *pgxpool.Pool) (id string, submitter, approver auth.Actor, startQty money.Decimal, ingredientID string) {
	t.Helper()
	ctx := context.Background()

	var outletID, businessID string
	if err := pool.QueryRow(ctx,
		`SELECT id, business_id FROM outlets LIMIT 1`).Scan(&outletID, &businessID); err != nil {
		t.Fatalf("ambil outlet: %v", err)
	}

	rows, err := pool.Query(ctx, `
		SELECT id FROM users WHERE business_id = $1 ORDER BY username LIMIT 2`, businessID)
	if err != nil {
		t.Fatalf("ambil user: %v", err)
	}
	var userIDs []string
	for rows.Next() {
		var uid string
		if err := rows.Scan(&uid); err != nil {
			rows.Close()
			t.Fatalf("scan user: %v", err)
		}
		userIDs = append(userIDs, uid)
	}
	rows.Close()
	if len(userIDs) < 2 {
		t.Fatalf("butuh 2 user, dapat %d", len(userIDs))
	}
	submitter = auth.Actor{OutletID: outletID, BusinessID: businessID, UserID: userIDs[0]}
	approver = auth.Actor{OutletID: outletID, BusinessID: businessID, UserID: userIDs[1]}

	var start string
	err = pool.QueryRow(ctx, `
		SELECT i.id, inv.quantity_on_hand::text
		FROM ingredients i
		JOIN inventory inv ON inv.ingredient_id = i.id AND inv.outlet_id = $1
		WHERE i.business_id = $2 AND i.is_active
		LIMIT 1`, outletID, businessID).Scan(&ingredientID, &start)
	if err != nil {
		t.Fatalf("ambil bahan: %v", err)
	}
	startQty, err = money.Parse(start)
	if err != nil {
		t.Fatalf("parse stok: %v", err)
	}

	svc := NewService(NewRepository(pool))
	o, err := svc.Create(ctx, submitter, CreateInput{
		Scope:         "SELECTED",
		IngredientIDs: []string{ingredientID},
	})
	if err != nil {
		t.Fatalf("buat opname: %v", err)
	}
	if _, err := svc.Start(ctx, submitter, o.ID); err != nil {
		t.Fatalf("mulai opname: %v", err)
	}

	reason := "uji opname"
	counted := startQty.Add(money.FromInt(1))
	if _, err := svc.Count(ctx, submitter, o.ID, CountInput{Items: []CountItem{{
		IngredientID:    ingredientID,
		CountedQuantity: counted.StringFixed(4),
		Reason:          &reason,
	}}}); err != nil {
		t.Fatalf("hitung opname: %v", err)
	}
	if _, err := svc.Submit(ctx, submitter, o.ID); err != nil {
		t.Fatalf("ajukan opname: %v", err)
	}

	t.Cleanup(func() {
		ctx := context.Background()
		pool.Exec(ctx, `DELETE FROM stock_opname_items WHERE opname_id = $1`, o.ID)
		pool.Exec(ctx, `DELETE FROM stock_opnames WHERE id = $1`, o.ID)
		pool.Exec(ctx, `UPDATE inventory SET quantity_on_hand = $3
			WHERE outlet_id = $1 AND ingredient_id = $2`,
			outletID, ingredientID, startQty.StringFixed(4))
	})
	return o.ID, submitter, approver, startQty, ingredientID
}

func TestApprove_SelfApprovalForbidden(t *testing.T) {
	pool := opnamePool(t)
	id, submitter, _, _, _ := submittedOpname(t, pool)

	_, err := NewService(NewRepository(pool)).Approve(context.Background(), submitter, id)
	var appErr *httpx.AppError
	if !errors.As(err, &appErr) || appErr.Code != "OPNAME_SELF_APPROVAL" {
		t.Fatalf("self approval = %v, mau OPNAME_SELF_APPROVAL", err)
	}
}

// TestApprove_ConcurrentProducesOneAdjustment menjalankan dua approve paralel
// oleh pengguna berbeda. Hanya satu yang boleh menulis adjustment.
func TestApprove_ConcurrentProducesOneAdjustment(t *testing.T) {
	pool := opnamePool(t)
	id, _, approver, _, ingredientID := submittedOpname(t, pool)
	svc := NewService(NewRepository(pool))

	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = svc.Approve(context.Background(), approver, id)
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
		if !errors.As(err, &appErr) || appErr.Code != "OPNAME_NOT_APPROVABLE" {
			t.Fatalf("error tak terduga: %v", err)
		}
	}
	if success != 1 {
		t.Fatalf("approve berhasil %d kali, harus tepat 1", success)
	}

	var adjustments int
	if err := pool.QueryRow(context.Background(), `
		SELECT count(*) FROM stock_movements
		WHERE reference_type = 'STOCK_OPNAME' AND reference_id = $1
		  AND movement_type = 'ADJUSTMENT' AND ingredient_id = $2`,
		id, ingredientID).Scan(&adjustments); err != nil {
		t.Fatalf("hitung adjustment: %v", err)
	}
	if adjustments != 1 {
		t.Fatalf("adjustment = %d, mau 1", adjustments)
	}
}