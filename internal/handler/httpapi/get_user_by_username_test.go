package httpapi_test

import (
	"encoding/json/v2"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"uuid"

	"github.com/frimo-dev/frimo-messenger/internal/handler/httpapi"
	"github.com/frimo-dev/frimo-messenger/internal/handler/httpapi/mocks"
	"github.com/frimo-dev/frimo-messenger/internal/service/auth"
	"github.com/frimo-dev/frimo-messenger/internal/service/user"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

func TestAPI_GetUserByUsername(t *testing.T) {
	userID := uuid.New()
	surname := "Иванов"

	internalErr := errors.New("database unavailable")

	tests := []struct {
		name     string
		username string

		publicUser user.PublicUser
		serviceErr error

		wantStatus  int
		wantCode    string
		wantMessage string
	}{
		{
			name:     "success with surname",
			username: "Alex123",

			publicUser: user.PublicUser{
				ID:       userID,
				Username: "Alex123",
				Name:     "Александр",
				Surname:  &surname,
			},

			wantStatus: http.StatusOK,
		},
		{
			name:     "success without surname",
			username: "Alex123",

			publicUser: user.PublicUser{
				ID:       userID,
				Username: "Alex123",
				Name:     "Александр",
				Surname:  nil,
			},

			wantStatus: http.StatusOK,
		},
		{
			name:       "user not found",
			username:   "UnknownUser",
			serviceErr: user.ErrUserNotFound,

			wantStatus:  http.StatusNotFound,
			wantCode:    "user_not_found",
			wantMessage: "user not found",
		},
		{
			name:       "internal error",
			username:   "Alex123",
			serviceErr: internalErr,

			wantStatus:  http.StatusInternalServerError,
			wantCode:    "internal_error",
			wantMessage: "internal server error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			authService := mocks.NewMockAuthService(ctrl)
			userService := mocks.NewMockUserService(ctrl)

			authService.EXPECT().
				Authenticate(gomock.Any(), "access-token").
				Return(auth.Identity{}, nil)

			userService.EXPECT().
				GetPublicUserByUsername(
					gomock.Any(),
					tt.username,
				).
				Return(tt.publicUser, tt.serviceErr)

			api := httpapi.New(
				zap.NewNop(),
				authService,
				userService,
			)

			req := httptest.NewRequest(
				http.MethodGet,
				"/users/by-username/"+tt.username,
				nil,
			)

			req.Header.Set(
				"Authorization",
				"Bearer access-token",
			)

			rec := httptest.NewRecorder()

			api.Handler().ServeHTTP(rec, req)

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
				ID       string  `json:"id"`
				Username string  `json:"username"`
				Name     string  `json:"name"`
				Surname  *string `json:"surname"`
			}

			if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
				t.Fatalf(
					"unmarshal response: %v; body = %s",
					err,
					rec.Body.String(),
				)
			}

			if response.ID != tt.publicUser.ID.String() {
				t.Errorf(
					"ID = %q, want %q",
					response.ID,
					tt.publicUser.ID.String(),
				)
			}

			if response.Username != tt.publicUser.Username {
				t.Errorf(
					"Username = %q, want %q",
					response.Username,
					tt.publicUser.Username,
				)
			}

			if response.Name != tt.publicUser.Name {
				t.Errorf(
					"Name = %q, want %q",
					response.Name,
					tt.publicUser.Name,
				)
			}

			switch {
			case tt.publicUser.Surname == nil:
				if response.Surname != nil {
					t.Errorf(
						"Surname = %q, want nil",
						*response.Surname,
					)
				}

			case response.Surname == nil:
				t.Errorf(
					"Surname = nil, want %q",
					*tt.publicUser.Surname,
				)

			case *response.Surname != *tt.publicUser.Surname:
				t.Errorf(
					"Surname = %q, want %q",
					*response.Surname,
					*tt.publicUser.Surname,
				)
			}
		})
	}
}

func TestAPI_GetUserByUsername_Unauthorized(t *testing.T) {
	ctrl := gomock.NewController(t)

	authService := mocks.NewMockAuthService(ctrl)
	userService := mocks.NewMockUserService(ctrl)

	api := httpapi.New(
		zap.NewNop(),
		authService,
		userService,
	)

	req := httptest.NewRequest(
		http.MethodGet,
		"/users/by-username/Alex123",
		nil,
	)

	rec := httptest.NewRecorder()

	api.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf(
			"status = %d, want %d; body = %s",
			rec.Code,
			http.StatusUnauthorized,
			rec.Body.String(),
		)
	}
}

func TestAPI_GetUserByUsername_MethodNotAllowed(t *testing.T) {
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
				"/users/by-username/Alex123",
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
