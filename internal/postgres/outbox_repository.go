package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/frimo-dev/frimo-messenger/internal/outbox"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type OutboxRepository struct {
	pool *pgxpool.Pool
}

func NewOutboxRepository(pool *pgxpool.Pool) *OutboxRepository {
	return &OutboxRepository{pool: pool}
}

func (r *OutboxRepository) ClaimNext(ctx context.Context, lockID string, now time.Time, lockExpiredBefore time.Time) (outbox.Event, error) {
	const query = `
		WITH candidate AS (
			SELECT id
			FROM outbox_events
			WHERE processed_at IS NULL
				AND failed_at IS NULL
				AND available_at <= $2
				AND (
					locked_at IS NULL
					OR locked_at < $3
				)
			ORDER BY available_at, created_at
			FOR UPDATE SKIP LOCKED
			LIMIT 1
		)
		UPDATE outbox_events AS event
		SET
			lock_id = $1,
			locked_at = $2,
			attempts = attempts + 1,
			last_error = NULL
		FROM candidate
		WHERE event.id = candidate.id
		RETURNING
			event.id,
			event.event_type,
			event.payload,
			event.created_at,
			event.available_at,
			event.attempts
	`

	var event outbox.Event

	err := r.pool.QueryRow(ctx, query, lockID, now, lockExpiredBefore).Scan(
		&event.ID,
		&event.Type,
		&event.Payload,
		&event.CreatedAt,
		&event.AvailableAt,
		&event.Attempts,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return outbox.Event{}, outbox.ErrNoEvents
		}

		return outbox.Event{}, fmt.Errorf("claim next outbox event: %w", err)
	}

	return event, nil
}

func (r *OutboxRepository) MarkProcessed(ctx context.Context, eventID string, lockID string, processedAt time.Time) error {
	const query = `
		UPDATE outbox_events
		SET
			processed_at = $3,
			locked_at = NULL,
			lock_id = NULL,
			last_error = NULL,
			payload = '{}'::jsonb
		WHERE
			id = $1
		  	AND lock_id = $2
			AND processed_at IS NULL
	`

	commandTag, err := r.pool.Exec(ctx, query, eventID, lockID, processedAt)
	if err != nil {
		return fmt.Errorf("mark outbox event processed: %w", err)
	}

	if commandTag.RowsAffected() != 1 {
		return fmt.Errorf("mark outbox event processed: %w", outbox.ErrLeaseLost)
	}

	return nil
}

func (r *OutboxRepository) ScheduleRetry(ctx context.Context, eventID string, lockID string, availableAt time.Time, lastError string) error {
	const query = `
		UPDATE outbox_events
		SET
			locked_at = NULL,
			lock_id = NULL,
			available_at = $3,
			last_error = LEFT($4, 1000)
		WHERE
		    id = $1
		  	AND lock_id = $2
			AND processed_at IS NULL
			AND failed_at IS NULL
	`
	commandTag, err := r.pool.Exec(ctx, query, eventID, lockID, availableAt, lastError)
	if err != nil {
		return fmt.Errorf("schedule outbox retry: %w", err)
	}

	if commandTag.RowsAffected() != 1 {
		return fmt.Errorf("schedule outbox retry: %w", outbox.ErrLeaseLost)
	}

	return nil
}

func (r *OutboxRepository) MarkFailed(ctx context.Context, eventID string, lockID string, failedAt time.Time, lastError string) error {
	const query = `
		UPDATE outbox_events
		SET
		    locked_at = NULL,
			lock_id = NULL,
			failed_at = $3,
			last_error = LEFT($4, 1000)
		WHERE
		    id = $1
		  	AND lock_id = $2
			AND processed_at IS NULL
	`

	commandTag, err := r.pool.Exec(ctx, query, eventID, lockID, failedAt, lastError)
	if err != nil {
		return fmt.Errorf("mark outbox event failed: %w", err)
	}

	if commandTag.RowsAffected() != 1 {
		return fmt.Errorf("mark outbox event failed: %w", outbox.ErrLeaseLost)
	}

	return nil
}

var _ outbox.Repository = (*OutboxRepository)(nil)
