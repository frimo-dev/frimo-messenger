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

func (r *OutboxRepository) ClaimNext(ctx context.Context, now time.Time, lockExpiredBefore time.Time) (outbox.Event, error) {
	const query = `
		WITH candidate AS (
			SELECT id
			FROM outbox_events
			WHERE processed_at IS NULL
				AND failed_at IS NULL
				AND available_at <= $1
				AND (
					locked_at IS NULL
					OR locked_at < $2
				)
			ORDER BY available_at, created_at
			FOR UPDATE SKIP LOCKED
			LIMIT 1
		)
		UPDATE outbox_events AS event
		SET
			locked_at = $1,
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

	err := r.pool.QueryRow(ctx, query, now, lockExpiredBefore).Scan(
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

func (r *OutboxRepository) MarkProcessed(ctx context.Context, eventID string, processedAt time.Time) error {
	const query = `
		UPDATE outbox_events
		SET
			processed_at = $2,
			locked_at = NULL,
			last_error = NULL,
			payload = '{}'::jsonb
		WHERE
			id = $1
			AND processed_at IS NULL
	`

	commandTag, err := r.pool.Exec(ctx, query, eventID, processedAt)
	if err != nil {
		return fmt.Errorf("mark outbox event processed: %w", err)
	}

	if commandTag.RowsAffected() != 1 {
		return fmt.Errorf("mark outbox event processed: unexpected affected rows: %d", commandTag.RowsAffected())
	}

	return nil
}

func (r *OutboxRepository) ScheduleRetry(ctx context.Context, eventID string, availableAt time.Time, lastError string) error {
	const query = `
		UPDATE outbox_events
		SET
			locked_at = NULL,
			available_at = $2,
			last_error = LEFT($3, 1000)
		WHERE id = $1
			AND processed_at IS NULL
			AND failed_at IS NULL
	`
	_, err := r.pool.Exec(ctx, query, eventID, availableAt, lastError)
	if err != nil {
		return fmt.Errorf("schedule outbox retry: %w", err)
	}

	return nil
}

func (r *OutboxRepository) MarkFailed(ctx context.Context, eventID string, failedAt time.Time, lastError string) error {
	const query = `
		UPDATE outbox_events
		SET
			failed_at = $2,
			locked_at = NULL,
			last_error = LEFT($3, 1000)
		WHERE id = $1
			AND processed_at IS NULL
	`

	_, err := r.pool.Exec(ctx, query, eventID, failedAt, lastError)
	if err != nil {
		return fmt.Errorf("mark outbox event failed: %w", err)
	}

	return nil
}

var _ outbox.Repository = (*OutboxRepository)(nil)
