package token_test

import (
	"testing"
	"time"
	"uuid"

	"github.com/frimo-dev/frimo-messenger/internal/security/token"
	"github.com/golang-jwt/jwt/v5"
)

func TestJWTIssuer_Success(t *testing.T) {
	userID := uuid.New()
	issuedAt := time.Date(
		2026, 8, 29,
		12, 0, 0, 0,
		time.UTC,
	)

	secret := []byte("some-test-secret")
	lifetime := 15 * time.Minute

	jwtIssuer := token.NewJWTIssuer(secret, lifetime)

	strJWT, err := jwtIssuer.Issue(userID, issuedAt)
	if err != nil {
		t.Fatalf("failed to issue token: %v", err)
	}

	claims := &jwt.RegisteredClaims{}

	parsedToken, err := jwt.ParseWithClaims(
		strJWT,
		claims,
		func(token *jwt.Token) (any, error) {
			return secret, nil
		},
		jwt.WithValidMethods([]string{
			jwt.SigningMethodHS256.Alg(),
		}),
		jwt.WithTimeFunc(func() time.Time {
			return issuedAt.Add(5 * time.Minute)
		}),
	)
	if err != nil {
		t.Fatalf("failed to parse token: %v", err)
	}

	if !parsedToken.Valid {
		t.Fatal("expected token to be valid")
	}

	if claims.Subject != userID.String() {
		t.Errorf(
			"unexpected subject: got %q, want %q",
			claims.Subject,
			userID.String(),
		)
	}

	if !claims.IssuedAt.Time.Equal(issuedAt) {
		t.Errorf(
			"unexpected issued at: got %v, want %v",
			claims.IssuedAt.Time,
			issuedAt,
		)
	}

	expectedExpiresAt := issuedAt.Add(lifetime)

	if !claims.ExpiresAt.Time.Equal(expectedExpiresAt) {
		t.Errorf(
			"unexpected expires at: got %v, want %v",
			claims.ExpiresAt.Time,
			expectedExpiresAt,
		)
	}
}
