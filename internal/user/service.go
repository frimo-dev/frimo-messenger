package user

import (
	"context"
	"fmt"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

type PasswordHasher interface {
	Hash(password string) (string, error)
}

type VerificationTokenGenerator interface {
	Generate() (rawToken string, tokenHash []byte, err error)
}

type RegistrationResult struct {
	User                 User
	RawVerificationToken string
}

type Service struct {
	repository                Repository
	passwordHasher            PasswordHasher
	tokenGenerator            VerificationTokenGenerator
	now                       func() time.Time
	verificationTokenLifetime time.Duration
}

func NewService(repository Repository, passwordHasher PasswordHasher, tokenGenerator VerificationTokenGenerator, now func() time.Time, verificationTokenLifetime time.Duration) *Service {
	return &Service{
		repository:                repository,
		passwordHasher:            passwordHasher,
		tokenGenerator:            tokenGenerator,
		now:                       now,
		verificationTokenLifetime: verificationTokenLifetime,
	}
}

func (s *Service) Register(ctx context.Context, input RegistrationInput) (RegistrationResult, error) {
	email := normalizeEmail(input.Email)

	if err := validateEmail(email); err != nil {
		return RegistrationResult{}, err
	}

	if err := validatePassword(input.Password); err != nil {
		return RegistrationResult{}, err
	}

	passwordHash, err := s.passwordHasher.Hash(input.Password)
	if err != nil {
		return RegistrationResult{}, fmt.Errorf("hash password: %w", err)
	}

	rawToken, tokenHash, err := s.tokenGenerator.Generate()
	if err != nil {
		return RegistrationResult{}, fmt.Errorf("generate verification token: %w", err)
	}

	now := s.now().UTC()

	createdUser, err := s.repository.Create(
		ctx,
		CreationInput{
			ID:                    uuid.NewString(),
			Email:                 email,
			PasswordHash:          passwordHash,
			VerificationID:        uuid.NewString(),
			VerificationTokenHash: tokenHash,
			VerificationExpiresAt: now.Add(s.verificationTokenLifetime),
			CreatedAt:             now,
		},
	)

	if err != nil {
		return RegistrationResult{}, err
	}

	return RegistrationResult{
		User:                 createdUser,
		RawVerificationToken: rawToken,
	}, nil
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
