package httpx

import (
	"net"
	"net/http"
	"strings"
)

// ClientIP mengembalikan IP klien. Di produksi backend berada di belakang Nginx,
// jadi X-Forwarded-For diutamakan. Nginx wajib menimpa header ini agar tidak
// bisa dipalsukan klien.
func ClientIP(r *http.Request) string {
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		first, _, _ := strings.Cut(forwarded, ",")
		if ip := strings.TrimSpace(first); ip != "" {
			return ip
		}
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
