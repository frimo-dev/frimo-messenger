package auth

import "context"

type Observer interface {
	RefreshResultStoreFailed(ctx context.Context, err error)
	AccessTokenStoreFailed(ctx context.Context, err error)
	SessionCacheUpdateFailed(ctx context.Context, err error)
	SessionCacheReadFailed(ctx context.Context, err error)
}
