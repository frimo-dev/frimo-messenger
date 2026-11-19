package message_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"uuid"

	"github.com/frimo-dev/frimo-messenger/internal/service/message"
	"github.com/frimo-dev/frimo-messenger/internal/service/message/mocks"
	"go.uber.org/mock/gomock"
)

func TestService_SendMessage_Direct_Success(t *testing.T) {
	ctrl := gomock.NewController(t)

	repository := mocks.NewMockRepository(ctrl)
	chatRepository := mocks.NewMockChatRepository(ctrl)

	senderID := uuid.New()
	recipientID := uuid.New()
	clientMessageID := uuid.New()

	chatID := uuid.New()
	topicID := uuid.New()

	// Именно этот ID должен вернуть сервис.
	// Он может отличаться от только что сгенерированного ID
	// при идемпотентном повторении запроса.
	storedMessageID := uuid.NewV7()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	chatRepository.EXPECT().
		GetOrCreateDirectChat(
			gomock.Any(),
			senderID,
			recipientID,
		).
		DoAndReturn(
			func(
				gotCtx context.Context,
				gotSenderID uuid.UUID,
				gotRecipientID uuid.UUID,
			) (message.DirectChat, error) {
				if gotCtx != ctx {
					t.Error("GetOrCreateDirectChat received wrong context")
				}

				return message.DirectChat{
					ChatID:  chatID,
					TopicID: topicID,
				}, nil
			},
		)

	repository.EXPECT().
		SaveMessage(
			gomock.Any(),
			gomock.Any(),
		).
		DoAndReturn(
			func(
				gotCtx context.Context,
				input message.SaveMessageInput,
			) (uuid.UUID, error) {
				if gotCtx != ctx {
					t.Error("SaveMessage received wrong context")
				}

				// Сервис должен сам сгенерировать UUIDv7.
				if input.ID == uuid.Nil() {
					t.Error("message ID must not be nil")
				}

				if input.ID[6]>>4 != 7 {
					t.Errorf(
						"message ID version = %d, want 7",
						input.ID[6]>>4,
					)
				}

				if input.ID[8]&0xc0 != 0x80 {
					t.Error("message ID has invalid UUID variant")
				}

				if input.Type != "USER" {
					t.Errorf(
						"message type = %q, want USER",
						input.Type,
					)
				}

				if input.SenderID != senderID {
					t.Errorf(
						"sender ID = %s, want %s",
						input.SenderID,
						senderID,
					)
				}

				if input.TopicID != topicID {
					t.Errorf(
						"topic ID = %s, want %s",
						input.TopicID,
						topicID,
					)
				}

				// Проверяем нормализацию текста.
				if input.Text != "Привет, мир!" {
					t.Errorf(
						"text = %q, want %q",
						input.Text,
						"Привет, мир!",
					)
				}

				if input.ClientMessageID != clientMessageID {
					t.Errorf(
						"client message ID = %s, want %s",
						input.ClientMessageID,
						clientMessageID,
					)
				}

				// RequestHash должен быть рассчитан сервисом.
				if input.RequestHash == ([32]byte{}) {
					t.Error("request hash must not be empty")
				}

				return storedMessageID, nil
			},
		)

	service := message.NewService(repository, chatRepository)

	result, err := service.SendMessage(
		ctx,
		message.SendMessageInput{
			SenderID:        senderID,
			RecipientID:     &recipientID,
			ClientMessageID: clientMessageID,
			MessageText:     " \tПривет, мир! \n",
		},
	)
	if err != nil {
		t.Fatalf("SendMessage() unexpected error: %v", err)
	}

	expectedResult := message.SendMessageResult{
		ChatID:    chatID,
		TopicID:   topicID,
		MessageID: storedMessageID,
	}

	if result != expectedResult {
		t.Errorf(
			"SendMessage() result = %+v, want %+v",
			result,
			expectedResult,
		)
	}
}

