package emailverification_test

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/frimo-dev/frimo-messenger/internal/security/token"
	"github.com/frimo-dev/frimo-messenger/internal/service/emailverification"
)

type spyRepository struct {
	receivedTokenHash   []byte
	receivedConfirmedAt time.Time
	err                 error
	called              bool
}

func (r *spyRepository) Confirm(
	_ context.Context,
	tokenHash []byte,
	confirmedAt time.Time,
) error {
	r.called = true
	r.receivedTokenHash = tokenHash
	r.receivedConfirmedAt = confirmedAt

	return r.err
}

func (r *spyRepository) ResendVerificationToken(_ context.Context, input emailverification.ResendInput) error {
	return nil
}

func TestServiceConfirmHashesTokenAndConfirmsVerification(
	t *testing.T,
) {
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

	service := emailverification.NewService(
		repository,
		func() time.Time {
			return now
		},
	)

	rawToken := "raw-verification-token"

	err := service.Confirm(
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
