package auth_test

import (
	"bytes"
	"context"
	"errors"
	"net/netip"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/frimo-dev/frimo-messenger/internal/service/auth"
	"github.com/frimo-dev/frimo-messenger/internal/service/auth/mocks"
	"go.uber.org/mock/gomock"
)

func TestService_Login_Success(t *testing.T) {
	ctrl := gomock.NewController(t)

	repository := mocks.NewMockRepository(ctrl)
	sessionRepository := mocks.NewMockSessionRepository(ctrl)
	accessTokenStorage := mocks.NewMockAccessTokenStorage(ctrl)
	observer := mocks.NewMockObserver(ctrl)
	passwordManager := mocks.NewMockPasswordManager(ctrl)
	tokenGenerator := mocks.NewMockTokenGenerator(ctrl)

	now := time.Date(2026, time.August, 30, 12, 0, 0, 0, time.UTC)
	verifiedAt := now.Add(-time.Hour)
	userID := uuid.New()

	accessTokenLifetime := 15 * time.Minute
	sessionInactivityTimeout := 30 * 24 * time.Hour

	ip := netip.MustParseAddr("192.0.2.10")

	input := auth.LoginInput{
		Email:      " Test@Example.COM ",
		Password:   "correct-password",
		IP:         ip,
		DeviceName: "  Chrome on Windows  ",
	}

	repository.EXPECT().
		GetUserForLogin(gomock.Any(), "test@example.com").
		Return(auth.LoginUser{
			ID:           userID,
			PasswordHash: "password-hash",
			VerifiedAt:   &verifiedAt,
		}, nil)

	passwordManager.EXPECT().
		Verify("password-hash", "correct-password").
		Return(nil)

	refreshTokenHash := []byte("refresh-token-hash")
	accessTokenHash := []byte("access-token-hash")

	var createdSession auth.Session
	var createdRefreshToken auth.RefreshToken

	gomock.InOrder(
		tokenGenerator.EXPECT().
			Generate().
			Return("refresh-token", refreshTokenHash, nil),

		sessionRepository.EXPECT().
			CreateSession(gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(
				_ context.Context,
				session auth.Session,
				refreshToken auth.RefreshToken,
			) error {
				createdSession = session
				createdRefreshToken = refreshToken
				return nil
			}),

		tokenGenerator.EXPECT().
			Generate().
			Return("access-token", accessTokenHash, nil),
	)

	accessTokenStorage.EXPECT().
		Store(
			gomock.Any(),
			accessTokenHash,
			gomock.Any(),
			accessTokenLifetime,
		).
		DoAndReturn(func(
			_ context.Context,
			hash []byte,
			identity auth.Identity,
			ttl time.Duration,
		) error {
			if !bytes.Equal(hash, accessTokenHash) {
				t.Errorf("access token hash = %q, want %q", hash, accessTokenHash)
			}

			if identity.UserID != userID {
				t.Errorf(
					"identity UserID = %v, want %v",
					identity.UserID,
					userID,
				)
			}

			if identity.SessionID != createdSession.ID {
				t.Errorf(
					"identity SessionID = %v, want %v",
					identity.SessionID,
					createdSession.ID,
				)
			}

			if ttl != accessTokenLifetime {
				t.Errorf(
					"access token TTL = %v, want %v",
					ttl,
					accessTokenLifetime,
				)
			}

			return nil
		})

	service := auth.NewService(
		repository,
		sessionRepository,
		accessTokenStorage,
		nil, // SessionStorage
		nil, // RefreshResultStorage
		observer,
		passwordManager,
		tokenGenerator,
		nil, // VerificationTokenCipher
		func() time.Time { return now },
		30*time.Minute,
		accessTokenLifetime,
		sessionInactivityTimeout,
		time.Minute,
	)

	pair, err := service.Login(context.Background(), input)
	if err != nil {
		t.Fatalf("Login() unexpected error: %v", err)
	}

	if pair.AccessToken != "access-token" {
		t.Errorf(
			"AccessToken = %q, want %q",
			pair.AccessToken,
			"access-token",
		)
	}

	if pair.RefreshToken != "refresh-token" {
		t.Errorf(
			"RefreshToken = %q, want %q",
			pair.RefreshToken,
			"refresh-token",
		)
	}

	if createdSession.ID == uuid.Nil() {
		t.Error("session ID must not be nil")
	}

	if createdSession.UserID != userID {
		t.Errorf(
			"session UserID = %v, want %v",
			createdSession.UserID,
			userID,
		)
	}

	if createdSession.DeviceName != "Chrome on Windows" {
		t.Errorf(
			"session DeviceName = %q, want %q",
			createdSession.DeviceName,
			"Chrome on Windows",
		)
	}

	if createdSession.CreatedIP != ip {
		t.Errorf(
			"session CreatedIP = %v, want %v",
			createdSession.CreatedIP,
			ip,
		)
	}

	if createdSession.LastIP != ip {
		t.Errorf(
			"session LastIP = %v, want %v",
			createdSession.LastIP,
			ip,
		)
	}

	if !createdSession.CreatedAt.Equal(now) {
		t.Errorf(
			"session CreatedAt = %v, want %v",
			createdSession.CreatedAt,
			now,
		)
	}

	if !createdSession.LastSeenAt.Equal(now) {
		t.Errorf(
			"session LastSeenAt = %v, want %v",
			createdSession.LastSeenAt,
			now,
		)
	}

	expectedExpiresAt := now.Add(sessionInactivityTimeout)

	if !createdSession.ExpiresAt.Equal(expectedExpiresAt) {
		t.Errorf(
			"session ExpiresAt = %v, want %v",
			createdSession.ExpiresAt,
			expectedExpiresAt,
		)
	}

	if createdRefreshToken.ID == uuid.Nil() {
		t.Error("refresh token ID must not be nil")
	}

	if !bytes.Equal(createdRefreshToken.TokenHash, refreshTokenHash) {
		t.Errorf(
			"refresh token hash = %q, want %q",
			createdRefreshToken.TokenHash,
			refreshTokenHash,
		)
	}

	if !createdRefreshToken.CreatedAt.Equal(now) {
		t.Errorf(
			"refresh token CreatedAt = %v, want %v",
			createdRefreshToken.CreatedAt,
			now,
		)
	}
}

