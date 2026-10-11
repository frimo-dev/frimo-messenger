package httpapi_test

import (
	"encoding/json/v2"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"

	"github.com/frimo-dev/frimo-messenger/internal/handler/httpapi"
	"github.com/frimo-dev/frimo-messenger/internal/handler/httpapi/mocks"
	"github.com/frimo-dev/frimo-messenger/internal/service/auth"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

func TestAPI_Login(t *testing.T) {
	internalErr := errors.New("database unavailable")

	const (
		validEmail      = "test@example.com"
		validPassword   = "correct-password"
		validDeviceName = "Chrome on Windows"
	)

	validBody := `{
		"email": "test@example.com",
		"password": "correct-password",
		"device_name": "Chrome on Windows"
	}`

	tests := []struct {
		name       string
		body       string
		remoteAddr string
		realIP     string

		prepare func(authService *mocks.MockAuthService)

		wantStatus       int
		wantAccessToken  string
		wantRefreshToken string
		wantCode         string
		wantMessage      string
	}{
		{
			name: "success",
			body: validBody,

			prepare: func(authService *mocks.MockAuthService) {
				authService.EXPECT().
					Login(
						gomock.Any(),
						auth.LoginInput{
							Email:      validEmail,
							Password:   validPassword,
							DeviceName: validDeviceName,
							IP:         netip.MustParseAddr("192.0.2.1"),
						},
					).
					Return(
						auth.TokenPair{
							AccessToken:  "access-token",
							RefreshToken: "refresh-token",
						},
						nil,
					)
			},

			wantStatus:       http.StatusOK,
			wantAccessToken:  "access-token",
			wantRefreshToken: "refresh-token",
		},
		{
			name:       "success with X-Real-IP",
			body:       validBody,
			realIP:     "203.0.113.10",
			remoteAddr: "192.0.2.20:12345",

			prepare: func(authService *mocks.MockAuthService) {
				authService.EXPECT().
					Login(
						gomock.Any(),
						auth.LoginInput{
							Email:      validEmail,
							Password:   validPassword,
							DeviceName: validDeviceName,
							IP:         netip.MustParseAddr("203.0.113.10"),
						},
					).
					Return(
						auth.TokenPair{
							AccessToken:  "access-token",
							RefreshToken: "refresh-token",
						},
						nil,
					)
			},

			wantStatus:       http.StatusOK,
			wantAccessToken:  "access-token",
			wantRefreshToken: "refresh-token",
		},
		{
			name: "validation error",
			body: `{
				"email": "not-an-email",
				"password": "correct-password",
				"device_name": "Chrome"
			}`,

			prepare: func(authService *mocks.MockAuthService) {
				authService.EXPECT().
					Login(
						gomock.Any(),
						auth.LoginInput{
							Email:      "not-an-email",
							Password:   validPassword,
							DeviceName: "Chrome",
							IP:         netip.MustParseAddr("192.0.2.1"),
						},
					).
					Return(
						auth.TokenPair{},
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
			name: "invalid credentials",
			body: `{
				"email": "test@example.com",
				"password": "wrong-password",
				"device_name": "Chrome"
			}`,

			prepare: func(authService *mocks.MockAuthService) {
				authService.EXPECT().
					Login(
						gomock.Any(),
						auth.LoginInput{
							Email:      validEmail,
							Password:   "wrong-password",
							DeviceName: "Chrome",
							IP:         netip.MustParseAddr("192.0.2.1"),
						},
					).
					Return(
						auth.TokenPair{},
						auth.ErrInvalidCredentials,
					)
			},

			wantStatus:  http.StatusUnauthorized,
			wantCode:    "invalid_credentials",
			wantMessage: "invalid credentials",
		},
		{
			name: "email not verified",
			body: validBody,

			prepare: func(authService *mocks.MockAuthService) {
				authService.EXPECT().
					Login(
						gomock.Any(),
						auth.LoginInput{
							Email:      validEmail,
							Password:   validPassword,
							DeviceName: validDeviceName,
							IP:         netip.MustParseAddr("192.0.2.1"),
						},
					).
					Return(
						auth.TokenPair{},
						auth.ErrEmailNotVerified,
					)
			},

			wantStatus:  http.StatusForbidden,
			wantCode:    "email_not_verified",
			wantMessage: "email not verified",
		},
		{
			name: "internal error",
			body: validBody,

			prepare: func(authService *mocks.MockAuthService) {
				authService.EXPECT().
					Login(
						gomock.Any(),
						auth.LoginInput{
							Email:      validEmail,
							Password:   validPassword,
							DeviceName: validDeviceName,
							IP:         netip.MustParseAddr("192.0.2.1"),
						},
					).
					Return(
						auth.TokenPair{},
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
				"password": "correct-password",
				"device_name": "Chrome",
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
				"password": "correct-password",
				"device_name": "Chrome"
			}{
				"email": "second@example.com",
				"password": "another-password",
				"device_name": "Firefox"
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
			name: "invalid email field type",
			body: `{
				"email": 123,
				"password": "correct-password",
				"device_name": "Chrome"
			}`,

			wantStatus:  http.StatusBadRequest,
			wantCode:    "invalid_request",
			wantMessage: "invalid request body",
		},
		{
			name: "invalid device name field type",
			body: `{
				"email": "test@example.com",
				"password": "correct-password",
				"device_name": 123
			}`,

			wantStatus:  http.StatusBadRequest,
			wantCode:    "invalid_request",
			wantMessage: "invalid request body",
		},
		{
			name:   "invalid X-Real-IP",
			body:   validBody,
			realIP: "invalid-ip",

			prepare: func(authService *mocks.MockAuthService) {
				authService.EXPECT().
					Login(
						gomock.Any(),
						auth.LoginInput{
							Email:      validEmail,
							Password:   validPassword,
							DeviceName: validDeviceName,
							IP:         netip.Addr{},
						},
					).
					Return(
						auth.TokenPair{
							AccessToken:  "access-token",
							RefreshToken: "refresh-token",
						},
						nil,
					)
			},

			wantStatus:       http.StatusOK,
			wantAccessToken:  "access-token",
			wantRefreshToken: "refresh-token",
		},
		{
			name:       "invalid remote address",
			body:       validBody,
			remoteAddr: "invalid-remote-address",

			prepare: func(authService *mocks.MockAuthService) {
				authService.EXPECT().
					Login(
						gomock.Any(),
						auth.LoginInput{
							Email:      validEmail,
							Password:   validPassword,
							DeviceName: validDeviceName,
							IP:         netip.Addr{},
						},
					).
					Return(
						auth.TokenPair{
							AccessToken:  "access-token",
							RefreshToken: "refresh-token",
						},
						nil,
					)
			},

			wantStatus:       http.StatusOK,
			wantAccessToken:  "access-token",
			wantRefreshToken: "refresh-token",
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
				"/auth/login",
				strings.NewReader(tt.body),
			)

			if tt.remoteAddr != "" {
				req.RemoteAddr = tt.remoteAddr
			} else {
				req.RemoteAddr = "192.0.2.1:12345"
			}

			if tt.realIP != "" {
				req.Header.Set("X-Real-IP", tt.realIP)
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

			// Проверяем оба токена в ответе.
			var response struct {
				AccessToken  string `json:"access_token"`
				RefreshToken string `json:"refresh_token"`
			}

			if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
				t.Fatalf(
					"unmarshal success response: %v; body = %s",
					err,
					rec.Body.String(),
				)
			}

			if response.AccessToken != tt.wantAccessToken {
				t.Errorf(
					"AccessToken = %q, want %q",
					response.AccessToken,
					tt.wantAccessToken,
				)
			}

			if response.RefreshToken != tt.wantRefreshToken {
				t.Errorf(
					"RefreshToken = %q, want %q",
					response.RefreshToken,
					tt.wantRefreshToken,
				)
			}
		})
	}
}

func TestAPI_Login_MethodNotAllowed(t *testing.T) {
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
				"/auth/login",
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
