package httpapi

import (
	"encoding/json/v2"
	"net/http"
)

type resendRequest struct {
	Email string `json:"email"`
}

type resendResponse struct {
	Status string `json:"status"`
}

func (a *API) resendVerificationToken(w http.ResponseWriter, r *http.Request) {
	var request resendRequest

	if err := json.UnmarshalRead(r.Body, &request); err != nil {
		a.respondError(r.Context(), w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	// вызываем сервис
}