func TestService_Login_Errors(t *testing.T) {
	now := time.Date(2026, time.August, 30, 12, 0, 0, 0, time.UTC)
	verifiedAt := now.Add(-time.Hour)
	userID := uuid.New()

	repositoryErr := errors.New("repository failed")
	verifyErr := errors.New("password verification failed")
	generateErr := errors.New("token generation failed")
	createSessionErr := errors.New("create session failed")

	validUser := auth.LoginUser{
		ID:           userID,
		PasswordHash: "password-hash",
		VerifiedAt:   &verifiedAt,
	}

	type dependencies struct {
		repository        *mocks.MockRepository
		sessionRepository *mocks.MockSessionRepository
		passwordManager   *mocks.MockPasswordManager
		tokenGenerator    *mocks.MockTokenGenerator
	}

	expectValidCredentials := func(d dependencies) {
		d.repository.EXPECT().
			GetUserForLogin(gomock.Any(), "test@example.com").
			Return(validUser, nil)

		d.passwordManager.EXPECT().
			Verify("password-hash", "correct-password").
			Return(nil)
	}

	tests := []struct {
		name     string
		input    auth.LoginInput
		prepare  func(d dependencies)
		wantErr  error
		wantCode string
	}{
		{
			name: "invalid email",
			input: auth.LoginInput{
				Email:      "not-an-email",
				Password:   "correct-password",
				DeviceName: "Chrome",
			},
			wantCode: "invalid_email",
		},
		{
			name: "email required",
			input: auth.LoginInput{
				Email:      "   ",
				Password:   "correct-password",
				DeviceName: "Chrome",
			},
			wantCode: "email_required",
		},
		{
			name: "password required",
			input: auth.LoginInput{
				Email:      "test@example.com",
				Password:   "",
				DeviceName: "Chrome",
			},
			wantCode: "password_required",
		},
		{
			name: "user not found",
			input: auth.LoginInput{
				Email:      "test@example.com",
				Password:   "correct-password",
				DeviceName: "Chrome",
			},
			prepare: func(d dependencies) {
				d.repository.EXPECT().
					GetUserForLogin(gomock.Any(), "test@example.com").
					Return(auth.LoginUser{}, auth.ErrUserNotFound)
			},
			wantErr: auth.ErrInvalidCredentials,
		},
		{
			name: "repository error",
			input: auth.LoginInput{
				Email:      "test@example.com",
				Password:   "correct-password",
				DeviceName: "Chrome",
			},
			prepare: func(d dependencies) {
				d.repository.EXPECT().
					GetUserForLogin(gomock.Any(), "test@example.com").
					Return(auth.LoginUser{}, repositoryErr)
			},
			wantErr: repositoryErr,
		},
		{
			name: "invalid password",
			input: auth.LoginInput{
				Email:      "test@example.com",
				Password:   "wrong-password",
				DeviceName: "Chrome",
			},
			prepare: func(d dependencies) {
				d.repository.EXPECT().
					GetUserForLogin(gomock.Any(), "test@example.com").
					Return(validUser, nil)

				d.passwordManager.EXPECT().
					Verify("password-hash", "wrong-password").
					Return(auth.ErrInvalidCredentials)
			},
			wantErr: auth.ErrInvalidCredentials,
		},
		{
			name: "password verification error",
			input: auth.LoginInput{
				Email:      "test@example.com",
				Password:   "correct-password",
				DeviceName: "Chrome",
			},
			prepare: func(d dependencies) {
				d.repository.EXPECT().
					GetUserForLogin(gomock.Any(), "test@example.com").
					Return(validUser, nil)

				d.passwordManager.EXPECT().
					Verify("password-hash", "correct-password").
					Return(verifyErr)
			},
			wantErr: verifyErr,
		},
		{
			name: "email not verified",
			input: auth.LoginInput{
				Email:      "test@example.com",
				Password:   "correct-password",
				DeviceName: "Chrome",
			},
			prepare: func(d dependencies) {
				d.repository.EXPECT().
					GetUserForLogin(gomock.Any(), "test@example.com").
					Return(auth.LoginUser{
						ID:           userID,
						PasswordHash: "password-hash",
						VerifiedAt:   nil,
					}, nil)

				d.passwordManager.EXPECT().
					Verify("password-hash", "correct-password").
					Return(nil)
			},
			wantErr: auth.ErrEmailNotVerified,
		},
		{
			name: "device name required",
			input: auth.LoginInput{
				Email:      "test@example.com",
				Password:   "correct-password",
				DeviceName: "   ",
			},
			prepare: func(d dependencies) {
				expectValidCredentials(d)
			},
			wantCode: "device_name_required",
		},
		{
			name: "device name too long",
			input: auth.LoginInput{
				Email:      "test@example.com",
				Password:   "correct-password",
				DeviceName: strings.Repeat("a", 256),
			},
			prepare: func(d dependencies) {
				expectValidCredentials(d)
			},
			wantCode: "device_name_too_long",
		},
		{
			name: "refresh token generation error",
			input: auth.LoginInput{
				Email:      "test@example.com",
				Password:   "correct-password",
				DeviceName: "Chrome",
			},
			prepare: func(d dependencies) {
				expectValidCredentials(d)

				d.tokenGenerator.EXPECT().
					Generate().
					Return("", nil, generateErr)
			},
			wantErr: generateErr,
		},
		{
			name: "create session error",
			input: auth.LoginInput{
				Email:      "test@example.com",
				Password:   "correct-password",
				DeviceName: "Chrome",
			},
			prepare: func(d dependencies) {
				expectValidCredentials(d)

				gomock.InOrder(
					d.tokenGenerator.EXPECT().
						Generate().
						Return(
							"refresh-token",
							[]byte("refresh-hash"),
							nil,
						),

					d.sessionRepository.EXPECT().
						CreateSession(
							gomock.Any(),
							gomock.Any(),
							gomock.Any(),
						).
						Return(createSessionErr),
				)
			},
			wantErr: createSessionErr,
		},
		{
			name: "access token generation error",
			input: auth.LoginInput{
				Email:      "test@example.com",
				Password:   "correct-password",
				DeviceName: "Chrome",
			},
			prepare: func(d dependencies) {
				expectValidCredentials(d)

				gomock.InOrder(
					d.tokenGenerator.EXPECT().
						Generate().
						Return(
							"refresh-token",
							[]byte("refresh-hash"),
							nil,
						),

					d.sessionRepository.EXPECT().
						CreateSession(
							gomock.Any(),
							gomock.Any(),
							gomock.Any(),
						).
						Return(nil),

					d.tokenGenerator.EXPECT().
						Generate().
						Return("", nil, generateErr),
				)
			},
			wantErr: generateErr,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			repository := mocks.NewMockRepository(ctrl)
			sessionRepository := mocks.NewMockSessionRepository(ctrl)
			accessTokenStorage := mocks.NewMockAccessTokenStorage(ctrl)
			observer := mocks.NewMockObserver(ctrl)
			passwordManager := mocks.NewMockPasswordManager(ctrl)
			tokenGenerator := mocks.NewMockTokenGenerator(ctrl)

			service := auth.NewService(
				repository,
				sessionRepository,
				accessTokenStorage,
				nil, // SessionStorage
				nil, // RefreshResultStorage
				observer,
				passwordManager,
				tokenGenerator,
				nil, // VerificationTokenCipher
				func() time.Time { return now },
				30*time.Minute,
				15*time.Minute,
				30*24*time.Hour,
				time.Minute,
			)

			if tt.prepare != nil {
				tt.prepare(dependencies{
					repository:        repository,
					sessionRepository: sessionRepository,
					passwordManager:   passwordManager,
					tokenGenerator:    tokenGenerator,
				})
			}

			pair, err := service.Login(context.Background(), tt.input)

			if pair != (auth.TokenPair{}) {
				t.Errorf("expected empty token pair, got %+v", pair)
			}

			if tt.wantCode != "" {
				if err == nil {
					t.Fatal("expected validation error, got nil")
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
						"validation code = %q, want %q",
						validationErr.Code,
						tt.wantCode,
					)
				}

				return
			}

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf(
					"Login() error = %v, want %v",
					err,
					tt.wantErr,
				)
			}
		})
	}
}

