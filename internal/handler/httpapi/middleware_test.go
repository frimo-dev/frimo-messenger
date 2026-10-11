package httpapi_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"uuid"

	"github.com/frimo-dev/frimo-messenger/internal/execution"
	"github.com/frimo-dev/frimo-messenger/internal/handler/httpapi"
	"github.com/frimo-dev/frimo-messenger/internal/handler/httpapi/mocks"
	"github.com/frimo-dev/frimo-messenger/internal/service/auth"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

func TestRecoveryMiddleware(t *testing.T) {
	tests := []struct {
		name        string
		panicValue  any
		wantStatus  int
		wantCode    string
		wantMessage string
		wantPanic   bool
	}{
		{
			name:       "normal request",
			wantStatus: http.StatusCreated,
		},
		{
			name:        "string panic",
			panicValue:  "test panic",
			wantStatus:  http.StatusInternalServerError,
			wantCode:    "internal_error",
			wantMessage: "internal server error",
		},
		{
			name:        "error panic",
			panicValue:  errors.New("unexpected error"),
			wantStatus:  http.StatusInternalServerError,
			wantCode:    "internal_error",
			wantMessage: "internal server error",
		},
		{
			name:       "abort handler panic",
			panicValue: http.ErrAbortHandler,
			wantPanic:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if tt.panicValue != nil {
					panic(tt.panicValue)
				}

				w.WriteHeader(http.StatusCreated)
			})

			handler := httpapi.RecoveryMiddleware(zap.NewNop(), next)

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			rec := httptest.NewRecorder()

			if tt.wantPanic {
				defer func() {
					recovered := recover()

					if recovered != http.ErrAbortHandler {
						t.Errorf(
							"panic = %v, want %v",
							recovered,
							http.ErrAbortHandler,
						)
					}
				}()

				handler.ServeHTTP(rec, req)
				t.Fatal("expected panic, but handler returned normally")
			}

			handler.ServeHTTP(rec, req)

			if tt.wantStatus == http.StatusInternalServerError {
				assertErrorResponse(
					t,
					rec,
					tt.wantStatus,
					tt.wantCode,
					tt.wantMessage,
				)

				return
			}

			if rec.Code != tt.wantStatus {
				t.Fatalf(
					"status = %d, want %d",
					rec.Code,
					tt.wantStatus,
				)
			}
		})
	}
}

func TestRequestLoggingMiddleware(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		path       string
		nextStatus int
		writeBody  bool
	}{
		{
			name:       "explicit status",
			method:     http.MethodGet,
			path:       "/test",
			nextStatus: http.StatusCreated,
		},
		{
			name:       "default status",
			method:     http.MethodGet,
			path:       "/test",
			nextStatus: http.StatusOK,
			writeBody:  true,
		},
		{
			name:       "no write",
			method:     http.MethodGet,
			path:       "/test",
			nextStatus: http.StatusOK,
		},
		{
			name:       "no content",
			method:     http.MethodDelete,
			path:       "/resource",
			nextStatus: http.StatusNoContent,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nextCalled := false
			var requestID string

			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				nextCalled = true

				requestID = execution.IDFromContext(r.Context())

				if tt.writeBody {
					_, _ = w.Write([]byte("response body"))
					return
				}

				if tt.nextStatus != http.StatusOK {
					w.WriteHeader(tt.nextStatus)
				}
			})

			handler := httpapi.RequestLoggingMiddleware(
				zap.NewNop(),
				next,
			)

			req := httptest.NewRequest(tt.method, tt.path, nil)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if !nextCalled {
				t.Fatal("next handler was not called")
			}

			if requestID == "" {
				t.Error("request ID is empty")
			} else if _, err := uuid.Parse(requestID); err != nil {
				t.Errorf("request ID = %q is not a valid UUID: %v", requestID, err)
			}

			if rec.Code != tt.nextStatus {
				t.Errorf(
					"status = %d, want %d",
					rec.Code,
					tt.nextStatus,
				)
			}

			if tt.writeBody && rec.Body.String() != "response body" {
				t.Errorf(
					"body = %q, want %q",
					rec.Body.String(),
					"response body",
				)
			}
		})
	}
}

