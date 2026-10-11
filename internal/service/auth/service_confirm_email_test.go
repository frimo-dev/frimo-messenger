package auth_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/frimo-dev/frimo-messenger/internal/service/auth"
	"github.com/frimo-dev/frimo-messenger/internal/service/auth/mocks"
	"go.uber.org/mock/gomock"
)

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

	const rawToken = "raw-verification-token"
	tokenHash := []byte("verification-token-hash")

	tests := []struct {
		name     string
		rawToken string

		prepare func(
			repository *mocks.MockRepository,
			tokenGenerator *mocks.MockTokenGenerator,
		)

		wantErr error
	}{
		{
			name:     "success",
			rawToken: rawToken,

			prepare: func(
				repository *mocks.MockRepository,
				tokenGenerator *mocks.MockTokenGenerator,
			) {
				tokenGenerator.EXPECT().
					Hash(rawToken).
					Return(tokenHash)

				repository.EXPECT().
					ConfirmEmail(
						gomock.Any(),
						tokenHash,
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
			rawToken: rawToken,

			prepare: func(
				repository *mocks.MockRepository,
				tokenGenerator *mocks.MockTokenGenerator,
			) {
				tokenGenerator.EXPECT().
					Hash(rawToken).
					Return(tokenHash)

				repository.EXPECT().
					ConfirmEmail(
						gomock.Any(),
						tokenHash,
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
			tokenGenerator := mocks.NewMockTokenGenerator(ctrl)

			service := auth.NewService(
				repository,
				nil, // SessionRepository
				nil, // AccessTokenStorage
				nil, // SessionStorage
				nil, // RefreshResultStorage
				nil, // Observer
				nil, // PasswordManager
				tokenGenerator,
				nil, // VerificationTokenCipher
				func() time.Time {
					return now
				},
				30*time.Minute,  // verificationTokenLifetime
				15*time.Minute,  // accessTokenLifetime
				30*24*time.Hour, // sessionInactivityTimeout
				time.Minute,     // refreshRetryTTL
			)

			if tt.prepare != nil {
				tt.prepare(repository, tokenGenerator)
			}

			err := service.ConfirmEmail(
				context.Background(),
				tt.rawToken,
			)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf(
						"ConfirmEmail() error = %v, want %v",
						err,
						tt.wantErr,
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
