package scylla

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"
	"uuid"

	"github.com/frimo-dev/frimo-messenger/internal/service/message"
	"github.com/gocql/gocql"
)

const (
	messageSendWindow = 15 * time.Second
	initialVersion    = 1
)

type MessageRepository struct {
	session *gocql.Session
}

func NewMessageRepository(session *gocql.Session) *MessageRepository {
	return &MessageRepository{session: session}
}

type sendReservation struct {
	MessageID uuid.UUID
	TopicID   uuid.UUID
	CreatedAt time.Time
}

func (r *MessageRepository) SaveMessage(ctx context.Context, input message.SaveMessageInput) (uuid.UUID, error) {
	reservation, err := r.reserveMessageSend(ctx, input)
	if err != nil {
		return uuid.Nil(), err
	}

	bucket := reservation.CreatedAt.Format("2006-01-02")

	// Поскольку в ScyllaDB отсутствуют транзакции, то возможно, что в message_send_requests появилась запись,
	// а в messages нет. Тогда нужно просто проверить это, и если сообщение не сохранено, то сохранить
	exists, err := r.messageExists(ctx, reservation.TopicID, bucket, reservation.MessageID)
	if err != nil {
		return uuid.Nil(), fmt.Errorf("failed check message existence: %w", err)
	}

	if exists {
		return reservation.MessageID, nil
	}

	// Если старое сообщение так и не было сохранено, и пользователь повторил отправку через 5 минут, то может
	// возникнуть ситуация, что неожиданно для участников топика появится сообщение. Поэтому поставим timeout
	// на повторение запроса не более 15-ти секунд(будем предполагать, что за это время много сообщений не успеют
	// написать, и появление сообщения не станет чем-то неожиданным). Нужно не путать с ситуацией, когда сообщение
	// сохранено, но пользователю просто не пришла информация об этом(был обрыв соединения) и он повторяет запрос.
	// Для такой ситуации timeout 5 минут
	deadline := reservation.CreatedAt.Add(messageSendWindow)
	if !time.Now().Before(deadline) {
		return uuid.Nil(), message.ErrSendMessageExpired
	}

	writeCtx, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()

	// У ScyllaDB есть свой внутренний timestamp для каждой записи в таблицу для того, чтобы конкурентные запросы
	// INSERT могли решить чей результат применится, а чей нет. Применится запрос с наибольшим timestamp.
	// Но мы можем сами указать этот timestamp, тем самым сами решить какой запрос приоритетнее.

	// В нашей ситуации это нужно для случая, когда придет несколько запросов идемпотентности. Мы им зафиксируем
	// timestamp времени создания сообщения. Один запрос перед записью на какое-то время зависнет,
	// а второй выполнится, вдобавок прилетит запрос на изменение записи(редактирование сообщения) который тоже
	// выполнится. Потом оживет зависший запрос идемпотентности, но из-за старого(заранее зафиксированного)
	// timestamp не выполнится. И в БД будет консистентное состояние
	writeTimestamp := reservation.CreatedAt.UnixMicro()

	if err := r.insertInitialVersion(
		writeCtx,
		reservation.MessageID,
		input.SenderID,
		reservation.CreatedAt,
		input.Text,
		writeTimestamp,
	); err != nil {
		return uuid.Nil(), fmt.Errorf("failed insert initial message version: %w", err)
	}

	if err := r.insertMessage(
		writeCtx,
		reservation.MessageID,
		reservation.TopicID,
		bucket,
		input.SenderID,
		input.Type,
		reservation.CreatedAt,
		input.Text,
		writeTimestamp,
	); err != nil {
		return uuid.Nil(), fmt.Errorf("failed insert message: %w", err)
	}

	return reservation.MessageID, nil
}

// reserveMessageSend создаёт запись идемпотентности
// либо возвращает ранее зарегистрированную операцию.
func (r *MessageRepository) reserveMessageSend(ctx context.Context, input message.SaveMessageInput) (sendReservation, error) {
	const query = `
		INSERT INTO message_send_requests (
			sender_id,
			client_message_id,
			message_id,
			topic_id,
			request_hash,
			created_at
		)
		VALUES (?, ?, ?, ?, ?, ?)
		IF NOT EXISTS;
	`

	createdAt, err := getTimeFromUuidV7(input.ID)
	if err != nil {
		return sendReservation{}, fmt.Errorf("invalid message ID: %w", err)
	}

	// Если вставка не удалась(если данные уже есть, поскольку тут LWT),
	// то в values окажутся данные, которые уже есть в таблице
	values := make(map[string]any)

	// TODO: перед production вернуться к LocalQuorum и LocalSerial
	applied, err := r.session.Query(
		query,
		toCQLUUID(input.SenderID),
		toCQLUUID(input.ClientMessageID),
		toCQLUUID(input.ID),
		toCQLUUID(input.TopicID),
		input.RequestHash[:],
		createdAt,
	).
		WithContext(ctx).
		Consistency(gocql.LocalQuorum).
		SerialConsistency(gocql.LocalSerial).
		MapScanCAS(values)
	if err != nil {
		return sendReservation{}, fmt.Errorf("failed reserve message send: %w", err)
	}

	if applied {
		return sendReservation{MessageID: input.ID, TopicID: input.TopicID, CreatedAt: createdAt}, nil
	}

	return parseExistingReservation(values, input)
}

