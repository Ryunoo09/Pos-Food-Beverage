package auth

import "context"

type actorKey struct{}

// WithActor menyimpan aktor terautentikasi di context.
func WithActor(ctx context.Context, a Actor) context.Context {
	return context.WithValue(ctx, actorKey{}, a)
}

// ActorFrom mengambil aktor dari context. false bila request tidak
// melewati middleware AuthRequired.
func ActorFrom(ctx context.Context) (Actor, bool) {
	a, ok := ctx.Value(actorKey{}).(Actor)
	return a, ok
}
