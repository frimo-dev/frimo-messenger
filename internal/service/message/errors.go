package message

import "errors"

var (
	ErrInvalidSender          = errors.New("invalid sender")
	ErrInvalidClientMessageID = errors.New("invalid client message ID")
	ErrInvalidRecipientID     = errors.New("invalid recipient ID")
	ErrInvalidMessageText     = errors.New("invalid message text")

	ErrIdempotencyConflict = errors.New("ClientMessageID already used for another request")
	ErrSendMessageExpired  = errors.New("message send window has expired")

	ErrRecipientNotFound = errors.New("recipient not found")

	ErrCannotMessageSelf          = errors.New("chat with the same ID is not allowed")
	ErrCannotMessageMultiuserChat = errors.New("multi-user chat is not allowed")
	ErrInvalidDestination         = errors.New("invalid destination")
)
