package user

import (
	"context"
	"uuid"
)

type Repository interface {
	GetUserByID(ctx context.Context, userID uuid.UUID) (User, error)
	GetPublicUserByUsername(ctx context.Context, username string) (PublicUser, error)
}