func TestAuthenticationMiddleware(t *testing.T) {
	identity := auth.Identity{
		UserID:    uuid.New(),
		SessionID: uuid.New(),
	}

	authErr := errors.New("authentication service failed")

	tests := []struct {
		name          string
		authorization string

		prepare func(authService *mocks.MockAuthService)

		wantStatus     int
		wantNextCalled bool
		wantCode       string
		wantMessage    string
	}{
		{
			name:          "success",
			authorization: "Bearer access-token",

			prepare: func(authService *mocks.MockAuthService) {
				authService.EXPECT().
					Authenticate(gomock.Any(), "access-token").
					Return(identity, nil)
			},

			wantStatus:     http.StatusNoContent,
			wantNextCalled: true,
		},
		{
			name:          "bearer case insensitive",
			authorization: "bEaReR access-token",

			prepare: func(authService *mocks.MockAuthService) {
				authService.EXPECT().
					Authenticate(gomock.Any(), "access-token").
					Return(identity, nil)
			},

			wantStatus:     http.StatusNoContent,
			wantNextCalled: true,
		},
		{
			name:          "extra whitespace",
			authorization: "   Bearer    access-token   ",

			prepare: func(authService *mocks.MockAuthService) {
				authService.EXPECT().
					Authenticate(gomock.Any(), "access-token").
					Return(identity, nil)
			},

			wantStatus:     http.StatusNoContent,
			wantNextCalled: true,
		},
		{
			name:        "missing authorization",
			wantStatus:  http.StatusUnauthorized,
			wantCode:    "authentication_error",
			wantMessage: "authentication failed",
		},
		{
			name:          "missing token",
			authorization: "Bearer",

			wantStatus:  http.StatusUnauthorized,
			wantCode:    "authentication_error",
			wantMessage: "authentication failed",
		},
		{
			name:          "empty bearer token",
			authorization: "Bearer ",

			wantStatus:  http.StatusUnauthorized,
			wantCode:    "authentication_error",
			wantMessage: "authentication failed",
		},
		{
			name:          "missing scheme",
			authorization: "access-token",

			wantStatus:  http.StatusUnauthorized,
			wantCode:    "authentication_error",
			wantMessage: "authentication failed",
		},
		{
			name:          "wrong scheme",
			authorization: "Basic access-token",

			wantStatus:  http.StatusUnauthorized,
			wantCode:    "authentication_error",
			wantMessage: "authentication failed",
		},
		{
			name:          "too many parts",
			authorization: "Bearer access-token extra",

			wantStatus:  http.StatusUnauthorized,
			wantCode:    "authentication_error",
			wantMessage: "authentication failed",
		},
		{
			name:          "invalid access token",
			authorization: "Bearer invalid-token",

			prepare: func(authService *mocks.MockAuthService) {
				authService.EXPECT().
					Authenticate(gomock.Any(), "invalid-token").
					Return(auth.Identity{}, auth.ErrAccessTokenInvalid)
			},

			wantStatus:  http.StatusUnauthorized,
			wantCode:    "authentication_error",
			wantMessage: "authentication failed",
		},
		{
			name:          "inactive session",
			authorization: "Bearer inactive-token",

			prepare: func(authService *mocks.MockAuthService) {
				authService.EXPECT().
					Authenticate(gomock.Any(), "inactive-token").
					Return(auth.Identity{}, auth.ErrSessionInactive)
			},

			wantStatus:  http.StatusUnauthorized,
			wantCode:    "authentication_error",
			wantMessage: "authentication failed",
		},
		{
			name:          "unexpected authentication error",
			authorization: "Bearer access-token",

			prepare: func(authService *mocks.MockAuthService) {
				authService.EXPECT().
					Authenticate(gomock.Any(), "access-token").
					Return(auth.Identity{}, authErr)
			},

			wantStatus:  http.StatusInternalServerError,
			wantCode:    "internal_error",
			wantMessage: "internal server error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			authService := mocks.NewMockAuthService(ctrl)

			if tt.prepare != nil {
				tt.prepare(authService)
			}

			nextCalled := false

			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				nextCalled = true
				w.WriteHeader(http.StatusNoContent)
			})

			handler := httpapi.AuthenticationMiddleware(
				zap.NewNop(),
				authService,
				next,
			)

			req := httptest.NewRequest(
				http.MethodGet,
				"/protected",
				nil,
			)

			if tt.authorization != "" {
				req.Header.Set("Authorization", tt.authorization)
			}

			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if nextCalled != tt.wantNextCalled {
				t.Errorf(
					"nextCalled = %v, want %v",
					nextCalled,
					tt.wantNextCalled,
				)
			}

			if tt.wantStatus != http.StatusNoContent {
				assertErrorResponse(
					t,
					rec,
					tt.wantStatus,
					tt.wantCode,
					tt.wantMessage,
				)

				return
			}

			if rec.Code != tt.wantStatus {
				t.Errorf(
					"status = %d, want %d; body = %s",
					rec.Code,
					tt.wantStatus,
					rec.Body.String(),
				)
			}

			if rec.Body.Len() != 0 {
				t.Errorf(
					"body = %q, want empty body",
					rec.Body.String(),
				)
			}
		})
	}
}
