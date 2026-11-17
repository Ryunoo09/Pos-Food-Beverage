// Package backup menjalankan pg_dump terjadwal, mengenkripsi hasilnya,
// menyimpan checksum, mencatat ke backup_records, dan menghapus backup lama.
//
// Butuh binary `pg_dump` dan `openssl` di image/container.
package backup

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Runner menjalankan satu backup dan menjadwalkannya harian.
type Runner struct {
	dbURL     string
	dir       string
	key       string
	retention time.Duration
	pool      *pgxpool.Pool
	now       func() time.Time
}

// NewRunner membuat runner. key kosong = backup tidak dienkripsi (dev).
// retentionDays <= 0 memakai 14 hari.
func NewRunner(dbURL, dir, key string, retentionDays int, pool *pgxpool.Pool) *Runner {
	if retentionDays <= 0 {
		retentionDays = 14
	}
	return &Runner{
		dbURL:     dbURL,
		dir:       dir,
		key:       key,
		retention: time.Duration(retentionDays) * 24 * time.Hour,
		pool:      pool,
		now:       time.Now,
	}
}

// Run membuat satu backup: pg_dump (opsional terenkripsi), checksum, catat
// riwayat, lalu bersihkan file lama.
func (r *Runner) Run(ctx context.Context, kind string) error {
	if err := os.MkdirAll(r.dir, 0o750); err != nil {
		return fmt.Errorf("backup: buat direktori: %w", err)
	}

	started := r.now()
	name := "backup_" + started.Format("20060102_150405") + ".dump"
	if r.key != "" {
		name += ".enc"
	}
	path := filepath.Join(r.dir, name)

	if err := r.dump(ctx, path); err != nil {
		r.record(ctx, kind, "FAILED", name, nil, "", started, err)
		return err
	}

	sum, size, err := checksumFile(path)
	if err != nil {
		r.record(ctx, kind, "FAILED", name, nil, "", started, err)
		return err
	}
	if err := os.WriteFile(path+".sha256",
		[]byte(fmt.Sprintf("%s  %s\n", sum, name)), 0o640); err != nil {
		r.record(ctx, kind, "FAILED", name, nil, sum, started, err)
		return err
	}

	r.record(ctx, kind, "SUCCESS", name, &size, sum, started, nil)
	r.prune()
	return nil
}

// StartSchedule menjalankan backup harian pada jam lokal server sampai ctx
// selesai. Ponytail: satu jadwal harian; tambah tier mingguan/bulanan hanya
// bila retensi harian terbukti kurang.
func (r *Runner) StartSchedule(ctx context.Context, hour int) {
	for {
		next := nextRun(r.now(), hour)
		timer := time.NewTimer(next.Sub(r.now()))
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}

		if err := r.Run(ctx, "SCHEDULED"); err != nil {
			slog.Error("backup terjadwal gagal", "error", err)
		}
	}
}

// nextRun mengembalikan jadwal berikutnya pada jam `hour` (0-23).
func nextRun(now time.Time, hour int) time.Time {
	next := time.Date(now.Year(), now.Month(), now.Day(), hour, 0, 0, 0, now.Location())
	if !next.After(now) {
		next = next.AddDate(0, 0, 1)
	}
	return next
}

// dump menulis pg_dump ke path; bila key diisi, output dialirkan ke openssl
// dengan key lewat environment (bukan argv) agar tidak terlihat di ps.
func (r *Runner) dump(ctx context.Context, path string) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o640)
	if err != nil {
		return fmt.Errorf("backup: buat file: %w", err)
	}
	defer file.Close()

	dump := exec.CommandContext(ctx, "pg_dump", "-Fc", "--no-password", r.dbURL)
	var stderr bytes.Buffer
	dump.Stderr = &stderr

	if r.key == "" {
		dump.Stdout = file
		if err := dump.Run(); err != nil {
			return fmt.Errorf("backup: pg_dump: %w: %s", err, strings.TrimSpace(stderr.String()))
		}
		return nil
	}

	pipe, err := dump.StdoutPipe()
	if err != nil {
		return fmt.Errorf("backup: pipe pg_dump: %w", err)
	}
	enc := exec.CommandContext(ctx, "openssl", "enc", "-aes-256-cbc", "-pbkdf2",
		"-pass", "env:BACKUP_ENCRYPTION_KEY")
	enc.Env = append(os.Environ(), "BACKUP_ENCRYPTION_KEY="+r.key)
	enc.Stdin = pipe
	enc.Stdout = file
	enc.Stderr = &stderr

	if err := dump.Start(); err != nil {
		return fmt.Errorf("backup: jalankan pg_dump: %w", err)
	}
	if err := enc.Start(); err != nil {
		return fmt.Errorf("backup: jalankan openssl: %w", err)
	}
	dumpErr := dump.Wait()
	encErr := enc.Wait()
	if dumpErr != nil {
		return fmt.Errorf("backup: pg_dump: %w: %s", dumpErr, strings.TrimSpace(stderr.String()))
	}
	if encErr != nil {
		return fmt.Errorf("backup: enkripsi: %w: %s", encErr, strings.TrimSpace(stderr.String()))
	}
	return nil
}

// record menulis satu baris backup_records; kegagalan catat tidak menggagalkan
// backup yang sebenarnya sudah jadi.
func (r *Runner) record(ctx context.Context, kind, status, name string, size *int64, sum string, started time.Time, cause error) {
	if r.pool == nil {
		return
	}

	var errText *string
	if cause != nil {
		s := cause.Error()
		errText = &s
	}
	var sha *string
	if sum != "" {
		sha = &sum
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, err := r.pool.Exec(ctx, `
		INSERT INTO backup_records
			(type, status, file_name, size_bytes, sha256, encrypted,
			 storage_location, started_at, finished_at, error)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, now(), $9)`,
		kind, status, name, size, sha, r.key != "", r.dir, started, errText)
	if err != nil {
		slog.Warn("backup: catat riwayat gagal", "error", err, "file", name)
	}
}

// prune menghapus file backup dan checksum yang lebih tua dari retensi.
func (r *Runner) prune() {
	cutoff := r.now().Add(-r.retention)
	entries, err := os.ReadDir(r.dir)
	if err != nil {
		slog.Warn("backup: baca direktori untuk retensi gagal", "error", err)
		return
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "backup_") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.ModTime().After(cutoff) {
			continue
		}
		if err := os.Remove(filepath.Join(r.dir, entry.Name())); err != nil {
			slog.Warn("backup: hapus file lama gagal", "file", entry.Name(), "error", err)
		}
	}
}

// checksumFile menghitung sha256 dan ukuran file.
func checksumFile(path string) (string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, fmt.Errorf("backup: buka file: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	size, err := io.Copy(h, f)
	if err != nil {
		return "", 0, fmt.Errorf("backup: hash file: %w", err)
	}
	return hex.EncodeToString(h.Sum(nil)), size, nil
}
