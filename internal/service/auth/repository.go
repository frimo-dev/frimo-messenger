package auth

import (
	"context"
	"net/netip"
	"time"
	"uuid"

	"github.com/frimo-dev/frimo-messenger/internal/outbox"
)

type VerificationInput struct {
	ID              uuid.UUID
	TokenHash       []byte
	TokenCiphertext []byte
	ExpiresAt       time.Time
}

type CreateUserInput struct {
	ID           uuid.UUID
	Email        string
	PasswordHash string
	CreatedAt    time.Time

	Verification VerificationInput
	OutboxEvent  outbox.Event
}

type ResendVerificationInput struct {
	Email       string
	RequestedAt time.Time

	Verification VerificationInput
	OutboxEvent  outbox.Event
}

type LoginUser struct {
	ID           uuid.UUID
	PasswordHash string
	VerifiedAt   *time.Time
}

type Repository interface {
	GetUserForLogin(ctx context.Context, email string) (LoginUser, error)
	CreateUser(ctx context.Context, data CreateUserInput) (User, error)
	ConfirmEmail(ctx context.Context, tokenHash []byte, confirmedAt time.Time) error
	ResendVerification(ctx context.Context, input ResendVerificationInput) error
}

type Session struct {
	ID     uuid.UUID
	UserID uuid.UUID

	DeviceName string

	CreatedIP netip.Addr
	LastIP    netip.Addr

	CreatedAt  time.Time
	LastSeenAt time.Time
	ExpiresAt  time.Time
}

type RefreshToken struct {
	ID        uuid.UUID
	TokenHash []byte
	CreatedAt time.Time
}

type SessionState struct {
	RevokedAt *time.Time
	ExpiresAt time.Time
}

type ExtendSessionInput struct {
	OldRefreshTokenHash []byte
	NewRefreshToken     RefreshToken
	SessionLifetime     time.Duration
}

type SessionRepository interface {
	CreateSession(ctx context.Context, session Session, refreshToken RefreshToken) error
	GetSessionState(ctx context.Context, sessionID uuid.UUID) (SessionState, error)
	ExtendSession(ctx context.Context, input ExtendSessionInput) (Identity, error)
}
