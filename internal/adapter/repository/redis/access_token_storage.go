package redis

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
	"uuid"

	"github.com/frimo-dev/frimo-messenger/internal/service/auth"
	"github.com/redis/go-redis/v9"
)

const accessTokenKeyPrefix = "auth:access:"

type AccessTokenStorage struct {
	cache *redis.Client
}

func NewAccessTokenStorage(cache *redis.Client) *AccessTokenStorage {
	return &AccessTokenStorage{cache: cache}
}

func (s *AccessTokenStorage) Store(ctx context.Context, tokenHash []byte, identity auth.Identity, ttl time.Duration) error {
	key := accessTokenKeyPrefix + hex.EncodeToString(tokenHash)
	value := identity.UserID.String() + ":" + identity.SessionID.String()

	if err := s.cache.Set(ctx, key, value, ttl).Err(); err != nil {
		return fmt.Errorf("failed to set access token: %w", err)
	}

	return nil
}

func (s *AccessTokenStorage) Get(ctx context.Context, tokenHash []byte) (auth.Identity, error) {
	key := accessTokenKeyPrefix + hex.EncodeToString(tokenHash)

	value, err := s.cache.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return auth.Identity{}, auth.ErrAccessTokenInvalid
	}
	if err != nil {
		return auth.Identity{}, fmt.Errorf("failed to get credentials: %w", err)
	}

	rawUserID, rawSessionID, ok := strings.Cut(value, ":")
	if !ok {
		return auth.Identity{}, errors.New("invalid credentials format")
	}

	userID, err := uuid.Parse(rawUserID)
	if err != nil {
		return auth.Identity{}, fmt.Errorf("failed to parse access token user ID: %w", err)
	}

	sessionID, err := uuid.Parse(rawSessionID)
	if err != nil {
		return auth.Identity{}, fmt.Errorf("failed to parse access token session ID: %w", err)
	}

	return auth.Identity{UserID: userID, SessionID: sessionID}, nil
}

var _ auth.AccessTokenStorage = (*AccessTokenStorage)(nil)
