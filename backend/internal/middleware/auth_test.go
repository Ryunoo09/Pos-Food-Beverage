package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ryunoo09/pos-food-beverage/backend/internal/auth"
)

// Menutup matriks permission dasar tanpa DB: 401 tanpa token, 403 tanpa
// permission, 200 saat permission dimiliki. Sesuai plan 10.4.
func TestPermissionRequired(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	tests := []struct {
		name string
		ctx  context.Context
		perm string
		want int
	}{
		{"tanpa aktor", context.Background(), "orders.view", http.StatusUnauthorized},
		{"tanpa permission", actorCtx(nil), "orders.view", http.StatusForbidden},
		{"permission lain", actorCtx([]string{"products.view"}), "orders.view", http.StatusForbidden},
		{"punya permission", actorCtx([]string{"orders.view"}), "orders.view", http.StatusNoContent},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(tc.ctx)
			PermissionRequired(tc.perm)(ok).ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("status = %d, mau %d", rec.Code, tc.want)
			}
		})
	}
}

func TestPermissionRequired_AllRequired(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil).
		WithContext(actorCtx([]string{"orders.view"}))
	PermissionRequired("orders.view", "orders.confirm")(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})).
		ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("satu permission kurang harus 403, dapat %d", rec.Code)
	}
}

func TestAuthRequired_MissingAndInvalidToken(t *testing.T) {
	svc := auth.NewService(auth.NewRepository(unreachablePool(t)), "secret", time.Minute, time.Hour)
	h := AuthRequired(svc, "secret")(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

	for name, header := range map[string]string{
		"tanpa token":     "",
		"token sampah":    "Bearer bukan.token.valid",
		"skema salah":     "Basic abc",
	} {
		t.Run(name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if header != "" {
				req.Header.Set("Authorization", header)
			}
			h.ServeHTTP(rec, req)
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, mau 401", rec.Code)
			}
		})
	}
}

// Token SSE hanya boleh membuka stream; endpoint biasa harus menolaknya.
func TestAuthRequired_RejectsSSEToken(t *testing.T) {
	svc := auth.NewService(auth.NewRepository(unreachablePool(t)), "secret", time.Minute, time.Hour)
	token, err := auth.NewSSEToken("secret", time.Minute, "user-1", "biz-1")
	if err != nil {
		t.Fatalf("buat token SSE: %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	AuthRequired(svc, "secret")(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})).
		ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("token SSE di endpoint biasa = %d, mau 401", rec.Code)
	}
}

func actorCtx(perms []string) context.Context {
	return auth.WithActor(context.Background(), auth.Actor{
		UserID:      "user-1",
		BusinessID:  "biz-1",
		OutletID:    "outlet-1",
		Permissions: perms,
	})
}

// Pool ke port tertutup: test ini tidak boleh menyentuh DB.
func unreachablePool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	cfg, err := pgxpool.ParseConfig("postgres://u:p@127.0.0.1:1/db?sslmode=disable&connect_timeout=1")
	if err != nil {
		t.Fatalf("parse config: %v", err)
	}
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatalf("buka pool: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}