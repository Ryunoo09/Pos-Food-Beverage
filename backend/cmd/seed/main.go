// Command seed mengisi data dev: satu bisnis, satu outlet, empat role default,
// dan akun dummy untuk login. Aman dijalankan berulang (idempotent): baris
// dicari dulu, lalu di-upsert, dan password di-reset ke nilai di bawah.
//
// Jalankan dari folder backend/: go run ./cmd/seed
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ryunoo09/pos-food-beverage/backend/internal/auth"
	"github.com/ryunoo09/pos-food-beverage/backend/internal/config"
	"github.com/ryunoo09/pos-food-beverage/backend/internal/db"
)

const (
	businessName = "Food POS Demo"
	outletCode   = "MAIN"
	outletName   = "Outlet Utama"
)

type roleSpec struct {
	code        string
	name        string
	description string
	// all = semua permission; bila false, pakai permissions dikurangi except.
	all         bool
	permissions []string
	except      []string
}

type accountSpec struct {
	username string
	fullName string
	password string
	role     string
}

// Role default mengikuti matriks PRD I.3. CASHIER dan KITCHEN sengaja dibatasi;
// OWNER memegang seluruh permission.
var roles = []roleSpec{
	{code: "OWNER", name: "Pemilik", description: "Akses penuh termasuk backup dan role.", all: true},
	{
		code:        "MANAGER",
		name:        "Manajer",
		description: "Operasional penuh, tanpa backup dan tanpa kelola role.",
		all:         true,
		except: []string{
			"backups.view", "backups.create", "backups.download", "backups.restore",
			"roles.manage", "users.create", "users.deactivate",
		},
	},
	{
		code:        "CASHIER",
		name:        "Kasir",
		description: "Buka shift, buat pesanan, terima pembayaran.",
		permissions: []string{
			"auth.sessions.view_own", "auth.sessions.revoke_own",
			"dashboard.view", "products.view", "tables.view",
			"orders.view", "orders.create", "orders.update_draft", "orders.confirm",
			"orders.apply_discount", "orders.void_item", "orders.cancel",
			"orders.reprint_receipt",
			"payments.create", "payments.view",
			"shifts.open", "shifts.close", "cash_movements.create",
		},
	},
	{
		code:        "KITCHEN",
		name:        "Dapur",
		description: "Layar dapur: lihat dan ubah status pesanan.",
		permissions: []string{
			"auth.sessions.view_own", "auth.sessions.revoke_own",
			"orders.view", "products.view",
			"kitchen.orders.view", "kitchen.orders.update_status", "kitchen.orders.set_priority",
		},
	},
}

var accounts = []accountSpec{
	{username: "owner", fullName: "Owner Demo", password: "Owner#Kopi2026", role: "OWNER"},
	{username: "manager", fullName: "Manager Demo", password: "Manager#Kopi2026", role: "MANAGER"},
	{username: "kasir", fullName: "Kasir Demo", password: "Kasir#Kopi2026", role: "CASHIER"},
	{username: "dapur", fullName: "Dapur Demo", password: "Dapur#Kopi2026", role: "KITCHEN"},
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cfg, err := config.Load()
	if err != nil {
		fail("load config: %v", err)
	}

	pool, err := db.New(ctx, cfg.DatabaseURL)
	if err != nil {
		fail("koneksi database: %v", err)
	}
	defer pool.Close()

	if err := seed(ctx, pool); err != nil {
		fail("seed: %v", err)
	}

	fmt.Println("\nAkun dev (password di-reset setiap seed dijalankan):")
	fmt.Printf("  %-9s %-14s %s\n", "USERNAME", "ROLE", "PASSWORD")
	for _, account := range accounts {
		fmt.Printf("  %-9s %-14s %s\n", account.username, account.role, account.password)
	}
	fmt.Println("\nContoh: owner / Owner#Kopi2026")
}

