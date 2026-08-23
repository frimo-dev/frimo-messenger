package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

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

	defer func() {
		// Если изменения были закомичены, то Rollback просто вернет ошибку
		_ = tx.Rollback(ctx)
	}()

	const selectQuery = `
		SELECT expires_at, used_at, revoked_at
		FROM email_verifications
		WHERE token_hash = $1
		FOR UPDATE`

	var expiresAt time.Time
	var usedAt *time.Time
	var revokedAt *time.Time

	err = tx.QueryRow(ctx, selectQuery, tokenHash).Scan(&expiresAt, &usedAt, &revokedAt)
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

	const updateVerificationQuery = `
		UPDATE email_verifications
		SET
			used_at = $1,
			token_ciphertext = NULL
		WHERE token_hash = $2`

	if _, err := tx.Exec(ctx, updateVerificationQuery, confirmedAt, tokenHash); err != nil {
		return fmt.Errorf("failed update user email verification: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed commit email verification transaction: %w", err)
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
