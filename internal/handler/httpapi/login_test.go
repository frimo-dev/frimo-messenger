package httpapi_test

import (
	"encoding/json/v2"
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

func TestAPI_Login_Success(t *testing.T) {
	ctrl := gomock.NewController(t)

	authService := mocks.NewMockAuthService(ctrl)
	userService := mocks.NewMockUserService(ctrl)
	accessTokenVerifier := mocks.NewMockAccessTokenVerifier(ctrl)

	authService.
		EXPECT().
		Login(
			gomock.Any(),
			"test@example.com",
			"correct-password",
		).
		Return("access-token", nil)

	api := httpapi.New(zap.NewNop(), accessTokenVerifier, authService, userService)

	req := httptest.NewRequest(
		http.MethodPost,
		"/auth/login",
		strings.NewReader(`{"email": "test@example.com","password": "correct-password"}`),
	)

	rec := httptest.NewRecorder()

	api.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusOK,
			rec.Code,
		)
	}

	if contentType := rec.Header().Get("Content-Type"); contentType != "application/json" {
		t.Errorf(
			"expected Content-Type %q, got %q",
			"application/json",
			contentType,
		)
	}

	var response struct {
		AccessToken string `json:"access_token"`
	}

	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if response.AccessToken != "access-token" {
		t.Errorf(
			"expected access token %q, got %q",
			"access-token",
			response.AccessToken,
		)
	}
}

func TestAPI_Login_Errors(t *testing.T) {
	internalErr := errors.New("database unavailable")

	tests := []struct {
		name string
		body string

		prepare func(authService *mocks.MockAuthService)

		wantStatus  int
		wantCode    string
		wantMessage string
	}{
		{
			name:        "invalid json",
			body:        `{"email":`,
			wantStatus:  http.StatusBadRequest,
			wantCode:    "invalid_request",
			wantMessage: "invalid request body",
		},
		{
			name: "validation error",
			body: `{"email": "not-an-email","password": "correct-password"}`,

			prepare: func(authService *mocks.MockAuthService) {
				authService.
					EXPECT().
					Login(
						gomock.Any(),
						"not-an-email",
						"correct-password",
					).
					Return(
						"",
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
			body: `{"email": "test@example.com","password": "wrong-password"}`,

			prepare: func(authService *mocks.MockAuthService) {
				authService.
					EXPECT().
					Login(
						gomock.Any(),
						"test@example.com",
						"wrong-password",
					).
					Return("", auth.ErrInvalidCredentials)
			},

			wantStatus:  http.StatusUnauthorized,
			wantCode:    "invalid_credentials",
			wantMessage: "invalid credentials",
		},
		{
			name: "email not verified",
			body: `{"email": "test@example.com","password": "correct-password"}`,

			prepare: func(authService *mocks.MockAuthService) {
				authService.
					EXPECT().
					Login(
						gomock.Any(),
						"test@example.com",
						"correct-password",
					).
					Return("", auth.ErrEmailNotVerified)
			},

			wantStatus:  http.StatusForbidden,
			wantCode:    "email_not_verified",
			wantMessage: "email not verified",
		},
		{
			name: "internal error",
			body: `{"email": "test@example.com","password": "correct-password"}`,

			prepare: func(authService *mocks.MockAuthService) {
				authService.
					EXPECT().
					Login(
						gomock.Any(),
						"test@example.com",
						"correct-password",
					).
					Return("", internalErr)
			},

			wantStatus:  http.StatusInternalServerError,
			wantCode:    "internal_error",
			wantMessage: "internal server error",
		},
		{
			name: "unknown field",
			body: `{"email": "test@example.com","password": "correct-password","admin": true}`,

			wantStatus:  http.StatusBadRequest,
			wantCode:    "invalid_request",
			wantMessage: "invalid request body",
		},
		{
			name: "multiple json objects",
			body: `{"email": "test@example.com","password": "correct-password"}{"email": "second@example.com","password": "another-password"}`,

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
			name: "invalid field type",
			body: `{"email": 123,"password": "correct-password"}`,

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
			accessTokenVerifier := mocks.NewMockAccessTokenVerifier(ctrl)

			if tt.prepare != nil {
				tt.prepare(authService)
			}

			api := httpapi.New(zap.NewNop(), accessTokenVerifier, authService, userService)

			req := httptest.NewRequest(
				http.MethodPost,
				"/auth/login",
				strings.NewReader(tt.body),
			)

			rec := httptest.NewRecorder()

			api.Handler().ServeHTTP(rec, req)

			assertErrorResponse(
				t,
				rec,
				tt.wantStatus,
				tt.wantCode,
				tt.wantMessage,
			)
		})
	}
}

func TestAPI_Login_MethodNotAllowed(t *testing.T) {
	ctrl := gomock.NewController(t)

	authService := mocks.NewMockAuthService(ctrl)
	userService := mocks.NewMockUserService(ctrl)
	accessTokenVerifier := mocks.NewMockAccessTokenVerifier(ctrl)

	api := httpapi.New(zap.NewNop(), accessTokenVerifier, authService, userService)

	req := httptest.NewRequest(
		http.MethodGet,
		"/auth/login",
		nil,
	)

	rec := httptest.NewRecorder()

	api.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusMethodNotAllowed,
			rec.Code,
		)
	}
}

func assertErrorResponse(
	t *testing.T,
	rec *httptest.ResponseRecorder,
	wantStatus int,
	wantCode string,
	wantMessage string,
) {
	t.Helper()

	if rec.Code != wantStatus {
		t.Fatalf(
			"expected status %d, got %d",
			wantStatus,
			rec.Code,
		)
	}

	if contentType := rec.Header().Get("Content-Type"); contentType != "application/json" {
		t.Errorf(
			"expected Content-Type %q, got %q",
			"application/json",
			contentType,
		)
	}

	var response struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal error response: %v", err)
	}

	if response.Error.Code != wantCode {
		t.Errorf(
			"expected error code %q, got %q",
			wantCode,
			response.Error.Code,
		)
	}

	if response.Error.Message != wantMessage {
		t.Errorf(
			"expected error message %q, got %q",
			wantMessage,
			response.Error.Message,
		)
	}
}
