package auth

import (
	"context"
	"time"
	"uuid"

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

type ResendInput struct {
	Email    string
	ResendAt time.Time

	VerificationID              uuid.UUID
	VerificationTokenHash       []byte
	VerificationTokenCiphertext []byte
	VerificationExpiresAt       time.Time

	OutboxEvent outbox.Event
}

type Repository interface {
	Create(ctx context.Context, data CreationInput) (User, error)
	Confirm(ctx context.Context, tokenHash []byte, confirmedAt time.Time) error
	ResendVerificationToken(ctx context.Context, input ResendInput) error
}
