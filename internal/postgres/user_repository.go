package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/frimo-dev/frimo-messenger/internal/service/user"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Стандартный код ошибки PostgreSQL, означающий попытку нарушить уникальность ключа
const uniqueViolationCode = "23505"

type UserRepository struct {
	pool *pgxpool.Pool
}

func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{
		pool: pool,
	}
}

func (ur *UserRepository) Create(ctx context.Context, input user.CreationInput) (user.User, error) {
	tx, err := ur.pool.Begin(ctx)
	if err != nil {
		return user.User{}, fmt.Errorf("create transaction: %w", err)
	}

	defer func() {
		_ = tx.Rollback(ctx)
	}()

	const query = `
		INSERT INTO users (
		    id,
		    email,
		    password_hash,
		    created_at
		)
		VALUES ($1, $2, $3, $4)`

	_, err = tx.Exec(
		ctx,
		query,
		input.ID,
		input.Email,
		input.PasswordHash,
		input.CreatedAt,
	)
	if err != nil {
		if isEmailUniqueViolation(err) {
			return user.User{}, user.ErrEmailAlreadyExists
		}

		return user.User{}, fmt.Errorf("insert user: %w", err)
	}

	const insertVerificationQuery = `
		INSERT INTO email_verifications (
		    id,
		    user_id,
		    token_hash,
		    token_ciphertext,
		    expires_at,
		    created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6)`

	_, err = tx.Exec(ctx, insertVerificationQuery, input.VerificationID, input.ID, input.VerificationTokenHash, input.VerificationTokenCiphertext, input.VerificationExpiresAt, input.CreatedAt)
	if err != nil {
		return user.User{}, fmt.Errorf("insert verification: %w", err)
	}

	const insertOutboxQuery = `
		INSERT INTO outbox_events (
			id,
			event_type,
			payload,
			created_at,
			available_at
		)
		VALUES ($1, $2, $3, $4, $5)
	`

	_, err = tx.Exec(
		ctx,
		insertOutboxQuery,
		input.OutboxEvent.ID,
		input.OutboxEvent.Type,
		input.OutboxEvent.Payload,
		input.OutboxEvent.CreatedAt,
		input.OutboxEvent.AvailableAt,
	)
	if err != nil {
		return user.User{}, fmt.Errorf("insert outbox event: %w", err)
	}

	err = tx.Commit(ctx)
	if err != nil {
		return user.User{}, fmt.Errorf("commit transaction: %w", err)
	}

	return user.User{ID: input.ID, Email: input.Email, CreatedAt: input.CreatedAt}, nil
}

func isEmailUniqueViolation(err error) bool {
	var postgresError *pgconn.PgError

	if !errors.As(err, &postgresError) {
		return false
	}

	return postgresError.Code == uniqueViolationCode &&
		postgresError.ConstraintName == "users_email_unique_idx"
}

var _ user.Repository = (*UserRepository)(nil)
