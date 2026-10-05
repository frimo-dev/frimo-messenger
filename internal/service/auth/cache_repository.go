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

type SessionStorage interface {
	SetSession(ctx context.Context, sessionID uuid.UUID, isActive bool, ttl time.Duration) error
	GetSession(ctx context.Context, sessionID uuid.UUID) (bool, error)
}

type RefreshResultStorage interface {
	Store(ctx context.Context, oldRefreshTokenHash []byte, tokenPair TokenPair, ttl time.Duration) error
	Get(ctx context.Context, oldRefreshTokenHash []byte) (TokenPair, error)
}
