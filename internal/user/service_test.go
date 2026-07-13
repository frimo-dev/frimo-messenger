package user

import (
	"context"
	"errors"
	"testing"
)

type stubHasher struct {
	receivedPassword string
	hash             string
	err              error
}

func (h *stubHasher) Hash(password string) (string, error) {
	h.receivedPassword = password

	if h.err != nil {
		return "", h.err
	}

	return h.hash, nil
}

type spyRepository struct {
	receivedEmail        string
	receivedPasswordHash string
	user                 User
	err                  error
}

func (r *spyRepository) Create(_ context.Context, email string, passwordHash string) (User, error) {
	r.receivedEmail = email
	r.receivedPasswordHash = passwordHash

	if r.err != nil {
		return User{}, r.err
	}

	return r.user, nil
}

func TestServiceRegisterNormalizesEmailAndHashesPassword(t *testing.T) {
	repository := &spyRepository{
		user: User{
			ID:    "user-1",
			Email: "misha@example.com",
		},
	}

	hasher := &stubHasher{
		hash: "encoded-password-hash",
	}

	service := NewService(repository, hasher)

	_, err := service.Register(
		context.Background(),
		RegisterInput{
			Email:    "  Misha@Example.com  ",
			Password: "long-secret-password",
		},
	)
	if err != nil {
		t.Fatalf("register user: %v", err)
	}

	if hasher.receivedPassword != "long-secret-password" {
		t.Fatalf(
			"expected hasher to receive original password, got %q",
			hasher.receivedPassword,
		)
	}

	if repository.receivedEmail != "misha@example.com" {
		t.Fatalf(
			"expected normalized email, got %q",
			repository.receivedEmail,
		)
	}

	if repository.receivedPasswordHash != "encoded-password-hash" {
		t.Fatalf(
			"expected repository to receive password hash, got %q",
			repository.receivedPasswordHash,
		)
	}

	if repository.receivedPasswordHash == "long-secret-password" {
		t.Fatal("repository received plaintext password")
	}
}

func TestServiceRegisterRejectsInvalidEmail(t *testing.T) {
	repository := &spyRepository{}
	hasher := &stubHasher{}

	service := NewService(repository, hasher)

	_, err := service.Register(
		context.Background(),
		RegisterInput{
			Email:    "not-an-email",
			Password: "long-secret-password",
		},
	)

	if err == nil {
		t.Fatal("expected validation error")
	}

	var validationErr *ValidationError

	if !errors.As(err, &validationErr) {
		t.Fatalf(
			"expected ValidationError, got %T",
			err,
		)
	}

	if validationErr.Code != "invalid_email" {
		t.Fatalf(
			"expected code %q, got %q",
			"invalid_email",
			validationErr.Code,
		)
	}

	if hasher.receivedPassword != "" {
		t.Fatal("password must not be hashed when email is invalid")
	}

	if repository.receivedEmail != "" {
		t.Fatal("repository must not be called when email is invalid")
	}
}
