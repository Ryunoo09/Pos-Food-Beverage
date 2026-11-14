package middleware

import (
	"net/http"
	"strings"

	"github.com/ryunoo09/pos-food-beverage/backend/internal/auth"
	"github.com/ryunoo09/pos-food-beverage/backend/internal/httpx"
)

// AuthRequired memverifikasi Bearer token lalu memuat role & permission
// terbaru dari DB (bukan dari token) agar perubahan hak akses langsung berlaku.
// Query permission per request diterima untuk MVP; tambahkan cache TTL 60s bila
// profiling menunjukkan beban DB.
func AuthRequired(svc *auth.Service, secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			requestID := httpx.RequestIDFrom(ctx)

			raw, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
			if !ok || raw == "" {
				httpx.WriteError(w, httpx.NewError(http.StatusUnauthorized, "AUTH_TOKEN_MISSING",
					"Token akses tidak ditemukan."), requestID)
				return
			}

			claims, err := auth.ParseAccessToken(secret, raw)
			if err != nil {
				httpx.WriteError(w, httpx.NewError(http.StatusUnauthorized, "AUTH_TOKEN_EXPIRED",
					"Sesi sudah berakhir. Silakan login ulang."), requestID)
				return
			}

			actor, err := svc.LoadActor(ctx, claims.Subject, claims.BusinessID, requestID,
				httpx.ClientIP(r), r.UserAgent())
			if err != nil {
				httpx.WriteError(w, err, requestID)
				return
			}

			perms, err := svc.Permissions(ctx, claims.Subject)
			if err != nil {
				httpx.WriteError(w, err, requestID)
				return
			}
			actor.Permissions = perms

			next.ServeHTTP(w, r.WithContext(auth.WithActor(ctx, actor)))
		})
	}
}

// PermissionRequired menolak request bila salah satu permission tidak dimiliki.
// Prinsip: cek permission, bukan nama role. Default deny.
func PermissionRequired(perms ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			actor, ok := auth.ActorFrom(r.Context())
			if !ok {
				httpx.WriteError(w, httpx.NewError(http.StatusUnauthorized, "AUTH_TOKEN_MISSING",
					"Token akses tidak ditemukan."), httpx.RequestIDFrom(r.Context()))
				return
			}

			owned := make(map[string]struct{}, len(actor.Permissions))
			for _, p := range actor.Permissions {
				owned[p] = struct{}{}
			}
			for _, want := range perms {
				if _, has := owned[want]; !has {
					httpx.WriteError(w, httpx.NewError(http.StatusForbidden, "FORBIDDEN",
						"Anda tidak memiliki akses untuk aksi ini."), httpx.RequestIDFrom(r.Context()))
					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}
