package auth

import (
	"context"
	"time"
	"uuid"

	"github.com/frimo-dev/frimo-messenger/internal/outbox"
)

type VerificationInput struct {
	ID              uuid.UUID
	TokenHash       []byte
	TokenCiphertext []byte
	ExpiresAt       time.Time

	OutboxEvent outbox.Event
}

type CreateUserInput struct {
	ID           uuid.UUID
	Email        string
	PasswordHash string
	CreatedAt    time.Time

	Verification VerificationInput
}

type ResendVerificationInput struct {
	Email       string
	RequestedAt time.Time

	Verification VerificationInput
}

type Repository interface {
	CreateUser(ctx context.Context, data CreateUserInput) (User, error)
	ConfirmEmail(ctx context.Context, tokenHash []byte, confirmedAt time.Time) error
	ResendVerification(ctx context.Context, input ResendVerificationInput) error
}
