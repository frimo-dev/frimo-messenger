package httpapi

import (
	"context"

	"github.com/frimo-dev/frimo-messenger/internal/service/auth"
)

type contextKey string

const identityKey contextKey = "identity"

func withIdentity(ctx context.Context, identity auth.Identity) context.Context {
	return context.WithValue(ctx, identityKey, identity)
}

func identityFromContext(ctx context.Context) (auth.Identity, bool) {
	identity, ok := ctx.Value(identityKey).(auth.Identity)
	return identity, ok
}
