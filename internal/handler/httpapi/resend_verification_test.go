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

func TestAPI_ResendVerification_Success(t *testing.T) {
	ctrl := gomock.NewController(t)

	authService := mocks.NewMockAuthService(ctrl)

	authService.EXPECT().ResendVerification(gomock.Any(), "test@example.com").Return(nil)

	api := httpapi.New(zap.NewNop(), authService)

	req := httptest.NewRequest(
		http.MethodPost,
		"/auth/resend",
		strings.NewReader(`{
			"email": "test@example.com"
		}`),
	)

	rec := httptest.NewRecorder()

	api.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected status %d, got %d", http.StatusAccepted, rec.Code)
	}
}

func TestAPI_ResendVerification_InvalidJSON(t *testing.T) {
	ctrl := gomock.NewController(t)

	authService := mocks.NewMockAuthService(ctrl)

	api := httpapi.New(zap.NewNop(), authService)

	req := httptest.NewRequest(
		http.MethodPost,
		"/auth/resend",
		strings.NewReader(`{"email":`),
	)

	rec := httptest.NewRecorder()

	api.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestAPI_ResendVerification_ValidationError(t *testing.T) {
	ctrl := gomock.NewController(t)

	authService := mocks.NewMockAuthService(ctrl)

	validationErr := &auth.ValidationError{
		Code:    "invalid_email",
		Field:   "email",
		Message: "email has invalid format",
	}

	authService.EXPECT().ResendVerification(gomock.Any(), "not-an-email").Return(validationErr)

	api := httpapi.New(zap.NewNop(), authService)

	req := httptest.NewRequest(
		http.MethodPost,
		"/auth/resend",
		strings.NewReader(`{
			"email": "not-an-email"
		}`),
	)

	rec := httptest.NewRecorder()

	api.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestAPI_ResendVerification_HiddenErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{
			name: "user not found",
			err:  auth.ErrUserNotFound,
		},
		{
			name: "already verified",
			err:  auth.ErrAlreadyVerified,
		},
		{
			name: "resend cooldown",
			err:  auth.ErrResendCooldown,
		},
		{
			name: "hourly limit",
			err:  auth.ErrResendHourlyLimit,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			authService := mocks.NewMockAuthService(ctrl)

			authService.EXPECT().ResendVerification(gomock.Any(), "test@example.com").Return(tt.err)

			api := httpapi.New(zap.NewNop(), authService)

			req := httptest.NewRequest(
				http.MethodPost,
				"/auth/resend",
				strings.NewReader(`{
					"email": "test@example.com"
				}`),
			)

			rec := httptest.NewRecorder()

			api.Handler().ServeHTTP(rec, req)

			if rec.Code != http.StatusAccepted {
				t.Fatalf("expected status %d, got %d", http.StatusAccepted, rec.Code)
			}
		})
	}
}

func TestAPI_ResendVerification_InternalError(t *testing.T) {
	ctrl := gomock.NewController(t)

	authService := mocks.NewMockAuthService(ctrl)

	serviceErr := errors.New("database unavailable")

	authService.EXPECT().ResendVerification(gomock.Any(), "test@example.com").Return(serviceErr)

	api := httpapi.New(zap.NewNop(), authService)

	req := httptest.NewRequest(
		http.MethodPost,
		"/auth/resend",
		strings.NewReader(`{
			"email": "test@example.com"
		}`),
	)

	rec := httptest.NewRecorder()

	api.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, rec.Code)
	}
}

func TestAPI_ResendVerification_MethodNotAllowed(t *testing.T) {
	ctrl := gomock.NewController(t)

	authService := mocks.NewMockAuthService(ctrl)

	api := httpapi.New(zap.NewNop(), authService)

	req := httptest.NewRequest(http.MethodGet, "/auth/resend", nil)

	rec := httptest.NewRecorder()

	api.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}
