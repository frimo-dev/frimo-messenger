package httpapi

import (
	"errors"
	"net/http"
	"uuid"

	"github.com/frimo-dev/frimo-messenger/internal/execution"
	"github.com/frimo-dev/frimo-messenger/internal/service/auth"
	"go.uber.org/zap"
)

type refreshRequest struct {
	OperationID  uuid.UUID `json:"operation_id"`
	RefreshToken string    `json:"refresh_token"`
}

func (r refreshRequest) Validate() error {
	switch {
	case r.OperationID == uuid.Nil():
		return ValidationError{
			Field: "operation_id",
			Code:  "required",
		}

	case r.RefreshToken == "":
		return ValidationError{
			Field: "refresh_token",
			Code:  "required",
		}
	}

	return nil
}

type refreshResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

func (a *API) refresh(w http.ResponseWriter, r *http.Request) {
	var request refreshRequest

	if err := decodeJSON(r, &request); err != nil {
		a.respondRequestError(r.Context(), w, err)
		return
	}

	tokenPair, err := a.authService.Refresh(r.Context(),
		auth.RefreshInput{
			OperationID:     request.OperationID,
			RawRefreshToken: request.RefreshToken,
		})
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrRefreshRetryUnavailable),
			errors.Is(err, auth.ErrRefreshTokenReuse),
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
				zap.String("request_id", execution.IDFromContext(r.Context())),
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
