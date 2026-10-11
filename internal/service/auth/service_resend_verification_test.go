package auth_test

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"errors"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/frimo-dev/frimo-messenger/internal/dto"
	"github.com/frimo-dev/frimo-messenger/internal/service/auth"
	"github.com/frimo-dev/frimo-messenger/internal/service/auth/mocks"
	"go.uber.org/mock/gomock"
)

func TestService_ResendVerification_Success(t *testing.T) {
	ctrl := gomock.NewController(t)

	repository := mocks.NewMockRepository(ctrl)
	tokenGenerator := mocks.NewMockTokenGenerator(ctrl)
	tokenCipher := mocks.NewMockVerificationTokenCipher(ctrl)

	now := time.Date(2026, time.August, 26, 12, 0, 0, 0, time.UTC)
	verificationLifetime := 30 * time.Minute

	service := auth.NewService(
		repository,
		nil, // SessionRepository
		nil, // AccessTokenStorage
		nil, // SessionStorage
		nil, // RefreshResultStorage
		nil, // Observer
		nil, // PasswordManager
		tokenGenerator,
		tokenCipher,
		func() time.Time { return now },
		verificationLifetime,
		15*time.Minute,
		30*24*time.Hour,
		time.Minute,
	)

	rawToken := "raw-verification-token"
	tokenHash := []byte("verification-token-hash")
	encryptedToken := []byte("encrypted-token")

	tokenGenerator.EXPECT().
		Generate().
		Return(rawToken, tokenHash, nil)

	var capturedAAD []byte

	tokenCipher.EXPECT().
		Encrypt([]byte(rawToken), gomock.Any()).
		DoAndReturn(func(_ []byte, aad []byte) ([]byte, error) {
			capturedAAD = append([]byte(nil), aad...)
			return encryptedToken, nil
		})

	var capturedInput auth.ResendVerificationInput

	repository.EXPECT().
		ResendVerification(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, input auth.ResendVerificationInput) error {
			capturedInput = input
			return nil
		})

	err := service.ResendVerification(
		context.Background(),
		" Test@Example.COM ",
	)
	if err != nil {
		t.Fatalf("ResendVerification() unexpected error: %v", err)
	}

	if capturedInput.Email != "test@example.com" {
		t.Errorf("Email = %q, want %q", capturedInput.Email, "test@example.com")
	}

	if !capturedInput.RequestedAt.Equal(now) {
		t.Errorf("RequestedAt = %v, want %v", capturedInput.RequestedAt, now)
	}

	verification := capturedInput.Verification

	if verification.ID == uuid.Nil() {
		t.Error("verification ID must not be nil")
	}

	if !bytes.Equal(verification.TokenHash, tokenHash) {
		t.Error("unexpected verification token hash")
	}

	if !bytes.Equal(verification.TokenCiphertext, encryptedToken) {
		t.Error("unexpected verification token ciphertext")
	}

	expectedExpiresAt := now.Add(verificationLifetime)
	if !verification.ExpiresAt.Equal(expectedExpiresAt) {
		t.Errorf(
			"ExpiresAt = %v, want %v",
			verification.ExpiresAt,
			expectedExpiresAt,
		)
	}

	if !bytes.Equal(capturedAAD, verification.ID[:]) {
		t.Error("cipher AAD must equal verification ID")
	}

	event := capturedInput.OutboxEvent

	if event.ID == uuid.Nil() {
		t.Error("outbox event ID must not be nil")
	}

	if event.Type != dto.EmailVerificationRequestedType {
		t.Errorf("event Type = %q, want %q", event.Type, dto.EmailVerificationRequestedType)
	}

	if !event.CreatedAt.Equal(now) {
		t.Errorf("event CreatedAt = %v, want %v", event.CreatedAt, now)
	}

	if !event.AvailableAt.Equal(now) {
		t.Errorf("event AvailableAt = %v, want %v", event.AvailableAt, now)
	}

	var payload dto.EmailVerificationRequested

	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		t.Fatalf("unmarshal event payload: %v", err)
	}

	if payload.VerificationID != verification.ID {
		t.Error("payload verification ID does not match verification ID")
	}

	if payload.Recipient != "test@example.com" {
		t.Errorf("payload Recipient = %q, want %q", payload.Recipient, "test@example.com")
	}
}