func parseExistingReservation(values map[string]any, input message.SaveMessageInput) (sendReservation, error) {
	storedMessageID, ok := values["message_id"].(gocql.UUID)
	if !ok {
		return sendReservation{}, fmt.Errorf("invalid existing reservation message_id: %T", values["message_id"])
	}

	storedTopicID, ok := values["topic_id"].(gocql.UUID)
	if !ok {
		return sendReservation{}, fmt.Errorf("invalid existing reservation topic_id: %T", values["topic_id"])
	}

	storedHash, ok := values["request_hash"].([]byte)
	if !ok {
		return sendReservation{}, fmt.Errorf("invalid existing reservation request_hash: %T", values["request_hash"])
	}

	storedCreatedAt, ok := values["created_at"].(time.Time)
	if !ok {
		return sendReservation{}, fmt.Errorf("invalid existing reservation created_at: %T", values["created_at"])
	}

	// Повторное использование ClientMessageID с другими данными запрещено
	if !bytes.Equal(storedHash, input.RequestHash[:]) {
		return sendReservation{}, message.ErrIdempotencyConflict
	}

	if fromCQLUUID(storedTopicID) != input.TopicID {
		return sendReservation{}, message.ErrIdempotencyConflict
	}

	return sendReservation{
		MessageID: fromCQLUUID(storedMessageID),
		TopicID:   fromCQLUUID(storedTopicID),
		CreatedAt: storedCreatedAt,
	}, nil
}

func (r *MessageRepository) messageExists(ctx context.Context, topicID uuid.UUID, bucket string, messageID uuid.UUID) (bool, error) {
	const query = `
		SELECT message_id
		FROM messages
		WHERE topic_id = ?
		  AND bucket = ?
		  AND message_id = ?
	`

	var storedID gocql.UUID

	err := r.session.Query(query, toCQLUUID(topicID), bucket, toCQLUUID(messageID)).WithContext(ctx).Consistency(gocql.LocalQuorum).Scan(&storedID)
	if errors.Is(err, gocql.ErrNotFound) {
		return false, nil
	}

	if err != nil {
		return false, err
	}

	return true, nil
}

// insertInitialVersion создаёт неизменяемую версию 1.
func (r *MessageRepository) insertInitialVersion(
	ctx context.Context,
	messageID uuid.UUID,
	senderID uuid.UUID,
	createdAt time.Time,
	text string,
	writeTimestamp int64,
) error {
	const query = `
		INSERT INTO message_versions (
			message_id,
			version,
			created_by,
			created_at,
			text,
			attachments_json
		)
		VALUES (?, ?, ?, ?, ?, ?)
		USING TIMESTAMP ?
	`

	return r.session.Query(
		query,
		toCQLUUID(messageID),
		int32(initialVersion),
		toCQLUUID(senderID),
		createdAt,
		text,
		"[]",
		writeTimestamp,
	).
		WithContext(ctx).
		Consistency(gocql.LocalQuorum).
		Idempotent(true).
		Exec()
}

// insertMessage создаёт основную запись для истории.
func (r *MessageRepository) insertMessage(ctx context.Context, messageID uuid.UUID, topicID uuid.UUID, bucket string, senderID uuid.UUID, messageType string, createdAt time.Time, text string, writeTimestamp int64) error {
	const query = `
		INSERT INTO messages (
			message_id,
			topic_id,
			bucket,
			message_type,
			created_at,
			sender_id,
			current_version,
			text,
			attachments_json
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		USING TIMESTAMP ?
	`

	return r.session.Query(
		query,
		toCQLUUID(messageID),
		toCQLUUID(topicID),
		bucket,
		messageType,
		createdAt,
		toCQLUUID(senderID),
		int32(initialVersion),
		text,
		"[]",
		writeTimestamp,
	).
		WithContext(ctx).
		Consistency(gocql.LocalQuorum).
		Idempotent(true).
		Exec()
}

func getTimeFromUuidV7(id uuid.UUID) (time.Time, error) {
	if id[6]>>4 != 7 {
		return time.Time{}, errors.New("expected UUIDv7")
	}

	if id[8]&0xc0 != 0x80 {
		return time.Time{}, errors.New("invalid UUID variant")
	}

	milliseconds := int64(id[0])<<40 |
		int64(id[1])<<32 |
		int64(id[2])<<24 |
		int64(id[3])<<16 |
		int64(id[4])<<8 |
		int64(id[5])

	return time.UnixMilli(milliseconds).UTC(), nil
}

// Преобразование UUID приложения в UUID драйвера.
// Предполагается, что uuid.UUID — массив [16]byte.
func toCQLUUID(id uuid.UUID) gocql.UUID {
	return gocql.UUID(id)
}

func fromCQLUUID(id gocql.UUID) uuid.UUID {
	return uuid.UUID(id)
}
