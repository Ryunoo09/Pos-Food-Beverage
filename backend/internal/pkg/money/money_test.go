package money

import "testing"

func TestRoundHalfUp(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"100.004", "100.00"},
		{"100.005", "100.01"},
		{"100.015", "100.02"},
		{"99.995", "100.00"},
	}
	for _, c := range cases {
		d, err := Parse(c.in)
		if err != nil {
			t.Fatal(err)
		}
		if got := RoundHalfUp(d, 2).StringFixed(2); got != c.want {
			t.Errorf("%s: got %s, want %s", c.in, got, c.want)
		}
	}
}

func TestPercent(t *testing.T) {
	base, _ := Parse("47300.00")
	rate, _ := Parse("11")
	if got := Percent(base, rate, 2).StringFixed(2); got != "5203.00" {
		t.Errorf("PPN 11%% dari 47300 = %s, want 5203.00", got)
	}
}

func TestParse_Invalid(t *testing.T) {
	if _, err := Parse("12,5"); err == nil {
		t.Error("format bukan NUMERIC diterima")
	}
}
