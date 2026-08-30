package httpapi_test

import (
	"encoding/json"
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

func TestAPI_Register_Success(t *testing.T) {
	ctrl := gomock.NewController(t)

	authService := mocks.NewMockAuthService(ctrl)
	userService := mocks.NewMockUserService(ctrl)
	accessTokenVerifier := mocks.NewMockAccessTokenVerifier(ctrl)

	userID := uuid.New()

	authService.EXPECT().Register(gomock.Any(),
		auth.RegistrationInput{
			Email:    "test@example.com",
			Password: "very-secure-password",
		},
	).Return(
		auth.User{
			ID:    userID,
			Email: "test@example.com",
		},
		nil,
	)

	api := httpapi.New(zap.NewNop(), accessTokenVerifier, authService, userService)

	req := httptest.NewRequest(
		http.MethodPost,
		"/auth/register",
		strings.NewReader(`{
			"email": "test@example.com",
			"password": "very-secure-password"
		}`),
	)

	rec := httptest.NewRecorder()

	api.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected status %d, got %d", http.StatusAccepted, rec.Code)
	}

	var response struct {
		ID    string `json:"id"`
		Email string `json:"email"`
	}

	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if response.ID != userID.String() {
		t.Errorf("expected ID %q, got %q", userID.String(), response.ID)
	}

	if response.Email != "test@example.com" {
		t.Errorf("expected email %q, got %q", "test@example.com", response.Email)
	}
}

func TestAPI_Register_InvalidJSON(t *testing.T) {
	ctrl := gomock.NewController(t)

	authService := mocks.NewMockAuthService(ctrl)
	userService := mocks.NewMockUserService(ctrl)
	accessTokenVerifier := mocks.NewMockAccessTokenVerifier(ctrl)

	api := httpapi.New(zap.NewNop(), accessTokenVerifier, authService, userService)
	req := httptest.NewRequest(http.MethodPost, "/auth/register", strings.NewReader(`{"email":`))
	rec := httptest.NewRecorder()

	api.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestAPI_Register_ValidationError(t *testing.T) {
	ctrl := gomock.NewController(t)

	authService := mocks.NewMockAuthService(ctrl)
	userService := mocks.NewMockUserService(ctrl)
	accessTokenVerifier := mocks.NewMockAccessTokenVerifier(ctrl)

	validationErr := &auth.ValidationError{
		Code:    "invalid_email",
		Field:   "email",
		Message: "email has invalid format",
	}

	authService.EXPECT().Register(
		gomock.Any(),
		auth.RegistrationInput{
			Email:    "not-an-email",
			Password: "very-secure-password",
		},
	).Return(auth.User{}, validationErr)

	api := httpapi.New(zap.NewNop(), accessTokenVerifier, authService, userService)

	req := httptest.NewRequest(
		http.MethodPost,
		"/auth/register",
		strings.NewReader(`{
			"email": "not-an-email",
			"password": "very-secure-password"
		}`),
	)

	rec := httptest.NewRecorder()

	api.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestAPI_Register_EmailAlreadyExists(t *testing.T) {
	ctrl := gomock.NewController(t)

	authService := mocks.NewMockAuthService(ctrl)
	userService := mocks.NewMockUserService(ctrl)
	accessTokenVerifier := mocks.NewMockAccessTokenVerifier(ctrl)

	authService.EXPECT().Register(
		gomock.Any(),
		auth.RegistrationInput{
			Email:    "test@example.com",
			Password: "very-secure-password",
		},
	).Return(auth.User{}, auth.ErrEmailAlreadyExists)

	api := httpapi.New(zap.NewNop(), accessTokenVerifier, authService, userService)

	req := httptest.NewRequest(
		http.MethodPost,
		"/auth/register",
		strings.NewReader(`{
			"email": "test@example.com",
			"password": "very-secure-password"
		}`),
	)

	rec := httptest.NewRecorder()

	api.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected status %d, got %d", http.StatusConflict, rec.Code)
	}
}

func TestAPI_Register_InternalError(t *testing.T) {
	ctrl := gomock.NewController(t)

	authService := mocks.NewMockAuthService(ctrl)
	userService := mocks.NewMockUserService(ctrl)
	accessTokenVerifier := mocks.NewMockAccessTokenVerifier(ctrl)

	serviceErr := errors.New("database unavailable")

	authService.EXPECT().Register(gomock.Any(), gomock.Any()).Return(auth.User{}, serviceErr)

	api := httpapi.New(zap.NewNop(), accessTokenVerifier, authService, userService)

	req := httptest.NewRequest(
		http.MethodPost,
		"/auth/register",
		strings.NewReader(`{
			"email": "test@example.com",
			"password": "very-secure-password"
		}`),
	)

	rec := httptest.NewRecorder()

	api.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, rec.Code)
	}
}

