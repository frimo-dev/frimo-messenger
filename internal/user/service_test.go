package user_test

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/frimo-dev/frimo-messenger/internal/user"
	"github.com/google/uuid"
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

type spyUserRepository struct {
	receivedInput user.CreationInput
	createdUser   user.User
	err           error
	called        bool
}

func (r *spyUserRepository) Create(
	_ context.Context,
	input user.CreationInput,
) (user.User, error) {
	r.called = true
	r.receivedInput = input

	if r.err != nil {
		return user.User{}, r.err
	}

	return r.createdUser, nil
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

	repository := &spyUserRepository{
		createdUser: user.User{
			ID:        "user-id",
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

	service := user.NewService(
		repository,
		passwordHasher,
		tokenGenerator,
		func() time.Time { return now },
		time.Minute*30,
	)

	result, err := service.Register(
		context.Background(),
		user.RegistrationInput{
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
		repository.receivedInput.VerificationTokenHash,
		[]byte("verification-token-hash"),
	) {
		t.Fatalf(
			"unexpected verification token hash: %q",
			repository.receivedInput.VerificationTokenHash,
		)
	}

	if repository.receivedInput.ID == "" {
		t.Fatal("expected generated user ID")
	}

	if _, err := uuid.Parse(repository.receivedInput.ID); err != nil {
		t.Fatalf(
			"expected valid user UUID, got %q",
			repository.receivedInput.ID,
		)
	}

	if repository.receivedInput.VerificationID == "" {
		t.Fatal("expected generated verification ID")
	}

	if result.User.ID != "user-id" {
		t.Fatalf(
			"expected user ID %q, got %q",
			"user-id",
			result.User.ID,
		)
	}

	if result.RawVerificationToken != "raw-verification-token" {
		t.Fatalf(
			"expected raw token %q, got %q",
			"raw-verification-token",
			result.RawVerificationToken,
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
