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
	"github.com/frimo-dev/frimo-messenger/internal/security/token"
	"github.com/frimo-dev/frimo-messenger/internal/service/auth"
	"github.com/frimo-dev/frimo-messenger/internal/service/auth/mocks"
	"go.uber.org/mock/gomock"
)

func TestService_Register_Success(t *testing.T) {
	ctrl := gomock.NewController(t)

	repository := mocks.NewMockRepository(ctrl)
	jwtIssuer := mocks.NewMockAccessTokenIssuer(ctrl)
	passwordManager := mocks.NewMockPasswordManager(ctrl)
	tokenGenerator := mocks.NewMockVerificationTokenGenerator(ctrl)
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

	passwordManager.EXPECT().Hash("very-secure-password").Return("hashed-password", nil)

	rawToken := "raw-verification-token"
	tokenHash := []byte("verification-token-hash")
	tokenGenerator.EXPECT().Generate().Return(rawToken, tokenHash, nil)

	var capturedAAD []byte
	encryptedToken := []byte("encrypted-token")
	tokenCipher.
		EXPECT().
		Encrypt([]byte(rawToken), gomock.Any()).DoAndReturn(
		func(plaintext []byte, additionalData []byte) ([]byte, error) {
			capturedAAD = append([]byte(nil), additionalData...)
			return encryptedToken, nil
		})

	createdUser := auth.User{Email: "test@example.com"}
	var capturedInput auth.CreateUserInput
	repository.EXPECT().
		CreateUser(
			gomock.Any(),
			gomock.Any(),
		).
		DoAndReturn(func(_ context.Context, input auth.CreateUserInput) (auth.User, error) {
			capturedInput = input
			return createdUser, nil
		})

	verificationLifetime := 30 * time.Minute
	service := auth.NewService(
		repository,
		jwtIssuer,
		passwordManager,
		tokenGenerator,
		tokenCipher,
		func() time.Time {
			return now
		},
		verificationLifetime,
	)

	user, err := service.Register(context.Background(), input)
	if err != nil {
		t.Fatalf("Register() unexpected error: %v", err)
	}

	if user != createdUser {
		t.Errorf("expected user %+v, got %+v", createdUser, user)
	}

	if capturedInput.Email != "test@example.com" {
		t.Errorf("expected normalized email %q, got %q", "test@example.com", capturedInput.Email)
	}

	if capturedInput.PasswordHash != "hashed-password" {
		t.Errorf("expected password hash %q, got %q", "hashed-password", capturedInput.PasswordHash)
	}

	if capturedInput.ID == uuid.Nil() {
		t.Error("user ID must not be nil")
	}

	if !capturedInput.CreatedAt.Equal(now) {
		t.Errorf("expected created at %v, got %v", now, capturedInput.CreatedAt)
	}

	verification := capturedInput.Verification

	if verification.ID == uuid.Nil() {
		t.Error("verification ID must not be nil")
	}

	if !bytes.Equal(verification.TokenHash, tokenHash) {
		t.Errorf("unexpected verification token hash")
	}

	if !bytes.Equal(verification.TokenCiphertext, encryptedToken) {
		t.Errorf("unexpected verification token ciphertext")
	}

	expectedExpiresAt := now.Add(verificationLifetime)
	if !verification.ExpiresAt.Equal(expectedExpiresAt) {
		t.Errorf("expected expires at %v, got %v", expectedExpiresAt, verification.ExpiresAt)
	}

	// verification.ID должен быть использован как AAD при Encrypt
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
		t.Errorf("expected created at %v, got %v", now, event.CreatedAt)
	}

	if !event.AvailableAt.Equal(now) {
		t.Errorf("expected available at %v, got %v", now, event.AvailableAt)
	}

	var payload dto.EmailVerificationRequested

	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		t.Fatalf("unmarshal event payload: %v", err)
	}

	if payload.VerificationID != verification.ID {
		t.Errorf("payload verification ID does not match verification ID")
	}

	if payload.Recipient != "test@example.com" {
		t.Errorf("expected recipient %q, got %q", "test@example.com", payload.Recipient)
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
			tokenGenerator *mocks.MockVerificationTokenGenerator,
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
				_ *mocks.MockVerificationTokenGenerator,
				_ *mocks.MockVerificationTokenCipher,
			) {
				passwordManager.
					EXPECT().
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
				tokenGenerator *mocks.MockVerificationTokenGenerator,
				_ *mocks.MockVerificationTokenCipher,
			) {
				passwordManager.
					EXPECT().
					Hash("correct-password").
					Return("password-hash", nil)

				tokenGenerator.
					EXPECT().
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
				tokenGenerator *mocks.MockVerificationTokenGenerator,
				tokenCipher *mocks.MockVerificationTokenCipher,
			) {
				passwordManager.
					EXPECT().
					Hash("correct-password").
					Return("password-hash", nil)

				rawToken := "raw-verification-token"

				tokenGenerator.
					EXPECT().
					Generate().
					Return(
						rawToken,
						[]byte("verification-token-hash"),
						nil,
					)

				tokenCipher.
					EXPECT().
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
				tokenGenerator *mocks.MockVerificationTokenGenerator,
				tokenCipher *mocks.MockVerificationTokenCipher,
			) {
				passwordManager.
					EXPECT().
					Hash("correct-password").
					Return("password-hash", nil)

				rawToken := "raw-verification-token"

				tokenGenerator.
					EXPECT().
					Generate().
					Return(
						rawToken,
						[]byte("verification-token-hash"),
						nil,
					)

				tokenCipher.
					EXPECT().
					Encrypt([]byte(rawToken), gomock.Any()).
					Return([]byte("encrypted-token"), nil)

				repository.
					EXPECT().
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
			jwtIssuer := mocks.NewMockAccessTokenIssuer(ctrl)
			passwordManager := mocks.NewMockPasswordManager(ctrl)
			tokenGenerator := mocks.NewMockVerificationTokenGenerator(ctrl)
			tokenCipher := mocks.NewMockVerificationTokenCipher(ctrl)

			service := auth.NewService(
				repository,
				jwtIssuer,
				passwordManager,
				tokenGenerator,
				tokenCipher,
				time.Now,
				30*time.Minute,
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

func TestService_ConfirmEmail(t *testing.T) {
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

	repositoryErr := errors.New("repository failed")

	tests := []struct {
		name     string
		rawToken string

		prepare func(repository *mocks.MockRepository)

		wantErr error
	}{
		{
			name:     "success",
			rawToken: "raw-verification-token",

			prepare: func(repository *mocks.MockRepository) {
				expectedHash := token.Hash(
					"raw-verification-token",
				)

				repository.
					EXPECT().
					ConfirmEmail(
						gomock.Any(),
						expectedHash,
						now,
					).
					Return(nil)
			},
		},
		{
			name:     "empty token",
			rawToken: "",
			wantErr:  auth.ErrInvalidToken,
		},
		{
			name:     "repository error",
			rawToken: "raw-verification-token",

			prepare: func(repository *mocks.MockRepository) {
				expectedHash := token.Hash(
					"raw-verification-token",
				)

				repository.
					EXPECT().
					ConfirmEmail(
						gomock.Any(),
						expectedHash,
						now,
					).
					Return(repositoryErr)
			},

			wantErr: repositoryErr,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			repository := mocks.NewMockRepository(ctrl)
			jwtIssuer := mocks.NewMockAccessTokenIssuer(ctrl)
			passwordManager := mocks.NewMockPasswordManager(ctrl)
			tokenGenerator := mocks.NewMockVerificationTokenGenerator(ctrl)
			tokenCipher := mocks.NewMockVerificationTokenCipher(ctrl)

			service := auth.NewService(
				repository,
				jwtIssuer,
				passwordManager,
				tokenGenerator,
				tokenCipher,
				func() time.Time {
					return now
				},
				30*time.Minute,
			)

			if tt.prepare != nil {
				tt.prepare(repository)
			}

			err := service.ConfirmEmail(
				context.Background(),
				tt.rawToken,
			)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf(
						"expected error %v, got %v",
						tt.wantErr,
						err,
					)
				}

				return
			}

			if err != nil {
				t.Fatalf(
					"ConfirmEmail() unexpected error: %v",
					err,
				)
			}
		})
	}
}

