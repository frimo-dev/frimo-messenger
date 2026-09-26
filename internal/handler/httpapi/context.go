package httpapi

import (
	"context"
	"uuid"

	"github.com/frimo-dev/frimo-messenger/internal/service/auth"
)

type contextKey string

const requestIDKey contextKey = "request_id"
const userIDKey contextKey = "user_id"
const identityKey contextKey = "identity"

func withRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey, requestID)
}

func requestIDFromContext(ctx context.Context) string {
	requestID, _ := ctx.Value(requestIDKey).(string)
	return requestID
}

func withUserID(ctx context.Context, userID uuid.UUID) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}

func userIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	userID, ok := ctx.Value(userIDKey).(uuid.UUID)
	return userID, ok
}

func withIdentity(ctx context.Context, identity auth.Identity) context.Context {
	return context.WithValue(ctx, identityKey, identity)
}

func identityFromContext(ctx context.Context) (auth.Identity, bool) {
	identity, ok := ctx.Value(identityKey).(auth.Identity)
	return identity, ok
}
