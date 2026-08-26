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
	passwordHasher := mocks.NewMockPasswordHasher(ctrl)
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

	passwordHasher.EXPECT().Hash("very-secure-password").Return("hashed-password", nil)

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
		passwordHasher,
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

func TestService_Register_InvalidEmail(t *testing.T) {
	ctrl := gomock.NewController(t)

	repository := mocks.NewMockRepository(ctrl)
	passwordHasher := mocks.NewMockPasswordHasher(ctrl)
	tokenGenerator := mocks.NewMockVerificationTokenGenerator(ctrl)
	tokenCipher := mocks.NewMockVerificationTokenCipher(ctrl)

	service := auth.NewService(
		repository,
		passwordHasher,
		tokenGenerator,
		tokenCipher,
		time.Now,
		30*time.Minute,
	)

	_, err := service.Register(context.Background(), auth.RegistrationInput{
		Email:    "not-an-email",
		Password: "very-secure-password",
	})

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var validationErr *auth.ValidationError

	if !errors.As(err, &validationErr) {
		t.Fatalf("expected ValidationError, got %T: %v", err, err)
	}

	if validationErr.Code != "invalid_email" {
		t.Errorf("expected code %q, got %q", "invalid_email", validationErr.Code)
	}
}

func TestService_Register_PasswordTooShort(t *testing.T) {
	ctrl := gomock.NewController(t)

	repository := mocks.NewMockRepository(ctrl)
	passwordHasher := mocks.NewMockPasswordHasher(ctrl)
	tokenGenerator := mocks.NewMockVerificationTokenGenerator(ctrl)
	tokenCipher := mocks.NewMockVerificationTokenCipher(ctrl)

	service := auth.NewService(
		repository,
		passwordHasher,
		tokenGenerator,
		tokenCipher,
		time.Now,
		30*time.Minute,
	)

	_, err := service.Register(context.Background(), auth.RegistrationInput{
		Email:    "correct@example.com",
		Password: strings.Repeat("p", 11),
	})

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var validationErr *auth.ValidationError

	if !errors.As(err, &validationErr) {
		t.Fatalf("expected ValidationError, got %T: %v", err, err)
	}

	if validationErr.Code != "password_too_short" {
		t.Errorf("expected code %q, got %q", "password_too_short", validationErr.Code)
	}
}

func TestService_Register_PasswordTooLong(t *testing.T) {
	ctrl := gomock.NewController(t)

	repository := mocks.NewMockRepository(ctrl)
	passwordHasher := mocks.NewMockPasswordHasher(ctrl)
	tokenGenerator := mocks.NewMockVerificationTokenGenerator(ctrl)
	tokenCipher := mocks.NewMockVerificationTokenCipher(ctrl)

	service := auth.NewService(
		repository,
		passwordHasher,
		tokenGenerator,
		tokenCipher,
		time.Now,
		30*time.Minute,
	)

	_, err := service.Register(context.Background(), auth.RegistrationInput{
		Email:    "correct@example.com",
		Password: strings.Repeat("p", 129),
	})

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var validationErr *auth.ValidationError

	if !errors.As(err, &validationErr) {
		t.Fatalf("expected ValidationError, got %T: %v", err, err)
	}

	if validationErr.Code != "password_too_long" {
		t.Errorf("expected code %q, got %q", "password_too_long", validationErr.Code)
	}
}

func TestService_Register_PasswordHashError(t *testing.T) {
	ctrl := gomock.NewController(t)

	repository := mocks.NewMockRepository(ctrl)
	passwordHasher := mocks.NewMockPasswordHasher(ctrl)
	tokenGenerator := mocks.NewMockVerificationTokenGenerator(ctrl)
	tokenCipher := mocks.NewMockVerificationTokenCipher(ctrl)

	service := auth.NewService(
		repository,
		passwordHasher,
		tokenGenerator,
		tokenCipher,
		time.Now,
		30*time.Minute,
	)

	input := auth.RegistrationInput{
		Email:    "correct@example.com",
		Password: "correct-password",
	}

	hashErr := errors.New("hash failed")
	passwordHasher.EXPECT().Hash(input.Password).Return("", hashErr)

	createdUser, err := service.Register(context.Background(), input)

	if !errors.Is(err, hashErr) {
		t.Fatalf("expected hash error, got %v", err)
	}

	if createdUser != (auth.User{}) {
		t.Errorf("expected empty user, got %v", createdUser)
	}
}

