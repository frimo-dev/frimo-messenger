package httpapi_test

import (
	"encoding/json/v2"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
	"uuid"

	"github.com/frimo-dev/frimo-messenger/internal/handler/httpapi"
	"github.com/frimo-dev/frimo-messenger/internal/handler/httpapi/mocks"
	"github.com/frimo-dev/frimo-messenger/internal/service/auth"
	"github.com/frimo-dev/frimo-messenger/internal/service/user"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

func TestAPI_Me(t *testing.T) {
	userID := uuid.New()
	sessionID := uuid.New()

	identity := auth.Identity{
		UserID:    userID,
		SessionID: sessionID,
	}

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

	serviceErr := errors.New("user service failed")
	authErr := errors.New("authentication service failed")

	tests := []struct {
		name          string
		authorization string

		prepare func(
			authService *mocks.MockAuthService,
			userService *mocks.MockUserService,
		)

		wantStatus  int
		wantUser    *user.User
		wantCode    string
		wantMessage string
	}{
		{
			name:          "success",
			authorization: "Bearer access-token",

			prepare: func(
				authService *mocks.MockAuthService,
				userService *mocks.MockUserService,
			) {
				gomock.InOrder(
					authService.EXPECT().
						Authenticate(gomock.Any(), "access-token").
						Return(identity, nil),

					userService.EXPECT().
						GetUserByID(gomock.Any(), userID).
						Return(
							user.User{
								ID:        userID,
								Email:     "test@example.com",
								CreatedAt: createdAt,
							},
							nil,
						),
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
				authService *mocks.MockAuthService,
				userService *mocks.MockUserService,
			) {
				gomock.InOrder(
					authService.EXPECT().
						Authenticate(gomock.Any(), "access-token").
						Return(identity, nil),

					userService.EXPECT().
						GetUserByID(gomock.Any(), userID).
						Return(user.User{}, user.ErrUserNotFound),
				)
			},

			wantStatus:  http.StatusUnauthorized,
			wantCode:    "authentication_error",
			wantMessage: "authentication failed",
		},
		{
			name:          "user service error",
			authorization: "Bearer access-token",

			prepare: func(
				authService *mocks.MockAuthService,
				userService *mocks.MockUserService,
			) {
				gomock.InOrder(
					authService.EXPECT().
						Authenticate(gomock.Any(), "access-token").
						Return(identity, nil),

					userService.EXPECT().
						GetUserByID(gomock.Any(), userID).
						Return(user.User{}, serviceErr),
				)
			},

			wantStatus:  http.StatusInternalServerError,
			wantCode:    "internal_error",
			wantMessage: "internal server error",
		},
		{
			name: "missing authorization",

			wantStatus:  http.StatusUnauthorized,
			wantCode:    "authentication_error",
			wantMessage: "authentication failed",
		},
		{
			name:          "invalid authorization scheme",
			authorization: "Basic access-token",

			wantStatus:  http.StatusUnauthorized,
			wantCode:    "authentication_error",
			wantMessage: "authentication failed",
		},
		{
			name:          "empty bearer token",
			authorization: "Bearer ",

			wantStatus:  http.StatusUnauthorized,
			wantCode:    "authentication_error",
			wantMessage: "authentication failed",
		},
		{
			name:          "malformed authorization",
			authorization: "Bearer token extra",

			wantStatus:  http.StatusUnauthorized,
			wantCode:    "authentication_error",
			wantMessage: "authentication failed",
		},
		{
			name:          "invalid access token",
			authorization: "Bearer invalid-token",

			prepare: func(
				authService *mocks.MockAuthService,
				_ *mocks.MockUserService,
			) {
				authService.EXPECT().
					Authenticate(gomock.Any(), "invalid-token").
					Return(auth.Identity{}, auth.ErrAccessTokenInvalid)
			},

			wantStatus:  http.StatusUnauthorized,
			wantCode:    "authentication_error",
			wantMessage: "authentication failed",
		},
		{
			name:          "inactive session",
			authorization: "Bearer inactive-token",

			prepare: func(
				authService *mocks.MockAuthService,
				_ *mocks.MockUserService,
			) {
				authService.EXPECT().
					Authenticate(gomock.Any(), "inactive-token").
					Return(auth.Identity{}, auth.ErrSessionInactive)
			},

			wantStatus:  http.StatusUnauthorized,
			wantCode:    "authentication_error",
			wantMessage: "authentication failed",
		},
		{
			name:          "authentication service error",
			authorization: "Bearer access-token",

			prepare: func(
				authService *mocks.MockAuthService,
				_ *mocks.MockUserService,
			) {
				authService.EXPECT().
					Authenticate(gomock.Any(), "access-token").
					Return(auth.Identity{}, authErr)
			},

			wantStatus:  http.StatusInternalServerError,
			wantCode:    "internal_error",
			wantMessage: "internal server error",
		},
		{
			name:          "case insensitive bearer scheme",
			authorization: "bEaReR access-token",

			prepare: func(
				authService *mocks.MockAuthService,
				userService *mocks.MockUserService,
			) {
				gomock.InOrder(
					authService.EXPECT().
						Authenticate(gomock.Any(), "access-token").
						Return(identity, nil),

					userService.EXPECT().
						GetUserByID(gomock.Any(), userID).
						Return(
							user.User{
								ID:        userID,
								Email:     "test@example.com",
								CreatedAt: createdAt,
							},
							nil,
						),
				)
			},

			wantStatus: http.StatusOK,
			wantUser: &user.User{
				ID:        userID,
				Email:     "test@example.com",
				CreatedAt: createdAt,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			authService := mocks.NewMockAuthService(ctrl)
			userService := mocks.NewMockUserService(ctrl)

			if tt.prepare != nil {
				tt.prepare(authService, userService)
			}

			api := httpapi.New(
				zap.NewNop(),
				authService,
				userService,
			)

			req := httptest.NewRequest(
				http.MethodGet,
				"/me",
				nil,
			)

			if tt.authorization != "" {
				req.Header.Set("Authorization", tt.authorization)
			}

			rec := httptest.NewRecorder()

			api.Handler().ServeHTTP(rec, req)

			// Ошибочные ответы проверяем через общий helper.
			if tt.wantStatus != http.StatusOK {
				assertErrorResponse(
					t,
					rec,
					tt.wantStatus,
					tt.wantCode,
					tt.wantMessage,
				)

				return
			}

			// Проверяем статус успешного ответа.
			if rec.Code != http.StatusOK {
				t.Fatalf(
					"status = %d, want %d; body = %s",
					rec.Code,
					http.StatusOK,
					rec.Body.String(),
				)
			}

			// Проверяем Content-Type.
			if contentType := rec.Header().Get("Content-Type"); contentType != "application/json" {
				t.Errorf(
					"Content-Type = %q, want %q",
					contentType,
					"application/json",
				)
			}

			// Проверяем данные пользователя.
			var response struct {
				ID        uuid.UUID `json:"id"`
				Email     string    `json:"email"`
				CreatedAt time.Time `json:"created_at"`
			}

			if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
				t.Fatalf(
					"unmarshal response: %v; body = %s",
					err,
					rec.Body.String(),
				)
			}

			if response.ID != tt.wantUser.ID {
				t.Errorf(
					"ID = %v, want %v",
					response.ID,
					tt.wantUser.ID,
				)
			}

			if response.Email != tt.wantUser.Email {
				t.Errorf(
					"Email = %q, want %q",
					response.Email,
					tt.wantUser.Email,
				)
			}

			if !response.CreatedAt.Equal(tt.wantUser.CreatedAt) {
				t.Errorf(
					"CreatedAt = %v, want %v",
					response.CreatedAt,
					tt.wantUser.CreatedAt,
				)
			}
		})
	}
}

func TestAPI_Me_MethodNotAllowed(t *testing.T) {
	tests := []struct {
		name   string
		method string
	}{
		{
			name:   "POST",
			method: http.MethodPost,
		},
		{
			name:   "PUT",
			method: http.MethodPut,
		},
		{
			name:   "PATCH",
			method: http.MethodPatch,
		},
		{
			name:   "DELETE",
			method: http.MethodDelete,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			authService := mocks.NewMockAuthService(ctrl)
			userService := mocks.NewMockUserService(ctrl)

			api := httpapi.New(
				zap.NewNop(),
				authService,
				userService,
			)

			req := httptest.NewRequest(
				tt.method,
				"/me",
				nil,
			)

			rec := httptest.NewRecorder()

			api.Handler().ServeHTTP(rec, req)

			if rec.Code != http.StatusMethodNotAllowed {
				t.Fatalf(
					"status = %d, want %d; body = %s",
					rec.Code,
					http.StatusMethodNotAllowed,
					rec.Body.String(),
				)
			}
		})
	}
}
