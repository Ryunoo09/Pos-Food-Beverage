# Food POS UMKM

Aplikasi point-of-sale untuk restoran dan kedai skala UMKM: kasir, layar dapur (KDS) realtime, denah meja, resep, stok bahan baku, opname, pembelian, shift kas, pembayaran, laporan, audit, dan backup.

Backend Go melayani REST API dan SSE. Frontend React melayani layar kasir, KDS, dan halaman admin. Database PostgreSQL 16. Satu VPS, satu port publik lewat Nginx.

## Fitur Utama

- **POS / kasir**: pesanan `DINE_IN`, `TAKEAWAY`, `PICKUP`, `DELIVERY`; varian, modifier, diskon item dan pesanan; hitung total di backend.
- **KDS realtime**: daftar pesanan dapur, prioritas, ubah status item, stream SSE per outlet.
- **Menu**: kategori, produk, varian, modifier, riwayat harga, ketersediaan.
- **Resep dan stok**: pemakaian bahan per varian, pemotongan stok atomik saat konfirmasi, stok negatif opsional dengan penanda kekurangan.
- **Opname**: sesi hitung fisik, submit, approve/reject, koreksi lewat movement.
- **Pembelian**: purchase order, approval, penerimaan, pembelian langsung, biaya rata-rata bergerak (MWAC).
- **Supplier dan waste**: data supplier, relasi bahan, pembuangan bahan dengan reversal.
- **Shift dan kas**: buka/tutup shift, uang awal, kas masuk/keluar, rekap selisih.
- **Pembayaran dan refund**: multi-metode, void pembayaran, refund dengan approval.
- **Laporan dan ekspor**: penjualan, keuangan/HPP, stok, performa menu, shift; ekspor CSV.
- **Audit dan backup**: `audit_logs` append-only, backup terjadwal terenkripsi.

## Arsitektur

```text
Browser (React SPA + EventSource)
      |
    Nginx (port 80)
    /            \
frontend        /api -> backend Go (REST + SSE) -> PostgreSQL 16
(static)
```

- **Modular monolith**: satu binary backend, modul internal terpisah per domain (`internal/order`, `internal/inventory`, dan seterusnya). Alasannya: konfirmasi pesanan harus memotong stok, membuat kitchen order, dan mengubah status meja dalam satu transaksi DB. Memecah ke service terpisah menambah kompleksitas tanpa manfaat untuk satu VPS.
- **SSE per outlet**: backend menyimpan klien dan ring buffer event di memori. Event dipublikasikan setelah commit DB.
- **Frontend dan backend terpisah**: frontend hanya menampilkan dan memanggil API; semua kalkulasi uang dan aturan bisnis ada di backend.

### Alur konfirmasi pesanan

`POST /api/v1/orders/{id}/confirm`:

1. Wajib `Idempotency-Key`. Kunci disimpan dengan scope `outlet:user:endpoint`, TTL 24 jam.
2. Transaksi DB dibuka, baris pesanan dikunci (`SELECT ... FOR UPDATE`) lalu status `DRAFT` diperiksa ulang.
3. Total dihitung ulang di server (harga, diskon, service, pajak, pembulatan).
4. Baris inventory dikunci urut `ingredient_id ASC`, stok bahan dipotong sebagai `stock_movements` tipe `SALE`; HPP ikut dihitung.
5. Kitchen order dibuat, status meja menjadi `OCCUPIED` (dine-in), nomor pesanan diambil dari `document_sequences`.
6. Response disimpan ke `idempotency_keys`. Permintaan ulang dengan kunci dan payload sama mengembalikan response yang sama, tanpa memotong stok dua kali.

## Tech Stack

| Bagian | Teknologi |
|---|---|
| Backend | Go 1.26 (`backend/go.mod`), `net/http`, `pgx/v5`, JWT HS256, Argon2id |
| Frontend | React 19.3, TypeScript 6.0, Vite 8.3, Tailwind CSS 4.3, React Router 7, TanStack Query 5, Zustand 5, Zod 4 |
| Database | PostgreSQL 16 |
| Infra | Docker Compose, Nginx, satu VPS |
| CI | GitHub Actions: build, vet, migrasi, seed, test, golangci-lint, type-check, lint, unit test, build frontend |

## Struktur Repo

