package user

import (
	"context"
	"time"
)

type CreationInput struct {
	ID                    string
	Email                 string
	PasswordHash          string
	VerificationID        string
	VerificationTokenHash []byte
	VerificationExpiresAt time.Time
	CreatedAt             time.Time
}

type Repository interface {
	Create(ctx context.Context, data CreationInput) (User, error)
}