func TestService_ResendVerification_Errors(t *testing.T) {
	generateTokenErr := errors.New("token generation failed")
	encryptTokenErr := errors.New("token encryption failed")
	repositoryErr := errors.New("repository failed")

	tests := []struct {
		name  string
		email string

		prepare func(
			repository *mocks.MockRepository,
			tokenGenerator *mocks.MockTokenGenerator,
			tokenCipher *mocks.MockVerificationTokenCipher,
		)

		wantErr  error
		wantCode string
	}{
		{
			name:     "empty email",
			email:    "   ",
			wantCode: "email_required",
		},
		{
			name:     "invalid email",
			email:    "not-an-email",
			wantCode: "invalid_email",
		},
		{
			name:     "email too long",
			email:    strings.Repeat("a", 250) + "@example.com",
			wantCode: "email_too_long",
		},
		{
			name:  "token generation error",
			email: "test@example.com",

			prepare: func(
				_ *mocks.MockRepository,
				tokenGenerator *mocks.MockTokenGenerator,
				_ *mocks.MockVerificationTokenCipher,
			) {
				tokenGenerator.EXPECT().
					Generate().
					Return("", nil, generateTokenErr)
			},

			wantErr: generateTokenErr,
		},
		{
			name:  "token encryption error",
			email: "test@example.com",

			prepare: func(
				_ *mocks.MockRepository,
				tokenGenerator *mocks.MockTokenGenerator,
				tokenCipher *mocks.MockVerificationTokenCipher,
			) {
				rawToken := "raw-verification-token"

				tokenGenerator.EXPECT().
					Generate().
					Return(
						rawToken,
						[]byte("verification-token-hash"),
						nil,
					)

				tokenCipher.EXPECT().
					Encrypt([]byte(rawToken), gomock.Any()).
					Return(nil, encryptTokenErr)
			},

			wantErr: encryptTokenErr,
		},
		{
			name:  "repository error",
			email: "test@example.com",

			prepare: func(
				repository *mocks.MockRepository,
				tokenGenerator *mocks.MockTokenGenerator,
				tokenCipher *mocks.MockVerificationTokenCipher,
			) {
				rawToken := "raw-verification-token"

				tokenGenerator.EXPECT().
					Generate().
					Return(
						rawToken,
						[]byte("verification-token-hash"),
						nil,
					)

				tokenCipher.EXPECT().
					Encrypt([]byte(rawToken), gomock.Any()).
					Return([]byte("encrypted-token"), nil)

				repository.EXPECT().
					ResendVerification(gomock.Any(), gomock.Any()).
					Return(repositoryErr)
			},

			wantErr: repositoryErr,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			repository := mocks.NewMockRepository(ctrl)
			tokenGenerator := mocks.NewMockTokenGenerator(ctrl)
			tokenCipher := mocks.NewMockVerificationTokenCipher(ctrl)

			service := auth.NewService(
				repository,
				nil, // SessionRepository
				nil, // AccessTokenStorage
				nil, // SessionStorage
				nil, // RefreshResultStorage
				nil, // Observer
				nil, // PasswordManager
				tokenGenerator,
				tokenCipher,
				time.Now,
				30*time.Minute,
				15*time.Minute,
				30*24*time.Hour,
				time.Minute,
			)

			if tt.prepare != nil {
				tt.prepare(repository, tokenGenerator, tokenCipher)
			}

			err := service.ResendVerification(
				context.Background(),
				tt.email,
			)

			if tt.wantCode != "" {
				if err == nil {
					t.Fatal("expected validation error, got nil")
				}

				var validationErr *auth.ValidationError

				if !errors.As(err, &validationErr) {
					t.Fatalf(
						"expected ValidationError, got %T: %v",
						err,
						err,
					)
				}

				if validationErr.Code != tt.wantCode {
					t.Errorf(
						"validation code = %q, want %q",
						validationErr.Code,
						tt.wantCode,
					)
				}

				return
			}

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf(
					"ResendVerification() error = %v, want %v",
					err,
					tt.wantErr,
				)
			}
		})
	}
}