func seed(ctx context.Context, pool *pgxpool.Pool) error {
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback setelah commit tidak berefek

	businessID, err := ensureBusiness(ctx, tx)
	if err != nil {
		return err
	}
	outletID, err := ensureOutlet(ctx, tx, businessID)
	if err != nil {
		return err
	}

	if _, err := tx.Exec(ctx,
		`INSERT INTO outlet_settings (outlet_id) VALUES ($1) ON CONFLICT DO NOTHING`, outletID); err != nil {
		return fmt.Errorf("outlet settings: %w", err)
	}

	roleIDs := make(map[string]string, len(roles))
	for _, role := range roles {
		roleID, err := upsertRole(ctx, tx, businessID, role)
		if err != nil {
			return err
		}
		roleIDs[role.code] = roleID
		if err := replaceRolePermissions(ctx, tx, roleID, role); err != nil {
			return fmt.Errorf("permission role %s: %w", role.code, err)
		}
	}

	for _, account := range accounts {
		roleID, ok := roleIDs[account.role]
		if !ok {
			return fmt.Errorf("role %s tidak dikenal", account.role)
		}
		hash, err := auth.HashPassword(account.password)
		if err != nil {
			return fmt.Errorf("hash %s: %w", account.username, err)
		}
		if err := upsertUser(ctx, tx, businessID, roleID, account, hash); err != nil {
			return err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

// ensureBusiness memakai bisnis pertama bila sudah ada, supaya seed tidak
// membuat bisnis kedua di database yang sudah berjalan.
func ensureBusiness(ctx context.Context, tx pgx.Tx) (string, error) {
	var id string
	err := tx.QueryRow(ctx, `SELECT id FROM businesses ORDER BY created_at, id LIMIT 1`).Scan(&id)
	if err == nil {
		return id, nil
	}
	if err != pgx.ErrNoRows {
		return "", fmt.Errorf("cari bisnis: %w", err)
	}

	err = tx.QueryRow(ctx,
		`INSERT INTO businesses (name, phone, address) VALUES ($1, $2, $3) RETURNING id`,
		businessName, "0812-0000-0000", "Jl. Contoh No. 1, Jakarta").Scan(&id)
	if err != nil {
		return "", fmt.Errorf("buat bisnis: %w", err)
	}
	return id, nil
}

func ensureOutlet(ctx context.Context, tx pgx.Tx, businessID string) (string, error) {
	var id string
	err := tx.QueryRow(ctx,
		`SELECT id FROM outlets WHERE business_id = $1 ORDER BY is_default DESC, created_at LIMIT 1`,
		businessID).Scan(&id)
	if err == nil {
		return id, nil
	}
	if err != pgx.ErrNoRows {
		return "", fmt.Errorf("cari outlet: %w", err)
	}

	err = tx.QueryRow(ctx,
		`INSERT INTO outlets (business_id, code, name, is_default) VALUES ($1, $2, $3, true) RETURNING id`,
		businessID, outletCode, outletName).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("buat outlet: %w", err)
	}
	return id, nil
}

func upsertRole(ctx context.Context, tx pgx.Tx, businessID string, role roleSpec) (string, error) {
	var id string
	err := tx.QueryRow(ctx,
		`INSERT INTO roles (business_id, code, name, description, is_system)
		 VALUES ($1, $2, $3, $4, true)
		 ON CONFLICT (business_id, code)
		 DO UPDATE SET name = EXCLUDED.name, description = EXCLUDED.description, is_system = true
		 RETURNING id`,
		businessID, role.code, role.name, role.description).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("upsert role %s: %w", role.code, err)
	}
	return id, nil
}

func replaceRolePermissions(ctx context.Context, tx pgx.Tx, roleID string, role roleSpec) error {
	if _, err := tx.Exec(ctx, `DELETE FROM role_permissions WHERE role_id = $1`, roleID); err != nil {
		return err
	}

	if role.all {
		_, err := tx.Exec(ctx,
			`INSERT INTO role_permissions (role_id, permission_id)
			 SELECT $1, id FROM permissions
			 WHERE NOT (code = ANY(coalesce($2::text[], '{}')))`, roleID, role.except)
		return err
	}

	_, err := tx.Exec(ctx,
		`INSERT INTO role_permissions (role_id, permission_id)
		 SELECT $1, id FROM permissions
		 WHERE code = ANY(coalesce($2::text[], '{}'))`, roleID, role.permissions)
	if err != nil {
		return err
	}

	// Kode permission yang salah ketik akan diam-diam menghasilkan 0 baris.
	var inserted int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM role_permissions WHERE role_id = $1`, roleID).Scan(&inserted); err != nil {
		return err
	}
	if inserted != len(role.permissions) {
		return fmt.Errorf("hanya %d dari %d permission terpasang, cek kode permission", inserted, len(role.permissions))
	}
	return nil
}

func upsertUser(ctx context.Context, tx pgx.Tx, businessID, roleID string, account accountSpec, hash string) error {
	var userID string
	err := tx.QueryRow(ctx,
		`INSERT INTO users (business_id, username, full_name, password_hash, must_change_password)
		 VALUES ($1, $2, $3, $4, false)
		 ON CONFLICT (business_id, username) DO UPDATE SET
		   full_name = EXCLUDED.full_name,
		   password_hash = EXCLUDED.password_hash,
		   must_change_password = false,
		   is_active = true,
		   failed_login_count = 0,
		   locked_until = NULL,
		   updated_at = now()
		 RETURNING id`,
		businessID, account.username, account.fullName, hash).Scan(&userID)
	if err != nil {
		return fmt.Errorf("upsert user %s: %w", account.username, err)
	}

	_, err = tx.Exec(ctx,
		`INSERT INTO user_roles (user_id, role_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, userID, roleID)
	if err != nil {
		return fmt.Errorf("role user %s: %w", account.username, err)
	}
	return nil
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "seed gagal: "+format+"\n", args...)
	os.Exit(1)
}
