package token

import (
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

func (i *JWTManager) Issue(userID uuid.UUID, issuedAt time.Time) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256,
		jwt.RegisteredClaims{
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(issuedAt),
			ExpiresAt: jwt.NewNumericDate(issuedAt.Add(i.lifetime)),
		})

	strToken, err := token.SignedString(i.secret)
	if err != nil {
		return "", fmt.Errorf("failed signing token: %w", err)
	}

	return strToken, nil
}
