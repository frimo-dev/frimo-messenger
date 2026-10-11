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

func TestService_Register_Success(t *testing.T) {
	ctrl := gomock.NewController(t)

	repository := mocks.NewMockRepository(ctrl)
	passwordManager := mocks.NewMockPasswordManager(ctrl)
	tokenGenerator := mocks.NewMockTokenGenerator(ctrl)
	tokenCipher := mocks.NewMockVerificationTokenCipher(ctrl)

	input := auth.RegistrationInput{
		Email:    " Test@Example.COM ",
		Password: "very-secure-password",
	}

	now := time.Date(
		2026,
		time.August,
		26,
		12,
		0,
		0,
		0,
		time.UTC,
	)

	passwordManager.EXPECT().
		Hash("very-secure-password").
		Return("hashed-password", nil)

	rawToken := "raw-verification-token"
	tokenHash := []byte("verification-token-hash")

	tokenGenerator.EXPECT().
		Generate().
		Return(rawToken, tokenHash, nil)

	var capturedAAD []byte
	encryptedToken := []byte("encrypted-token")

	tokenCipher.EXPECT().
		Encrypt([]byte(rawToken), gomock.Any()).
		DoAndReturn(
			func(plaintext []byte, additionalData []byte) ([]byte, error) {
				capturedAAD = append([]byte(nil), additionalData...)
				return encryptedToken, nil
			},
		)

	createdUser := auth.User{
		Email: "test@example.com",
	}

	var capturedInput auth.CreateUserInput

	repository.EXPECT().
		CreateUser(gomock.Any(), gomock.Any()).
		DoAndReturn(
			func(_ context.Context, input auth.CreateUserInput) (auth.User, error) {
				capturedInput = input
				return createdUser, nil
			},
		)

	verificationLifetime := 30 * time.Minute

	service := auth.NewService(
		repository,
		nil, // SessionRepository
		nil, // AccessTokenStorage
		nil, // SessionStorage
		nil, // RefreshResultStorage
		nil, // Observer
		passwordManager,
		tokenGenerator,
		tokenCipher,
		func() time.Time {
			return now
		},
		verificationLifetime,
		15*time.Minute,  // accessTokenLifetime
		30*24*time.Hour, // sessionInactivityTimeout
		time.Minute,     // refreshRetryTTL
	)

	user, err := service.Register(context.Background(), input)
	if err != nil {
		t.Fatalf("Register() unexpected error: %v", err)
	}

	if user != createdUser {
		t.Errorf("expected user %+v, got %+v", createdUser, user)
	}

	if capturedInput.Email != "test@example.com" {
		t.Errorf(
			"expected normalized email %q, got %q",
			"test@example.com",
			capturedInput.Email,
		)
	}

	if capturedInput.PasswordHash != "hashed-password" {
		t.Errorf(
			"expected password hash %q, got %q",
			"hashed-password",
			capturedInput.PasswordHash,
		)
	}

	if capturedInput.ID == uuid.Nil() {
		t.Error("user ID must not be nil")
	}

	if !capturedInput.CreatedAt.Equal(now) {
		t.Errorf(
			"expected created at %v, got %v",
			now,
			capturedInput.CreatedAt,
		)
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
			"expected expires at %v, got %v",
			expectedExpiresAt,
			verification.ExpiresAt,
		)
	}

	// verification.ID должен использоваться как AAD при шифровании.
	if !bytes.Equal(capturedAAD, verification.ID[:]) {
		t.Error("cipher AAD must equal verification ID")
	}

	event := capturedInput.OutboxEvent

	if event.ID == uuid.Nil() {
		t.Error("outbox event ID must not be nil")
	}

	if event.Type != dto.EmailVerificationRequestedType {
		t.Errorf("unexpected event type %q", event.Type)
	}

	if !event.CreatedAt.Equal(now) {
		t.Errorf(
			"expected event CreatedAt %v, got %v",
			now,
			event.CreatedAt,
		)
	}

	if !event.AvailableAt.Equal(now) {
		t.Errorf(
			"expected event AvailableAt %v, got %v",
			now,
			event.AvailableAt,
		)
	}

	var payload dto.EmailVerificationRequested

	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		t.Fatalf("unmarshal event payload: %v", err)
	}

	if payload.VerificationID != verification.ID {
		t.Error("payload verification ID does not match verification ID")
	}

	if payload.Recipient != "test@example.com" {
		t.Errorf(
			"expected recipient %q, got %q",
			"test@example.com",
			payload.Recipient,
		)
	}
}

