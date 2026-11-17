package backup

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestNextRun: jadwal berikutnya selalu di masa depan pada jam yang diminta.
func TestNextRun(t *testing.T) {
	loc := time.FixedZone("WIB", 7*3600)
	now := time.Date(2026, 11, 17, 1, 30, 0, 0, loc)

	next := nextRun(now, 2)
	if want := time.Date(2026, 11, 17, 2, 0, 0, 0, loc); !next.Equal(want) {
		t.Errorf("nextRun = %v, mau %v", next, want)
	}

	// Sudah lewat jam 2: jadwal pindah ke besok.
	next = nextRun(time.Date(2026, 11, 17, 3, 0, 0, 0, loc), 2)
	if want := time.Date(2026, 11, 18, 2, 0, 0, 0, loc); !next.Equal(want) {
		t.Errorf("nextRun lewat jam = %v, mau %v", next, want)
	}
}

// TestPrune_RemovesOnlyOldBackups: hanya file backup lama yang dihapus; file
// baru dan file lain tetap.
func TestPrune_RemovesOnlyOldBackups(t *testing.T) {
	dir := t.TempDir()
	r := NewRunner("postgres://test", dir, "", 14, nil)
	r.now = func() time.Time { return time.Date(2026, 11, 17, 10, 0, 0, 0, time.UTC) }

	old := filepath.Join(dir, "backup_lama.dump.enc")
	fresh := filepath.Join(dir, "backup_baru.dump.enc")
	other := filepath.Join(dir, "catatan.txt")
	for _, p := range []string{old, fresh, other} {
		if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
			t.Fatalf("tulis %s: %v", p, err)
		}
	}
	oldTime := r.now().Add(-15 * 24 * time.Hour)
	if err := os.Chtimes(old, oldTime, oldTime); err != nil {
		t.Fatalf("ubah waktu: %v", err)
	}

	r.prune()

	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Error("backup lama tidak terhapus")
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Errorf("backup baru ikut terhapus: %v", err)
	}
	if _, err := os.Stat(other); err != nil {
		t.Errorf("file non-backup ikut terhapus: %v", err)
	}
}
