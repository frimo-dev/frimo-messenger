package httpapi

import (
	"encoding/json/v2"
	"errors"
	"net/http"

	"github.com/frimo-dev/frimo-messenger/internal/service/auth"
	"go.uber.org/zap"
)

const maxRegisterBodySize = 16 * 1024

type registerRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type registerResponse struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

func (a *API) registerUser(w http.ResponseWriter, r *http.Request) {
	var request registerRequest

	if err := decodeJSON(w, r, &request); err != nil {
		a.respondError(r.Context(), w, http.StatusBadRequest, "invalid_request", "invalid request body")
		return
	}

	createdUser, err := a.authService.Register(r.Context(), auth.RegistrationInput{
		Email:    request.Email,
		Password: request.Password,
	})

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

		case errors.Is(err, auth.ErrEmailAlreadyExists):
			a.respondError(
				r.Context(),
				w,
				http.StatusConflict,
				"email_already_exists",
				"user with this email already exists",
			)

		default:
			a.logger.Error(
				"failed to register user",
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
		"user registered successfully",
		zap.String(string(requestIDKey), requestIDFromContext(r.Context())),
		zap.String("user_id", createdUser.ID.String()),
	)

	a.respondJSON(r.Context(), w, http.StatusAccepted, registerResponse{
		ID:    createdUser.ID.String(),
		Email: createdUser.Email,
	})
}

func decodeJSON(w http.ResponseWriter, r *http.Request, destination any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxRegisterBodySize)

	return json.UnmarshalRead(r.Body, destination, json.RejectUnknownMembers(true))
}