func TestAPI_Register_MethodNotAllowed(t *testing.T) {
	ctrl := gomock.NewController(t)

	authService := mocks.NewMockAuthService(ctrl)
	userService := mocks.NewMockUserService(ctrl)
	accessTokenVerifier := mocks.NewMockAccessTokenVerifier(ctrl)

	api := httpapi.New(zap.NewNop(), accessTokenVerifier, authService, userService)

	req := httptest.NewRequest(
		http.MethodGet,
		"/auth/register",
		nil,
	)

	rec := httptest.NewRecorder()

	api.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestAPI_Register_UnknownField(t *testing.T) {
	ctrl := gomock.NewController(t)

	authService := mocks.NewMockAuthService(ctrl)
	userService := mocks.NewMockUserService(ctrl)
	accessTokenVerifier := mocks.NewMockAccessTokenVerifier(ctrl)

	api := httpapi.New(zap.NewNop(), accessTokenVerifier, authService, userService)

	req := httptest.NewRequest(
		http.MethodPost,
		"/auth/register",
		strings.NewReader(`{
			"email": "test@example.com",
			"password": "very-secure-password",
			"admin": true
		}`),
	)

	rec := httptest.NewRecorder()

	api.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestAPI_Register_MultipleJSONObjects(t *testing.T) {
	ctrl := gomock.NewController(t)

	authService := mocks.NewMockAuthService(ctrl)
	userService := mocks.NewMockUserService(ctrl)
	accessTokenVerifier := mocks.NewMockAccessTokenVerifier(ctrl)

	api := httpapi.New(zap.NewNop(), accessTokenVerifier, authService, userService)

	req := httptest.NewRequest(
		http.MethodPost,
		"/auth/register",
		strings.NewReader(`
			{
				"email": "test@example.com",
				"password": "very-secure-password"
			}
			{
				"email": "second@example.com",
				"password": "another-password"
			}
		`),
	)

	rec := httptest.NewRecorder()

	api.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestAPI_Register_EmptyBody(t *testing.T) {
	ctrl := gomock.NewController(t)

	authService := mocks.NewMockAuthService(ctrl)
	userService := mocks.NewMockUserService(ctrl)
	accessTokenVerifier := mocks.NewMockAccessTokenVerifier(ctrl)

	api := httpapi.New(zap.NewNop(), accessTokenVerifier, authService, userService)

	req := httptest.NewRequest(
		http.MethodPost,
		"/auth/register",
		nil,
	)

	rec := httptest.NewRecorder()

	api.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestAPI_Register_BodyTooLarge(t *testing.T) {
	ctrl := gomock.NewController(t)

	authService := mocks.NewMockAuthService(ctrl)
	userService := mocks.NewMockUserService(ctrl)
	accessTokenVerifier := mocks.NewMockAccessTokenVerifier(ctrl)

	api := httpapi.New(zap.NewNop(), accessTokenVerifier, authService, userService)

	largePassword := strings.Repeat("a", 20*1024)

	body := `{
		"email": "test@example.com",
		"password": "` + largePassword + `"
	}`

	req := httptest.NewRequest(
		http.MethodPost,
		"/auth/register",
		strings.NewReader(body),
	)

	rec := httptest.NewRecorder()

	api.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestAPI_Register_InvalidFieldType(t *testing.T) {
	ctrl := gomock.NewController(t)

	authService := mocks.NewMockAuthService(ctrl)
	userService := mocks.NewMockUserService(ctrl)
	accessTokenVerifier := mocks.NewMockAccessTokenVerifier(ctrl)

	api := httpapi.New(zap.NewNop(), accessTokenVerifier, authService, userService)

	req := httptest.NewRequest(
		http.MethodPost,
		"/auth/register",
		strings.NewReader(`{
			"email": 123,
			"password": "very-secure-password"
		}`),
	)

	rec := httptest.NewRecorder()

	api.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}
