package order

import "fmt"

// orderNo membentuk nomor pesanan resmi, contoh ORD-261113-0007.
func orderNo(period string, n int) string {
	return fmt.Sprintf("ORD-%s-%04d", period, n)
}