func TestService_SendMessage_Validation(t *testing.T) {
	senderID := uuid.New()
	recipientID := uuid.New()
	clientMessageID := uuid.New()

	chatID := uuid.New()
	topicID := uuid.New()

	newValidInput := func() message.SendMessageInput {
		return message.SendMessageInput{
			SenderID:        senderID,
			RecipientID:     &recipientID,
			ClientMessageID: clientMessageID,
			MessageText:     "Привет!",
		}
	}

	tests := []struct {
		name    string
		modify  func(*message.SendMessageInput)
		wantErr error
	}{
		{
			name: "nil sender ID",
			modify: func(input *message.SendMessageInput) {
				input.SenderID = uuid.Nil()
			},
			wantErr: message.ErrInvalidSender,
		},
		{
			name: "nil client message ID",
			modify: func(input *message.SendMessageInput) {
				input.ClientMessageID = uuid.Nil()
			},
			wantErr: message.ErrInvalidClientMessageID,
		},
		{
			name: "empty message text",
			modify: func(input *message.SendMessageInput) {
				input.MessageText = ""
			},
			wantErr: message.ErrInvalidMessageText,
		},
		{
			name: "whitespace only message",
			modify: func(input *message.SendMessageInput) {
				input.MessageText = " \t\n "
			},
			wantErr: message.ErrInvalidMessageText,
		},
		{
			name: "message text too long",
			modify: func(input *message.SendMessageInput) {
				input.MessageText = strings.Repeat("a", 4097)
			},
			wantErr: message.ErrInvalidMessageText,
		},
		{
			name: "nil recipient ID",
			modify: func(input *message.SendMessageInput) {
				zeroID := uuid.Nil()
				input.RecipientID = &zeroID
			},
			wantErr: message.ErrInvalidRecipientID,
		},
		{
			name: "cannot message self",
			modify: func(input *message.SendMessageInput) {
				input.RecipientID = &input.SenderID
			},
			wantErr: message.ErrCannotMessageSelf,
		},
		{
			name: "missing destination",
			modify: func(input *message.SendMessageInput) {
				input.RecipientID = nil
			},
			wantErr: message.ErrInvalidDestination,
		},
		{
			name: "group messages not implemented",
			modify: func(input *message.SendMessageInput) {
				input.RecipientID = nil
				input.ChatID = &chatID
				input.TopicID = &topicID
			},
			wantErr: message.ErrInvalidDestination,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			repository := mocks.NewMockRepository(ctrl)
			chatRepository := mocks.NewMockChatRepository(ctrl)

			// Никаких EXPECT() не задаём.
			// Любой вызов репозитория при ошибке
			// валидации будет считаться ошибкой теста.

			service := message.NewService(
				repository,
				chatRepository,
			)

			input := newValidInput()
			tt.modify(&input)

			result, err := service.SendMessage(
				context.Background(),
				input,
			)

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf(
					"SendMessage() error = %v, want %v",
					err,
					tt.wantErr,
				)
			}

			if result != (message.SendMessageResult{}) {
				t.Errorf(
					"SendMessage() result = %+v, want zero result",
					result,
				)
			}
		})
	}
}

func TestService_SendMessage_ChatRepositoryError(t *testing.T) {
	senderID := uuid.New()
	recipientID := uuid.New()
	clientMessageID := uuid.New()

	tests := []struct {
		name string
		err  error
	}{
		{
			name: "recipient not found",
			err:  message.ErrRecipientNotFound,
		},
		{
			name: "postgres unavailable",
			err:  errors.New("postgres unavailable"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			repository := mocks.NewMockRepository(ctrl)
			chatRepository := mocks.NewMockChatRepository(ctrl)

			chatRepository.EXPECT().
				GetOrCreateDirectChat(
					gomock.Any(),
					senderID,
					recipientID,
				).
				Return(
					message.DirectChat{},
					tt.err,
				)

			// SaveMessage не должен вызываться,
			// если получить или создать чат не удалось.

			service := message.NewService(
				repository,
				chatRepository,
			)

			result, err := service.SendMessage(
				context.Background(),
				message.SendMessageInput{
					SenderID:        senderID,
					RecipientID:     &recipientID,
					ClientMessageID: clientMessageID,
					MessageText:     "Привет!",
				},
			)

			// Проверяем, что fmt.Errorf сохранил
			// исходную ошибку через %w.
			if !errors.Is(err, tt.err) {
				t.Fatalf(
					"SendMessage() error = %v, want %v",
					err,
					tt.err,
				)
			}

			if result != (message.SendMessageResult{}) {
				t.Errorf(
					"SendMessage() result = %+v, want zero result",
					result,
				)
			}
		})
	}
}

