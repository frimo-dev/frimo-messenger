package httpapi_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/frimo-dev/frimo-messenger/internal/handler/httpapi"
	"go.uber.org/zap"
)

func TestRecoveryMiddlewareReturnsInternalServerError(
	t *testing.T,
) {
	logger := zap.NewNop()

	panicHandler := http.HandlerFunc(
		func(
			http.ResponseWriter,
			*http.Request,
		) {
			panic("test panic")
		},
	)

	handler := httpapi.RecoveryMiddleware(
		logger,
		panicHandler,
	)

	request := httptest.NewRequest(
		http.MethodGet,
		"/test",
		nil,
	)

	recorder := httptest.NewRecorder()

	handler.ServeHTTP(
		recorder,
		request,
	)

	if recorder.Code !=
		http.StatusInternalServerError {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusInternalServerError,
			recorder.Code,
		)
	}
}

func TestRecoveryMiddlewarePassesThroughNormalRequest(t *testing.T) {
	logger := zap.NewNop()

	next := http.HandlerFunc(
		func(
			w http.ResponseWriter,
			_ *http.Request,
		) {
			w.WriteHeader(http.StatusCreated)
		},
	)

	handler := httpapi.RecoveryMiddleware(
		logger,
		next,
	)

	request := httptest.NewRequest(
		http.MethodPost,
		"/test",
		nil,
	)

	recorder := httptest.NewRecorder()

	handler.ServeHTTP(
		recorder,
		request,
	)

	if recorder.Code != http.StatusCreated {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusCreated,
			recorder.Code,
		)
	}
}
