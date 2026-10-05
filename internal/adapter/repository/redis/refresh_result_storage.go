package redis

import (
	"context"
	"encoding/hex"
	"encoding/json/v2"
	"errors"
	"fmt"
	"time"

	"github.com/frimo-dev/frimo-messenger/internal/service/auth"
	"github.com/redis/go-redis/v9"
)

const refreshResultKeyPrefix = "auth:refresh-result:"

type refreshResult struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

type RefreshResultStorage struct {
	cache *redis.Client
}

func NewRefreshResultStorage(cache *redis.Client) *RefreshResultStorage {
	return &RefreshResultStorage{cache: cache}
}

func (s *RefreshResultStorage) Store(ctx context.Context, oldRefreshTokenHash []byte, tokenPair auth.TokenPair, ttl time.Duration) error {
	key := refreshResultKeyPrefix + hex.EncodeToString(oldRefreshTokenHash)
	value, err := json.Marshal(refreshResult{
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
	})
	if err != nil {
		return fmt.Errorf("failed to serialize refresh result to JSON: %w", err)
	}

	if err := s.cache.Set(ctx, key, value, ttl).Err(); err != nil {
		return fmt.Errorf("failed to store refresh result: %w", err)
	}

	return nil
}

func (s *RefreshResultStorage) Get(ctx context.Context, oldRefreshTokenHash []byte) (auth.TokenPair, error) {
	key := refreshResultKeyPrefix + hex.EncodeToString(oldRefreshTokenHash)

	value, err := s.cache.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return auth.TokenPair{}, auth.ErrRefreshResultNotFound
	}
	if err != nil {
		return auth.TokenPair{}, fmt.Errorf("failed to get refresh result: %w", err)
	}

	var result refreshResult
	if err := json.Unmarshal([]byte(value), &result); err != nil {
		return auth.TokenPair{}, fmt.Errorf("failed to deserialize refresh result: %w", err)
	}

	return auth.TokenPair{AccessToken: result.AccessToken, RefreshToken: result.RefreshToken}, nil
}