```text
Pos-Food-Beverage/
├── backend/
│   ├── cmd/server/        # binary API
│   ├── cmd/seed/          # bisnis, outlet, role, akun demo
│   ├── cmd/seed-domain/   # menu, bahan, supplier, meja, metode bayar
│   ├── internal/          # modul domain + db/migrations
│   ├── Dockerfile
│   ├── Makefile
│   └── go.mod             # module github.com/ryunoo09/pos-food-beverage/backend
├── frontend/
│   ├── src/            # api/, components/, features/, hooks/, store/, types/, utils/
│   ├── Dockerfile
│   └── package.json
├── nginx/nginx.conf
├── docker-compose.yml
├── docker-compose.prod.yml
├── .env.example
└── .github/workflows/ci.yml
```

Folder `docs/` berisi PRD, plan, dan runbook internal. Folder itu tidak di-commit, jadi README ini dibuat mandiri.

## Modul Backend

| Modul | Tanggung jawab |
|---|---|
| `auth` | Login, refresh, logout, sesi, riwayat login, ganti password, token SSE |
| `middleware` | `RequestID`, `Recover`, `AccessLog`, `AuthRequired`, `PermissionRequired`, `SSEAuthRequired` |
| `dashboard` | Ringkasan penjualan, transaksi, menu terlaris, stok rendah |
| `menu` | Kategori, produk, varian, modifier, riwayat harga, ketersediaan |
| `recipe` | Resep per varian (berversi), komponen bahan, biaya resep |
| `order` | Draft, item, konfirmasi, void item, cancel, pindah meja, struk |
| `pos` | Preview total keranjang tanpa menyimpan pesanan |
| `table` | Denah meja dan status meja |
| `kitchen` | Daftar KDS, riwayat, status item, prioritas |
| `sse` | Hub per outlet, ring buffer, replay `Last-Event-ID`, ping 30 detik |
| `inventory` | Bahan, satuan, kategori bahan, saldo stok, movement, adjustment |
| `opname` | Sesi opname: buat, mulai, hitung fisik, submit, approve/reject |
| `purchase` | Purchase order, approval, penerimaan, pembelian langsung, MWAC |
| `supplier` | Data supplier dan relasi bahan |
| `waste` | Pencatatan bahan/menu terbuang dan reversal |
| `payment` | Pembayaran, void, refund, approval refund |
| `discount` | Master diskon (persen/nominal), aktivasi |
| `shift` | Buka/tutup shift, kas masuk/keluar, rekap kas |
| `expense` | Pengeluaran operasional dan kategori pengeluaran |
| `report` | Laporan penjualan, keuangan, stok, performa menu, shift, ekspor CSV |
| `audit` | Penulisan dan pembacaan `audit_logs`, ekspor |
| `backup` | `pg_dump` terjadwal, enkripsi, checksum, retensi, catatan backup |
| `settings` | Pengaturan outlet, pajak/service, metode pembayaran |
| `httpx`, `pkg/money`, `db` | Envelope response dan `AppError`, decimal uang, pool dan migrasi |

## Konvensi API

Semua endpoint di bawah prefix `/api/v1/`. Autentikasi memakai header `Authorization: Bearer <access_token>`. Health check publik di `GET /health`.

Envelope response (sukses, list, error):

```json
{ "data": { "id": "..." }, "meta": { "request_id": "..." } }
{ "data": [], "meta": { "page": 1, "per_page": 20, "total": 0, "total_pages": 0, "request_id": "..." } }
{ "error": { "code": "ORDER_NOT_FOUND", "message": "Pesanan tidak ditemukan." }, "meta": { "request_id": "..." } }
```

Status code yang dipakai: `200`, `201`, `204`, `400` validasi, `401` token tidak ada/invalid, `403` permission kurang, `404` tidak ditemukan, `409` konflik state/idempotensi, `422` pelanggaran aturan bisnis, `429` lockout, `500` internal.

### Permission default-deny

Setiap route wajib memakai `AuthRequired` dan mendeklarasikan permission lewat `PermissionRequired(...)`, kecuali route publik (login, refresh) dan route sesi milik sendiri. `cmd/server/routes_test.go` memindai `main.go`: route baru tanpa autentikasi atau tanpa permission menggagalkan test. Permission diperiksa di service/middleware backend, bukan hanya disembunyikan di UI.

### Idempotency

Header `Idempotency-Key` wajib pada:

- `POST /api/v1/orders/{id}/confirm`
- `POST /api/v1/orders/{id}/payments`
- `POST /api/v1/payments/{id}/void`
- `POST /api/v1/orders/{id}/refunds`
- `POST /api/v1/shifts/{id}/close`

