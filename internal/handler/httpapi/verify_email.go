package httpapi

import (
	"errors"
	"net/http"

	"github.com/frimo-dev/frimo-messenger/internal/service/auth"
	"go.uber.org/zap"
)

type verifyEmailResponse struct {
	Status string `json:"status"`
}

func (a *API) verifyEmail(w http.ResponseWriter, r *http.Request) {
	rawToken := r.URL.Query().Get("token")

	if rawToken == "" {
		a.respondError(
			r.Context(),
			w,
			http.StatusBadRequest,
			"verification_token_required",
			"verification token is required",
		)
		return
	}

	err := a.authService.ConfirmEmail(r.Context(), rawToken)
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrInvalidToken):
			a.respondError(
				r.Context(),
				w,
				http.StatusBadRequest,
				"invalid_verification_token",
				"verification token is invalid",
			)
		case errors.Is(err, auth.ErrExpiredToken):
			a.respondError(
				r.Context(),
				w,
				http.StatusBadRequest,
				"verification_token_expired",
				"verification token has expired",
			)
		case errors.Is(err, auth.ErrUsedToken):
			a.respondError(
				r.Context(),
				w,
				http.StatusConflict,
				"verification_token_used",
				"verification token has already been used",
			)
		default:
			a.logger.Error(
				"failed to verify email",
				zap.Error(err),
				zap.String(string(requestIDKey), requestIDFromContext(r.Context())),
			)
			a.respondError(
				r.Context(),
				w,
				http.StatusInternalServerError,
				"internal_error",
				"internal server error",
			)
		}
		return
	}

	a.logger.Info(
		"user verified successfully",
		zap.String(string(requestIDKey), requestIDFromContext(r.Context())),
	)

	a.respondJSON(r.Context(), w, http.StatusOK, verifyEmailResponse{Status: "email_verified"})
}
