package report

import "testing"

func TestPercent(t *testing.T) {
	cases := []struct {
		part, total, want string
	}{
		{"60000.00", "60000.00", "100.00"},
		{"15000.00", "60000.00", "25.00"},
		{"-1552700.00", "60000.00", "-2587.83"},
		{"100.00", "0", "0.00"},
	}
	for _, c := range cases {
		got, err := percent(c.part, c.total)
		if err != nil {
			t.Fatalf("percent(%q,%q) error: %v", c.part, c.total, err)
		}
		if got != c.want {
			t.Errorf("percent(%q,%q) = %q, want %q", c.part, c.total, got, c.want)
		}
	}
}

func TestAddAndSubMoney(t *testing.T) {
	sum, err := addMoney("0", "23300.00")
	if err != nil {
		t.Fatal(err)
	}
	if sum != "23300.00" {
		t.Errorf("addMoney = %q, want 23300.00", sum)
	}

	diff, err := subMoney("60000.00", "0.00")
	if err != nil {
		t.Fatal(err)
	}
	if diff != "60000.00" {
		t.Errorf("subMoney = %q, want 60000.00", diff)
	}

	// Laba bersih negatif harus tetap terformat 2 desimal.
	afterWaste, _ := subMoney("60000.00", "12700.00")
	net, err := subMoney(afterWaste, "1600000.00")
	if err != nil {
		t.Fatal(err)
	}
	if net != "-1552700.00" {
		t.Errorf("net profit = %q, want -1552700.00", net)
	}
}

func TestDays(t *testing.T) {
	got := days("2026-11-14", "2026-11-16")
	want := []string{"2026-11-14", "2026-11-15", "2026-11-16"}
	if len(got) != len(want) {
		t.Fatalf("days len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("days[%d] = %q, want %q", i, got[i], want[i])
		}
	}

	if got := days("2026-11-14", "2026-11-14"); len(got) != 1 {
		t.Errorf("satu hari = %d baris, want 1", len(got))
	}
}

func TestLabels(t *testing.T) {
	if got := methodLabel("CASH"); got != "Tunai" {
		t.Errorf("methodLabel(CASH) = %q", got)
	}
	if got := methodLabel("UNKNOWN"); got != "UNKNOWN" {
		t.Errorf("methodLabel tak dikenal harus apa adanya, dapat %q", got)
	}
	if got := serviceLabel("PICKUP"); got != "Ambil sendiri" {
		t.Errorf("serviceLabel(PICKUP) = %q", got)
	}
}

func TestStockStatus(t *testing.T) {
	cases := []struct{ onHand, min, want string }{
		{"9.0000", "4.0000", "OK"},
		{"3.0000", "4.0000", "LOW"},
		{"1.0000", "4.0000", "CRITICAL"},
		{"0.0000", "4.0000", "CRITICAL"},
		{"5.0000", "0", "OK"},
	}
	for _, c := range cases {
		if got := stockStatus(c.onHand, c.min); got != c.want {
			t.Errorf("stockStatus(%q,%q) = %q, want %q", c.onHand, c.min, got, c.want)
		}
	}
}

func TestSumAmounts(t *testing.T) {
	got := sumAmounts([]MethodAmount{{Method: "CASH", Amount: "10000.00"}, {Method: "QRIS", Amount: "2500.50"}})
	if got != "12500.50" {
		t.Errorf("sumAmounts = %q, want 12500.50", got)
	}
	if got := sumAmounts(nil); got != "0" {
		t.Errorf("sumAmounts kosong = %q, want 0", got)
	}
}
