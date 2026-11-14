package httpx

import "context"

type requestIDKey struct{}

// WithRequestID menyimpan request id di context.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey{}, id)
}

// RequestIDFrom mengambil request id dari context. Kosong bila middleware
// RequestID tidak dipasang.
func RequestIDFrom(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey{}).(string)
	return id
}
