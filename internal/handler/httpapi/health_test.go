package httpapi_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/frimo-dev/frimo-messenger/internal/handler/httpapi"
	"github.com/frimo-dev/frimo-messenger/internal/handler/httpapi/mocks"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

func TestAPI_Health_Success(t *testing.T) {
	ctrl := gomock.NewController(t)

	authService := mocks.NewMockAuthService(ctrl)

	api := httpapi.New(zap.NewNop(), authService)

	req := httptest.NewRequest(
		http.MethodGet,
		"/health",
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

	if response.Status != "ok" {
		t.Errorf("expected status %q, got %q", "ok", response.Status)
	}
}

func TestAPI_Health_MethodNotAllowed(t *testing.T) {
	ctrl := gomock.NewController(t)

	authService := mocks.NewMockAuthService(ctrl)

	api := httpapi.New(zap.NewNop(), authService)

	req := httptest.NewRequest(
		http.MethodPost,
		"/health",
		nil,
	)

	rec := httptest.NewRecorder()

	api.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}
