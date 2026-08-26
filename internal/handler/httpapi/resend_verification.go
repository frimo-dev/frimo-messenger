package httpapi

import (
	"encoding/json/v2"
	"errors"
	"net/http"

	"github.com/frimo-dev/frimo-messenger/internal/service/auth"
	"go.uber.org/zap"
)

type resendRequest struct {
	Email string `json:"email"`
}

func (a *API) resendVerificationToken(w http.ResponseWriter, r *http.Request) {
	var request resendRequest

	if err := json.UnmarshalRead(r.Body, &request); err != nil {
		a.respondError(r.Context(), w, http.StatusBadRequest, "invalid_request", "invalid request body")
		return
	}

	err := a.authService.ResendVerification(r.Context(), request.Email)
	if err != nil {
		var validationErr *auth.ValidationError

		switch {
		case errors.As(err, &validationErr):
			a.respondError(
				r.Context(),
				w,
				http.StatusBadRequest,
				validationErr.Code,
				validationErr.Message,
			)
		case errors.Is(err, auth.ErrUserNotFound),
			errors.Is(err, auth.ErrAlreadyVerified),
			errors.Is(err, auth.ErrResendCooldown),
			errors.Is(err, auth.ErrResendHourlyLimit):

			a.logger.Info(
				"verification resend not scheduled",
				zap.String("request_id", requestIDFromContext(r.Context())),
				zap.Error(err),
			)

		default:
			a.logger.Error(
				"resend verification email failed",
				zap.String("request_id", requestIDFromContext(r.Context())),
				zap.Error(err),
			)

			a.respondError(
				r.Context(),
				w,
				http.StatusInternalServerError,
				"internal_error",
				"internal server error",
			)
			return
		}
	}

	w.WriteHeader(http.StatusAccepted)
}
