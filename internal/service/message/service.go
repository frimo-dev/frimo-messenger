package message

import (
	"context"
	"fmt"
	"uuid"
)

type Service struct {
	repository     Repository
	chatRepository ChatRepository
}

func NewService(repository Repository, chatRepository ChatRepository) *Service {
	return &Service{repository: repository, chatRepository: chatRepository}
}

type SendMessageInput struct {
	SenderID uuid.UUID

	RecipientID *uuid.UUID
	ChatID      *uuid.UUID
	TopicID     *uuid.UUID

	ClientMessageID uuid.UUID
	MessageText     string
}

type SendMessageResult struct {
	ChatID    uuid.UUID
	TopicID   uuid.UUID
	MessageID uuid.UUID
}

func (s *Service) SendMessage(ctx context.Context, input SendMessageInput) (SendMessageResult, error) {
	if input.SenderID == uuid.Nil() {
		return SendMessageResult{}, ErrInvalidSender
	}

	if input.ClientMessageID == uuid.Nil() {
		return SendMessageResult{}, ErrInvalidClientMessageID
	}

	input.MessageText = normalizeMessageText(input.MessageText)

	if err := validateMessageText(input.MessageText); err != nil {
		return SendMessageResult{}, err
	}

	// если id получателя не нулевое, значит отправка в direct чат
	if input.RecipientID != nil {
		if *input.RecipientID == uuid.Nil() {
			return SendMessageResult{}, ErrInvalidRecipientID
		}
		// TODO: временная проверка(потом чат сам с собой добавим)
		if input.SenderID == *input.RecipientID {
			return SendMessageResult{}, ErrCannotMessageSelf
		}

		directChat, err := s.chatRepository.GetOrCreateDirectChat(ctx, input.SenderID, *input.RecipientID)
		if err != nil {
			return SendMessageResult{}, fmt.Errorf("failed get or create direct chat: %w", err)
		}

		messageID, err := s.repository.SaveMessage(ctx, SaveMessageInput{
			ID: uuid.NewV7(),
			// TODO: вынести значения в константы
			Type:     "USER",
			SenderID: input.SenderID,
			TopicID:  directChat.TopicID,
			Text:     input.MessageText,

			ClientMessageID: input.ClientMessageID,
			RequestHash:     calculateRequestHash(*input.RecipientID, input.MessageText),
		})
		if err != nil {
			return SendMessageResult{}, fmt.Errorf("failed save message: %w", err)
		}

		return SendMessageResult{
			ChatID:    directChat.ChatID,
			TopicID:   directChat.TopicID,
			MessageID: messageID,
		}, nil
	}

	//if input.TopicID != nil && input.ChatID != nil {
	// TODO: здесь будет логика для отправки сообщения в многопользовательский чат
	//return SendMessageResult{}, ErrCannotMessageMultiuserChat
	//}

	return SendMessageResult{}, ErrInvalidDestination
}
