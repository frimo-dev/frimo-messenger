package httpapi

import (
	"errors"
	"net/http"

	"github.com/frimo-dev/frimo-messenger/internal/service/auth"
	"go.uber.org/zap"
)

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type refreshResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

func (a *API) refresh(w http.ResponseWriter, r *http.Request) {
	var request refreshRequest

	if err := decodeJSON(w, r, &request); err != nil {
		a.respondError(r.Context(), w, http.StatusBadRequest, "invalid_request", "invalid request body")
		return
	}

	tokenPair, err := a.authService.Refresh(r.Context(), request.RefreshToken)
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrAccessTokenNotStored):
			a.logger.Error(
				"failed to store access token",
				zap.Error(err),
				zap.String("request_id", requestIDFromContext(r.Context())),
			)

		case errors.Is(err, auth.ErrRefreshTokenNotFound),
			errors.Is(err, auth.ErrRefreshTokenUsed),
			errors.Is(err, auth.ErrSessionInactive):

			a.respondError(
				r.Context(),
				w,
				http.StatusUnauthorized,
				"authentication_error",
				"authentication failed",
			)
			return

		default:
			a.logger.Error(
				"failed to refresh session",
				zap.Error(err),
				zap.String("request_id", requestIDFromContext(r.Context())),
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

	a.respondJSON(r.Context(), w, http.StatusOK, refreshResponse{AccessToken: tokenPair.AccessToken, RefreshToken: tokenPair.RefreshToken})
}
