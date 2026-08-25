package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"
	"uuid"

	"github.com/frimo-dev/frimo-messenger/internal/service/auth"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Стандартный код ошибки PostgreSQL, означающий попытку нарушить уникальность ключа
const uniqueViolationCode = "23505"

type AuthRepository struct {
	pool *pgxpool.Pool
}

func NewAuthRepository(pool *pgxpool.Pool) *AuthRepository {
	return &AuthRepository{pool: pool}
}

func (r *AuthRepository) CreateUser(ctx context.Context, input auth.CreateUserInput) (auth.User, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return auth.User{}, fmt.Errorf("create transaction: %w", err)
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
			return auth.User{}, auth.ErrEmailAlreadyExists
		}

		return auth.User{}, fmt.Errorf("insert user: %w", err)
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

	_, err = tx.Exec(ctx, insertVerificationQuery, input.Verification.ID, input.ID, input.Verification.TokenHash, input.Verification.TokenCiphertext, input.Verification.ExpiresAt, input.CreatedAt)
	if err != nil {
		return auth.User{}, fmt.Errorf("insert verification: %w", err)
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
		input.Verification.OutboxEvent.ID,
		input.Verification.OutboxEvent.Type,
		input.Verification.OutboxEvent.Payload,
		input.Verification.OutboxEvent.CreatedAt,
		input.Verification.OutboxEvent.AvailableAt,
	)
	if err != nil {
		return auth.User{}, fmt.Errorf("insert outbox event: %w", err)
	}

	err = tx.Commit(ctx)
	if err != nil {
		return auth.User{}, fmt.Errorf("commit transaction: %w", err)
	}

	return auth.User{ID: input.ID, Email: input.Email, CreatedAt: input.CreatedAt}, nil
}

func (r *AuthRepository) ConfirmEmail(ctx context.Context, tokenHash []byte, confirmedAt time.Time) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed begin email verification transaction: %w", err)
	}

	defer tx.Rollback(ctx)

	const selectUserIDQuery = `
		SELECT user_id
		FROM email_verifications
		WHERE token_hash = $1
	`

	var userID uuid.UUID

	err = tx.QueryRow(ctx, selectUserIDQuery, tokenHash).Scan(&userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return auth.ErrInvalidToken
		}

		return fmt.Errorf("failed select verification token: %w", err)
	}

	const selectUserVerificationQuery = `
		SELECT email_verified_at
		FROM users
		WHERE id = $1
		FOR UPDATE;
	`

	var emailVerifiedAt *time.Time

	err = tx.QueryRow(ctx, selectUserVerificationQuery, userID).Scan(&emailVerifiedAt)
	if err != nil {
		return fmt.Errorf("failed select verification time: %w", err)
	}

	const selectQuery = `
		SELECT expires_at, used_at, revoked_at
		FROM email_verifications
		WHERE token_hash = $1
			AND user_id = $2
		FOR UPDATE;
	`

	var expiresAt time.Time
	var usedAt *time.Time
	var revokedAt *time.Time

	err = tx.QueryRow(ctx, selectQuery, tokenHash, userID).Scan(&expiresAt, &usedAt, &revokedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return auth.ErrInvalidToken
		}

		return fmt.Errorf("failed select verification token: %w", err)
	}

	if usedAt != nil {
		return auth.ErrUsedToken
	}

	if revokedAt != nil {
		return auth.ErrRevokedToken
	}

	if !confirmedAt.Before(expiresAt) {
		return auth.ErrExpiredToken
	}

	if emailVerifiedAt != nil {
		return auth.ErrAlreadyVerified
	}

	const updateVerificationQuery = `
		UPDATE email_verifications
		SET
			used_at = $1,
			token_ciphertext = NULL
		WHERE token_hash = $2
	`

	if _, err := tx.Exec(ctx, updateVerificationQuery, confirmedAt, tokenHash); err != nil {
		return fmt.Errorf("failed update user email verification: %w", err)
	}

	const updateUserQuery = `
		UPDATE users
		SET email_verified_at = $1
		WHERE id = $2
	`

	commandTag, err := tx.Exec(ctx, updateUserQuery, confirmedAt, userID)
	if err != nil {
		return fmt.Errorf("failed mark user email verified: %w", err)
	}

	if commandTag.RowsAffected() != 1 {
		return fmt.Errorf("failed mark user email verified: user not found")
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed commit email verification transaction: %w", err)
	}

	return nil
}

