package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/frimo-dev/frimo-messenger/internal/user"
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

	if err := decodeRegisterRequest(w, r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	createdUser, err := a.userService.Register(r.Context(), user.RegisterInput{
		Email:    request.Email,
		Password: request.Password,
	})

	if err != nil {
		var validationErr *user.ValidationError

		switch {
		case errors.As(err, &validationErr):
			writeError(
				w,
				http.StatusBadRequest,
				validationErr.Code,
				validationErr.Message,
			)

		case errors.Is(err, user.ErrEmailAlreadyExists):
			writeError(
				w,
				http.StatusConflict,
				"email_already_exists",
				"user with this email already exists",
			)

		default:
			writeError(
				w,
				http.StatusInternalServerError,
				"internal_error",
				"internal server error",
			)
		}

		return
	}

	writeJSON(w, http.StatusCreated, registerResponse{
		ID:    createdUser.ID,
		Email: createdUser.Email,
	})
}

func decodeRegisterRequest(w http.ResponseWriter, r *http.Request, destination *registerRequest) error {
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