Kunci sama + payload sama mengembalikan response tersimpan. Kunci sama + payload beda menghasilkan `409 IDEMPOTENCY_CONFLICT`. Kunci hilang menghasilkan `400 IDEMPOTENCY_KEY_REQUIRED`. Retensi kunci 24 jam.

### Contoh endpoint per area

| Area | Endpoint |
|---|---|
| Auth | `POST /auth/login`, `POST /auth/refresh`, `POST /auth/sse-token` |
| POS / order | `POST /orders`, `POST /orders/{id}/items`, `POST /orders/{id}/confirm`, `POST /pos/calculate` |
| Pembayaran | `POST /orders/{id}/payments`, `POST /payments/{id}/void`, `POST /refunds/{id}/approve` |
| Dapur / SSE | `GET /kitchen/orders`, `PATCH /kitchen/items/{id}/status`, `GET /kitchen/stream` |
| Menu / stok | `GET /products`, `POST /products/{id}/variants`, `POST /inventory/{ingredientId}/adjust` |
| Opname / beli | `POST /stock-opnames/{id}/approve`, `POST /purchase-orders/{id}/receivings` |
| Shift / laporan | `POST /shifts/{id}/close`, `GET /reports/sales`, `GET /reports/{type}/export` |
| Audit / setting | `GET /audit-logs`, `GET /settings/outlet`, `POST /settings/taxes` |

## Realtime SSE

Endpoint: `GET /api/v1/kitchen/stream` (permission `kitchen.orders.view`).

- `EventSource` tidak bisa mengirim header. Klien meminta token pendek lewat `POST /api/v1/auth/sse-token` (butuh access token), lalu mengirim `?token=<sse_token>`. Token SSE berlaku 60 detik dan hanya untuk stream.
- `useSSE` di frontend mengambil token baru setiap koneksi, melakukan reconnect dengan backoff 1/2/4/8/15 detik, dan mengirim `?last_event_id=...` saat menyambung ulang.
- Hub menyimpan 100 event terakhir per outlet dengan TTL 5 menit. Saat reconnect, event setelah `Last-Event-ID` dikirim ulang. Jika ID tidak ada di buffer (server restart atau gap terlalu jauh), server mengirim `full_reload`; klien memuat ulang state lewat REST.
- Event yang dipakai: `kitchen.order.created`, `kitchen.item.status_changed`, `kitchen.item.voided`, `order.status_changed`, `order.payment_status_changed`, `table.status_changed`, `ping`, `full_reload`.
- Event pembayaran tidak dikirim ke subscriber tanpa permission `payments.view`.
- Nginx wajib `proxy_buffering off` dan `proxy_read_timeout` panjang pada lokasi stream. Konfigurasi ada di `nginx/nginx.conf`; tanpa itu KDS tidak menerima event realtime.

## Database

Migrasi ada di `backend/internal/db/migrations` (`NNN_nama.up.sql` / `.down.sql`).

Tabel inti:

| Kelompok | Tabel |
|---|---|
| Inti / auth | `businesses`, `outlets`, `users`, `roles`, `permissions`, `role_permissions`, `user_roles`, `refresh_tokens`, `login_histories` |
| Pengaturan | `outlet_settings`, `tax_configs`, `document_sequences` |
| Menu | `categories`, `products`, `product_variants`, `modifier_groups`, `modifier_options`, `price_histories` |
| Pesanan | `tables`, `orders`, `order_items`, `order_item_modifiers`, `order_item_consumptions`, `order_status_histories`, `order_item_events`, `discounts` |
| Dapur | `kitchen_orders`, `kitchen_order_items` |
| Stok | `units`, `ingredient_categories`, `ingredients`, `inventory`, `stock_movements`, `ingredient_cost_history` |
| Resep / beli | `recipes`, `recipe_items`, `suppliers`, `purchase_orders`, `purchase_receivings` |
| Kas / opname | `shifts`, `payments`, `refunds`, `cash_movements`, `stock_opnames`, `stock_opname_items`, `waste_records`, `expenses` |
| Audit / backup | `audit_logs`, `backup_records`, `idempotency_keys` |

Aturan penting:

