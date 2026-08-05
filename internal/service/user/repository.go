package user

import (
	"context"
	"time"

	"github.com/frimo-dev/frimo-messenger/internal/outbox"
)

type CreationInput struct {
	ID           string
	Email        string
	PasswordHash string
	CreatedAt    time.Time

	VerificationID              string
	VerificationTokenHash       []byte
	VerificationTokenCiphertext []byte
	VerificationExpiresAt       time.Time

	OutboxEvent outbox.Event
}

type Repository interface {
	Create(ctx context.Context, data CreationInput) (User, error)
}
