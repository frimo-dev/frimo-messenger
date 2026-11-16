package httpapi

import (
	"errors"
	"net/http"
	"uuid"
)

type sendMessageRequest struct {
	RecipientID uuid.UUID `json:"recipient_id"`
	// the key to idempotence from client
	ClientMessageID uuid.UUID `json:"client_message_id"`
	MessageText     string    `json:"text"`
}

func (r sendMessageRequest) Validate() error {
	switch {
	case r.ClientMessageID == uuid.Nil():
		return errors.New("client_message_id is required")
	case r.RecipientID == uuid.Nil():
		return errors.New("recipient_id is required")
	case r.MessageText == "":
		return errors.New("text is required")
	default:
		return nil
	}
}

// TODO: добавить максимальный размер тела
func (a *API) sendMessage(w http.ResponseWriter, r *http.Request) {
	//identity, ok := identityFromContext(r.Context())
	//if !ok {
	//	a.respondError(r.Context(), w, http.StatusInternalServerError, "internal_error", "internal server error")
	//	return
	//}

	var request sendMessageRequest
	if err := decodeJSON(r, &request); err != nil {
		a.respondError(r.Context(), w, http.StatusBadRequest, "invalid_request", "invalid request body")
		return
	}
}