- **Uang `NUMERIC`**, bukan float: `NUMERIC(14,2)` untuk nominal, `NUMERIC(18,4)` untuk kuantitas, `NUMERIC(18,6)` untuk unit cost.
- **`stock_movements` append-only**: trigger menolak `UPDATE` dan `DELETE`. Koreksi dilakukan dengan movement kebalikan (`SALE_REVERSAL`, `WASTE_REVERSAL`, `ADJUSTMENT`).
- **`audit_logs` append-only**: trigger menolak update/delete, dilengkapi `prev_hash`/`hash` per baris.
- **Multi-outlet**: tabel operasional menyimpan `outlet_id`; query selalu dibatasi outlet aktor.
- Partial unique index menjaga aturan: satu shift `OPEN` per kasir, satu pesanan aktif per meja, satu adjustment opname per bahan per sesi.
- Ambang approval, stok negatif, pajak, dan pembulatan diatur di `outlet_settings`.

## Setup Lokal

Prasyarat: Go 1.26+, Node.js 22+ dan npm, PostgreSQL 16 (atau Docker Compose), `golang-migrate` untuk target Makefile migrasi.

```bash
cp .env.example .env
# Isi DB_PASSWORD dan, untuk produksi, JWT_SECRET acak minimal 32 byte.
```

Backend membaca `.env` otomatis dari folder kerja lalu parent. Environment variable OS menang atas isi `.env`.

1. Siapkan database kosong, misalnya `pos_food_db`.
2. Jalankan migrasi:

```bash
cd backend
make migrate-up DATABASE_URL="postgres://postgres:postgres@localhost:5432/pos_food_db?sslmode=disable"
```

Tanpa `migrate` CLI, terapkan berurutan: `for f in internal/db/migrations/*.up.sql; do psql "postgres://postgres:postgres@localhost:5432/pos_food_db?sslmode=disable" -v ON_ERROR_STOP=1 -f "$f"; done`.

3. Isi data dan akun demo:

```bash
cd backend
go run ./cmd/seed          # bisnis, outlet, 4 role, akun
go run ./cmd/seed-domain   # kategori, menu, bahan, supplier, meja, metode bayar
```

`seed-domain` membutuhkan `seed` karena memakai bisnis/outlet yang sudah dibuat. Keduanya idempoten.

Akun demo: `owner` (OWNER), `manager` (MANAGER), `kasir` (CASHIER), `dapur` (KITCHEN). Password dicetak oleh `go run ./cmd/seed` di akhir output; seed mereset password setiap kali dijalankan. Jangan pakai akun ini di produksi.

4. Jalankan backend:

```bash
cd backend
go run ./cmd/server
curl http://localhost:8080/health
```

5. Jalankan frontend:

```bash
cd frontend
npm install
npm run dev
```

Frontend di `http://localhost:3000`, proxy `/api` ke `http://localhost:8080`. Alternatif semua service: `docker compose up`.

## Menjalankan Test

Backend:

```bash
cd backend
go build ./...
go vet ./...
go test -race -count=1 ./...
golangci-lint run
```

Test integrasi yang butuh database nyata dilewati bila env var-nya kosong:

| Env var | Cakupan |
|---|---|
| `ORDER_TEST_DATABASE_URL` | Konkurensi confirm, idempotensi, pemotongan stok |
| `PAYMENT_TEST_DATABASE_URL` | Idempotensi pembayaran, kembalian tunai |
| `SHIFT_TEST_DATABASE_URL` | Close shift konkuren |
| `OPNAME_TEST_DATABASE_URL` | Approve opname konkuren |
| `REPORT_TEST_DATABASE_URL` | Invariant laporan, benchmark, target p95 |

Contoh:

```bash
ORDER_TEST_DATABASE_URL="postgres://postgres:postgres@localhost:5432/pos_food_test?sslmode=disable" \
REPORT_TEST_DATABASE_URL="postgres://postgres:postgres@localhost:5432/pos_food_test?sslmode=disable" \
SHIFT_TEST_DATABASE_URL="postgres://postgres:postgres@localhost:5432/pos_food_test?sslmode=disable" \
PAYMENT_TEST_DATABASE_URL="postgres://postgres:postgres@localhost:5432/pos_food_test?sslmode=disable" \
OPNAME_TEST_DATABASE_URL="postgres://postgres:postgres@localhost:5432/pos_food_test?sslmode=disable" \
go test -race -count=1 -timeout=300s ./...
```

Contoh test spesifik: `go test -count=1 ./internal/order -run 'TestConcurrentConfirm|TestConfirmIdempotent' -v`.

Benchmark dan target performa:

```bash
REPORT_TEST_DATABASE_URL="postgres://postgres:postgres@localhost:5432/pos_food_test?sslmode=disable" \
go test ./internal/report -run '^$' -bench . -benchmem
```

