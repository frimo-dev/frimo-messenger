package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

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
		a.respondError(r.Context(), w, http.StatusBadRequest, "invalid_request", err.Error())
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

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(destination); err != nil {
		return classifyJSONError(err)
	}

	// Проверяем, что второго JSON объекта нет, кроме пробельных символов и конца тела
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("request body must contain a single JSON object")
	}

	return nil
}

func classifyJSONError(err error) error {
	var syntaxError *json.SyntaxError

	if errors.As(err, &syntaxError) {
		return errors.New("request body contains malformed JSON")
	}

	var typeError *json.UnmarshalTypeError

	if errors.As(err, &typeError) {
		if typeError.Field != "" {
			return errors.New(
				"request field " + typeError.Field + " has an invalid type",
			)
		}

		return errors.New("request contains a value of an invalid type")
	}

	if errors.Is(err, io.EOF) {
		return errors.New("request body must not be empty")
	}

	var maxBytesError *http.MaxBytesError

	if errors.As(err, &maxBytesError) {
		return errors.New("request body is too large")
	}

	if strings.HasPrefix(err.Error(), "json: unknown field ") {
		return errors.New("request contains an unknown field")
	}

	return errors.New("request body could not be decoded")
}
