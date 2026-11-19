# RUNBOOK — Backup, Restore, dan Deploy

Target: satu VPS, Docker Compose (`docker-compose.prod.yml`), PostgreSQL 16.
Semua perintah dijalankan dari root repo di VPS. RPO ≤ 24 jam, RTO ≤ 4 jam.

---

## 1. Deploy pertama

```bash
git clone <repo> pos && cd pos
cp .env.example .env
# Isi .env: DB_PASSWORD, JWT_SECRET (acak ≥32 byte), BACKUP_ENCRYPTION_KEY (≥32 byte),
# APP_ENV=production, COOKIE_SECURE=true
docker compose -f docker-compose.prod.yml up -d --build
docker compose -f docker-compose.prod.yml exec backend /server -healthcheck
curl -s http://localhost/health
```

Migrasi dijalankan sekali sebelum aplikasi melayani trafik:

```bash
# dari mesin dengan golang-migrate, atau lewat container sementara
migrate -path backend/internal/db/migrations -database "$DATABASE_URL" up
docker compose -f docker-compose.prod.yml restart backend
```

Deploy ulang setelah `git pull`:

```bash
docker compose -f docker-compose.prod.yml up -d --build
```

Rollback image: `git checkout <tag-sebelumnya>` lalu ulangi perintah di atas
(image dibangun ulang, data DB tidak berubah).

---

## 2. Backup

Otomatis: backend menjalankan `pg_dump` tiap hari pada `BACKUP_SCHEDULE_HOUR`
(default 02:00, zona waktu container), terenkripsi AES-256-CBC, checksum
`.sha256`, dicatat di tabel `backup_records`, file lebih tua dari
`BACKUP_RETENTION_DAILY` hari dihapus.

Manual (dari root repo, butuh `pg_dump` di host):

```bash
cd backend
make backup-encrypted DATABASE_URL="postgres://USER:PASS@db-host:5432/db" \
  BACKUP_ENCRYPTION_KEY="<key>"
```

Manual di dalam container:

```bash
docker compose -f docker-compose.prod.yml exec backend \
  /server -healthcheck  # cek backend hidup

docker compose -f docker-compose.prod.yml exec db \
  sh -c 'pg_dump -Fc -U "$POSTGRES_USER" -d "$POSTGRES_DB" > /tmp/manual.dump'
docker compose -f docker-compose.prod.yml cp db:/tmp/manual.dump ./backups/
```

## 3. Verifikasi backup

```bash
cd backend
make verify-backup FILE=backups/backup_20261117_020000.dump.enc
# atau langsung
cd backups && sha256sum -c backup_20261117_020000.dump.enc.sha256
```

Backup yang gagal verifikasi **tidak boleh** dianggap cadangan.

Jadwal retensi: scheduler hanya membuat backup harian (retensi
`BACKUP_RETENTION_DAILY`). Tier mingguan/bulanan masih manual — salin file
harian ke direktori terpisah di luar VPS sesuai kebutuhan RPO/RTO:

## 4. Restore

Restore menimpa data. Lakukan saat toko tutup, dan pastikan backup baru sudah
dibuat sebelum mulai.

1. Mode maintenance: hentikan backend agar tidak ada tulis baru.
   ```bash
   docker compose -f docker-compose.prod.yml stop backend
   ```
2. Siapkan DB baru (hindari menimpa DB lama sebelum verifikasi):
   ```bash
   docker compose -f docker-compose.prod.yml exec db \
     psql -U "$POSTGRES_USER" -d postgres -c 'CREATE DATABASE pos_fb_restore;'
   ```
3. Dekripsi dan restore:
   ```bash
   cd backend
   openssl enc -d -aes-256-cbc -pbkdf2 -pass "env:BACKUP_ENCRYPTION_KEY" \
     -in ../backups/backup_<tanggal>.dump.enc \
     | pg_restore -h <db-host> -U <user> -d pos_fb_restore --no-owner
   ```
4. Verifikasi jumlah baris tabel inti dan saldo stok:
   ```sql
   SELECT count(*) FROM orders;
   SELECT count(*) FROM order_items;
   SELECT ingredient_id, quantity_on_hand FROM inventory ORDER BY ingredient_id LIMIT 10;
   ```
   Stok harus konsisten dengan movement:
   ```sql
   SELECT ingredient_id, SUM(quantity_delta) FROM stock_movements
   GROUP BY ingredient_id ORDER BY ingredient_id LIMIT 10;
   ```
   Nilai SUM(quantity_delta) per bahan harus sama dengan `quantity_on_hand`.
   Jika verifikasi gagal, jangan alihkan — DB lama masih utuh.
5. Alihkan aplikasi ke DB hasil restore (`DB_NAME=pos_fb_restore` di `.env`),
   nyalakan backend, cek `GET /health` dan beberapa pesanan terakhir.
6. Catat restore di `audit_logs` (aksi manual atau lewat aplikasi) dan simpan
   file backup asli minimal 14 hari.

## 5. Operasi harian

```bash
docker compose -f docker-compose.prod.yml ps
docker compose -f docker-compose.prod.yml logs -f backend
docker compose -f docker-compose.prod.yml logs -f nginx
curl -s http://localhost/health
```

Backup terjadwal gagal? Cek log `backup terjadwal gagal` di backend dan baris
`backup_records` dengan `status = 'FAILED'`. Penyebab umum: `pg_dump` tidak ada
di image (build ulang Dockerfile) atau volume `/backups` penuh.

Health `"db":"down"` atau 503 = backend tidak menjangkau PostgreSQL: cek
container `db`, disk penuh, dan connection limit.

Simpan salinan backup off-site (VPS/objek storage lain). Kunci enkripsi
disimpan terpisah dari file backup, jangan di repo.

## 6. HTTPS

Nginx produksi saat ini HTTP saja. Setelah domain mengarah ke VPS:

```bash
docker run --rm -v "$PWD/nginx/ssl:/etc/letsencrypt" certbot/certbot certonly \
  --standalone -d pos.contoh.id
```

Tambahkan server block `listen 443 ssl;` dengan `ssl_certificate` dan
`ssl_certificate_key` di `nginx/nginx.conf`, tambahkan volume
`./nginx/ssl:/etc/nginx/ssl:ro`, buka port 443 di compose, lalu:

```bash
docker compose -f docker-compose.prod.yml up -d
```
