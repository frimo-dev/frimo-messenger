package httpapi_test

import (
	"encoding/json"
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

func TestAPI_VerifyEmail_Success(t *testing.T) {
	ctrl := gomock.NewController(t)

	authService := mocks.NewMockAuthService(ctrl)

	authService.EXPECT().ConfirmEmail(gomock.Any(), "raw-verification-token").Return(nil)

	api := httpapi.New(zap.NewNop(), authService)

	req := httptest.NewRequest(
		http.MethodGet,
		"/auth/verify-email?token=raw-verification-token",
		nil,
	)

	rec := httptest.NewRecorder()

	api.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var response struct {
		Status string `json:"status"`
	}

	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if response.Status != "email_verified" {
		t.Errorf("expected status %q, got %q", "email_verified", response.Status)
	}
}

func TestAPI_VerifyEmail_MissingToken(t *testing.T) {
	ctrl := gomock.NewController(t)

	authService := mocks.NewMockAuthService(ctrl)

	api := httpapi.New(zap.NewNop(), authService)

	req := httptest.NewRequest(
		http.MethodGet,
		"/auth/verify-email",
		nil,
	)

	rec := httptest.NewRecorder()

	api.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestAPI_VerifyEmail_InvalidToken(t *testing.T) {
	ctrl := gomock.NewController(t)

	authService := mocks.NewMockAuthService(ctrl)

	authService.EXPECT().ConfirmEmail(gomock.Any(), "invalid-token").Return(auth.ErrInvalidToken)

	api := httpapi.New(zap.NewNop(), authService)

	req := httptest.NewRequest(
		http.MethodGet,
		"/auth/verify-email?token=invalid-token",
		nil,
	)

	rec := httptest.NewRecorder()

	api.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestAPI_VerifyEmail_ExpiredToken(t *testing.T) {
	ctrl := gomock.NewController(t)

	authService := mocks.NewMockAuthService(ctrl)

	authService.EXPECT().ConfirmEmail(gomock.Any(), "expired-token").Return(auth.ErrExpiredToken)

	api := httpapi.New(zap.NewNop(), authService)

	req := httptest.NewRequest(
		http.MethodGet,
		"/auth/verify-email?token=expired-token",
		nil,
	)

	rec := httptest.NewRecorder()

	api.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestAPI_VerifyEmail_UsedToken(t *testing.T) {
	ctrl := gomock.NewController(t)

	authService := mocks.NewMockAuthService(ctrl)

	authService.EXPECT().ConfirmEmail(gomock.Any(), "used-token").Return(auth.ErrUsedToken)

	api := httpapi.New(zap.NewNop(), authService)

	req := httptest.NewRequest(
		http.MethodGet,
		"/auth/verify-email?token=used-token",
		nil,
	)

	rec := httptest.NewRecorder()

	api.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected status %d, got %d", http.StatusConflict, rec.Code)
	}
}

func TestAPI_VerifyEmail_InternalError(t *testing.T) {
	ctrl := gomock.NewController(t)

	authService := mocks.NewMockAuthService(ctrl)

	serviceErr := errors.New("database unavailable")

	authService.EXPECT().ConfirmEmail(gomock.Any(), "raw-verification-token").Return(serviceErr)

	api := httpapi.New(zap.NewNop(), authService)

	req := httptest.NewRequest(
		http.MethodGet,
		"/auth/verify-email?token=raw-verification-token",
		nil,
	)

	rec := httptest.NewRecorder()

	api.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, rec.Code)
	}
}

func TestAPI_VerifyEmail_MethodNotAllowed(t *testing.T) {
	ctrl := gomock.NewController(t)

	authService := mocks.NewMockAuthService(ctrl)

	api := httpapi.New(zap.NewNop(), authService)

	req := httptest.NewRequest(
		http.MethodPost,
		"/auth/verify-email?token=raw-verification-token",
		nil,
	)

	rec := httptest.NewRecorder()

	api.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestAPI_VerifyEmail_RevokedToken(t *testing.T) {
	ctrl := gomock.NewController(t)

	authService := mocks.NewMockAuthService(ctrl)

	authService.EXPECT().ConfirmEmail(gomock.Any(), "revoked-token").Return(auth.ErrRevokedToken)

	api := httpapi.New(zap.NewNop(), authService)

	req := httptest.NewRequest(
		http.MethodGet,
		"/auth/verify-email?token=revoked-token",
		nil,
	)

	rec := httptest.NewRecorder()

	api.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}
