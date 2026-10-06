package observability

import (
	"context"

	"github.com/frimo-dev/frimo-messenger/internal/service/auth"
	"go.uber.org/zap"
)

var _ auth.Observer = (*AuthObserver)(nil)

type AuthObserver struct {
	logger *zap.Logger
}

func NewAuthObserver(logger *zap.Logger) *AuthObserver {
	return &AuthObserver{
		logger: logger,
	}
}

func (o *AuthObserver) RefreshResultStoreFailed(ctx context.Context, err error) {
	o.logger.Error("failed to store refresh result", zap.Error(err))
}

func (o *AuthObserver) AccessTokenStoreFailed(ctx context.Context, err error) {
	o.logger.Error("failed to store access token", zap.Error(err))
}

func (o *AuthObserver) SessionCacheUpdateFailed(ctx context.Context, err error) {
	o.logger.Error("failed to update session cache", zap.Error(err))
}

func (o *AuthObserver) SessionCacheReadFailed(ctx context.Context, err error) {
	o.logger.Error("failed to read session cache", zap.Error(err))
}
