package auth

import (
	"context"
	"time"
	"uuid"
)

type Identity struct {
	UserID    uuid.UUID
	SessionID uuid.UUID
}

type AccessTokenStorage interface {
	Store(ctx context.Context, tokenHash []byte, identity Identity, ttl time.Duration) error
	Get(ctx context.Context, tokenHash []byte) (Identity, error)
}
