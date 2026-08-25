package emailverification

import (
	"context"
	"time"
	"uuid"

	"github.com/frimo-dev/frimo-messenger/internal/outbox"
)

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
	Confirm(ctx context.Context, tokenHash []byte, confirmedAt time.Time) error
	ResendVerificationToken(ctx context.Context, input ResendInput) error
}
