# Food POS UMKM

Aplikasi Point of Sale untuk usaha makanan dan minuman skala UMKM: kasir, dapur, meja, resep, stok bahan baku, pembelian, shift kas, pengeluaran, dan laporan laba dalam satu sistem.

## Stack

| Bagian | Teknologi |
|---|---|
| Backend | Go 1.25, REST + SSE |
| Frontend | React 19, TypeScript, Vite, Tailwind |
| Database | PostgreSQL 16 |
| Infra | Docker Compose, Nginx |

## Struktur

```
backend/    Go API (cmd/server, internal/<modul>)
frontend/   React app (src/features/<modul>)
docs/       PRD, plan, runbook
```

## Setup Lokal

Butuh Go 1.25+, Node 22+, PostgreSQL 16 (atau Docker).

```bash
cp .env.example .env      # isi DB_PASSWORD
```

Backend membaca `.env` otomatis (`go run` dari `backend/` mencari `.env` di folder itu lalu parent). Environment variable OS selalu menang atas isi `.env`, jadi produksi bisa override tanpa mengubah file.

### Backend

```bash
cd backend
go mod download
go run ./cmd/server       # http://localhost:8080
curl http://localhost:8080/health
```

### Data dummy & akun login

Migrasi hanya mengisi `permissions` dan `units`; bisnis, outlet, role, dan user dibuat
oleh perintah seed. Aman dijalankan berulang (`make seed`): baris dicari dulu, password
di-reset ke nilai di bawah, dan role dikembalikan ke izin default.

```bash
cd backend
go run ./cmd/seed         # atau: make seed
```

| Username | Password | Role | Menu yang tampil |
|---|---|---|---|
| `owner` | `Owner#Kopi2026` | OWNER (107 izin) | semua |
| `manager` | `Manager#Kopi2026` | MANAGER (100 izin) | semua kecuali Backup & kelola Role |
| `kasir` | `Kasir#Kopi2026` | CASHIER (18 izin) | POS, Pesanan, Meja, Menu, Kategori |
| `dapur` | `Dapur#Kopi2026` | KITCHEN (7 izin) | Pesanan, Dapur, Menu, Kategori |

### Frontend

```bash
cd frontend
npm install
npm run dev               # http://localhost:3000, /api diproxy ke :8080
```

## Perintah

| Perintah | Fungsi |
|---|---|
| `cd backend && go test -race ./...` | Test backend |
| `cd backend && go run ./cmd/seed` | Isi data dev + akun login dummy |
| `cd backend && golangci-lint run` | Lint Go |
| `cd frontend && npm run type-check` | Typecheck |
| `cd frontend && npm run lint` | Lint frontend |
| `cd frontend && npm run test:unit` | Unit test |
| `docker compose up` | DB + backend + frontend |

`backend/Makefile` menyediakan target `build`, `run`, `seed`, `test`, `lint`, `migrate-up`, `migrate-down`, `backup` (butuh `make`, `migrate` CLI, dan `pg_dump`).

## Dokumentasi

- PRD: `docs/prd/PRD_FOOD_POS_UMKM.md`
- Plan implementasi: `docs/plan/`
- Runbook operasional: `docs/RUNBOOK.md`