func TestService_SendMessage_MessageRepositoryError(t *testing.T) {
	senderID := uuid.New()
	recipientID := uuid.New()
	clientMessageID := uuid.New()

	chatID := uuid.New()
	topicID := uuid.New()

	tests := []struct {
		name string
		err  error
	}{
		{
			name: "idempotency conflict",
			err:  message.ErrIdempotencyConflict,
		},
		{
			name: "send message expired",
			err:  message.ErrSendMessageExpired,
		},
		{
			name: "scylla unavailable",
			err:  errors.New("scylla unavailable"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			repository := mocks.NewMockRepository(ctrl)
			chatRepository := mocks.NewMockChatRepository(ctrl)

			chatRepository.EXPECT().
				GetOrCreateDirectChat(
					gomock.Any(),
					senderID,
					recipientID,
				).
				Return(
					message.DirectChat{
						ChatID:  chatID,
						TopicID: topicID,
					},
					nil,
				)

			repository.EXPECT().
				SaveMessage(
					gomock.Any(),
					gomock.Any(),
				).
				Return(
					uuid.Nil(),
					tt.err,
				)

			service := message.NewService(
				repository,
				chatRepository,
			)

			result, err := service.SendMessage(
				context.Background(),
				message.SendMessageInput{
					SenderID:        senderID,
					RecipientID:     &recipientID,
					ClientMessageID: clientMessageID,
					MessageText:     "Привет!",
				},
			)

			if !errors.Is(err, tt.err) {
				t.Fatalf(
					"SendMessage() error = %v, want %v",
					err,
					tt.err,
				)
			}

			if result != (message.SendMessageResult{}) {
				t.Errorf(
					"SendMessage() result = %+v, want zero result",
					result,
				)
			}
		})
	}
}

func TestService_SendMessage_RequestHash(t *testing.T) {
	senderID := uuid.New()

	recipientA := uuid.New()
	recipientB := uuid.New()

	tests := []struct {
		name           string
		recipientID    uuid.UUID
		text           string
		wantNormalized string
	}{
		{
			name:           "original message",
			recipientID:    recipientA,
			text:           "Привет!",
			wantNormalized: "Привет!",
		},
		{
			name:           "same normalized message",
			recipientID:    recipientA,
			text:           " \tПривет! \n",
			wantNormalized: "Привет!",
		},
		{
			name:           "different text",
			recipientID:    recipientA,
			text:           "Привет!!",
			wantNormalized: "Привет!!",
		},
		{
			name:           "different recipient",
			recipientID:    recipientB,
			text:           "Привет!",
			wantNormalized: "Привет!",
		},
	}

	hashes := make([][32]byte, len(tests))

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			repository := mocks.NewMockRepository(ctrl)
			chatRepository := mocks.NewMockChatRepository(ctrl)

			clientMessageID := uuid.New()
			topicID := uuid.New()

			chatRepository.EXPECT().
				GetOrCreateDirectChat(
					gomock.Any(),
					senderID,
					tt.recipientID,
				).
				Return(
					message.DirectChat{
						ChatID:  uuid.New(),
						TopicID: topicID,
					},
					nil,
				)

			repository.EXPECT().
				SaveMessage(
					gomock.Any(),
					gomock.Any(),
				).
				DoAndReturn(
					func(
						_ context.Context,
						input message.SaveMessageInput,
					) (uuid.UUID, error) {
						if input.Text != tt.wantNormalized {
							t.Errorf(
								"text = %q, want %q",
								input.Text,
								tt.wantNormalized,
							)
						}

						hashes[i] = input.RequestHash

						return uuid.NewV7(), nil
					},
				)

			service := message.NewService(
				repository,
				chatRepository,
			)

			_, err := service.SendMessage(
				context.Background(),
				message.SendMessageInput{
					SenderID:        senderID,
					RecipientID:     &tt.recipientID,
					ClientMessageID: clientMessageID,
					MessageText:     tt.text,
				},
			)
			if err != nil {
				t.Fatalf(
					"SendMessage() unexpected error: %v",
					err,
				)
			}
		})
	}

	if hashes[0] == ([32]byte{}) {
		t.Error("request hash must not be empty")
	}

	// Разные ClientMessageID и форматирование текста
	// не должны менять хэш одинакового сообщения.
	if hashes[0] != hashes[1] {
		t.Error(
			"same normalized message must produce the same hash",
		)
	}

	// Изменение текста должно менять хэш.
	if hashes[0] == hashes[2] {
		t.Error(
			"different message text must produce a different hash",
		)
	}

	// Изменение получателя должно менять хэш.
	if hashes[0] == hashes[3] {
		t.Error(
			"different recipient must produce a different hash",
		)
	}
}