func TestService_Register_Errors(t *testing.T) {
	hashErr := errors.New("hash failed")
	tokenGenerationErr := errors.New("token generation failed")
	tokenEncryptionErr := errors.New("token encryption failed")
	repositoryErr := errors.New("repository failed")

	tests := []struct {
		name  string
		input auth.RegistrationInput

		prepare func(
			repository *mocks.MockRepository,
			passwordManager *mocks.MockPasswordManager,
			tokenGenerator *mocks.MockTokenGenerator,
			tokenCipher *mocks.MockVerificationTokenCipher,
		)

		wantErr  error
		wantCode string
	}{
		{
			name: "invalid email",
			input: auth.RegistrationInput{
				Email:    "not-an-email",
				Password: "very-secure-password",
			},
			wantCode: "invalid_email",
		},
		{
			name: "password too short",
			input: auth.RegistrationInput{
				Email:    "correct@example.com",
				Password: strings.Repeat("p", 11),
			},
			wantCode: "password_too_short",
		},
		{
			name: "password too long",
			input: auth.RegistrationInput{
				Email:    "correct@example.com",
				Password: strings.Repeat("p", 129),
			},
			wantCode: "password_too_long",
		},
		{
			name: "password hash error",
			input: auth.RegistrationInput{
				Email:    "correct@example.com",
				Password: "correct-password",
			},

			prepare: func(
				_ *mocks.MockRepository,
				passwordManager *mocks.MockPasswordManager,
				_ *mocks.MockTokenGenerator,
				_ *mocks.MockVerificationTokenCipher,
			) {
				passwordManager.EXPECT().
					Hash("correct-password").
					Return("", hashErr)
			},

			wantErr: hashErr,
		},
		{
			name: "token generation error",
			input: auth.RegistrationInput{
				Email:    "correct@example.com",
				Password: "correct-password",
			},

			prepare: func(
				_ *mocks.MockRepository,
				passwordManager *mocks.MockPasswordManager,
				tokenGenerator *mocks.MockTokenGenerator,
				_ *mocks.MockVerificationTokenCipher,
			) {
				passwordManager.EXPECT().
					Hash("correct-password").
					Return("password-hash", nil)

				tokenGenerator.EXPECT().
					Generate().
					Return("", nil, tokenGenerationErr)
			},

			wantErr: tokenGenerationErr,
		},
		{
			name: "token encryption error",
			input: auth.RegistrationInput{
				Email:    "correct@example.com",
				Password: "correct-password",
			},

			prepare: func(
				_ *mocks.MockRepository,
				passwordManager *mocks.MockPasswordManager,
				tokenGenerator *mocks.MockTokenGenerator,
				tokenCipher *mocks.MockVerificationTokenCipher,
			) {
				passwordManager.EXPECT().
					Hash("correct-password").
					Return("password-hash", nil)

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
					Return(nil, tokenEncryptionErr)
			},

			wantErr: tokenEncryptionErr,
		},
		{
			name: "repository error",
			input: auth.RegistrationInput{
				Email:    "correct@example.com",
				Password: "correct-password",
			},

			prepare: func(
				repository *mocks.MockRepository,
				passwordManager *mocks.MockPasswordManager,
				tokenGenerator *mocks.MockTokenGenerator,
				tokenCipher *mocks.MockVerificationTokenCipher,
			) {
				passwordManager.EXPECT().
					Hash("correct-password").
					Return("password-hash", nil)

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
					CreateUser(gomock.Any(), gomock.Any()).
					Return(auth.User{}, repositoryErr)
			},

			wantErr: repositoryErr,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			repository := mocks.NewMockRepository(ctrl)
			passwordManager := mocks.NewMockPasswordManager(ctrl)
			tokenGenerator := mocks.NewMockTokenGenerator(ctrl)
			tokenCipher := mocks.NewMockVerificationTokenCipher(ctrl)

			service := auth.NewService(
				repository,
				nil, // SessionRepository
				nil, // AccessTokenStorage
				nil, // SessionStorage
				nil, // RefreshResultStorage
				nil, // Observer
				passwordManager,
				tokenGenerator,
				tokenCipher,
				time.Now,
				30*time.Minute,  // verificationTokenLifetime
				15*time.Minute,  // accessTokenLifetime
				30*24*time.Hour, // sessionInactivityTimeout
				time.Minute,     // refreshRetryTTL
			)

			if tt.prepare != nil {
				tt.prepare(
					repository,
					passwordManager,
					tokenGenerator,
					tokenCipher,
				)
			}

			user, err := service.Register(
				context.Background(),
				tt.input,
			)

			if user != (auth.User{}) {
				t.Errorf(
					"expected empty user, got %+v",
					user,
				)
			}

			if tt.wantCode != "" {
				if err == nil {
					t.Fatal("expected error, got nil")
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
						"expected validation code %q, got %q",
						tt.wantCode,
						validationErr.Code,
					)
				}

				return
			}

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf(
					"expected error %v, got %v",
					tt.wantErr,
					err,
				)
			}
		})
	}
}
