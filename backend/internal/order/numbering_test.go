package order

import "testing"

func TestOrderNo(t *testing.T) {
	cases := []struct {
		period string
		n      int
		want   string
	}{
		{"261113", 7, "ORD-261113-0007"},
		{"261113", 1, "ORD-261113-0001"},
		{"261201", 12345, "ORD-261201-12345"},
	}
	for _, c := range cases {
		if got := orderNo(c.period, c.n); got != c.want {
			t.Errorf("orderNo(%q, %d) = %q, want %q", c.period, c.n, got, c.want)
		}
	}
}
