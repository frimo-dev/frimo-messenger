package auth

import "errors"

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrEmailNotVerified   = errors.New("email not verified")
	ErrEmailAlreadyExists = errors.New("email already exists")

	ErrUserNotFound      = errors.New("user with such email does not exist")
	ErrAlreadyVerified   = errors.New("email already verified")
	ErrResendCooldown    = errors.New("verification resend cooldown")
	ErrResendHourlyLimit = errors.New("verification resend hourly limit")

	ErrInvalidToken = errors.New("invalid verification token")
	ErrExpiredToken = errors.New("verification token expired")
	ErrUsedToken    = errors.New("verification token already used")
	ErrRevokedToken = errors.New("verification token is revoked")

	ErrAccessTokenNotStored = errors.New("access token not stored")
	ErrAccessTokenInvalid   = errors.New("invalid access token")

	ErrSessionNotFound  = errors.New("session not found")
	ErrSessionCacheMiss = errors.New("session cache miss")
	ErrSessionInactive  = errors.New("session inactive")

	ErrRefreshTokenNotFound = errors.New("refresh token not found")
	ErrRefreshTokenReuse    = errors.New("refresh token already used")
	ErrRefreshRetry         = errors.New("refresh retry")
)

type ValidationError struct {
	Code    string
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return e.Message
}