// ResendVerification TODO: вынести лимиты cooldown в config
func (r *AuthRepository) ResendVerification(ctx context.Context, input auth.ResendVerificationInput) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed begin resend verification token transaction: %w", err)
	}

	defer tx.Rollback(ctx)

	const selectUserQuery = `
		SELECT id, email_verified_at
		FROM users
		WHERE email = $1
		FOR UPDATE;
	`
	var userID uuid.UUID
	var emailVerifiedAt *time.Time

	if err := tx.QueryRow(ctx, selectUserQuery, input.Email).Scan(&userID, &emailVerifiedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return auth.ErrUserNotFound
		}

		return fmt.Errorf("failed select user for resend verification token: %w", err)
	}

	if emailVerifiedAt != nil {
		return auth.ErrAlreadyVerified
	}

	const selectLastHourCreatedQuery = `
		SELECT COUNT(*)
		FROM email_verifications
		WHERE user_id = $1
			AND created_at >= $2 - INTERVAL '1 hour'
			AND created_at <= $2;
	`

	var count int

	if err := tx.QueryRow(ctx, selectLastHourCreatedQuery, userID, input.RequestedAt).Scan(&count); err != nil {
		return fmt.Errorf("failed counting verification tokens: %w", err)
	}

	if count >= 5 {
		return auth.ErrResendHourlyLimit
	}

	// MAX() - всегда возвращает одну строку, но если не нашлось записи, то значение будет NULL
	const selectLastCreatedQuery = `
		SELECT MAX(created_at) AS last_created_at
		FROM email_verifications
		WHERE user_id = $1;
	`

	var createdAt *time.Time

	if err := tx.QueryRow(ctx, selectLastCreatedQuery, userID).Scan(&createdAt); err != nil {
		return fmt.Errorf("failed selecting last created verification token: %w", err)
	}

	if createdAt != nil && input.RequestedAt.Sub(*createdAt) < time.Minute {
		return auth.ErrResendCooldown
	}

	const updateEmailVerificationQuery = `
		UPDATE email_verifications
		SET revoked_at = $2
		WHERE
		    user_id = $1
			AND used_at IS NULL
		  	AND revoked_at IS NULL
			AND expires_at > $2
	`

	if _, err := tx.Exec(ctx, updateEmailVerificationQuery, userID, input.RequestedAt); err != nil {
		return fmt.Errorf("failed revoke email verification: %w", err)
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

	_, err = tx.Exec(
		ctx,
		insertVerificationQuery,
		input.Verification.ID,
		userID,
		input.Verification.TokenHash,
		input.Verification.TokenCiphertext,
		input.Verification.ExpiresAt,
		input.RequestedAt,
	)
	if err != nil {
		return fmt.Errorf("failed insert verification: %w", err)
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
		input.Verification.OutboxEvent.ID,
		input.Verification.OutboxEvent.Type,
		input.Verification.OutboxEvent.Payload,
		input.Verification.OutboxEvent.CreatedAt,
		input.Verification.OutboxEvent.AvailableAt,
	)
	if err != nil {
		return fmt.Errorf("failed insert outbox event: %w", err)
	}

	err = tx.Commit(ctx)
	if err != nil {
		return fmt.Errorf("failed commit transaction: %w", err)
	}

	return nil
}

func (r *AuthRepository) GetForDelivery(ctx context.Context, verificationID uuid.UUID) (auth.DeliveryData, error) {
	const query = `
		SELECT
			id,
			token_ciphertext,
			expires_at,
			used_at,
			revoked_at
		FROM email_verifications
		WHERE id = $1
	`
	var data auth.DeliveryData
	var usedAt *time.Time
	var revokedAt *time.Time

	err := r.pool.QueryRow(ctx, query, verificationID).Scan(&data.ID, &data.TokenCiphertext, &data.ExpiresAt, &usedAt, &revokedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return auth.DeliveryData{}, auth.ErrDeliveryNotFound
		}
		return auth.DeliveryData{}, fmt.Errorf("failed to get email verification for delivery: %w", err)
	}

	if usedAt != nil || revokedAt != nil || len(data.TokenCiphertext) == 0 {
		return auth.DeliveryData{}, auth.ErrDeliveryInactive
	}

	return data, nil
}

func isEmailUniqueViolation(err error) bool {
	var postgresError *pgconn.PgError

	if !errors.As(err, &postgresError) {
		return false
	}

	return postgresError.Code == uniqueViolationCode &&
		postgresError.ConstraintName == "users_email_unique_idx"
}

var _ auth.Repository = (*AuthRepository)(nil)
var _ auth.DeliveryRepository = (*AuthRepository)(nil)
