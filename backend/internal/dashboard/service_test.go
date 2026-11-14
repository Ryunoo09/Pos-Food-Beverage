package dashboard

import "testing"

func TestParsePeriod(t *testing.T) {
	from, to, err := parsePeriod("2026-11-01", "2026-11-14")
	if err != nil || from != "2026-11-01" || to != "2026-11-14" {
		t.Fatalf("got %s..%s err=%v", from, to, err)
	}

	if _, _, err := parsePeriod("2026-11-14", "2026-11-01"); err == nil {
		t.Error("rentang terbalik diterima")
	}
	if _, _, err := parsePeriod("2025-01-01", "2026-11-01"); err == nil {
		t.Error("rentang lebih dari 366 hari diterima")
	}
	if _, _, err := parsePeriod("14-11-2026", ""); err == nil {
		t.Error("format tanggal salah diterima")
	}

	from, to, err = parsePeriod("", "")
	if err != nil || from == "" || from != to {
		t.Errorf("default periode = %s..%s err=%v", from, to, err)
	}
}

func TestAverage(t *testing.T) {
	if got := average("0", 0); got != "0.00" {
		t.Errorf("tanpa transaksi = %s", got)
	}
	if got := average("10000.00", 3); got != "3333.33" {
		t.Errorf("AOV = %s, want 3333.33", got)
	}
}
