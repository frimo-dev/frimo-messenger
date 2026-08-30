package token

import (
	"errors"
	"fmt"
	"time"
	"uuid"

	"github.com/golang-jwt/jwt/v5"
)

type JWTManager struct {
	secret   []byte
	lifetime time.Duration
}

func NewJWTManager(secret []byte, lifetime time.Duration) *JWTManager {
	return &JWTManager{secret: secret, lifetime: lifetime}
}

func (m *JWTManager) Issue(userID uuid.UUID, issuedAt time.Time) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256,
		jwt.RegisteredClaims{
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(issuedAt),
			ExpiresAt: jwt.NewNumericDate(issuedAt.Add(m.lifetime)),
		})

	strToken, err := token.SignedString(m.secret)
	if err != nil {
		return "", fmt.Errorf("failed signing token: %w", err)
	}

	return strToken, nil
}

func (m *JWTManager) Verify(rawToken string) (uuid.UUID, error) {
	claims := &jwt.RegisteredClaims{}

	_, err := jwt.ParseWithClaims(
		rawToken,
		claims,
		func(token *jwt.Token) (any, error) {
			return m.secret, nil
		},
		jwt.WithValidMethods([]string{
			jwt.SigningMethodHS256.Alg(),
		}),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return uuid.Nil(), fmt.Errorf("%w: %v", ErrTokenExpired, err)
		}

		return uuid.Nil(), fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}

	subject, err := uuid.Parse(claims.Subject)
	if err != nil {
		return uuid.Nil(), fmt.Errorf("%w: invalid subject: %v", ErrInvalidToken, err)
	}

	return subject, nil
}
