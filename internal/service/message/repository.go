package message

import (
	"context"
	"uuid"
)

type SaveMessageInput struct {
	ID       uuid.UUID
	Type     string
	SenderID uuid.UUID
	TopicID  uuid.UUID
	Text     string

	ClientMessageID uuid.UUID
	RequestHash     [32]byte
}

// Repository для работы со Scylla
type Repository interface {
	SaveMessage(ctx context.Context, input SaveMessageInput) (uuid.UUID, error)
}

type DirectChat struct {
	ChatID  uuid.UUID
	TopicID uuid.UUID
}

// ChatRepository для работы с Postgres
type ChatRepository interface {
	GetOrCreateDirectChat(ctx context.Context, senderID uuid.UUID, recipientID uuid.UUID) (DirectChat, error)
}
