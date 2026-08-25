package auth_test

import (
	"bytes"
	"context"
	"testing"
	"time"
	"uuid"

	"github.com/frimo-dev/frimo-messenger/internal/security/token"
	"github.com/frimo-dev/frimo-messenger/internal/service/auth"
)

type spyPasswordHasher struct {
	receivedPassword string
	hash             string
	err              error
}

func (h *spyPasswordHasher) Hash(password string) (string, error) {
	h.receivedPassword = password

	if h.err != nil {
		return "", h.err
	}

	return h.hash, nil
}

type spyTokenGenerator struct {
	rawToken  string
	tokenHash []byte
	err       error
	called    bool
}

func (g *spyTokenGenerator) Generate() (string, []byte, error) {
	g.called = true

	if g.err != nil {
		return "", nil, g.err
	}

	return g.rawToken, g.tokenHash, nil
}

type spyRepository struct {
	receivedInput auth.CreateUserInput
	createdUser   auth.User

	receivedTokenHash   []byte
	receivedConfirmedAt time.Time

	err    error
	called bool
}

func (r *spyRepository) CreateUser(
	_ context.Context,
	input auth.CreateUserInput,
) (auth.User, error) {
	r.called = true
	r.receivedInput = input

	if r.err != nil {
		return auth.User{}, r.err
	}

	return r.createdUser, nil
}

func (r *spyRepository) ConfirmEmail(
	_ context.Context,
	tokenHash []byte,
	confirmedAt time.Time,
) error {
	r.called = true
	r.receivedTokenHash = tokenHash
	r.receivedConfirmedAt = confirmedAt

	return r.err
}

func (r *spyRepository) ResendVerification(_ context.Context, input auth.ResendVerificationInput) error {
	return nil
}

type stubTokenCipher struct {
	receivedPlaintext      []byte
	receivedAdditionalData []byte
	ciphertext             []byte
	err                    error
}

func (c *stubTokenCipher) Encrypt(
	plaintext []byte,
	additionalData []byte,
) ([]byte, error) {
	c.receivedPlaintext = append(
		[]byte(nil),
		plaintext...,
	)

	c.receivedAdditionalData = append(
		[]byte(nil),
		additionalData...,
	)

	if c.err != nil {
		return nil, c.err
	}

	return append([]byte(nil), c.ciphertext...), nil
}

// TODO: разделить тест на много маленьких
func TestServiceRegisterCreatesUserAndVerificationToken(t *testing.T) {
	now := time.Date(
		2026,
		time.July,
		20,
		12,
		0,
		0,
		0,
		time.UTC,
	)

	userID := uuid.New()

	repository := &spyRepository{
		createdUser: auth.User{
			ID:        userID,
			Email:     "misha@example.com",
			CreatedAt: now,
		},
	}

	passwordHasher := &spyPasswordHasher{
		hash: "encoded-password-hash",
	}

	tokenGenerator := &spyTokenGenerator{
		rawToken:  "raw-verification-token",
		tokenHash: []byte("verification-token-hash"),
	}

	tokenCipher := &stubTokenCipher{
		ciphertext: []byte("encrypted-token"),
	}

	service := auth.NewService(
		repository,
		passwordHasher,
		tokenGenerator,
		tokenCipher,
		func() time.Time { return now },
		time.Minute*30,
	)

	user, err := service.Register(
		context.Background(),
		auth.RegistrationInput{
			Email:    "  Misha@Example.com  ",
			Password: "long-secret-password",
		},
	)
	if err != nil {
		t.Fatalf("register user: %v", err)
	}

	if passwordHasher.receivedPassword != "long-secret-password" {
		t.Fatalf(
			"expected hasher to receive plaintext password, got %q",
			passwordHasher.receivedPassword,
		)
	}

	if !tokenGenerator.called {
		t.Fatal("expected token generator to be called")
	}

	if !repository.called {
		t.Fatal("expected repository to be called")
	}

	if repository.receivedInput.Email != "misha@example.com" {
		t.Fatalf(
			"expected normalized email, got %q",
			repository.receivedInput.Email,
		)
	}

	if repository.receivedInput.PasswordHash != "encoded-password-hash" {
		t.Fatalf(
			"expected password hash %q, got %q",
			"encoded-password-hash",
			repository.receivedInput.PasswordHash,
		)
	}

	if repository.receivedInput.PasswordHash == "long-secret-password" {
		t.Fatal("repository received plaintext password")
	}

	if !bytes.Equal(
		repository.receivedInput.Verification.TokenHash,
		[]byte("verification-token-hash"),
	) {
		t.Fatalf(
			"unexpected verification token hash: %q",
			repository.receivedInput.Verification.TokenHash,
		)
	}

	if repository.receivedInput.ID == uuid.Nil() {
		t.Fatal("expected generated user ID")
	}

	if repository.receivedInput.Verification.ID == uuid.Nil() {
		t.Fatal("expected generated verification ID")
	}

	if user.ID != userID {
		t.Fatalf(
			"expected user ID %q, got %q",
			"user-id",
			user.ID,
		)
	}

	if !repository.receivedInput.CreatedAt.Equal(now) {
		t.Fatalf(
			"expected created at %v, got %v",
			now,
			repository.receivedInput.CreatedAt,
		)
	}
}

func TestServiceConfirmHashesTokenAndConfirmsVerification(t *testing.T) {
	repository := &spyRepository{}

	now := time.Date(
		2026,
		time.July,
		20,
		15,
		30,
		0,
		0,
		time.FixedZone("UTC+3", 3*60*60),
	)

	passwordHasher := &spyPasswordHasher{
		hash: "encoded-password-hash",
	}

	tokenGenerator := &spyTokenGenerator{
		rawToken:  "raw-verification-token",
		tokenHash: []byte("verification-token-hash"),
	}

	tokenCipher := &stubTokenCipher{
		ciphertext: []byte("encrypted-token"),
	}

	service := auth.NewService(
		repository,
		passwordHasher,
		tokenGenerator,
		tokenCipher,
		func() time.Time { return now },
		time.Minute*30,
	)

	rawToken := "raw-verification-token"

	err := service.ConfirmEmail(
		context.Background(),
		rawToken,
	)
	if err != nil {
		t.Fatalf("confirm email: %v", err)
	}

	if !repository.called {
		t.Fatal("expected repository to be called")
	}

	expectedHash := token.Hash(rawToken)

	if !bytes.Equal(
		repository.receivedTokenHash,
		expectedHash,
	) {
		t.Fatalf(
			"expected token hash %x, got %x",
			expectedHash,
			repository.receivedTokenHash,
		)
	}

	expectedConfirmedAt := now.UTC()

	if !repository.receivedConfirmedAt.Equal(
		expectedConfirmedAt,
	) {
		t.Fatalf(
			"expected confirmed at %v, got %v",
			expectedConfirmedAt,
			repository.receivedConfirmedAt,
		)
	}

	if repository.receivedConfirmedAt.Location() != time.UTC {
		t.Fatalf(
			"expected UTC location, got %v",
			repository.receivedConfirmedAt.Location(),
		)
	}
}
