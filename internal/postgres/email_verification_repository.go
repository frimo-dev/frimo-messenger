package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"
	"uuid"

	"github.com/frimo-dev/frimo-messenger/internal/service/emailverification"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type EmailVerificationRepository struct {
	pool *pgxpool.Pool
}

func NewEmailVerificationRepository(pool *pgxpool.Pool) *EmailVerificationRepository {
	return &EmailVerificationRepository{pool: pool}
}

func (r *EmailVerificationRepository) Confirm(ctx context.Context, tokenHash []byte, confirmedAt time.Time) error {
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
			return emailverification.ErrInvalidToken
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
			return emailverification.ErrInvalidToken
		}

		return fmt.Errorf("failed select verification token: %w", err)
	}

	if usedAt != nil {
		return emailverification.ErrUsedToken
	}

	if revokedAt != nil {
		return emailverification.ErrRevokedToken
	}

	if !confirmedAt.Before(expiresAt) {
		return emailverification.ErrExpiredToken
	}

	if emailVerifiedAt != nil {
		return emailverification.ErrAlreadyVerified
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

// ResendVerificationToken TODO: вынести лимиты cooldown в config
func (r *EmailVerificationRepository) ResendVerificationToken(ctx context.Context, input emailverification.ResendInput) error {
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
			return emailverification.ErrUserNotFound
		}

		return fmt.Errorf("failed select user for resend verification token: %w", err)
	}

	if emailVerifiedAt != nil {
		return emailverification.ErrAlreadyVerified
	}

	const selectLastHourCreatedQuery = `
		SELECT COUNT(*)
		FROM email_verifications
		WHERE user_id = $1
			AND created_at >= $2 - INTERVAL '1 hour'
			AND created_at <= $2;
	`

	var count int

	if err := tx.QueryRow(ctx, selectLastHourCreatedQuery, userID, input.ResendAt).Scan(&count); err != nil {
		return fmt.Errorf("failed counting verification tokens: %w", err)
	}

	if count >= 5 {
		return emailverification.ErrResendHourlyLimit
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

	if createdAt != nil && input.ResendAt.Sub(*createdAt) < time.Minute {
		return emailverification.ErrResendCooldown
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

	if _, err := tx.Exec(ctx, updateEmailVerificationQuery, userID, input.ResendAt); err != nil {
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
		input.VerificationID,
		userID,
		input.VerificationTokenHash,
		input.VerificationTokenCiphertext,
		input.VerificationExpiresAt,
		input.ResendAt,
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
		input.OutboxEvent.ID,
		input.OutboxEvent.Type,
		input.OutboxEvent.Payload,
		input.OutboxEvent.CreatedAt,
		input.OutboxEvent.AvailableAt,
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

func (r *EmailVerificationRepository) GetForDelivery(ctx context.Context, verificationID string) (emailverification.DeliveryData, error) {
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
	var data emailverification.DeliveryData
	var usedAt *time.Time
	var revokedAt *time.Time

	err := r.pool.QueryRow(ctx, query, verificationID).Scan(&data.ID, &data.TokenCiphertext, &data.ExpiresAt, &usedAt, &revokedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return emailverification.DeliveryData{}, emailverification.ErrDeliveryNotFound
		}
		return emailverification.DeliveryData{}, fmt.Errorf("failed to get email verification for delivery: %w", err)
	}

	if usedAt != nil || revokedAt != nil || len(data.TokenCiphertext) == 0 {
		return emailverification.DeliveryData{}, emailverification.ErrDeliveryInactive
	}

	return data, nil
}

var _ emailverification.Repository = (*EmailVerificationRepository)(nil)
var _ emailverification.DeliveryRepository = (*EmailVerificationRepository)(nil)
