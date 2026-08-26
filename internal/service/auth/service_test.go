package auth_test

import (
	"bytes"
	"context"
	"encoding/json/v2"
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

	if user.Email != createdUser.Email {
		t.Errorf("expected email %q, got %q", createdUser.Email, user.Email)
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
