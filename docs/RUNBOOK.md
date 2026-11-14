# RUNBOOK — Operasional Food POS UMKM

Prosedur untuk backup, restore, dan pengecekan sistem. Target: RPO <= 24 jam, RTO <= 4 jam.

## 1. Backup Manual

```bash
cd backend
DATABASE_URL="postgres://pos_user:PASSWORD@localhost:5432/pos_fb?sslmode=disable" make backup
```

Menghasilkan `backups/backup_<timestamp>.dump` (format custom `pg_dump`, terkompresi).

Backup terenkripsi:

```bash
DATABASE_URL="..." BACKUP_ENCRYPTION_KEY="..." make backup-encrypted
```

## 2. Verifikasi Backup

```bash
# integritas file
sha256sum -c backups/backup_20260101_020000.dump.sha256

# isi arsip masih bisa dibaca
pg_restore --list backups/backup_20260101_020000.dump > /dev/null && echo "OK"
```

Backup yang gagal verifikasi **tidak boleh** dianggap cadangan.

## 3. Restore

Restore ke database produksi hanya dengan persetujuan OWNER dan dicatat di audit log.

```bash
# 1. Mode pemeliharaan: hentikan penulisan
docker compose stop backend frontend

# 2. Pulihkan ke database baru dulu (jangan langsung timpa produksi)
createdb pos_fb_restore
pg_restore -d pos_fb_restore --clean --if-exists backups/backup_20260101_020000.dump

# 3. Verifikasi
psql -d pos_fb_restore -c "SELECT count(*) FROM orders;"
psql -d pos_fb_restore -c "SELECT ingredient_id, quantity_on_hand FROM inventory LIMIT 10;"
psql -d pos_fb_restore -c "SELECT SUM(quantity_delta) FROM stock_movements GROUP BY ingredient_id LIMIT 10;"
# SUM(quantity_delta) per bahan harus sama dengan quantity_on_hand

# 4. Alihkan: arahkan DB_NAME ke database hasil restore, lalu jalankan ulang
docker compose up -d db backend frontend
curl -f http://localhost:8080/health
```

Jika langkah 3 gagal, **jangan** alihkan. Database produksi lama masih utuh.

## 4. Rollback Aplikasi

```bash
docker compose down backend frontend
git checkout <commit-stabil-sebelumnya>
docker compose up -d --build backend frontend
curl -f http://localhost:8080/health
```

Migrasi database yang sudah berjalan tidak otomatis dibalik. Jalankan `make migrate-down` hanya bila migrasi tersebut terbukti penyebab masalah, karena dapat menghapus data.

## 5. Pengecekan Harian

```bash
curl -s http://localhost:8080/health
# {"status":"ok","db":"ok"}

docker compose logs --since 24h backend | grep '"level":"ERROR"'
ls -lh backups/ | tail -5
```

Health `"db":"down"` atau respons `503` berarti backend tidak dapat menjangkau PostgreSQL: cek container `db`, disk penuh, dan connection limit.

## 6. Jadwal Backup

| Jenis | Waktu | Retensi |
|---|---|---|
| Harian | 02:00 waktu bisnis | 14 hari |
| Mingguan | Minggu 02:30 | 8 minggu |
| Bulanan | tanggal 1, 03:00 | 12 bulan |

Simpan salinan off-site terpisah dari server database. Kunci enkripsi disimpan terpisah dari file backup.
