package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"
	"uuid"

	"github.com/frimo-dev/frimo-messenger/internal/service/auth"
	"github.com/jackc/pgx/v5"
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

func (r *SessionRepository) GetSessionState(ctx context.Context, sessionID uuid.UUID) (auth.SessionState, error) {
	const query = `
		SELECT revoked_at, expires_at
		FROM auth_sessions
		WHERE id = $1
	`

	var sessionState auth.SessionState

	err := r.pool.QueryRow(ctx, query, sessionID).Scan(&sessionState.RevokedAt, &sessionState.ExpiresAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return auth.SessionState{}, auth.ErrSessionNotFound
		}
		return auth.SessionState{}, fmt.Errorf("failed to select session state: %w", err)
	}
	return sessionState, nil
}

func (r *SessionRepository) ExtendSession(ctx context.Context, input auth.ExtendSessionInput) (auth.Identity, error) {
	now := input.NewRefreshToken.CreatedAt.UTC()

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return auth.Identity{}, fmt.Errorf("failed to create transaction: %w", err)
	}

	defer func() {
		_ = tx.Rollback(ctx)
	}()

	const querySelectToken = `
		SELECT id, session_id, used_at, request_id
		FROM refresh_tokens
		WHERE token_hash = $1
		FOR UPDATE
	`

	var tokenID uuid.UUID
	var sessionID uuid.UUID
	var usedAt *time.Time
	var requestID *uuid.UUID

	err = tx.QueryRow(ctx, querySelectToken, input.OldRefreshTokenHash).Scan(&tokenID, &sessionID, &usedAt, &requestID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return auth.Identity{}, auth.ErrRefreshTokenNotFound
		}

		return auth.Identity{}, fmt.Errorf("failed to select refresh token: %w", err)
	}

	const querySelectSession = `
		SELECT user_id, expires_at, revoked_at
		FROM auth_sessions
		WHERE id = $1
		FOR UPDATE
	`

	var userID uuid.UUID
	var expiresAt time.Time
	var revokedAt *time.Time

	err = tx.QueryRow(ctx, querySelectSession, sessionID).Scan(&userID, &expiresAt, &revokedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return auth.Identity{}, auth.ErrSessionNotFound
		}

		return auth.Identity{}, fmt.Errorf("failed to select session: %w", err)
	}

	if usedAt != nil {
		if requestID != nil && *requestID == input.OperationID {
			return auth.Identity{UserID: userID, SessionID: sessionID}, auth.ErrRefreshRetry
		}

		const queryRevokeSession = `
			UPDATE auth_sessions
			SET revoked_at = $1
			WHERE id = $2
		`

		_, err = tx.Exec(ctx, queryRevokeSession, now, sessionID)
		if err != nil {
			return auth.Identity{}, fmt.Errorf("failed to revoke session: %w", err)
		}

		err = tx.Commit(ctx)
		if err != nil {
			return auth.Identity{}, fmt.Errorf("failed to commit transaction: %w", err)
		}

		return auth.Identity{UserID: userID, SessionID: sessionID}, auth.ErrRefreshTokenReuse
	}

	if revokedAt != nil || !expiresAt.After(now) {
		return auth.Identity{}, auth.ErrSessionInactive
	}

	const queryUpdateRefreshToken = `
		UPDATE refresh_tokens
		SET used_at = $1, request_id = $2
		WHERE id = $3
	`

	_, err = tx.Exec(ctx, queryUpdateRefreshToken, now, input.OperationID, tokenID)
	if err != nil {
		return auth.Identity{}, fmt.Errorf("failed to update refresh token: %w", err)
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
		input.NewRefreshToken.ID,
		sessionID,
		input.NewRefreshToken.TokenHash,
		input.NewRefreshToken.CreatedAt,
	)
	if err != nil {
		return auth.Identity{}, fmt.Errorf("failed to insert refresh token: %w", err)
	}

	const queryUpdateSession = `
		UPDATE auth_sessions
		SET expires_at = $1
		WHERE id = $2
	`

	_, err = tx.Exec(ctx, queryUpdateSession, now.Add(input.SessionLifetime), sessionID)
	if err != nil {
		return auth.Identity{}, fmt.Errorf("failed to update session: %w", err)
	}

	err = tx.Commit(ctx)
	if err != nil {
		return auth.Identity{}, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return auth.Identity{UserID: userID, SessionID: sessionID}, nil
}
