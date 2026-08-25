package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"
	"uuid"

	"github.com/frimo-dev/frimo-messenger/internal/dto"
	"github.com/frimo-dev/frimo-messenger/internal/outbox"
	"github.com/frimo-dev/frimo-messenger/internal/security/token"
)

type PasswordHasher interface {
	Hash(password string) (string, error)
}

type VerificationTokenGenerator interface {
	Generate() (rawToken string, tokenHash []byte, err error)
}

type VerificationTokenCipher interface {
	Encrypt(plaintext []byte, additionalData []byte) ([]byte, error)
}

type Service struct {
	repository                Repository
	passwordHasher            PasswordHasher
	tokenGenerator            VerificationTokenGenerator
	tokenCipher               VerificationTokenCipher
	now                       func() time.Time
	verificationTokenLifetime time.Duration
}

func NewService(repository Repository, passwordHasher PasswordHasher, tokenGenerator VerificationTokenGenerator, tokenCipher VerificationTokenCipher, now func() time.Time, verificationTokenLifetime time.Duration) *Service {
	return &Service{
		repository:                repository,
		passwordHasher:            passwordHasher,
		tokenGenerator:            tokenGenerator,
		tokenCipher:               tokenCipher,
		now:                       now,
		verificationTokenLifetime: verificationTokenLifetime,
	}
}

func (s *Service) Register(ctx context.Context, input RegistrationInput) (User, error) {
	email := normalizeEmail(input.Email)

	if err := validateEmail(email); err != nil {
		return User{}, err
	}

	if err := validatePassword(input.Password); err != nil {
		return User{}, err
	}

	passwordHash, err := s.passwordHasher.Hash(input.Password)
	if err != nil {
		return User{}, fmt.Errorf("hash password: %w", err)
	}

	now := s.now().UTC()
	userID := uuid.New()

	verificationInput, outboxEvent, err := s.prepareEmailVerification(email, now)
	if err != nil {
		return User{}, err
	}

	createdUser, err := s.repository.CreateUser(
		ctx,
		CreateUserInput{
			ID:           userID,
			Email:        email,
			PasswordHash: passwordHash,
			CreatedAt:    now,

			Verification: verificationInput,

			OutboxEvent: outboxEvent,
		},
	)
	if err != nil {
		return User{}, err
	}

	return createdUser, nil
}

func (s *Service) ConfirmEmail(ctx context.Context, rawToken string) error {
	if rawToken == "" {
		return ErrInvalidToken
	}

	tokenHash := token.Hash(rawToken)

	return s.repository.ConfirmEmail(ctx, tokenHash, s.now().UTC())
}

func (s *Service) ResendVerification(ctx context.Context, email string) error {
	email = normalizeEmail(email)

	if err := validateEmail(email); err != nil {
		return err
	}

	now := s.now().UTC()

	verificationInput, outboxEvent, err := s.prepareEmailVerification(email, now)
	if err != nil {
		return err
	}

	err = s.repository.ResendVerification(
		ctx,
		ResendVerificationInput{
			Email:       email,
			RequestedAt: now,

			Verification: verificationInput,

			OutboxEvent: outboxEvent,
		},
	)
	if err != nil {
		return err
	}

	return nil
}

func (s *Service) generateVerificationInput(now time.Time) (VerificationInput, error) {
	rawToken, tokenHash, err := s.tokenGenerator.Generate()
	if err != nil {
		return VerificationInput{}, fmt.Errorf("generate verification token: %w", err)
	}

	verificationID := uuid.New()

	tokenCiphertext, err := s.tokenCipher.Encrypt([]byte(rawToken), verificationID[:])
	if err != nil {
		return VerificationInput{}, fmt.Errorf("encrypt verification token: %w", err)
	}

	return VerificationInput{
		ID:              verificationID,
		TokenHash:       tokenHash,
		TokenCiphertext: tokenCiphertext,
		ExpiresAt:       now.Add(s.verificationTokenLifetime),
	}, nil
}

func (s *Service) prepareEmailVerification(email string, now time.Time) (VerificationInput, outbox.Event, error) {
	verification, err := s.generateVerificationInput(now)
	if err != nil {
		return VerificationInput{}, outbox.Event{}, err
	}

	payload, err := json.Marshal(
		dto.EmailVerificationRequested{
			VerificationID: verification.ID,
			Recipient:      email,
		},
	)
	if err != nil {
		return VerificationInput{}, outbox.Event{},
			fmt.Errorf("marshal verification event: %w", err)
	}

	event := outbox.Event{
		ID:          uuid.New(),
		Type:        dto.EmailVerificationRequestedType,
		Payload:     payload,
		CreatedAt:   now,
		AvailableAt: now,
	}

	return verification, event, nil
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func validateEmail(email string) error {
	if email == "" {
		return &ValidationError{
			Code:    "email_required",
			Field:   "email",
			Message: "email is required",
		}
	}

	if len(email) > 254 {
		return &ValidationError{
			Code:    "email_too_long",
			Field:   "email",
			Message: "email is too long",
		}
	}

	address, err := mail.ParseAddress(email)
	if err != nil || address.Address != email {
		return &ValidationError{
			Code:    "invalid_email",
			Field:   "email",
			Message: "email has invalid format",
		}
	}

	return nil
}

func validatePassword(password string) error {
	length := utf8.RuneCountInString(password)

	if length < 12 {
		return &ValidationError{
			Code:    "password_too_short",
			Field:   "password",
			Message: "password must contain at least 12 characters",
		}
	}

	if length > 128 {
		return &ValidationError{
			Code:    "password_too_long",
			Field:   "password",
			Message: "password must contain at most 128 characters",
		}
	}

	return nil
}
