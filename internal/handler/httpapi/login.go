package httpapi

import (
	"encoding/json/v2"
	"errors"
	"net/http"

	"github.com/frimo-dev/frimo-messenger/internal/service/auth"
	"go.uber.org/zap"
)

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginResponse struct {
	AccessToken string `json:"access_token"`
}

func (a *API) login(w http.ResponseWriter, r *http.Request) {
	var request loginRequest

	if err := json.UnmarshalRead(r.Body, &request, json.RejectUnknownMembers(true)); err != nil {
		a.respondError(r.Context(), w, http.StatusBadRequest, "invalid_request", "invalid request body")
		return
	}

	accessToken, err := a.authService.Login(r.Context(), request.Email, request.Password)
	if err != nil {
		var validationErr *auth.ValidationError

		switch {
		case errors.As(err, &validationErr):
			a.respondError(r.Context(), w, http.StatusBadRequest, validationErr.Code, validationErr.Message)
		case errors.Is(err, auth.ErrInvalidCredentials):
			a.respondError(r.Context(), w, http.StatusUnauthorized, "invalid_credentials", "invalid credentials")
		case errors.Is(err, auth.ErrEmailNotVerified):
			a.respondError(r.Context(), w, http.StatusForbidden, "email_not_verified", "email not verified")
		default:
			a.logger.Error(
				"failed to login user",
				zap.Error(err),
				zap.String("request_id", requestIDFromContext(r.Context())),
			)
			a.respondError(r.Context(), w, http.StatusInternalServerError, "internal_error", "internal server error")
		}

		return
	}

	a.logger.Info("user successfully logged in", zap.String("request_id", requestIDFromContext(r.Context())))

	a.respondJSON(r.Context(), w, http.StatusOK, loginResponse{AccessToken: accessToken})
}
