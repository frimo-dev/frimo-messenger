package redis

import (
	"context"
	"errors"
	"fmt"
	"time"
	"uuid"

	"github.com/frimo-dev/frimo-messenger/internal/service/auth"
	"github.com/redis/go-redis/v9"
)

const sessionKeyPrefix = "auth:session:"

type SessionStorage struct {
	cache *redis.Client
}

func NewSessionStorage(cache *redis.Client) *SessionStorage {
	return &SessionStorage{cache: cache}
}

func (s *SessionStorage) SetSession(ctx context.Context, sessionID uuid.UUID, isActive bool, ttl time.Duration) error {
	key := sessionKeyPrefix + sessionID.String()
	if err := s.cache.Set(ctx, key, isActive, ttl).Err(); err != nil {
		return fmt.Errorf("failed to set session: %w", err)
	}
	return nil
}

func (s *SessionStorage) GetSession(ctx context.Context, sessionID uuid.UUID) (bool, error) {
	key := sessionKeyPrefix + sessionID.String()
	isActive, err := s.cache.Get(ctx, key).Bool()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return false, auth.ErrSessionCacheMiss
		}
		return false, fmt.Errorf("failed to get session: %w", err)
	}

	return isActive, nil
}
