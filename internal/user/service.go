package user

import (
	"context"
	"net/mail"
	"strings"
	"unicode/utf8"
)

type Repository interface {
	Create(ctx context.Context, email string, password string) (User, error)
}

type PasswordHasher interface {
	Hash(password string) (string, error)
}

type Service struct {
	repository     Repository
	passwordHasher PasswordHasher
}

func NewService(repository Repository, passwordHasher PasswordHasher) *Service {
	return &Service{
		repository:     repository,
		passwordHasher: passwordHasher,
	}
}

func (s *Service) Register(ctx context.Context, data RegisterInput) (User, error) {
	email := normalizeEmail(data.Email)

	if err := validateEmail(email); err != nil {
		return User{}, err
	}

	if err := validatePassword(data.Password); err != nil {
		return User{}, err
	}

	passwordHash, err := s.passwordHasher.Hash(data.Password)
	if err != nil {
		return User{}, err
	}

	return s.repository.Create(
		ctx,
		email,
		passwordHash,
	)
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