func TestService_Login_AccessTokenStoreFailed(t *testing.T) {
	ctrl := gomock.NewController(t)

	repository := mocks.NewMockRepository(ctrl)
	sessionRepository := mocks.NewMockSessionRepository(ctrl)
	accessTokenStorage := mocks.NewMockAccessTokenStorage(ctrl)
	observer := mocks.NewMockObserver(ctrl)
	passwordManager := mocks.NewMockPasswordManager(ctrl)
	tokenGenerator := mocks.NewMockTokenGenerator(ctrl)

	now := time.Date(2026, time.August, 30, 12, 0, 0, 0, time.UTC)
	verifiedAt := now.Add(-time.Hour)
	userID := uuid.New()

	storeErr := errors.New("redis unavailable")

	repository.EXPECT().
		GetUserForLogin(gomock.Any(), "test@example.com").
		Return(auth.LoginUser{
			ID:           userID,
			PasswordHash: "password-hash",
			VerifiedAt:   &verifiedAt,
		}, nil)

	passwordManager.EXPECT().
		Verify("password-hash", "correct-password").
		Return(nil)

	gomock.InOrder(
		tokenGenerator.EXPECT().
			Generate().
			Return("refresh-token", []byte("refresh-hash"), nil),

		sessionRepository.EXPECT().
			CreateSession(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil),

		tokenGenerator.EXPECT().
			Generate().
			Return("access-token", []byte("access-hash"), nil),

		accessTokenStorage.EXPECT().
			Store(
				gomock.Any(),
				[]byte("access-hash"),
				gomock.Any(),
				15*time.Minute,
			).
			Return(storeErr),

		observer.EXPECT().
			AccessTokenStoreFailed(gomock.Any(), storeErr),
	)

	service := auth.NewService(
		repository,
		sessionRepository,
		accessTokenStorage,
		nil, // SessionStorage
		nil, // RefreshResultStorage
		observer,
		passwordManager,
		tokenGenerator,
		nil, // VerificationTokenCipher
		func() time.Time { return now },
		30*time.Minute,
		15*time.Minute,
		30*24*time.Hour,
		time.Minute,
	)

	pair, err := service.Login(context.Background(), auth.LoginInput{
		Email:      "test@example.com",
		Password:   "correct-password",
		DeviceName: "Chrome",
		IP:         netip.MustParseAddr("192.0.2.10"),
	})

	if err != nil {
		t.Fatalf("Login() unexpected error: %v", err)
	}

	if pair.AccessToken != "access-token" {
		t.Errorf(
			"AccessToken = %q, want %q",
			pair.AccessToken,
			"access-token",
		)
	}

	if pair.RefreshToken != "refresh-token" {
		t.Errorf(
			"RefreshToken = %q, want %q",
			pair.RefreshToken,
			"refresh-token",
		)
	}
}
