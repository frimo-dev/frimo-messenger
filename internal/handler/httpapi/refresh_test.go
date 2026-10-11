package httpapi_test

import (
	"encoding/json/v2"
	"errors"
	"fmt"
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

func TestAPI_Refresh(t *testing.T) {
	operationID := uuid.New()

	validBody := fmt.Sprintf(`{
		"operation_id": %q,
		"refresh_token": "refresh-token"
	}`, operationID.String())

	internalErr := errors.New("database unavailable")

	tests := []struct {
		name string
		body string

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
					Refresh(
						gomock.Any(),
						auth.RefreshInput{
							OperationID:     operationID,
							RawRefreshToken: "refresh-token",
						},
					).
					Return(
						auth.TokenPair{
							AccessToken:  "new-access-token",
							RefreshToken: "new-refresh-token",
						},
						nil,
					)
			},

			wantStatus:       http.StatusOK,
			wantAccessToken:  "new-access-token",
			wantRefreshToken: "new-refresh-token",
		},
		{
			name: "refresh retry unavailable",
			body: validBody,

			prepare: func(authService *mocks.MockAuthService) {
				authService.EXPECT().
					Refresh(
						gomock.Any(),
						auth.RefreshInput{
							OperationID:     operationID,
							RawRefreshToken: "refresh-token",
						},
					).
					Return(
						auth.TokenPair{},
						auth.ErrRefreshRetryUnavailable,
					)
			},

			wantStatus:  http.StatusUnauthorized,
			wantCode:    "authentication_error",
			wantMessage: "authentication failed",
		},
		{
			name: "refresh token reuse",
			body: validBody,

			prepare: func(authService *mocks.MockAuthService) {
				authService.EXPECT().
					Refresh(
						gomock.Any(),
						auth.RefreshInput{
							OperationID:     operationID,
							RawRefreshToken: "refresh-token",
						},
					).
					Return(
						auth.TokenPair{},
						auth.ErrRefreshTokenReuse,
					)
			},

			wantStatus:  http.StatusUnauthorized,
			wantCode:    "authentication_error",
			wantMessage: "authentication failed",
		},
		{
			name: "inactive session",
			body: validBody,

			prepare: func(authService *mocks.MockAuthService) {
				authService.EXPECT().
					Refresh(
						gomock.Any(),
						auth.RefreshInput{
							OperationID:     operationID,
							RawRefreshToken: "refresh-token",
						},
					).
					Return(
						auth.TokenPair{},
						auth.ErrSessionInactive,
					)
			},

			wantStatus:  http.StatusUnauthorized,
			wantCode:    "authentication_error",
			wantMessage: "authentication failed",
		},
		{
			name: "internal error",
			body: validBody,

			prepare: func(authService *mocks.MockAuthService) {
				authService.EXPECT().
					Refresh(
						gomock.Any(),
						auth.RefreshInput{
							OperationID:     operationID,
							RawRefreshToken: "refresh-token",
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
			name: "missing operation ID",
			body: `{
				"refresh_token": "refresh-token"
			}`,

			wantStatus: http.StatusBadRequest,
		},
		{
			name: "nil operation ID",
			body: `{
				"operation_id": "00000000-0000-0000-0000-000000000000",
				"refresh_token": "refresh-token"
			}`,

			wantStatus: http.StatusBadRequest,
		},
		{
			name: "invalid operation ID",
			body: `{
				"operation_id": "invalid-uuid",
				"refresh_token": "refresh-token"
			}`,

			wantStatus: http.StatusBadRequest,
		},
		{
			name: "missing refresh token",
			body: fmt.Sprintf(`{
				"operation_id": %q
			}`, operationID.String()),

			wantStatus: http.StatusBadRequest,
		},
		{
			name: "empty refresh token",
			body: fmt.Sprintf(`{
				"operation_id": %q,
				"refresh_token": ""
			}`, operationID.String()),

			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid JSON",
			body:       `{"operation_id":`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "empty body",
			body:       "",
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "unknown field",
			body: fmt.Sprintf(`{
				"operation_id": %q,
				"refresh_token": "refresh-token",
				"admin": true
			}`, operationID.String()),

			wantStatus: http.StatusBadRequest,
		},
		{
			name: "invalid refresh token type",
			body: fmt.Sprintf(`{
				"operation_id": %q,
				"refresh_token": 123
			}`, operationID.String()),

			wantStatus: http.StatusBadRequest,
		},
		{
			name: "multiple JSON objects",
			body: validBody + validBody,

			wantStatus: http.StatusBadRequest,
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
				"/auth/refresh",
				strings.NewReader(tt.body),
			)

			rec := httptest.NewRecorder()

			api.Handler().ServeHTTP(rec, req)

			if tt.wantStatus != http.StatusOK {
				// Для ошибок сервиса известны точные код и сообщение.
				if tt.wantCode != "" {
					assertErrorResponse(
						t,
						rec,
						tt.wantStatus,
						tt.wantCode,
						tt.wantMessage,
					)

					return
				}

				// Для ошибок decodeJSON пока проверяем статус и JSON.
				if rec.Code != tt.wantStatus {
					t.Fatalf(
						"status = %d, want %d; body = %s",
						rec.Code,
						tt.wantStatus,
						rec.Body.String(),
					)
				}

				if contentType := rec.Header().Get("Content-Type"); contentType != "application/json" {
					t.Errorf(
						"Content-Type = %q, want %q",
						contentType,
						"application/json",
					)
				}

				var response struct {
					Error struct {
						Code    string `json:"code"`
						Message string `json:"message"`
					} `json:"error"`
				}

				if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
					t.Fatalf(
						"unmarshal error response: %v; body = %s",
						err,
						rec.Body.String(),
					)
				}

				if response.Error.Code == "" {
					t.Error("error code is empty")
				}

				return
			}

			if rec.Code != http.StatusOK {
				t.Fatalf(
					"status = %d, want %d; body = %s",
					rec.Code,
					http.StatusOK,
					rec.Body.String(),
				)
			}

			if contentType := rec.Header().Get("Content-Type"); contentType != "application/json" {
				t.Errorf(
					"Content-Type = %q, want %q",
					contentType,
					"application/json",
				)
			}

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

func TestAPI_Refresh_MethodNotAllowed(t *testing.T) {
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
				"/auth/refresh",
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
