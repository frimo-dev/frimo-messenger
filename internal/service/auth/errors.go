package auth

import "errors"

var (
	ErrUserNotFound      = errors.New("user with such email does not exist")
	ErrAlreadyVerified   = errors.New("email already verified")
	ErrResendCooldown    = errors.New("verification resend cooldown")
	ErrResendHourlyLimit = errors.New("verification resend hourly limit")

	ErrInvalidToken = errors.New("invalid verification token")
	ErrExpiredToken = errors.New("verification token expired")
	ErrUsedToken    = errors.New("verification token already used")
	ErrRevokedToken = errors.New("verification token is revoked")
)

var ErrEmailAlreadyExists = errors.New("email already exists")

type ValidationError struct {
	Code    string
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return e.Message
}