func TestService_Register_TokenGenerationError(t *testing.T) {
	ctrl := gomock.NewController(t)

	repository := mocks.NewMockRepository(ctrl)
	passwordHasher := mocks.NewMockPasswordHasher(ctrl)
	tokenGenerator := mocks.NewMockVerificationTokenGenerator(ctrl)
	tokenCipher := mocks.NewMockVerificationTokenCipher(ctrl)

	service := auth.NewService(
		repository,
		passwordHasher,
		tokenGenerator,
		tokenCipher,
		time.Now,
		30*time.Minute,
	)

	input := auth.RegistrationInput{
		Email:    "correct@example.com",
		Password: "correct-password",
	}

	tokenGeneratorErr := errors.New("token generation failed")

	passwordHasher.EXPECT().Hash(input.Password).Return("password-hash", nil)
	tokenGenerator.EXPECT().Generate().Return("", []byte{}, tokenGeneratorErr)

	createdUser, err := service.Register(context.Background(), input)

	if !errors.Is(err, tokenGeneratorErr) {
		t.Fatalf("expected token generator error, got %v", err)
	}

	if createdUser != (auth.User{}) {
		t.Errorf("expected empty user, got %v", createdUser)
	}
}

func TestService_Register_TokenEncryptionError(t *testing.T) {
	ctrl := gomock.NewController(t)

	repository := mocks.NewMockRepository(ctrl)
	passwordHasher := mocks.NewMockPasswordHasher(ctrl)
	tokenGenerator := mocks.NewMockVerificationTokenGenerator(ctrl)
	tokenCipher := mocks.NewMockVerificationTokenCipher(ctrl)

	service := auth.NewService(
		repository,
		passwordHasher,
		tokenGenerator,
		tokenCipher,
		time.Now,
		30*time.Minute,
	)

	input := auth.RegistrationInput{
		Email:    "correct@example.com",
		Password: "correct-password",
	}

	passwordHasher.EXPECT().Hash(input.Password).Return("password-hash", nil)

	rawToken := "raw-verification-token"
	tokenHash := []byte("verification-token-hash")
	tokenGenerator.EXPECT().Generate().Return(rawToken, tokenHash, nil)

	tokenEncryptionErr := errors.New("token encryption failed")
	tokenCipher.EXPECT().Encrypt([]byte(rawToken), gomock.Any()).Return([]byte{}, tokenEncryptionErr)

	createdUser, err := service.Register(context.Background(), input)

	if !errors.Is(err, tokenEncryptionErr) {
		t.Fatalf("expected token encryption error, got %v", err)
	}

	if createdUser != (auth.User{}) {
		t.Errorf("expected empty user, got %v", createdUser)
	}
}

