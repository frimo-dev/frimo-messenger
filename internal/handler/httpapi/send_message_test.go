package httpapi_test

import (
	"encoding/json/v2"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"uuid"

	"github.com/frimo-dev/frimo-messenger/internal/handler/httpapi"
	"github.com/frimo-dev/frimo-messenger/internal/handler/httpapi/mocks"
	"github.com/frimo-dev/frimo-messenger/internal/service/auth"
	"github.com/frimo-dev/frimo-messenger/internal/service/message"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

func TestAPI_SendMessage(t *testing.T) {
	senderID := uuid.New()
	recipientID := uuid.New()
	clientMessageID := uuid.New()

	chatID := uuid.New()
	topicID := uuid.New()
	messageID := uuid.NewV7()

	const (
		accessToken = "valid-access-token"
		messageText = "Привет!"
	)

	validBody := fmt.Sprintf(`{
		"recipient_id": %q,
		"client_message_id": %q,
		"text": %q
	}`,
		recipientID.String(),
		clientMessageID.String(),
		messageText,
	)

	successResult := message.SendMessageResult{
		ChatID:    chatID,
		TopicID:   topicID,
		MessageID: messageID,
	}

	internalErr := errors.New("database unavailable")

	tests := []struct {
		name string
		body string

		// Ошибка, возвращаемая MessageService.
		// Если nil, ожидаем успешную отправку.
		serviceErr error

		// Должен ли обработчик вызвать MessageService.
		callService bool

		wantStatus  int
		wantCode    string
		wantMessage string
	}{
		{
			name:        "success",
			body:        validBody,
			callService: true,
			wantStatus:  http.StatusOK,
		},
		{
			name:        "invalid sender",
			body:        validBody,
			serviceErr:  message.ErrInvalidSender,
			callService: true,

			wantStatus:  http.StatusBadRequest,
			wantCode:    "invalid_request",
			wantMessage: "invalid request body",
		},
		{
			name:        "invalid client message ID",
			body:        validBody,
			serviceErr:  message.ErrInvalidClientMessageID,
			callService: true,

			wantStatus:  http.StatusBadRequest,
			wantCode:    "invalid_request",
			wantMessage: "invalid request body",
		},
		{
			name:        "invalid message text",
			body:        validBody,
			serviceErr:  message.ErrInvalidMessageText,
			callService: true,

			wantStatus:  http.StatusBadRequest,
			wantCode:    "invalid_request",
			wantMessage: "invalid request body",
		},
		{
			name:        "invalid recipient ID",
			body:        validBody,
			serviceErr:  message.ErrInvalidRecipientID,
			callService: true,

			wantStatus:  http.StatusBadRequest,
			wantCode:    "invalid_request",
			wantMessage: "invalid request body",
		},
		{
			name:        "cannot message self",
			body:        validBody,
			serviceErr:  message.ErrCannotMessageSelf,
			callService: true,

			wantStatus:  http.StatusBadRequest,
			wantCode:    "invalid_request",
			wantMessage: "invalid request body",
		},
		{
			name:        "recipient not found",
			body:        validBody,
			serviceErr:  message.ErrRecipientNotFound,
			callService: true,

			wantStatus:  http.StatusNotFound,
			wantCode:    "recipient_not_found",
			wantMessage: "recipient not found",
		},
		{
			name:        "idempotency conflict",
			body:        validBody,
			serviceErr:  message.ErrIdempotencyConflict,
			callService: true,

			wantStatus:  http.StatusConflict,
			wantCode:    "idempotency_conflict",
			wantMessage: "client_message_id already used",
		},
		{
			name:        "send message expired",
			body:        validBody,
			serviceErr:  message.ErrSendMessageExpired,
			callService: true,

			wantStatus:  http.StatusConflict,
			wantCode:    "send_expired",
			wantMessage: "message send window expired",
		},
		{
			name:        "internal error",
			body:        validBody,
			serviceErr:  internalErr,
			callService: true,

			wantStatus:  http.StatusInternalServerError,
			wantCode:    "internal_error",
			wantMessage: "internal server error",
		},
		{
			name: "missing recipient ID",
			body: fmt.Sprintf(`{
				"client_message_id": %q,
				"text": %q
			}`,
				clientMessageID.String(),
				messageText,
			),

			wantStatus:  http.StatusBadRequest,
			wantCode:    "invalid_request",
			wantMessage: "invalid request body",
		},
		{
			name: "nil recipient ID",
			body: fmt.Sprintf(`{
				"recipient_id": %q,
				"client_message_id": %q,
				"text": %q
			}`,
				uuid.Nil().String(),
				clientMessageID.String(),
				messageText,
			),

			wantStatus:  http.StatusBadRequest,
			wantCode:    "invalid_request",
			wantMessage: "invalid request body",
		},
		{
			name: "invalid recipient ID format",
			body: fmt.Sprintf(`{
				"recipient_id": "invalid-uuid",
				"client_message_id": %q,
				"text": %q
			}`,
				clientMessageID.String(),
				messageText,
			),

			wantStatus:  http.StatusBadRequest,
			wantCode:    "invalid_request",
			wantMessage: "invalid request body",
		},
		{
			name: "missing client message ID",
			body: fmt.Sprintf(`{
				"recipient_id": %q,
				"text": %q
			}`,
				recipientID.String(),
				messageText,
			),

			wantStatus:  http.StatusBadRequest,
			wantCode:    "invalid_request",
			wantMessage: "invalid request body",
		},
		{
			name: "nil client message ID",
			body: fmt.Sprintf(`{
				"recipient_id": %q,
				"client_message_id": %q,
				"text": %q
			}`,
				recipientID.String(),
				uuid.Nil().String(),
				messageText,
			),

			wantStatus:  http.StatusBadRequest,
			wantCode:    "invalid_request",
			wantMessage: "invalid request body",
		},
		{
			name: "invalid client message ID format",
			body: fmt.Sprintf(`{
				"recipient_id": %q,
				"client_message_id": "invalid-uuid",
				"text": %q
			}`,
				recipientID.String(),
				messageText,
			),

			wantStatus:  http.StatusBadRequest,
			wantCode:    "invalid_request",
			wantMessage: "invalid request body",
		},
		{
			name: "missing text",
			body: fmt.Sprintf(`{
				"recipient_id": %q,
				"client_message_id": %q
			}`,
				recipientID.String(),
				clientMessageID.String(),
			),

			wantStatus:  http.StatusBadRequest,
			wantCode:    "invalid_request",
			wantMessage: "invalid request body",
		},
		{
			name: "empty text",
			body: fmt.Sprintf(`{
				"recipient_id": %q,
				"client_message_id": %q,
				"text": ""
			}`,
				recipientID.String(),
				clientMessageID.String(),
			),

			wantStatus:  http.StatusBadRequest,
			wantCode:    "invalid_request",
			wantMessage: "invalid request body",
		},
		{
			name: "invalid text type",
			body: fmt.Sprintf(`{
				"recipient_id": %q,
				"client_message_id": %q,
				"text": 123
			}`,
				recipientID.String(),
				clientMessageID.String(),
			),

			wantStatus:  http.StatusBadRequest,
			wantCode:    "invalid_request",
			wantMessage: "invalid request body",
		},
		{
			name: "invalid JSON",
			body: `{"recipient_id":`,

			wantStatus:  http.StatusBadRequest,
			wantCode:    "invalid_request",
			wantMessage: "invalid request body",
		},
		{
			name: "empty body",
			body: "",

			wantStatus:  http.StatusBadRequest,
			wantCode:    "invalid_request",
			wantMessage: "invalid request body",
		},
		{
			name: "unknown field",
			body: fmt.Sprintf(`{
				"recipient_id": %q,
				"client_message_id": %q,
				"text": %q,
				"sender_id": %q
			}`,
				recipientID.String(),
				clientMessageID.String(),
				messageText,
				senderID.String(),
			),

			wantStatus:  http.StatusBadRequest,
			wantCode:    "invalid_request",
			wantMessage: "invalid request body",
		},
		{
			name: "invalid recipient ID type",
			body: fmt.Sprintf(`{
				"recipient_id": 123,
				"client_message_id": %q,
				"text": %q
			}`,
				clientMessageID.String(),
				messageText,
			),

			wantStatus:  http.StatusBadRequest,
			wantCode:    "invalid_request",
			wantMessage: "invalid request body",
		},
		{
			name: "invalid client message ID type",
			body: fmt.Sprintf(`{
				"recipient_id": %q,
				"client_message_id": 123,
				"text": %q
			}`,
				recipientID.String(),
				messageText,
			),

			wantStatus:  http.StatusBadRequest,
			wantCode:    "invalid_request",
			wantMessage: "invalid request body",
		},
		{
			name: "multiple JSON objects",
			body: validBody + validBody,

			wantStatus:  http.StatusBadRequest,
			wantCode:    "invalid_request",
			wantMessage: "invalid request body",
		},
		{
			name: "body too large",
			body: fmt.Sprintf(`{
				"recipient_id": %q,
				"client_message_id": %q,
				"text": %q
			}`,
				recipientID.String(),
				clientMessageID.String(),
				strings.Repeat("a", 33*1024),
			),

			wantStatus:  http.StatusBadRequest,
			wantCode:    "invalid_request",
			wantMessage: "invalid request body",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			authService := mocks.NewMockAuthService(ctrl)
			userService := mocks.NewMockUserService(ctrl)
			messageService := mocks.NewMockMessageService(ctrl)

			// Каждый сценарий здесь выполняется
			// от имени авторизованного пользователя.
			authService.EXPECT().
				Authenticate(
					gomock.Any(),
					accessToken,
				).
				Return(
					auth.Identity{
						UserID: senderID,
					},
					nil,
				)

			// Ожидаем SendMessage только для запросов,
			// успешно прошедших HTTP-валидацию.
			if tt.callService {
				expectedInput := message.SendMessageInput{
					SenderID:        senderID,
					RecipientID:     &recipientID,
					ClientMessageID: clientMessageID,
					MessageText:     messageText,
				}

				result := successResult
				if tt.serviceErr != nil {
					result = message.SendMessageResult{}
				}

				messageService.EXPECT().
					SendMessage(
						gomock.Any(),
						expectedInput,
					).
					Return(result, tt.serviceErr)
			}

			api := httpapi.New(
				zap.NewNop(),
				authService,
				userService,
				messageService,
			)

			req := httptest.NewRequest(
				http.MethodPost,
				"/messages",
				strings.NewReader(tt.body),
			)

			req.Header.Set(
				"Authorization",
				"Bearer "+accessToken,
			)
			req.Header.Set(
				"Content-Type",
				"application/json",
			)

			rec := httptest.NewRecorder()

			api.Handler().ServeHTTP(rec, req)

			if tt.wantStatus != http.StatusOK {
				assertErrorResponse(
					t,
					rec,
					tt.wantStatus,
					tt.wantCode,
					tt.wantMessage,
				)
				return
			}

			if rec.Code != http.StatusOK {
				t.Fatalf(
					"status = %d, want %d; body = %s",
					rec.Code,
					http.StatusOK,
					rec.Body.String(),
				)
			}

			if contentType := rec.Header().Get("Content-Type"); contentType != "application/json" {
				t.Errorf(
					"Content-Type = %q, want %q",
					contentType,
					"application/json",
				)
			}

			var response struct {
				ChatID    uuid.UUID `json:"chat_id"`
				TopicID   uuid.UUID `json:"topic_id"`
				MessageID uuid.UUID `json:"message_id"`
			}

			if err := json.Unmarshal(
				rec.Body.Bytes(),
				&response,
			); err != nil {
				t.Fatalf(
					"unmarshal success response: %v; body = %s",
					err,
					rec.Body.String(),
				)
			}

			if response.ChatID != chatID {
				t.Errorf(
					"ChatID = %s, want %s",
					response.ChatID,
					chatID,
				)
			}

			if response.TopicID != topicID {
				t.Errorf(
					"TopicID = %s, want %s",
					response.TopicID,
					topicID,
				)
			}

			if response.MessageID != messageID {
				t.Errorf(
					"MessageID = %s, want %s",
					response.MessageID,
					messageID,
				)
			}
		})
	}
}

