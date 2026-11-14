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

### Backend

```bash
cd backend
go mod download
go run ./cmd/server       # http://localhost:8080
curl http://localhost:8080/health
```

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
| `cd backend && golangci-lint run` | Lint Go |
| `cd frontend && npm run type-check` | Typecheck |
| `cd frontend && npm run lint` | Lint frontend |
| `cd frontend && npm run test:unit` | Unit test |
| `docker compose up` | DB + backend + frontend |

`backend/Makefile` menyediakan target `build`, `run`, `test`, `lint`, `migrate-up`, `migrate-down`, `backup` (butuh `make`, `migrate` CLI, dan `pg_dump`).

## Dokumentasi

- PRD: `docs/prd/PRD_FOOD_POS_UMKM.md`
- Plan implementasi: `docs/plan/`
- Runbook operasional: `docs/RUNBOOK.md`
