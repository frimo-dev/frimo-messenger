package auth_test

import (
	"context"
	"errors"
	"testing"
	"time"
	"uuid"

	"github.com/frimo-dev/frimo-messenger/internal/service/auth"
	"github.com/frimo-dev/frimo-messenger/internal/service/auth/mocks"
	"go.uber.org/mock/gomock"
)

func TestService_Authenticate(t *testing.T) {
	now := time.Date(2026, time.October, 11, 12, 0, 0, 0, time.UTC)

	const rawAccessToken = "raw-access-token"
	accessTokenHash := []byte("access-token-hash")

	identity := auth.Identity{
		UserID:    uuid.New(),
		SessionID: uuid.New(),
	}

	redisErr := errors.New("redis unavailable")
	postgresErr := errors.New("postgres unavailable")

	tests := []struct {
		name string

		prepare func(
			accessTokenStorage *mocks.MockAccessTokenStorage,
			sessionStorage *mocks.MockSessionStorage,
			sessionRepository *mocks.MockSessionRepository,
			observer *mocks.MockObserver,
		)

		wantIdentity auth.Identity
		wantErr      error
	}{
		{
			name: "active session cache hit",
			prepare: func(
				access *mocks.MockAccessTokenStorage,
				sessions *mocks.MockSessionStorage,
				_ *mocks.MockSessionRepository,
				_ *mocks.MockObserver,
			) {
				access.EXPECT().
					Get(gomock.Any(), accessTokenHash).
					Return(identity, nil)

				sessions.EXPECT().
					GetSession(gomock.Any(), identity.SessionID).
					Return(true, nil)
			},
			wantIdentity: identity,
		},
		{
			name: "inactive session cache hit",
			prepare: func(
				access *mocks.MockAccessTokenStorage,
				sessions *mocks.MockSessionStorage,
				_ *mocks.MockSessionRepository,
				_ *mocks.MockObserver,
			) {
				access.EXPECT().
					Get(gomock.Any(), accessTokenHash).
					Return(identity, nil)

				sessions.EXPECT().
					GetSession(gomock.Any(), identity.SessionID).
					Return(false, nil)
			},
			wantErr: auth.ErrSessionInactive,
		},
		{
			name: "access token storage error",
			prepare: func(
				access *mocks.MockAccessTokenStorage,
				_ *mocks.MockSessionStorage,
				_ *mocks.MockSessionRepository,
				_ *mocks.MockObserver,
			) {
				access.EXPECT().
					Get(gomock.Any(), accessTokenHash).
					Return(auth.Identity{}, redisErr)
			},
			wantErr: redisErr,
		},
		{
			name: "session cache miss active session",
			prepare: func(
				access *mocks.MockAccessTokenStorage,
				sessions *mocks.MockSessionStorage,
				repository *mocks.MockSessionRepository,
				_ *mocks.MockObserver,
			) {
				access.EXPECT().
					Get(gomock.Any(), accessTokenHash).
					Return(identity, nil)

				sessions.EXPECT().
					GetSession(gomock.Any(), identity.SessionID).
					Return(false, auth.ErrSessionCacheMiss)

				repository.EXPECT().
					GetSessionState(gomock.Any(), identity.SessionID).
					Return(auth.SessionState{
						ExpiresAt: now.Add(24 * time.Hour),
					}, nil)

				sessions.EXPECT().
					SetSession(
						gomock.Any(),
						identity.SessionID,
						true,
						24*time.Hour,
					).
					Return(nil)
			},
			wantIdentity: identity,
		},
		{
			name: "session cache miss revoked session",
			prepare: func(
				access *mocks.MockAccessTokenStorage,
				sessions *mocks.MockSessionStorage,
				repository *mocks.MockSessionRepository,
				_ *mocks.MockObserver,
			) {
				revokedAt := now.Add(-time.Hour)

				access.EXPECT().
					Get(gomock.Any(), accessTokenHash).
					Return(identity, nil)

				sessions.EXPECT().
					GetSession(gomock.Any(), identity.SessionID).
					Return(false, auth.ErrSessionCacheMiss)

				repository.EXPECT().
					GetSessionState(gomock.Any(), identity.SessionID).
					Return(auth.SessionState{
						RevokedAt: &revokedAt,
						ExpiresAt: now.Add(24 * time.Hour),
					}, nil)

				sessions.EXPECT().
					SetSession(
						gomock.Any(),
						identity.SessionID,
						false,
						5*time.Minute,
					).
					Return(nil)
			},
			wantErr: auth.ErrSessionInactive,
		},
		{
			name: "session cache miss expired session",
			prepare: func(
				access *mocks.MockAccessTokenStorage,
				sessions *mocks.MockSessionStorage,
				repository *mocks.MockSessionRepository,
				_ *mocks.MockObserver,
			) {
				access.EXPECT().
					Get(gomock.Any(), accessTokenHash).
					Return(identity, nil)

				sessions.EXPECT().
					GetSession(gomock.Any(), identity.SessionID).
					Return(false, auth.ErrSessionCacheMiss)

				repository.EXPECT().
					GetSessionState(gomock.Any(), identity.SessionID).
					Return(auth.SessionState{
						ExpiresAt: now.Add(-time.Minute),
					}, nil)

				sessions.EXPECT().
					SetSession(
						gomock.Any(),
						identity.SessionID,
						false,
						5*time.Minute,
					).
					Return(nil)
			},
			wantErr: auth.ErrSessionInactive,
		},
		{
			name: "session cache read error falls back to postgres",
			prepare: func(
				access *mocks.MockAccessTokenStorage,
				sessions *mocks.MockSessionStorage,
				repository *mocks.MockSessionRepository,
				observer *mocks.MockObserver,
			) {
				access.EXPECT().
					Get(gomock.Any(), accessTokenHash).
					Return(identity, nil)

				sessions.EXPECT().
					GetSession(gomock.Any(), identity.SessionID).
					Return(false, redisErr)

				observer.EXPECT().
					SessionCacheReadFailed(gomock.Any(), redisErr)

				repository.EXPECT().
					GetSessionState(gomock.Any(), identity.SessionID).
					Return(auth.SessionState{
						ExpiresAt: now.Add(time.Hour),
					}, nil)

				sessions.EXPECT().
					SetSession(
						gomock.Any(),
						identity.SessionID,
						true,
						time.Hour,
					).
					Return(nil)
			},
			wantIdentity: identity,
		},
		{
			name: "postgres fallback error",
			prepare: func(
				access *mocks.MockAccessTokenStorage,
				sessions *mocks.MockSessionStorage,
				repository *mocks.MockSessionRepository,
				_ *mocks.MockObserver,
			) {
				access.EXPECT().
					Get(gomock.Any(), accessTokenHash).
					Return(identity, nil)

				sessions.EXPECT().
					GetSession(gomock.Any(), identity.SessionID).
					Return(false, auth.ErrSessionCacheMiss)

				repository.EXPECT().
					GetSessionState(gomock.Any(), identity.SessionID).
					Return(auth.SessionState{}, postgresErr)
			},
			wantErr: postgresErr,
		},
		{
			name: "session cache update error is recoverable",
			prepare: func(
				access *mocks.MockAccessTokenStorage,
				sessions *mocks.MockSessionStorage,
				repository *mocks.MockSessionRepository,
				observer *mocks.MockObserver,
			) {
				access.EXPECT().
					Get(gomock.Any(), accessTokenHash).
					Return(identity, nil)

				sessions.EXPECT().
					GetSession(gomock.Any(), identity.SessionID).
					Return(false, auth.ErrSessionCacheMiss)

				repository.EXPECT().
					GetSessionState(gomock.Any(), identity.SessionID).
					Return(auth.SessionState{
						ExpiresAt: now.Add(time.Hour),
					}, nil)

				sessions.EXPECT().
					SetSession(
						gomock.Any(),
						identity.SessionID,
						true,
						time.Hour,
					).
					Return(redisErr)

				observer.EXPECT().
					SessionCacheUpdateFailed(gomock.Any(), redisErr)
			},
			wantIdentity: identity,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			accessTokenStorage := mocks.NewMockAccessTokenStorage(ctrl)
			sessionStorage := mocks.NewMockSessionStorage(ctrl)
			sessionRepository := mocks.NewMockSessionRepository(ctrl)
			tokenGenerator := mocks.NewMockTokenGenerator(ctrl)
			observer := mocks.NewMockObserver(ctrl)

			tokenGenerator.EXPECT().
				Hash(rawAccessToken).
				Return(accessTokenHash)

			tt.prepare(
				accessTokenStorage,
				sessionStorage,
				sessionRepository,
				observer,
			)

			service := auth.NewService(
				nil, // Repository
				sessionRepository,
				accessTokenStorage,
				sessionStorage,
				nil, // RefreshResultStorage
				observer,
				nil, // PasswordManager
				tokenGenerator,
				nil, // VerificationTokenCipher
				func() time.Time { return now },
				30*time.Minute,  // verificationTokenLifetime
				15*time.Minute,  // accessTokenLifetime
				30*24*time.Hour, // sessionInactivityTimeout
				time.Minute,     // refreshRetryTTL
			)

			gotIdentity, err := service.Authenticate(
				context.Background(),
				rawAccessToken,
			)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf(
						"Authenticate() error = %v, want %v",
						err,
						tt.wantErr,
					)
				}

				if gotIdentity != (auth.Identity{}) {
					t.Errorf(
						"Authenticate() identity = %+v, want empty",
						gotIdentity,
					)
				}

				return
			}

			if err != nil {
				t.Fatalf("Authenticate() unexpected error: %v", err)
			}

			if gotIdentity != tt.wantIdentity {
				t.Errorf(
					"Authenticate() identity = %+v, want %+v",
					gotIdentity,
					tt.wantIdentity,
				)
			}
		})
	}
}
