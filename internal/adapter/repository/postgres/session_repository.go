package postgres

import (
	"context"
	"fmt"

	"github.com/frimo-dev/frimo-messenger/internal/service/auth"
	"github.com/jackc/pgx/v5/pgxpool"
)

type SessionRepository struct {
	pool *pgxpool.Pool
}

func NewSessionRepository(pool *pgxpool.Pool) *SessionRepository {
	return &SessionRepository{pool: pool}
}

func (r *SessionRepository) CreateSession(ctx context.Context, session auth.Session, refreshToken auth.RefreshToken) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to create transaction: %w", err)
	}

	defer func() {
		_ = tx.Rollback(ctx)
	}()

	const queryInsertSession = `
		INSERT INTO auth_sessions (
		    id,
		    user_id,
		    device_name,
		    created_ip,
			last_ip,
		    created_at,
		    last_seen_at,
		    expires_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	_, err = tx.Exec(
		ctx,
		queryInsertSession,
		session.ID,
		session.UserID,
		session.DeviceName,
		session.CreatedIP,
		session.LastIP,
		session.CreatedAt,
		session.LastSeenAt,
		session.ExpiresAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert session: %w", err)
	}

	const queryInsertRefreshToken = `
		INSERT INTO refresh_tokens (
		    id,
		    session_id,
		    token_hash,
		    created_at
		)
		VALUES ($1, $2, $3, $4)
	`

	_, err = tx.Exec(
		ctx,
		queryInsertRefreshToken,
		refreshToken.ID,
		session.ID,
		refreshToken.TokenHash,
		refreshToken.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert refresh token: %w", err)
	}

	err = tx.Commit(ctx)
	if err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}