func TestService_ResendVerification_Success(t *testing.T) {
	ctrl := gomock.NewController(t)

	repository := mocks.NewMockRepository(ctrl)
	jwtIssuer := mocks.NewMockAccessTokenIssuer(ctrl)
	passwordManager := mocks.NewMockPasswordManager(ctrl)
	tokenGenerator := mocks.NewMockVerificationTokenGenerator(ctrl)
	tokenCipher := mocks.NewMockVerificationTokenCipher(ctrl)

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

	verificationLifetime := 30 * time.Minute

	service := auth.NewService(
		repository,
		jwtIssuer,
		passwordManager,
		tokenGenerator,
		tokenCipher,
		func() time.Time {
			return now
		},
		verificationLifetime,
	)

	rawToken := "raw-verification-token"
	tokenHash := []byte("verification-token-hash")

	tokenGenerator.EXPECT().Generate().Return(rawToken, tokenHash, nil)

	encryptedToken := []byte("encrypted-token")

	var capturedAAD []byte

	tokenCipher.EXPECT().Encrypt([]byte(rawToken), gomock.Any()).DoAndReturn(
		func(_ []byte, additionalData []byte) ([]byte, error) {
			capturedAAD = append([]byte(nil), additionalData...)

			return encryptedToken, nil
		})

	var capturedInput auth.ResendVerificationInput

	repository.EXPECT().ResendVerification(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, input auth.ResendVerificationInput) error {
			capturedInput = input
			return nil
		})

	err := service.ResendVerification(context.Background(), " Test@Example.COM ")
	if err != nil {
		t.Fatalf("ResendVerification() unexpected error: %v", err)
	}

	if capturedInput.Email != "test@example.com" {
		t.Errorf("expected normalized email %q, got %q", "test@example.com", capturedInput.Email)
	}

	if !capturedInput.RequestedAt.Equal(now) {
		t.Errorf("expected RequestedAt %v, got %v", now, capturedInput.RequestedAt)
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
		t.Errorf("expected ExpiresAt %v, got %v", expectedExpiresAt, verification.ExpiresAt)
	}

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
		t.Errorf("expected event CreatedAt %v, got %v", now, event.CreatedAt)
	}

	if !event.AvailableAt.Equal(now) {
		t.Errorf("expected event AvailableAt %v, got %v", now, event.AvailableAt)
	}

	var payload dto.EmailVerificationRequested

	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		t.Fatalf("unmarshal event payload: %v", err)
	}

	if payload.VerificationID != verification.ID {
		t.Error("payload verification ID does not match verification ID")
	}

	if payload.Recipient != "test@example.com" {
		t.Errorf("expected recipient %q, got %q", "test@example.com", payload.Recipient)
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
			tokenGenerator *mocks.MockVerificationTokenGenerator,
			tokenCipher *mocks.MockVerificationTokenCipher,
		)

		wantErr  error
		wantCode string
	}{
		{
			name:     "invalid email",
			email:    "not-an-email",
			wantCode: "invalid_email",
		},
		{
			name:  "token generation error",
			email: "test@example.com",

			prepare: func(
				_ *mocks.MockRepository,
				tokenGenerator *mocks.MockVerificationTokenGenerator,
				_ *mocks.MockVerificationTokenCipher,
			) {
				tokenGenerator.
					EXPECT().
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
				tokenGenerator *mocks.MockVerificationTokenGenerator,
				tokenCipher *mocks.MockVerificationTokenCipher,
			) {
				rawToken := "raw-verification-token"

				tokenGenerator.
					EXPECT().
					Generate().
					Return(
						rawToken,
						[]byte("verification-token-hash"),
						nil,
					)

				tokenCipher.
					EXPECT().
					Encrypt(
						[]byte(rawToken),
						gomock.Any(),
					).
					Return(nil, encryptTokenErr)
			},

			wantErr: encryptTokenErr,
		},
		{
			name:  "repository error",
			email: "test@example.com",

			prepare: func(
				repository *mocks.MockRepository,
				tokenGenerator *mocks.MockVerificationTokenGenerator,
				tokenCipher *mocks.MockVerificationTokenCipher,
			) {
				rawToken := "raw-verification-token"

				tokenGenerator.
					EXPECT().
					Generate().
					Return(
						rawToken,
						[]byte("verification-token-hash"),
						nil,
					)

				tokenCipher.
					EXPECT().
					Encrypt(
						[]byte(rawToken),
						gomock.Any(),
					).
					Return(
						[]byte("encrypted-token"),
						nil,
					)

				repository.
					EXPECT().
					ResendVerification(
						gomock.Any(),
						gomock.Any(),
					).
					Return(repositoryErr)
			},

			wantErr: repositoryErr,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			repository := mocks.NewMockRepository(ctrl)
			jwtIssuer := mocks.NewMockAccessTokenIssuer(ctrl)
			passwordManager := mocks.NewMockPasswordManager(ctrl)
			tokenGenerator := mocks.NewMockVerificationTokenGenerator(ctrl)
			tokenCipher := mocks.NewMockVerificationTokenCipher(ctrl)

			service := auth.NewService(
				repository,
				jwtIssuer,
				passwordManager,
				tokenGenerator,
				tokenCipher,
				time.Now,
				30*time.Minute,
			)

			if tt.prepare != nil {
				tt.prepare(
					repository,
					tokenGenerator,
					tokenCipher,
				)
			}

			err := service.ResendVerification(
				context.Background(),
				tt.email,
			)

			if tt.wantCode != "" {
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
						"expected code %q, got %q",
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

func TestService_Login(t *testing.T) {
	now := time.Date(
		2026,
		time.August,
		30,
		12,
		0,
		0,
		0,
		time.UTC,
	)

	userID := uuid.New()
	verifiedAt := now.Add(-time.Hour)

	repositoryErr := errors.New("repository failed")
	verifyErr := errors.New("verify failed")
	issueErr := errors.New("issue token failed")

	tests := []struct {
		name     string
		email    string
		password string

		prepare func(
			repository *mocks.MockRepository,
			jwtIssuer *mocks.MockAccessTokenIssuer,
			passwordManager *mocks.MockPasswordManager,
		)

		wantToken string
		wantErr   error
		wantCode  string
	}{
		{
			name:     "success",
			email:    " Test@Example.COM ",
			password: "correct-password",

			prepare: func(
				repository *mocks.MockRepository,
				jwtIssuer *mocks.MockAccessTokenIssuer,
				passwordManager *mocks.MockPasswordManager,
			) {
				repository.
					EXPECT().
					GetUserForLogin(gomock.Any(), "test@example.com").
					Return(auth.LoginUser{
						ID:           userID,
						PasswordHash: "password-hash",
						VerifiedAt:   &verifiedAt,
					}, nil)

				passwordManager.
					EXPECT().
					Verify("password-hash", "correct-password").
					Return(nil)

				jwtIssuer.
					EXPECT().
					Issue(userID, now).
					Return("access-token", nil)
			},

			wantToken: "access-token",
		},
		{
			name:     "invalid email",
			email:    "not-an-email",
			password: "correct-password",
			wantCode: "invalid_email",
		},
		{
			name:     "password required",
			email:    "test@example.com",
			password: "",
			wantCode: "password_required",
		},
		{
			name:     "user not found",
			email:    "test@example.com",
			password: "correct-password",

			prepare: func(
				repository *mocks.MockRepository,
				_ *mocks.MockAccessTokenIssuer,
				_ *mocks.MockPasswordManager,
			) {
				repository.
					EXPECT().
					GetUserForLogin(gomock.Any(), "test@example.com").
					Return(auth.LoginUser{}, auth.ErrUserNotFound)
			},

			wantErr: auth.ErrInvalidCredentials,
		},
		{
			name:     "repository error",
			email:    "test@example.com",
			password: "correct-password",

			prepare: func(
				repository *mocks.MockRepository,
				_ *mocks.MockAccessTokenIssuer,
				_ *mocks.MockPasswordManager,
			) {
				repository.
					EXPECT().
					GetUserForLogin(gomock.Any(), "test@example.com").
					Return(auth.LoginUser{}, repositoryErr)
			},

			wantErr: repositoryErr,
		},
		{
			name:     "invalid password",
			email:    "test@example.com",
			password: "wrong-password",

			prepare: func(
				repository *mocks.MockRepository,
				_ *mocks.MockAccessTokenIssuer,
				passwordManager *mocks.MockPasswordManager,
			) {
				repository.
					EXPECT().
					GetUserForLogin(gomock.Any(), "test@example.com").
					Return(auth.LoginUser{
						ID:           userID,
						PasswordHash: "password-hash",
						VerifiedAt:   &verifiedAt,
					}, nil)

				passwordManager.
					EXPECT().
					Verify("password-hash", "wrong-password").
					Return(auth.ErrInvalidCredentials)
			},

			wantErr: auth.ErrInvalidCredentials,
		},
		{
			name:     "password verify error",
			email:    "test@example.com",
			password: "correct-password",

			prepare: func(
				repository *mocks.MockRepository,
				_ *mocks.MockAccessTokenIssuer,
				passwordManager *mocks.MockPasswordManager,
			) {
				repository.
					EXPECT().
					GetUserForLogin(gomock.Any(), "test@example.com").
					Return(auth.LoginUser{
						ID:           userID,
						PasswordHash: "password-hash",
						VerifiedAt:   &verifiedAt,
					}, nil)

				passwordManager.
					EXPECT().
					Verify("password-hash", "correct-password").
					Return(verifyErr)
			},

			wantErr: verifyErr,
		},
		{
			name:     "email not verified",
			email:    "test@example.com",
			password: "correct-password",

			prepare: func(
				repository *mocks.MockRepository,
				_ *mocks.MockAccessTokenIssuer,
				passwordManager *mocks.MockPasswordManager,
			) {
				repository.
					EXPECT().
					GetUserForLogin(gomock.Any(), "test@example.com").
					Return(auth.LoginUser{
						ID:           userID,
						PasswordHash: "password-hash",
						VerifiedAt:   nil,
					}, nil)

				passwordManager.
					EXPECT().
					Verify("password-hash", "correct-password").
					Return(nil)
			},

			wantErr: auth.ErrEmailNotVerified,
		},
		{
			name:     "access token issue error",
			email:    "test@example.com",
			password: "correct-password",

			prepare: func(
				repository *mocks.MockRepository,
				jwtIssuer *mocks.MockAccessTokenIssuer,
				passwordManager *mocks.MockPasswordManager,
			) {
				repository.
					EXPECT().
					GetUserForLogin(gomock.Any(), "test@example.com").
					Return(auth.LoginUser{
						ID:           userID,
						PasswordHash: "password-hash",
						VerifiedAt:   &verifiedAt,
					}, nil)

				passwordManager.
					EXPECT().
					Verify("password-hash", "correct-password").
					Return(nil)

				jwtIssuer.
					EXPECT().
					Issue(userID, now).
					Return("", issueErr)
			},

			wantErr: issueErr,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			repository := mocks.NewMockRepository(ctrl)
			jwtIssuer := mocks.NewMockAccessTokenIssuer(ctrl)
			passwordManager := mocks.NewMockPasswordManager(ctrl)
			tokenGenerator := mocks.NewMockVerificationTokenGenerator(ctrl)
			tokenCipher := mocks.NewMockVerificationTokenCipher(ctrl)

			service := auth.NewService(
				repository,
				jwtIssuer,
				passwordManager,
				tokenGenerator,
				tokenCipher,
				func() time.Time {
					return now
				},
				30*time.Minute,
			)

			if tt.prepare != nil {
				tt.prepare(
					repository,
					jwtIssuer,
					passwordManager,
				)
			}

			gotToken, err := service.Login(
				context.Background(),
				tt.email,
				tt.password,
			)

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

				if gotToken != "" {
					t.Errorf(
						"expected empty access token, got %q",
						gotToken,
					)
				}

				return
			}

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf(
						"expected error %v, got %v",
						tt.wantErr,
						err,
					)
				}

				if gotToken != "" {
					t.Errorf(
						"expected empty access token, got %q",
						gotToken,
					)
				}

				return
			}

			if err != nil {
				t.Fatalf("Login() unexpected error: %v", err)
			}

			if gotToken != tt.wantToken {
				t.Errorf(
					"expected access token %q, got %q",
					tt.wantToken,
					gotToken,
				)
			}
		})
	}
}
