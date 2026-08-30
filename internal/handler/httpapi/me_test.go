package httpapi_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
	"uuid"

	"github.com/frimo-dev/frimo-messenger/internal/handler/httpapi"
	"github.com/frimo-dev/frimo-messenger/internal/handler/httpapi/mocks"
	"github.com/frimo-dev/frimo-messenger/internal/security/token"
	"github.com/frimo-dev/frimo-messenger/internal/service/user"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

func TestAPI_Me(t *testing.T) {
	userID := uuid.New()

	createdAt := time.Date(
		2026,
		time.August,
		30,
		12,
		0,
		0,
		0,
		time.UTC,
	)

	serviceErr := errors.New("service failed")

	tests := []struct {
		name          string
		authorization string

		prepare func(
			verifier *mocks.MockAccessTokenVerifier,
			userService *mocks.MockUserService,
		)

		wantStatus int
		wantUser   *user.User
	}{
		{
			name:          "success",
			authorization: "Bearer access-token",

			prepare: func(
				verifier *mocks.MockAccessTokenVerifier,
				userService *mocks.MockUserService,
			) {
				verifier.
					EXPECT().
					Verify("access-token").
					Return(userID, nil)

				userService.
					EXPECT().
					GetUserByID(
						gomock.Any(),
						userID,
					).
					Return(
						user.User{
							ID:        userID,
							Email:     "test@example.com",
							CreatedAt: createdAt,
						},
						nil,
					)
			},

			wantStatus: http.StatusOK,
			wantUser: &user.User{
				ID:        userID,
				Email:     "test@example.com",
				CreatedAt: createdAt,
			},
		},
		{
			name:          "user not found",
			authorization: "Bearer access-token",

			prepare: func(
				verifier *mocks.MockAccessTokenVerifier,
				userService *mocks.MockUserService,
			) {
				verifier.
					EXPECT().
					Verify("access-token").
					Return(userID, nil)

				userService.
					EXPECT().
					GetUserByID(
						gomock.Any(),
						userID,
					).
					Return(
						user.User{},
						user.ErrUserNotFound,
					)
			},

			wantStatus: http.StatusUnauthorized,
		},
		{
			name:          "user service error",
			authorization: "Bearer access-token",

			prepare: func(
				verifier *mocks.MockAccessTokenVerifier,
				userService *mocks.MockUserService,
			) {
				verifier.
					EXPECT().
					Verify("access-token").
					Return(userID, nil)

				userService.
					EXPECT().
					GetUserByID(
						gomock.Any(),
						userID,
					).
					Return(
						user.User{},
						serviceErr,
					)
			},

			wantStatus: http.StatusInternalServerError,
		},
		{
			name:       "missing authorization",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:          "invalid token",
			authorization: "Bearer invalid-token",

			prepare: func(
				verifier *mocks.MockAccessTokenVerifier,
				_ *mocks.MockUserService,
			) {
				verifier.
					EXPECT().
					Verify("invalid-token").
					Return(
						uuid.Nil(),
						token.ErrInvalidToken,
					)
			},

			wantStatus: http.StatusUnauthorized,
		},
		{
			name:          "expired token",
			authorization: "Bearer expired-token",

			prepare: func(
				verifier *mocks.MockAccessTokenVerifier,
				_ *mocks.MockUserService,
			) {
				verifier.
					EXPECT().
					Verify("expired-token").
					Return(
						uuid.Nil(),
						token.ErrTokenExpired,
					)
			},

			wantStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			verifier := mocks.NewMockAccessTokenVerifier(ctrl)
			authService := mocks.NewMockAuthService(ctrl)
			userService := mocks.NewMockUserService(ctrl)

			if tt.prepare != nil {
				tt.prepare(
					verifier,
					userService,
				)
			}

			api := httpapi.New(
				zap.NewNop(),
				verifier,
				authService,
				userService,
			)

			req := httptest.NewRequest(
				http.MethodGet,
				"/me",
				nil,
			)

			if tt.authorization != "" {
				req.Header.Set(
					"Authorization",
					tt.authorization,
				)
			}

			rec := httptest.NewRecorder()

			api.Handler().ServeHTTP(
				rec,
				req,
			)

			if rec.Code != tt.wantStatus {
				t.Fatalf(
					"expected status %d, got %d",
					tt.wantStatus,
					rec.Code,
				)
			}

			if tt.wantUser == nil {
				return
			}

			var response struct {
				ID        uuid.UUID `json:"id"`
				Email     string    `json:"email"`
				CreatedAt time.Time `json:"created_at"`
			}

			if err := json.Unmarshal(
				rec.Body.Bytes(),
				&response,
			); err != nil {
				t.Fatalf(
					"unmarshal response: %v",
					err,
				)
			}

			if response.ID != tt.wantUser.ID {
				t.Errorf(
					"expected ID %v, got %v",
					tt.wantUser.ID,
					response.ID,
				)
			}

			if response.Email != tt.wantUser.Email {
				t.Errorf(
					"expected email %q, got %q",
					tt.wantUser.Email,
					response.Email,
				)
			}

			if !response.CreatedAt.Equal(
				tt.wantUser.CreatedAt,
			) {
				t.Errorf(
					"expected created at %v, got %v",
					tt.wantUser.CreatedAt,
					response.CreatedAt,
				)
			}
		})
	}
}
