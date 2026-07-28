package httpapi

import (
	"errors"
	"log"
	"net/http"

	"github.com/frimo-dev/frimo-messenger/internal/emailverification"
)

type verifyEmailRequest struct {
	Token string `json:"token"`
}

type verifyEmailResponse struct {
	Status string `json:"status"`
}

func (a *API) verifyEmail(w http.ResponseWriter, r *http.Request) {
	var request verifyEmailRequest

	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	err := a.emailVerificationService.Confirm(r.Context(), request.Token)
	log.Printf("%v", err)

	if err != nil {
		switch {
		case errors.Is(err, emailverification.ErrInvalidToken):
			writeError(w, http.StatusBadRequest, "invalid_verification_token", "verification token is invalid")
		case errors.Is(err, emailverification.ErrExpiredToken):
			writeError(w, http.StatusBadRequest, "verification_token_expired", "verification token has expired")
		case errors.Is(err, emailverification.ErrUsedToken):
			writeError(w, http.StatusConflict, "verification_token_used", "verification token has already been used")
		default:
			writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
		}
		return
	}

	writeJSON(w, http.StatusOK, verifyEmailResponse{Status: "email_verified"})
}
