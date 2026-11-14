package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"

	"github.com/ryunoo09/pos-food-beverage/backend/internal/httpx"
)

// RequestID menerima X-Request-ID dari klien atau membuat baru, lalu
// menyimpannya di context dan mengembalikannya di response.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			id = newID()
		}

		w.Header().Set("X-Request-ID", id)
		ctx := httpx.WithRequestID(r.Context(), id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func newID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "unknown"
	}
	return hex.EncodeToString(b[:])
}
