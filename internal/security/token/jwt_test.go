package token_test

import (
	"errors"
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

	jwtIssuer := token.NewJWTManager(secret, lifetime)

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

func TestJWTManager_Verify(t *testing.T) {
	secret := []byte("some-test-secret")
	manager := token.NewJWTManager(secret, 15*time.Minute)

	userID := uuid.New()

	signToken := func(
		t *testing.T,
		signingMethod jwt.SigningMethod,
		claims jwt.RegisteredClaims,
		signingKey any,
	) string {
		t.Helper()

		jwtToken := jwt.NewWithClaims(
			signingMethod,
			claims,
		)

		rawToken, err := jwtToken.SignedString(signingKey)
		if err != nil {
			t.Fatalf("sign token: %v", err)
		}

		return rawToken
	}

	tests := []struct {
		name string

		rawToken func(t *testing.T) string

		wantUserID uuid.UUID
		wantErr    error
	}{
		{
			name: "success",

			rawToken: func(t *testing.T) string {
				now := time.Now()

				return signToken(
					t,
					jwt.SigningMethodHS256,
					jwt.RegisteredClaims{
						Subject: userID.String(),
						IssuedAt: jwt.NewNumericDate(
							now.Add(-time.Minute),
						),
						ExpiresAt: jwt.NewNumericDate(
							now.Add(time.Minute),
						),
					},
					secret,
				)
			},

			wantUserID: userID,
		},
		{
			name: "expired token",

			rawToken: func(t *testing.T) string {
				now := time.Now()

				return signToken(
					t,
					jwt.SigningMethodHS256,
					jwt.RegisteredClaims{
						Subject: userID.String(),
						IssuedAt: jwt.NewNumericDate(
							now.Add(-2 * time.Hour),
						),
						ExpiresAt: jwt.NewNumericDate(
							now.Add(-time.Hour),
						),
					},
					secret,
				)
			},

			wantErr: token.ErrTokenExpired,
		},
		{
			name: "invalid signature",

			rawToken: func(t *testing.T) string {
				now := time.Now()

				return signToken(
					t,
					jwt.SigningMethodHS256,
					jwt.RegisteredClaims{
						Subject: userID.String(),
						ExpiresAt: jwt.NewNumericDate(
							now.Add(time.Hour),
						),
					},
					[]byte("wrong-secret"),
				)
			},

			wantErr: token.ErrInvalidToken,
		},
		{
			name: "expiration missing",

			rawToken: func(t *testing.T) string {
				return signToken(
					t,
					jwt.SigningMethodHS256,
					jwt.RegisteredClaims{
						Subject: userID.String(),
					},
					secret,
				)
			},

			wantErr: token.ErrInvalidToken,
		},
		{
			name: "invalid subject",

			rawToken: func(t *testing.T) string {
				now := time.Now()

				return signToken(
					t,
					jwt.SigningMethodHS256,
					jwt.RegisteredClaims{
						Subject: "not-a-uuid",
						ExpiresAt: jwt.NewNumericDate(
							now.Add(time.Hour),
						),
					},
					secret,
				)
			},

			wantErr: token.ErrInvalidToken,
		},
		{
			name: "invalid signing method",

			rawToken: func(t *testing.T) string {
				now := time.Now()

				return signToken(
					t,
					jwt.SigningMethodHS384,
					jwt.RegisteredClaims{
						Subject: userID.String(),
						ExpiresAt: jwt.NewNumericDate(
							now.Add(time.Hour),
						),
					},
					secret,
				)
			},

			wantErr: token.ErrInvalidToken,
		},
		{
			name: "malformed token",

			rawToken: func(t *testing.T) string {
				return "not-a-jwt"
			},

			wantErr: token.ErrInvalidToken,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotUserID, err := manager.Verify(
				tt.rawToken(t),
			)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf(
						"expected error %v, got %v",
						tt.wantErr,
						err,
					)
				}

				if gotUserID != uuid.Nil() {
					t.Errorf(
						"expected nil user ID, got %v",
						gotUserID,
					)
				}

				return
			}

			if err != nil {
				t.Fatalf(
					"Verify() unexpected error: %v",
					err,
				)
			}

			if gotUserID != tt.wantUserID {
				t.Errorf(
					"expected user ID %v, got %v",
					tt.wantUserID,
					gotUserID,
				)
			}
		})
	}
}
