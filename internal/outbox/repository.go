package outbox

import (
	"context"
	"errors"
	"time"
)

var ErrNoEvents = errors.New("no outbox events")

type Repository interface {
	// ClaimNext находит следующее готовое событие и закрепляет его за этим worker’ом(атомарная операция)
	ClaimNext(ctx context.Context, now time.Time, lockExpiredBefore time.Time) (Event, error)

	// MarkProcessed помечает событие успешно обработаным, после этого событие больше не выбирается worker’ами
	MarkProcessed(ctx context.Context, eventID string, processedAt time.Time) error

	// ScheduleRetry помечает событие доступным через определенный промежуток времени
	ScheduleRetry(ctx context.Context, eventID string, availableAt time.Time, lastError string) error

	// MarkFailed помечает событие не выполняемым
	MarkFailed(ctx context.Context, eventID string, failedAt time.Time, lastError string) error
}