Target yang ditegakkan test: laporan penjualan 31 hari / 15.000 pesanan p95 <= 5 detik; dashboard satu hari / sekitar 500 pesanan p95 <= 3 detik.

Cakupan uji penting:

- Matriks permission: semua route `/api/v1` wajib auth dan permission, permission yang dipakai harus ada di seed migrasi, request tanpa token harus `401`.
- Konkurensi: 100 goroutine confirm pada inventory yang sama; stok akhir dan jumlah movement harus tepat.
- Idempotensi: key sama hanya menghasilkan satu movement dan response sama. Close shift dan approve opname juga diuji konkuren.

Frontend:

```bash
cd frontend
npm run type-check
npm run lint
npm run test:unit
npm run build
```

CI `.github/workflows/ci.yml` menjalankan urutan di atas untuk backend dan frontend, termasuk migrasi dan seed ke PostgreSQL 16.

## Deployment

Target: satu VPS, Docker Compose, PostgreSQL 16 di jaringan internal.

```bash
cp .env.example .env
# APP_ENV=production, COOKIE_SECURE=true,
# JWT_SECRET acak >= 32 byte, BACKUP_ENCRYPTION_KEY >= 32 byte, DB_PASSWORD.
docker compose -f docker-compose.prod.yml up -d --build
curl -s http://localhost/health
```

- `docker-compose.prod.yml` hanya mengekspos port `80` dari Nginx. Database dan backend tidak punya port publik.
- Nginx meneruskan `/api/` ke backend, `/api/v1/kitchen/stream` tanpa buffering, dan sisanya ke container frontend.
- Migrasi dijalankan sekali sebelum backend melayani trafik (lihat runbook).
- Health container memakai `/server -healthcheck`; endpoint `GET /health` mengembalikan status DB, versi, dan uptime.

Backup:

- Aktif bila `BACKUP_DIR` diisi. Backend menjalankan `pg_dump` harian pada `BACKUP_SCHEDULE_HOUR` (default 02:00).
- Hasil dienkripsi AES-256-CBC dengan `BACKUP_ENCRYPTION_KEY` bila diisi, disertai file `.sha256`.
- Retensi harian `BACKUP_RETENTION_DAILY` (default 14 hari). Status dicatat di `backup_records`.
- Verifikasi dan restore memakai `make verify-backup` dan `make restore-test`; prosedur lengkap ada di `docs/RUNBOOK.md` bila folder `docs/` tersedia di mesin operasi. RPO <= 24 jam, RTO <= 4 jam. Simpan salinan off-site dan kunci enkripsi terpisah dari file backup.

## Skrip Makefile

Dijalankan dari `backend/`:

| Target | Fungsi |
|---|---|
| `make build` | Build binary ke `bin/server` |
| `make run` | Jalankan server dev |
| `make seed` | Isi bisnis, outlet, role, akun demo |
| `make test` | `go test -race -count=1 ./...` |
| `make lint` | `golangci-lint run` |
| `make migrate-up` | Terapkan semua migrasi (butuh `DATABASE_URL`) |
| `make migrate-down` | Rollback satu migrasi |
| `make migrate-create name=...` | Buat file migrasi baru |
| `make backup` | `pg_dump` terkompresi + checksum |
| `make backup-encrypted` | `pg_dump` + enkripsi AES-256-CBC + checksum |
| `make verify-backup FILE=...` | Verifikasi checksum |
| `make restore-test FILE=...` | Uji file backup bisa dibaca `pg_restore` |

## Batasan Saat Ini

- **TLS belum aktif.** Nginx produksi hanya `listen 80`. Sertifikat dan server block 443 harus ditambahkan manual sebelum dipakai di internet publik.
- **Belum ada E2E Playwright.** Pengujian UI masih manual; otomatisasi yang ada hanya unit test Vitest, type-check, dan lint.
- **Retensi backup mingguan/bulanan masih manual.** Scheduler hanya mengelola backup harian; penyalinan tier mingguan/bulanan dilakukan operator.
- **`business_day_cutoff` belum diterapkan pada penomoran pesanan.** Periode nomor pesanan memakai tanggal server (komentar `ponytail` di `internal/order/service.go`); begitu juga periode laporan opname dan pembelian.
- **Bundle frontend masih di atas 500 kB.** Build Vite memberi peringatan chunk besar; code splitting per halaman belum dikerjakan.
- **SSE hub berbasis memori.** Hanya cocok untuk satu instance backend. Restart server mengosongkan buffer sehingga klien menerima `full_reload`.