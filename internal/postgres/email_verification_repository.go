package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	emailverification2 "github.com/frimo-dev/frimo-messenger/internal/service/emailverification"
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
		return fmt.Errorf("begin email verification transaction: %w", err)
	}

	defer func() {
		// Если изменения были закомичены, то Rollback просто вернет ошибку
		_ = tx.Rollback(ctx)
	}()

	const selectQuery = `
		SELECT expires_at, used_at
		FROM email_verifications
		WHERE token_hash = $1
		FOR UPDATE`

	var expiresAt time.Time
	var usedAt *time.Time

	err = tx.QueryRow(ctx, selectQuery, tokenHash).Scan(&expiresAt, &usedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return emailverification2.ErrInvalidToken
		}

		return fmt.Errorf("select verification token: %w", err)
	}

	if usedAt != nil {
		return emailverification2.ErrUsedToken
	}

	if !confirmedAt.Before(expiresAt) {
		return emailverification2.ErrExpiredToken
	}

	const updateVerificationQuery = `
		UPDATE email_verifications
		SET
			used_at = $1,
			token_ciphertext = NULL
		WHERE token_hash = $2`

	if _, err := tx.Exec(ctx, updateVerificationQuery, confirmedAt, tokenHash); err != nil {
		return fmt.Errorf("update user email verification: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit email verification transaction: %w", err)
	}

	return nil
}

func (r *EmailVerificationRepository) GetForDelivery(ctx context.Context, verificationID string) (emailverification2.DeliveryData, error) {
	const query = `
		SELECT
			id,
			token_ciphertext,
			expires_at,
			used_at
		FROM email_verifications
		WHERE id = $1
	`
	var data emailverification2.DeliveryData

	err := r.pool.QueryRow(ctx, query, verificationID).Scan(&data.ID, &data.TokenCiphertext, &data.ExpiresAt, &data.UsedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return emailverification2.DeliveryData{}, emailverification2.ErrDeliveryNotFound
		}
		return emailverification2.DeliveryData{}, fmt.Errorf("get email verification for delivery: %w", err)
	}

	if data.UsedAt != nil || len(data.TokenCiphertext) == 0 {
		return emailverification2.DeliveryData{}, emailverification2.ErrDeliveryInactive
	}

	return data, nil
}

var _ emailverification2.Repository = (*EmailVerificationRepository)(nil)
