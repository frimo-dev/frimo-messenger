package emailverification

import (
	"context"
	"time"

	"github.com/frimo-dev/frimo-messenger/internal/token"
)

type Service struct {
	repository Repository
	// now нужен для управляемых и детерминированных тестов
	now func() time.Time
}

func NewService(repository Repository, now func() time.Time) *Service {
	return &Service{
		repository: repository,
		now:        now,
	}
}

func (s *Service) Confirm(ctx context.Context, rawToken string) error {
	if rawToken == "" {
		return ErrInvalidToken
	}

	tokenHash := token.Hash(rawToken)

	return s.repository.Confirm(ctx, tokenHash, s.now().UTC())
}