func TestService_Register_RepositoryError(t *testing.T) {
	ctrl := gomock.NewController(t)

	repository := mocks.NewMockRepository(ctrl)
	passwordHasher := mocks.NewMockPasswordHasher(ctrl)
	tokenGenerator := mocks.NewMockVerificationTokenGenerator(ctrl)
	tokenCipher := mocks.NewMockVerificationTokenCipher(ctrl)

	service := auth.NewService(
		repository,
		passwordHasher,
		tokenGenerator,
		tokenCipher,
		time.Now,
		30*time.Minute,
	)

	input := auth.RegistrationInput{
		Email:    "correct@example.com",
		Password: "correct-password",
	}

	passwordHasher.EXPECT().Hash(input.Password).Return("password-hash", nil)

	rawToken := "raw-verification-token"
	tokenHash := []byte("verification-token-hash")
	tokenGenerator.EXPECT().Generate().Return(rawToken, tokenHash, nil)

	tokenCipher.EXPECT().Encrypt([]byte(rawToken), gomock.Any()).Return([]byte("encrypted-token"), nil)

	repositoryErr := errors.New("repository failed")
	repository.EXPECT().CreateUser(gomock.Any(), gomock.Any()).Return(auth.User{}, repositoryErr)

	createdUser, err := service.Register(context.Background(), input)

	if !errors.Is(err, repositoryErr) {
		t.Fatalf("expected repository error, got %v", err)
	}

	if createdUser != (auth.User{}) {
		t.Errorf("expected empty user, got %v", createdUser)
	}
}

func TestService_ConfirmEmail_Success(t *testing.T) {
	ctrl := gomock.NewController(t)

	repository := mocks.NewMockRepository(ctrl)
	passwordHasher := mocks.NewMockPasswordHasher(ctrl)
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

	service := auth.NewService(
		repository,
		passwordHasher,
		tokenGenerator,
		tokenCipher,
		func() time.Time {
			return now
		},
		30*time.Minute,
	)

	rawToken := "raw-verification-token"
	expectedHash := token.Hash(rawToken)

	repository.EXPECT().ConfirmEmail(gomock.Any(), expectedHash, now).Return(nil)

	err := service.ConfirmEmail(context.Background(), rawToken)
	if err != nil {
		t.Fatalf("ConfirmEmail() unexpected error: %v", err)
	}
}

func TestService_ConfirmEmail_EmptyToken(t *testing.T) {
	ctrl := gomock.NewController(t)

	repository := mocks.NewMockRepository(ctrl)
	passwordHasher := mocks.NewMockPasswordHasher(ctrl)
	tokenGenerator := mocks.NewMockVerificationTokenGenerator(ctrl)
	tokenCipher := mocks.NewMockVerificationTokenCipher(ctrl)

	service := auth.NewService(
		repository,
		passwordHasher,
		tokenGenerator,
		tokenCipher,
		time.Now,
		30*time.Minute,
	)

	err := service.ConfirmEmail(context.Background(), "")

	if !errors.Is(err, auth.ErrInvalidToken) {
		t.Fatalf("expected ErrInvalidToken, got %v", err)
	}
}

func TestService_ConfirmEmail_RepositoryError(t *testing.T) {
	ctrl := gomock.NewController(t)

	repository := mocks.NewMockRepository(ctrl)
	passwordHasher := mocks.NewMockPasswordHasher(ctrl)
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

	service := auth.NewService(
		repository,
		passwordHasher,
		tokenGenerator,
		tokenCipher,
		func() time.Time {
			return now
		},
		30*time.Minute,
	)

	rawToken := "raw-verification-token"
	expectedHash := token.Hash(rawToken)

	repositoryErr := errors.New("repository failed")

	repository.EXPECT().ConfirmEmail(gomock.Any(), expectedHash, now).Return(repositoryErr)

	err := service.ConfirmEmail(context.Background(), rawToken)
	if !errors.Is(err, repositoryErr) {
		t.Fatalf("expected repository error, got %v", err)
	}
}