func TestAPI_SendMessage_Authentication(t *testing.T) {
	senderID := uuid.New()
	recipientID := uuid.New()
	clientMessageID := uuid.New()

	validBody := fmt.Sprintf(`{
		"recipient_id": %q,
		"client_message_id": %q,
		"text": "Привет!"
	}`,
		recipientID.String(),
		clientMessageID.String(),
	)

	tests := []struct {
		name string

		authorization string

		authenticateErr error
		callAuth        bool

		wantStatus  int
		wantCode    string
		wantMessage string
	}{
		{
			name: "missing authorization header",

			wantStatus:  http.StatusUnauthorized,
			wantCode:    "authentication_error",
			wantMessage: "authentication failed",
		},
		{
			name:          "invalid authorization scheme",
			authorization: "Basic credentials",

			wantStatus:  http.StatusUnauthorized,
			wantCode:    "authentication_error",
			wantMessage: "authentication failed",
		},
		{
			name:            "invalid access token",
			authorization:   "Bearer invalid-token",
			authenticateErr: auth.ErrAccessTokenInvalid,
			callAuth:        true,

			wantStatus:  http.StatusUnauthorized,
			wantCode:    "authentication_error",
			wantMessage: "authentication failed",
		},
		{
			name:            "inactive session",
			authorization:   "Bearer invalid-token",
			authenticateErr: auth.ErrSessionInactive,
			callAuth:        true,

			wantStatus:  http.StatusUnauthorized,
			wantCode:    "authentication_error",
			wantMessage: "authentication failed",
		},
		{
			name:            "authentication internal error",
			authorization:   "Bearer invalid-token",
			authenticateErr: errors.New("redis unavailable"),
			callAuth:        true,

			wantStatus:  http.StatusInternalServerError,
			wantCode:    "internal_error",
			wantMessage: "internal server error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			authService := mocks.NewMockAuthService(ctrl)
			userService := mocks.NewMockUserService(ctrl)
			messageService := mocks.NewMockMessageService(ctrl)

			if tt.callAuth {
				authService.EXPECT().
					Authenticate(
						gomock.Any(),
						"invalid-token",
					).
					Return(
						auth.Identity{
							UserID: senderID,
						},
						tt.authenticateErr,
					)
			}

			// SendMessage не должен вызываться
			// при ошибке авторизации.
			// Поэтому EXPECT() для него не задаём.

			api := httpapi.New(
				zap.NewNop(),
				authService,
				userService,
				messageService,
			)

			req := httptest.NewRequest(
				http.MethodPost,
				"/messages",
				strings.NewReader(validBody),
			)

			if tt.authorization != "" {
				req.Header.Set(
					"Authorization",
					tt.authorization,
				)
			}

			rec := httptest.NewRecorder()

			api.Handler().ServeHTTP(rec, req)

			assertErrorResponse(
				t,
				rec,
				tt.wantStatus,
				tt.wantCode,
				tt.wantMessage,
			)
		})
	}
}

func TestAPI_SendMessage_MethodNotAllowed(t *testing.T) {
	tests := []struct {
		name   string
		method string
	}{
		{
			name:   "GET",
			method: http.MethodGet,
		},
		{
			name:   "PUT",
			method: http.MethodPut,
		},
		{
			name:   "PATCH",
			method: http.MethodPatch,
		},
		{
			name:   "DELETE",
			method: http.MethodDelete,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			authService := mocks.NewMockAuthService(ctrl)
			userService := mocks.NewMockUserService(ctrl)
			messageService := mocks.NewMockMessageService(ctrl)

			api := httpapi.New(
				zap.NewNop(),
				authService,
				userService,
				messageService,
			)

			req := httptest.NewRequest(
				tt.method,
				"/messages",
				nil,
			)

			rec := httptest.NewRecorder()

			api.Handler().ServeHTTP(rec, req)

			if rec.Code != http.StatusMethodNotAllowed {
				t.Fatalf(
					"status = %d, want %d; body = %s",
					rec.Code,
					http.StatusMethodNotAllowed,
					rec.Body.String(),
				)
			}
		})
	}
}
