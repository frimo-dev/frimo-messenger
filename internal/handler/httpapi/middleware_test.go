package httpapi_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"uuid"

	"github.com/frimo-dev/frimo-messenger/internal/handler/httpapi"
	"github.com/frimo-dev/frimo-messenger/internal/handler/httpapi/mocks"
	"github.com/frimo-dev/frimo-messenger/internal/security/token"
	"go.uber.org/mock/gomock"
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

func TestAuthenticationMiddleware(t *testing.T) {
	userID := uuid.New()
	verifyErr := errors.New("verify failed")

	tests := []struct {
		name          string
		authorization string

		prepare func(verifier *mocks.MockAccessTokenVerifier)

		wantStatus     int
		wantNextCalled bool
	}{
		{
			name:          "success",
			authorization: "Bearer access-token",

			prepare: func(verifier *mocks.MockAccessTokenVerifier) {
				verifier.
					EXPECT().
					Verify("access-token").
					Return(userID, nil)
			},

			wantStatus:     http.StatusNoContent,
			wantNextCalled: true,
		},
		{
			name:          "bearer case insensitive",
			authorization: "bEaReR access-token",

			prepare: func(verifier *mocks.MockAccessTokenVerifier) {
				verifier.
					EXPECT().
					Verify("access-token").
					Return(userID, nil)
			},

			wantStatus:     http.StatusNoContent,
			wantNextCalled: true,
		},
		{
			name:          "missing authorization",
			authorization: "",
			wantStatus:    http.StatusUnauthorized,
		},
		{
			name:          "missing token",
			authorization: "Bearer",
			wantStatus:    http.StatusUnauthorized,
		},
		{
			name:          "missing scheme",
			authorization: "access-token",
			wantStatus:    http.StatusUnauthorized,
		},
		{
			name:          "wrong scheme",
			authorization: "Basic access-token",
			wantStatus:    http.StatusUnauthorized,
		},
		{
			name:          "too many parts",
			authorization: "Bearer access-token extra",
			wantStatus:    http.StatusUnauthorized,
		},
		{
			name:          "invalid token",
			authorization: "Bearer invalid-token",

			prepare: func(verifier *mocks.MockAccessTokenVerifier) {
				verifier.
					EXPECT().
					Verify("invalid-token").
					Return(uuid.Nil(), token.ErrInvalidToken)
			},

			wantStatus: http.StatusUnauthorized,
		},
		{
			name:          "expired token",
			authorization: "Bearer expired-token",

			prepare: func(verifier *mocks.MockAccessTokenVerifier) {
				verifier.
					EXPECT().
					Verify("expired-token").
					Return(uuid.Nil(), token.ErrTokenExpired)
			},

			wantStatus: http.StatusUnauthorized,
		},
		{
			name:          "unexpected verifier error",
			authorization: "Bearer access-token",

			prepare: func(verifier *mocks.MockAccessTokenVerifier) {
				verifier.
					EXPECT().
					Verify("access-token").
					Return(uuid.Nil(), verifyErr)
			},

			wantStatus: http.StatusUnauthorized,
		},
		{
			name:          "extra whitespace",
			authorization: "   Bearer    access-token   ",

			prepare: func(verifier *mocks.MockAccessTokenVerifier) {
				verifier.
					EXPECT().
					Verify("access-token").
					Return(userID, nil)
			},

			wantStatus:     http.StatusNoContent,
			wantNextCalled: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			verifier := mocks.NewMockAccessTokenVerifier(ctrl)

			if tt.prepare != nil {
				tt.prepare(verifier)
			}

			nextCalled := false

			next := http.HandlerFunc(func(
				w http.ResponseWriter,
				_ *http.Request,
			) {
				nextCalled = true
				w.WriteHeader(http.StatusNoContent)
			})

			handler := httpapi.AuthenticationMiddleware(
				zap.NewNop(),
				verifier,
				next,
			)

			req := httptest.NewRequest(
				http.MethodGet,
				"/protected",
				nil,
			)

			if tt.authorization != "" {
				req.Header.Set(
					"Authorization",
					tt.authorization,
				)
			}

			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf(
					"expected status %d, got %d",
					tt.wantStatus,
					rec.Code,
				)
			}

			if nextCalled != tt.wantNextCalled {
				t.Errorf(
					"expected next called %v, got %v",
					tt.wantNextCalled,
					nextCalled,
				)
			}
		})
	}
}
