package emailverification

import "errors"

var (
	ErrInvalidToken = errors.New("invalid verification token")
	ErrExpiredToken = errors.New("verification token expired")
	ErrUsedToken    = errors.New("verification token already used")
	ErrRevokedToken = errors.New("verification token is revoked")
)
