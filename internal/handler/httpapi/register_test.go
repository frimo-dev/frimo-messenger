package httpapi_test

import (
	"encoding/json/v2"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"uuid"

	"github.com/frimo-dev/frimo-messenger/internal/handler/httpapi"
	"github.com/frimo-dev/frimo-messenger/internal/handler/httpapi/mocks"
	"github.com/frimo-dev/frimo-messenger/internal/service/auth"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

func TestAPI_Register(t *testing.T) {
	userID := uuid.New()

	const (
		validEmail    = "test@example.com"
		validPassword = "very-secure-password"
	)

	validBody := `{
		"email": "test@example.com",
		"password": "very-secure-password"
	}`

	internalErr := errors.New("database unavailable")

	tests := []struct {
		name string
		body string

		prepare func(authService *mocks.MockAuthService)

		wantStatus  int
		wantID      string
		wantEmail   string
		wantCode    string
		wantMessage string
	}{
		{
			name: "success",
			body: validBody,

			prepare: func(authService *mocks.MockAuthService) {
				authService.EXPECT().
					Register(
						gomock.Any(),
						auth.RegistrationInput{
							Email:    validEmail,
							Password: validPassword,
						},
					).
					Return(
						auth.User{
							ID:    userID,
							Email: validEmail,
						},
						nil,
					)
			},

			wantStatus: http.StatusAccepted,
			wantID:     userID.String(),
			wantEmail:  validEmail,
		},
		{
			name: "validation error",
			body: `{
				"email": "not-an-email",
				"password": "very-secure-password"
			}`,

			prepare: func(authService *mocks.MockAuthService) {
				authService.EXPECT().
					Register(
						gomock.Any(),
						auth.RegistrationInput{
							Email:    "not-an-email",
							Password: validPassword,
						},
					).
					Return(
						auth.User{},
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
			name: "email already exists",
			body: validBody,

			prepare: func(authService *mocks.MockAuthService) {
				authService.EXPECT().
					Register(
						gomock.Any(),
						auth.RegistrationInput{
							Email:    validEmail,
							Password: validPassword,
						},
					).
					Return(
						auth.User{},
						auth.ErrEmailAlreadyExists,
					)
			},

			wantStatus:  http.StatusConflict,
			wantCode:    "email_already_exists",
			wantMessage: "user with this email already exists",
		},
		{
			name: "internal error",
			body: validBody,

			prepare: func(authService *mocks.MockAuthService) {
				authService.EXPECT().
					Register(
						gomock.Any(),
						auth.RegistrationInput{
							Email:    validEmail,
							Password: validPassword,
						},
					).
					Return(
						auth.User{},
						internalErr,
					)
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
			name: "unknown field",
			body: `{
				"email": "test@example.com",
				"password": "very-secure-password",
				"admin": true
			}`,

			wantStatus:  http.StatusBadRequest,
			wantCode:    "invalid_request",
			wantMessage: "invalid request body",
		},
		{
			name: "multiple JSON objects",
			body: `{
				"email": "test@example.com",
				"password": "very-secure-password"
			}{
				"email": "second@example.com",
				"password": "another-password"
			}`,

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
			name: "body too large",
			body: `{
				"email": "test@example.com",
				"password": "` + strings.Repeat("a", 20*1024) + `"
			}`,

			wantStatus:  http.StatusBadRequest,
			wantCode:    "invalid_request",
			wantMessage: "invalid request body",
		},
		{
			name: "invalid email field type",
			body: `{
				"email": 123,
				"password": "very-secure-password"
			}`,

			wantStatus:  http.StatusBadRequest,
			wantCode:    "invalid_request",
			wantMessage: "invalid request body",
		},
		{
			name: "invalid password field type",
			body: `{
				"email": "test@example.com",
				"password": 123
			}`,

			wantStatus:  http.StatusBadRequest,
			wantCode:    "invalid_request",
			wantMessage: "invalid request body",
		},
		{
			name: "missing email",
			body: `{
				"password": "very-secure-password"
			}`,

			wantStatus:  http.StatusBadRequest,
			wantCode:    "invalid_request",
			wantMessage: "invalid request body",
		},
		{
			name: "empty email",
			body: `{
				"email": "",
				"password": "very-secure-password"
			}`,

			wantStatus:  http.StatusBadRequest,
			wantCode:    "invalid_request",
			wantMessage: "invalid request body",
		},
		{
			name: "missing password",
			body: `{
				"email": "test@example.com"
			}`,

			wantStatus:  http.StatusBadRequest,
			wantCode:    "invalid_request",
			wantMessage: "invalid request body",
		},
		{
			name: "empty password",
			body: `{
				"email": "test@example.com",
				"password": ""
			}`,

			wantStatus:  http.StatusBadRequest,
			wantCode:    "invalid_request",
			wantMessage: "invalid request body",
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
				"/auth/register",
				strings.NewReader(tt.body),
			)

			rec := httptest.NewRecorder()

			api.Handler().ServeHTTP(rec, req)

			// Ошибочные ответы проверяем через общий helper.
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

			// Проверяем статус успешного ответа.
			if rec.Code != http.StatusAccepted {
				t.Fatalf(
					"status = %d, want %d; body = %s",
					rec.Code,
					http.StatusAccepted,
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

			// Проверяем данные зарегистрированного пользователя.
			var response struct {
				ID    string `json:"id"`
				Email string `json:"email"`
			}

			if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
				t.Fatalf(
					"unmarshal success response: %v; body = %s",
					err,
					rec.Body.String(),
				)
			}

			if response.ID != tt.wantID {
				t.Errorf(
					"ID = %q, want %q",
					response.ID,
					tt.wantID,
				)
			}

			if response.Email != tt.wantEmail {
				t.Errorf(
					"Email = %q, want %q",
					response.Email,
					tt.wantEmail,
				)
			}
		})
	}
}

func TestAPI_Register_MethodNotAllowed(t *testing.T) {
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
				"/auth/register",
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
