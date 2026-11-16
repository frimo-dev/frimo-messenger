package postgres

import (
	"bytes"
	"context"
	"crypto/sha1"
	"errors"
	"fmt"
	"time"
	"uuid"

	"github.com/frimo-dev/frimo-messenger/internal/service/message"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ChatRepository struct {
	pool *pgxpool.Pool
}

func NewChatRepository(pool *pgxpool.Pool) *ChatRepository {
	return &ChatRepository{pool: pool}
}

// GetOrCreateDirectChat возвращает существующий DIRECT-чат либо создаёт его вместе с GENERAL-топиком и участниками.
func (r *ChatRepository) GetOrCreateDirectChat(ctx context.Context, senderID uuid.UUID, recipientID uuid.UUID) (message.DirectChat, error) {
	if senderID == uuid.Nil() {
		return message.DirectChat{}, message.ErrInvalidSender
	}

	if recipientID == uuid.Nil() {
		return message.DirectChat{}, message.ErrInvalidRecipientID
	}

	// TODO: потом можно будет
	if senderID == recipientID {
		return message.DirectChat{}, message.ErrCannotMessageSelf
	}

	chatID := generateDirectChatID(senderID, recipientID)

	chat, err := r.findDirectChat(ctx, chatID, senderID, recipientID)
	if err == nil {
		return chat, nil
	}

	if !errors.Is(err, pgx.ErrNoRows) {
		return message.DirectChat{}, fmt.Errorf("failed to find direct chat: %w", err)
	}

	return r.createDirectChat(ctx, chatID, senderID, recipientID)
}

func (r *ChatRepository) findDirectChat(ctx context.Context, chatID, senderID, recipientID uuid.UUID) (message.DirectChat, error) {
	const query = `
        SELECT
            c.type::text,
            t.id,
            (
                SELECT COUNT(*)
                FROM chat_members m
                WHERE m.chat_id = c.id
            ),
            (
                SELECT COUNT(*)
                FROM chat_members m
                WHERE m.chat_id = c.id
                  AND m.user_id IN ($2, $3)
                  AND m.role = 'MEMBER'
                  AND m.status = 'ACTIVE'
            )
        FROM chats c
        LEFT JOIN topics t
            ON t.chat_id = c.id
           AND t.type = 'GENERAL'
        WHERE c.id = $1
    `

	var chatType string
	var topicID uuid.UUID
	var membersCount int64
	var expectedCount int64

	err := r.pool.QueryRow(ctx, query, chatID, senderID, recipientID).Scan(&chatType, &topicID, &membersCount, &expectedCount)
	if err != nil {
		return message.DirectChat{}, err
	}

	// Наличие строки с ожидаемым ID ещё не доказывает, что это корректный DIRECT-чат для нужной пары.
	if chatType != "DIRECT" || topicID == uuid.Nil() || membersCount != 2 || expectedCount != 2 {
		return message.DirectChat{}, fmt.Errorf("invalid DIRECT chat state: %s", chatID)
	}

	return message.DirectChat{ChatID: chatID, TopicID: topicID}, nil
}

// createDirectChat создаёт чат, GENERAL-топик и двух участников в одной PostgreSQL-транзакции.
func (r *ChatRepository) createDirectChat(ctx context.Context, chatID, senderID, recipientID uuid.UUID) (message.DirectChat, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return message.DirectChat{}, fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		_ = tx.Rollback(ctx)
	}()

	const checkRecipientQuery = `
        SELECT EXISTS (
            SELECT 1
            FROM users
            WHERE id = $1
        )
    `

	var recipientExists bool

	err = tx.QueryRow(ctx, checkRecipientQuery, recipientID).Scan(&recipientExists)
	if err != nil {
		return message.DirectChat{}, fmt.Errorf("failed to check recipient: %w", err)
	}

	if !recipientExists {
		return message.DirectChat{}, message.ErrRecipientNotFound
	}

	topicID := uuid.NewV4()
	now := time.Now().UTC()

	const insertChatQuery = `
        INSERT INTO chats (
            id,
            type,
            title,
            topics_enabled,
            history_visibility,
            created_by,
            created_at
        )
        VALUES ($1, 'DIRECT', NULL, FALSE, 'VISIBLE', $2, $3)
        ON CONFLICT (id) DO NOTHING
        RETURNING id
    `

	var insertedID uuid.UUID

	err = tx.QueryRow(ctx, insertChatQuery, chatID, senderID, now).Scan(&insertedID)
	if errors.Is(err, pgx.ErrNoRows) {
		if rollbackErr := tx.Rollback(ctx); rollbackErr != nil {
			return message.DirectChat{}, fmt.Errorf("failed to rollback after chat conflict: %w", rollbackErr)
		}

		return r.findDirectChat(ctx, chatID, senderID, recipientID)
	}

	if err != nil {
		return message.DirectChat{}, fmt.Errorf("failed to insert direct chat: %w", err)
	}

	// Создаём обязательный GENERAL-топик.
	const insertTopicQuery = `
        INSERT INTO topics (
            id,
            chat_id,
            type,
            title,
            created_by,
            created_at
        )
        VALUES ($1, $2, 'GENERAL', 'General', $3, $4)
    `

	_, err = tx.Exec(ctx, insertTopicQuery, topicID, chatID, senderID, now)
	if err != nil {
		return message.DirectChat{}, fmt.Errorf("insert general topic: %w", err)
	}

	// Оба участника DIRECT-чата имеют роль MEMBER.
	const insertMembersQuery = `
        INSERT INTO chat_members (
            chat_id,
            user_id,
            role,
            status,
            joined_at
        )
        VALUES
            ($1, $2, 'MEMBER', 'ACTIVE', $4),
            ($1, $3, 'MEMBER', 'ACTIVE', $4)
    `

	_, err = tx.Exec(ctx, insertMembersQuery, chatID, senderID, recipientID, now)
	if err != nil {
		return message.DirectChat{}, fmt.Errorf("failed to insert direct chat members: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return message.DirectChat{}, fmt.Errorf("failed to commit direct chat: %w", err)
	}

	return message.DirectChat{ChatID: chatID, TopicID: topicID}, nil
}

func generateDirectChatID(userA, userB uuid.UUID) uuid.UUID {
	// Сортируем UUID
	if bytes.Compare(userA[:], userB[:]) > 0 {
		userA, userB = userB, userA
	}

	// userA — namespace, userB — name.
	hash := sha1.New()
	_, _ = hash.Write(userA[:])
	_, _ = hash.Write(userB[:])

	sum := hash.Sum(nil)

	var id uuid.UUID
	copy(id[:], sum[:16])

	// UUID version 5.
	id[6] = (id[6] & 0x0f) | 0x50

	// UUID variant RFC 9562.
	id[8] = (id[8] & 0x3f) | 0x80

	return id
}
