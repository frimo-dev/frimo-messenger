package auth

import (
	"crypto/rand"
	"math/big"
	"net/mail"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	usernamePrefix = "user_"
	randomLength   = 15

	alphabet = "abcdefghijklmnopqrstuvwxyz0123456789"
)

func generateUsername() (string, error) {
	var builder strings.Builder
	builder.Grow(len(usernamePrefix) + randomLength)

	builder.WriteString(usernamePrefix)

	max := big.NewInt(int64(len(alphabet)))

	for range randomLength {
		index, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}

		builder.WriteByte(alphabet[index.Int64()])
	}

	return builder.String(), nil
}

func onlyLetters(s string) bool {
	if s == "" {
		return false
	}

	for _, r := range s {
		if !unicode.IsLetter(r) {
			return false
		}
	}

	return true
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
	if password == "" {
		return &ValidationError{
			Code:    "password_required",
			Field:   "password",
			Message: "password is required",
		}
	}

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

func validateName(name string) error {
	if name == "" {
		return &ValidationError{
			Code:    "name_required",
			Field:   "name",
			Message: "name is required",
		}
	}

	length := utf8.RuneCountInString(name)

	if length < 1 {
		return &ValidationError{
			Code:    "name_too_short",
			Field:   "name",
			Message: "name must contain at least 1 characters",
		}
	}

	if length > 30 {
		return &ValidationError{
			Code:    "name_too_long",
			Field:   "name",
			Message: "name must contain at most 30 characters",
		}
	}

	if !onlyLetters(name) {
		return &ValidationError{
			Code:    "invalid_name",
			Field:   "name",
			Message: "name must contain only letters",
		}
	}

	return nil
}

func validateSurname(surname string) error {
	if surname == "" {
		return nil
	}

	length := utf8.RuneCountInString(surname)

	if length < 1 {
		return &ValidationError{
			Code:    "surname_too_short",
			Field:   "surname",
			Message: "surname must contain at least 1 characters",
		}
	}

	if length > 30 {
		return &ValidationError{
			Code:    "surname_too_long",
			Field:   "surname",
			Message: "surname must contain at most 30 characters",
		}
	}

	if !onlyLetters(surname) {
		return &ValidationError{
			Code:    "invalid_surname",
			Field:   "surname",
			Message: "surname must contain only letters",
		}
	}

	return nil
}

func validateDeviceName(deviceName string) error {
	if deviceName == "" {
		return &ValidationError{
			Code:    "device_name_required",
			Field:   "device_name",
			Message: "device name is required",
		}
	}

	length := utf8.RuneCountInString(deviceName)

	if length < 3 {
		return &ValidationError{
			Code:    "device_name_too_short",
			Field:   "device_name",
			Message: "device name must contain at least 3 characters",
		}
	}

	if length > 64 {
		return &ValidationError{
			Code:    "device_name_too_long",
			Field:   "device_name",
			Message: "device name must contain at most 64 characters",
		}
	}

	return nil
}

func validateUsername(username string) error {
	length := utf8.RuneCountInString(username)

	if length < 3 {
		return &ValidationError{
			Code:    "username_too_short",
			Field:   "username",
			Message: "username must contain at least 3 characters",
		}
	}

	if length > 20 {
		return &ValidationError{
			Code:    "username_too_long",
			Field:   "username",
			Message: "username must contain at most 20 characters",
		}
	}

	hasLetter := false

	for _, r := range username {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z':
			hasLetter = true
		case r >= '0' && r <= '9', r == '-' || r == '_':
		default:
			return &ValidationError{
				Code:    "username_invalid_characters",
				Field:   "username",
				Message: "username may contain only English letters, numbers, '-' and '_'",
			}
		}
	}

	if !hasLetter {
		return &ValidationError{
			Code:    "username_letter_required",
			Field:   "username",
			Message: "username must contain at least one English letter",
		}
	}

	return nil
}