func TestService_ResendVerification_Success(t *testing.T) {
	ctrl := gomock.NewController(t)

	repository := mocks.NewMockRepository(ctrl)
	passwordHasher := mocks.NewMockPasswordHasher(ctrl)
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
		passwordHasher,
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

func TestService_ResendVerification_InvalidEmail(t *testing.T) {
	ctrl := gomock.NewController(t)

	repository := mocks.NewMockRepository(ctrl)
	passwordHasher := mocks.NewMockPasswordHasher(ctrl)
	tokenGenerator := mocks.NewMockVerificationTokenGenerator(ctrl)
	tokenCipher := mocks.NewMockVerificationTokenCipher(ctrl)

	service := auth.NewService(
		repository,
		passwordHasher,
		tokenGenerator,
		tokenCipher,
		time.Now,
		30*time.Minute,
	)

	err := service.ResendVerification(context.Background(), "not-an-email")

	var validationErr *auth.ValidationError

	if !errors.As(err, &validationErr) {
		t.Fatalf("expected ValidationError, got %T: %v", err, err)
	}

	if validationErr.Code != "invalid_email" {
		t.Errorf("expected code %q, got %q", "invalid_email", validationErr.Code)
	}
}

func TestService_ResendVerification_TokenGenerationError(t *testing.T) {
	ctrl := gomock.NewController(t)

	repository := mocks.NewMockRepository(ctrl)
	passwordHasher := mocks.NewMockPasswordHasher(ctrl)
	tokenGenerator := mocks.NewMockVerificationTokenGenerator(ctrl)
	tokenCipher := mocks.NewMockVerificationTokenCipher(ctrl)

	service := auth.NewService(
		repository,
		passwordHasher,
		tokenGenerator,
		tokenCipher,
		time.Now,
		30*time.Minute,
	)

	generateTokenErr := errors.New("token generation failed")

	tokenGenerator.EXPECT().Generate().Return("", nil, generateTokenErr)

	err := service.ResendVerification(context.Background(), "test@example.com")

	if !errors.Is(err, generateTokenErr) {
		t.Fatalf("expected token generation error, got %v", err)
	}
}

func TestService_ResendVerification_TokenEncryptionError(t *testing.T) {
	ctrl := gomock.NewController(t)

	repository := mocks.NewMockRepository(ctrl)
	passwordHasher := mocks.NewMockPasswordHasher(ctrl)
	tokenGenerator := mocks.NewMockVerificationTokenGenerator(ctrl)
	tokenCipher := mocks.NewMockVerificationTokenCipher(ctrl)

	service := auth.NewService(
		repository,
		passwordHasher,
		tokenGenerator,
		tokenCipher,
		time.Now,
		30*time.Minute,
	)

	rawToken := "raw-verification-token"
	tokenHash := []byte("verification-token-hash")

	tokenGenerator.EXPECT().Generate().Return(rawToken, tokenHash, nil)

	encryptTokenErr := errors.New("token encryption failed")

	tokenCipher.EXPECT().Encrypt([]byte(rawToken), gomock.Any()).Return(nil, encryptTokenErr)

	err := service.ResendVerification(context.Background(), "test@example.com")

	if !errors.Is(err, encryptTokenErr) {
		t.Fatalf("expected token encryption error, got %v", err)
	}
}

func TestService_ResendVerification_RepositoryError(t *testing.T) {
	ctrl := gomock.NewController(t)

	repository := mocks.NewMockRepository(ctrl)
	passwordHasher := mocks.NewMockPasswordHasher(ctrl)
	tokenGenerator := mocks.NewMockVerificationTokenGenerator(ctrl)
	tokenCipher := mocks.NewMockVerificationTokenCipher(ctrl)

	service := auth.NewService(
		repository,
		passwordHasher,
		tokenGenerator,
		tokenCipher,
		time.Now,
		30*time.Minute,
	)

	rawToken := "raw-verification-token"
	tokenHash := []byte("verification-token-hash")

	tokenGenerator.EXPECT().Generate().Return(rawToken, tokenHash, nil)

	tokenCipher.EXPECT().Encrypt([]byte(rawToken), gomock.Any()).Return([]byte("encrypted-token"), nil)

	repositoryErr := errors.New("repository failed")

	repository.EXPECT().ResendVerification(gomock.Any(), gomock.Any()).Return(repositoryErr)

	err := service.ResendVerification(context.Background(), "test@example.com")

	if !errors.Is(err, repositoryErr) {
		t.Fatalf("expected repository error, got %v", err)
	}
}
