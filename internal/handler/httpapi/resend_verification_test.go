package httpapi_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/frimo-dev/frimo-messenger/internal/handler/httpapi"
	"github.com/frimo-dev/frimo-messenger/internal/handler/httpapi/mocks"
	"github.com/frimo-dev/frimo-messenger/internal/service/auth"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

func TestAPI_ResendVerification(t *testing.T) {
	const validEmail = "test@example.com"

	validBody := `{
		"email": "test@example.com"
	}`

	serviceErr := errors.New("database unavailable")

	tests := []struct {
		name string
		body string

		prepare func(authService *mocks.MockAuthService)

		wantStatus  int
		wantCode    string
		wantMessage string
	}{
		{
			name: "success",
			body: validBody,

			prepare: func(authService *mocks.MockAuthService) {
				authService.EXPECT().
					ResendVerification(gomock.Any(), validEmail).
					Return(nil)
			},

			wantStatus: http.StatusAccepted,
		},
		{
			name: "validation error",
			body: `{
				"email": "not-an-email"
			}`,

			prepare: func(authService *mocks.MockAuthService) {
				authService.EXPECT().
					ResendVerification(gomock.Any(), "not-an-email").
					Return(
						&auth.ValidationError{
							Code:    "invalid_email",
							Field:   "email",
							Message: "email has invalid format",
						},
					)
			},

			wantStatus:  http.StatusBadRequest,
			wantCode:    "invalid_email",
			wantMessage: "email has invalid format",
		},
		{
			name: "user not found",
			body: validBody,

			prepare: func(authService *mocks.MockAuthService) {
				authService.EXPECT().
					ResendVerification(gomock.Any(), validEmail).
					Return(auth.ErrUserNotFound)
			},

			wantStatus: http.StatusAccepted,
		},
		{
			name: "already verified",
			body: validBody,

			prepare: func(authService *mocks.MockAuthService) {
				authService.EXPECT().
					ResendVerification(gomock.Any(), validEmail).
					Return(auth.ErrAlreadyVerified)
			},

			wantStatus: http.StatusAccepted,
		},
		{
			name: "resend cooldown",
			body: validBody,

			prepare: func(authService *mocks.MockAuthService) {
				authService.EXPECT().
					ResendVerification(gomock.Any(), validEmail).
					Return(auth.ErrResendCooldown)
			},

			wantStatus: http.StatusAccepted,
		},
		{
			name: "resend hourly limit",
			body: validBody,

			prepare: func(authService *mocks.MockAuthService) {
				authService.EXPECT().
					ResendVerification(gomock.Any(), validEmail).
					Return(auth.ErrResendHourlyLimit)
			},

			wantStatus: http.StatusAccepted,
		},
		{
			name: "internal error",
			body: validBody,

			prepare: func(authService *mocks.MockAuthService) {
				authService.EXPECT().
					ResendVerification(gomock.Any(), validEmail).
					Return(serviceErr)
			},

			wantStatus:  http.StatusInternalServerError,
			wantCode:    "internal_error",
			wantMessage: "internal server error",
		},
		{
			name:        "invalid JSON",
			body:        `{"email":`,
			wantStatus:  http.StatusBadRequest,
			wantCode:    "invalid_request",
			wantMessage: "invalid request body",
		},
		{
			name:        "empty body",
			body:        "",
			wantStatus:  http.StatusBadRequest,
			wantCode:    "invalid_request",
			wantMessage: "invalid request body",
		},
		{
			name: "invalid email field type",
			body: `{
				"email": 123
			}`,

			wantStatus:  http.StatusBadRequest,
			wantCode:    "invalid_request",
			wantMessage: "invalid request body",
		},
		{
			name: "multiple JSON objects",
			body: `{
				"email": "test@example.com"
			}{
				"email": "second@example.com"
			}`,

			wantStatus:  http.StatusBadRequest,
			wantCode:    "invalid_request",
			wantMessage: "invalid request body",
		},
		{
			name: "missing email",
			body: `{}`,

			prepare: func(authService *mocks.MockAuthService) {
				authService.EXPECT().
					ResendVerification(gomock.Any(), "").
					Return(
						&auth.ValidationError{
							Code:    "invalid_email",
							Field:   "email",
							Message: "email is required",
						},
					)
			},

			wantStatus:  http.StatusBadRequest,
			wantCode:    "invalid_email",
			wantMessage: "email is required",
		},
		{
			name: "empty email",
			body: `{
				"email": ""
			}`,

			prepare: func(authService *mocks.MockAuthService) {
				authService.EXPECT().
					ResendVerification(gomock.Any(), "").
					Return(
						&auth.ValidationError{
							Code:    "invalid_email",
							Field:   "email",
							Message: "email is required",
						},
					)
			},

			wantStatus:  http.StatusBadRequest,
			wantCode:    "invalid_email",
			wantMessage: "email is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			authService := mocks.NewMockAuthService(ctrl)
			userService := mocks.NewMockUserService(ctrl)

			if tt.prepare != nil {
				tt.prepare(authService)
			}

			api := httpapi.New(
				zap.NewNop(),
				authService,
				userService,
			)

			req := httptest.NewRequest(
				http.MethodPost,
				"/auth/resend",
				strings.NewReader(tt.body),
			)

			rec := httptest.NewRecorder()

			api.Handler().ServeHTTP(rec, req)

			// Проверяем ошибочные ответы через общий helper.
			if tt.wantStatus != http.StatusAccepted {
				assertErrorResponse(
					t,
					rec,
					tt.wantStatus,
					tt.wantCode,
					tt.wantMessage,
				)

				return
			}

			// Проверяем успешный ответ.
			if rec.Code != http.StatusAccepted {
				t.Fatalf(
					"status = %d, want %d; body = %s",
					rec.Code,
					http.StatusAccepted,
					rec.Body.String(),
				)
			}

			// Успешный ответ не должен содержать тело.
			if rec.Body.Len() != 0 {
				t.Errorf(
					"body = %q, want empty body",
					rec.Body.String(),
				)
			}
		})
	}
}

func TestAPI_ResendVerification_MethodNotAllowed(t *testing.T) {
	tests := []struct {
		name   string
		method string
	}{
		{
			name:   "GET",
			method: http.MethodGet,
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
				"/auth/resend",
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
