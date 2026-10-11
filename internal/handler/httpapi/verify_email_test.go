package httpapi_test

import (
	"encoding/json/v2"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/frimo-dev/frimo-messenger/internal/handler/httpapi"
	"github.com/frimo-dev/frimo-messenger/internal/handler/httpapi/mocks"
	"github.com/frimo-dev/frimo-messenger/internal/service/auth"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

func TestAPI_VerifyEmail(t *testing.T) {
	serviceErr := errors.New("database unavailable")

	tests := []struct {
		name string
		url  string

		prepare func(authService *mocks.MockAuthService)

		wantStatus   int
		wantResponse string
		wantCode     string
		wantMessage  string
	}{
		{
			name: "success",
			url:  "/auth/verify-email?token=raw-verification-token",

			prepare: func(authService *mocks.MockAuthService) {
				authService.EXPECT().
					ConfirmEmail(gomock.Any(), "raw-verification-token").
					Return(nil)
			},

			wantStatus:   http.StatusOK,
			wantResponse: "email_verified",
		},
		{
			name: "missing token",
			url:  "/auth/verify-email",

			wantStatus:  http.StatusBadRequest,
			wantCode:    "verification_token_required",
			wantMessage: "verification token is required",
		},
		{
			name: "empty token",
			url:  "/auth/verify-email?token=",

			wantStatus:  http.StatusBadRequest,
			wantCode:    "verification_token_required",
			wantMessage: "verification token is required",
		},
		{
			name: "invalid token",
			url:  "/auth/verify-email?token=invalid-token",

			prepare: func(authService *mocks.MockAuthService) {
				authService.EXPECT().
					ConfirmEmail(gomock.Any(), "invalid-token").
					Return(auth.ErrInvalidToken)
			},

			wantStatus:  http.StatusBadRequest,
			wantCode:    "invalid_verification_token",
			wantMessage: "verification token is invalid",
		},
		{
			name: "expired token",
			url:  "/auth/verify-email?token=expired-token",

			prepare: func(authService *mocks.MockAuthService) {
				authService.EXPECT().
					ConfirmEmail(gomock.Any(), "expired-token").
					Return(auth.ErrExpiredToken)
			},

			wantStatus:  http.StatusBadRequest,
			wantCode:    "verification_token_expired",
			wantMessage: "verification token has expired",
		},
		{
			name: "used token",
			url:  "/auth/verify-email?token=used-token",

			prepare: func(authService *mocks.MockAuthService) {
				authService.EXPECT().
					ConfirmEmail(gomock.Any(), "used-token").
					Return(auth.ErrUsedToken)
			},

			wantStatus:  http.StatusConflict,
			wantCode:    "verification_token_used",
			wantMessage: "verification token has already been used",
		},
		{
			name: "revoked token",
			url:  "/auth/verify-email?token=revoked-token",

			prepare: func(authService *mocks.MockAuthService) {
				authService.EXPECT().
					ConfirmEmail(gomock.Any(), "revoked-token").
					Return(auth.ErrRevokedToken)
			},

			wantStatus:  http.StatusBadRequest,
			wantCode:    "verification_token_revoked",
			wantMessage: "verification token has revoked",
		},
		{
			name: "internal error",
			url:  "/auth/verify-email?token=raw-verification-token",

			prepare: func(authService *mocks.MockAuthService) {
				authService.EXPECT().
					ConfirmEmail(gomock.Any(), "raw-verification-token").
					Return(serviceErr)
			},

			wantStatus:  http.StatusInternalServerError,
			wantCode:    "internal_error",
			wantMessage: "internal server error",
		},
		{
			name: "token with URL encoding",
			url:  "/auth/verify-email?token=token%2Bwith%2Fspecial%3Dchars",

			prepare: func(authService *mocks.MockAuthService) {
				authService.EXPECT().
					ConfirmEmail(gomock.Any(), "token+with/special=chars").
					Return(nil)
			},

			wantStatus:   http.StatusOK,
			wantResponse: "email_verified",
		},
		{
			name: "token with additional query parameters",
			url:  "/auth/verify-email?token=raw-verification-token&source=email",

			prepare: func(authService *mocks.MockAuthService) {
				authService.EXPECT().
					ConfirmEmail(gomock.Any(), "raw-verification-token").
					Return(nil)
			},

			wantStatus:   http.StatusOK,
			wantResponse: "email_verified",
		},
		{
			name: "first token is used when multiple tokens provided",
			url:  "/auth/verify-email?token=first-token&token=second-token",

			prepare: func(authService *mocks.MockAuthService) {
				authService.EXPECT().
					ConfirmEmail(gomock.Any(), "first-token").
					Return(nil)
			},

			wantStatus:   http.StatusOK,
			wantResponse: "email_verified",
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
				http.MethodGet,
				tt.url,
				nil,
			)

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

			// Проверяем JSON-ответ.
			var response struct {
				Status string `json:"status"`
			}

			if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
				t.Fatalf(
					"unmarshal success response: %v; body = %s",
					err,
					rec.Body.String(),
				)
			}

			if response.Status != tt.wantResponse {
				t.Errorf(
					"status = %q, want %q",
					response.Status,
					tt.wantResponse,
				)
			}
		})
	}
}

func TestAPI_VerifyEmail_MethodNotAllowed(t *testing.T) {
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
				"/auth/verify-email?token=raw-verification-token",
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
