package main

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/ryunoo09/pos-food-beverage/backend/internal/config"
	"github.com/ryunoo09/pos-food-beverage/backend/internal/sse"
)

// Uji matriks permission lintas endpoint (plan 10.4): setiap route /api/v1
// wajib terautentikasi dan mendeklarasikan permission (default deny), kecuali
// login/refresh. Sumber kebenaran route adalah main.go, jadi route baru yang
// lupa diberi pengaman akan menggagalkan test ini.
var routePattern = regexp.MustCompile(`mux\.Handle(?:Func)?\("([A-Z]+) (/[^"]+)"`)

var permPattern = regexp.MustCompile(`perm\("([^"]+)"\)`)

// Route publik: tidak butuh token. Selain ini harus 401 tanpa token.
var publicRoutes = map[string]bool{
	"POST /api/v1/auth/login":   true,
	"POST /api/v1/auth/refresh": true,
}

// Route terautentikasi tanpa permission khusus (sesi milik sendiri).
var authOnlyRoutes = map[string]bool{
	"POST /api/v1/auth/logout":          true,
	"POST /api/v1/auth/logout-all":      true,
	"GET /api/v1/auth/me":               true,
	"POST /api/v1/auth/sse-token":       true,
	"POST /api/v1/auth/change-password": true,
}

// Tanpa token, login tidak boleh 401; refresh memang 401 bila cookie absen.
var publicNon401 = map[string]bool{
	"POST /api/v1/auth/login": true,
}

func TestRoutesRequireAuthAndPermission(t *testing.T) {
	source := readMainGo(t)

	for _, line := range strings.Split(source, "\n") {
		m := routePattern.FindStringSubmatch(line)
		if m == nil || !strings.HasPrefix(m[2], "/api/v1/") {
			continue
		}
		route := m[1] + " " + m[2]
		if publicRoutes[route] {
			continue
		}
		if !strings.Contains(line, "uthed(") {
			t.Errorf("route %s tidak memakai middleware autentikasi", route)
		}
		if !strings.Contains(line, "perm(") && !authOnlyRoutes[route] {
			t.Errorf("route %s tidak mendeklarasikan permission", route)
		}
	}
}

func TestRoutePermissionsAreSeeded(t *testing.T) {
	seeded := seededPermissions(t)

	for _, code := range permPattern.FindAllStringSubmatch(readMainGo(t), -1) {
		if !seeded[code[1]] {
			t.Errorf("permission %q dipakai route tapi tidak ada di migrasi seed", code[1])
		}
	}
}

// TestRoutesReturn401WithoutToken memastikan pengaman benar-benar terpasang di
// router, bukan hanya ada di teks. Pool diarahkan ke port tertutup: auth gagal
// sebelum handler menyentuh DB.
func TestRoutesReturn401WithoutToken(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(&discardWriter{}, nil))
	cfg := config.Config{JWTSecret: "test-secret", AccessTTL: time.Minute, RefreshTTL: time.Hour}
	router := newRouter(unreachablePool(t), logger, cfg, sse.NewHub())

	const uuid = "00000000-0000-0000-0000-000000000000"
	placeholder := regexp.MustCompile(`\{[^}]+\}`)

	for _, line := range strings.Split(readMainGo(t), "\n") {
		m := routePattern.FindStringSubmatch(line)
		if m == nil || !strings.HasPrefix(m[2], "/api/v1/") {
			continue
		}
		route := m[1] + " " + m[2]
		path := placeholder.ReplaceAllString(m[2], uuid)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(m[1], path, nil))

		if publicRoutes[route] {
			if publicNon401[route] && rec.Code == http.StatusUnauthorized {
				t.Errorf("route publik %s menolak tanpa token", route)
			}
			continue
		}
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("route %s tanpa token = %d, mau 401", route, rec.Code)
		}
	}
}

func readMainGo(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("baca main.go: %v", err)
	}
	return string(raw)
}

// seededPermissions membaca semua migrasi up agar kode permission yang
// ditambahkan belakangan tetap terhitung.
func seededPermissions(t *testing.T) map[string]bool {
	t.Helper()
	files, err := filepath.Glob(filepath.Join("..", "..", "internal", "db", "migrations", "*.up.sql"))
	if err != nil || len(files) == 0 {
		t.Fatalf("cari migrasi: %v (dapat %d file)", err, len(files))
	}

	seeded := map[string]bool{}
	quoted := regexp.MustCompile(`'([a-z_]+(?:\.[a-z_]+)+)'`)
	for _, file := range files {
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("baca %s: %v", file, err)
		}
		for _, m := range quoted.FindAllStringSubmatch(string(raw), -1) {
			seeded[m[1]] = true
		}
	}
	return seeded
}