package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/frimo-dev/frimo-messenger/internal/emailverification"
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
			return emailverification.ErrInvalidToken
		}

		return fmt.Errorf("select verification token: %w", err)
	}

	if usedAt != nil {
		return emailverification.ErrUsedToken
	}

	if !confirmedAt.Before(expiresAt) {
		return emailverification.ErrExpiredToken
	}

	const updateVerificationQuery = `
		UPDATE email_verifications
		SET used_at = $1
		WHERE token_hash = $2`

	if _, err := tx.Exec(ctx, updateVerificationQuery, confirmedAt, tokenHash); err != nil {
		return fmt.Errorf("update user email verification: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit email verification transaction: %w", err)
	}

	return nil
}

var _ emailverification.Repository = (*EmailVerificationRepository)(nil)
