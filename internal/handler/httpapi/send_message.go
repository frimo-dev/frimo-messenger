package httpapi

import (
	"errors"
	"net/http"
	"uuid"

	"github.com/frimo-dev/frimo-messenger/internal/execution"
	"github.com/frimo-dev/frimo-messenger/internal/service/message"
	"go.uber.org/zap"
)

const maxSendMessageBodySize = 32768

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

type SendMessageResponse struct {
	ChatID    uuid.UUID `json:"chat_id"`
	TopicID   uuid.UUID `json:"topic_id"`
	MessageID uuid.UUID `json:"message_id"`
}

func (a *API) sendMessage(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxSendMessageBodySize)

	identity, ok := identityFromContext(r.Context())
	if !ok {
		a.respondError(r.Context(), w, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}

	var request sendMessageRequest
	if err := decodeJSON(r, &request); err != nil {
		a.respondError(r.Context(), w, http.StatusBadRequest, "invalid_request", "invalid request body")
		return
	}

	sendMessageResult, err := a.messageService.SendMessage(
		r.Context(),
		message.SendMessageInput{
			SenderID:        identity.UserID,
			RecipientID:     &request.RecipientID,
			ClientMessageID: request.ClientMessageID,
			MessageText:     request.MessageText,
		})
	if err != nil {
		switch {
		case errors.Is(err, message.ErrInvalidSender),
			errors.Is(err, message.ErrInvalidClientMessageID),
			errors.Is(err, message.ErrInvalidMessageText),
			errors.Is(err, message.ErrInvalidRecipientID),
			errors.Is(err, message.ErrCannotMessageSelf):

			a.respondError(
				r.Context(), w,
				http.StatusBadRequest,
				"invalid_request",
				"invalid request body",
			)

		case errors.Is(err, message.ErrRecipientNotFound):
			a.respondError(
				r.Context(), w,
				http.StatusNotFound,
				"recipient_not_found",
				"recipient not found",
			)

		case errors.Is(err, message.ErrIdempotencyConflict):
			a.respondError(
				r.Context(), w,
				http.StatusConflict,
				"idempotency_conflict",
				"client_message_id already used",
			)

		case errors.Is(err, message.ErrSendMessageExpired):
			a.respondError(
				r.Context(), w,
				http.StatusConflict,
				"send_expired",
				"message send window expired",
			)

		default:
			a.logger.Error(
				"failed to send message",
				zap.Error(err),
				zap.String("request_id", execution.IDFromContext(r.Context())),
			)

			a.respondError(
				r.Context(), w,
				http.StatusInternalServerError,
				"internal_error",
				"internal server error",
			)
		}
		return
	}

	a.respondJSON(r.Context(), w, http.StatusOK,
		SendMessageResponse{
			ChatID:    sendMessageResult.ChatID,
			TopicID:   sendMessageResult.TopicID,
			MessageID: sendMessageResult.MessageID,
		},
	)
}
