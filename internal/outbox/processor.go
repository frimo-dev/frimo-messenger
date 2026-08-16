package outbox

import (
	"context"
	"errors"
	"math/rand"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type Handler interface {
	Handle(ctx context.Context, event Event) error
}

type Clock func() time.Time

var ErrNoEvents = errors.New("no outbox dto")

type Repository interface {
	// ClaimNext находит следующее готовое событие и закрепляет его за этим worker’ом(атомарная операция)
	ClaimNext(ctx context.Context, lockID string, now time.Time, lockExpiredBefore time.Time) (Event, error)

	// MarkProcessed помечает событие успешно обработанным, после этого событие больше не выбирается worker’ами
	MarkProcessed(ctx context.Context, eventID string, lockID string, processedAt time.Time) error

	// ScheduleRetry помечает событие доступным через определенный промежуток времени
	ScheduleRetry(ctx context.Context, eventID string, lockID string, availableAt time.Time, lastError string) error

	// MarkFailed помечает событие не выполняемым
	MarkFailed(ctx context.Context, eventID string, lockID string, failedAt time.Time, lastError string) error
}

type Processor struct {
	repository Repository
	handler    Handler
	logger     *zap.Logger

	workTimeout  time.Duration
	pollInterval time.Duration
	lockLease    time.Duration
	maxAttempts  int
	now          Clock
}

func NewProcessor(repository Repository, handler Handler, logger *zap.Logger, now Clock, workTimeout time.Duration) *Processor {
	if now == nil {
		now = time.Now
	}

	return &Processor{
		repository:   repository,
		handler:      handler,
		logger:       logger,
		workTimeout:  workTimeout,
		pollInterval: 2 * time.Second,
		lockLease:    time.Minute,
		maxAttempts:  10,
		now:          now,
	}
}

func (p *Processor) Run(ctx context.Context) error {
	timer := time.NewTimer(0)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-timer.C:
			processed, err := p.processOne(ctx)
			if err != nil {
				if errors.Is(err, context.Canceled) && ctx.Err() != nil {
					return nil
				}
				p.logger.Error("failed process outbox event", zap.Error(err))
			}

			if processed {
				timer.Reset(0)
			} else {
				timer.Reset(p.pollInterval)
			}
		}
	}
}

func (p *Processor) processOne(ctx context.Context) (bool, error) {
	now := p.now().UTC()
	lockID := uuid.NewString()

	event, err := p.repository.ClaimNext(ctx, lockID, now, now.Add(-p.lockLease))
	if err != nil {
		if errors.Is(err, ErrNoEvents) {
			return false, nil
		}

		return false, err
	}

	workCtx, cancel := context.WithTimeout(context.Background(), p.workTimeout)
	defer cancel()

	handleErr := p.handler.Handle(workCtx, event)
	if handleErr != nil {
		if errors.Is(handleErr, ErrNonRetryable) || event.Attempts >= p.maxAttempts {
			markErr := p.repository.MarkFailed(workCtx, event.ID, lockID, p.now().UTC(), handleErr.Error())
			if markErr != nil {
				return true, errors.Join(handleErr, markErr)
			}

			p.logger.Error(
				"outbox event permanently failed",
				zap.Error(handleErr),
				zap.String("event_id", event.ID),
				zap.String("event_type", event.Type),
				zap.Int("attempts", event.Attempts),
			)

			return true, nil
		}

		retryAt := p.now().UTC().Add(retryDelay(event.Attempts))

		markErr := p.repository.ScheduleRetry(workCtx, event.ID, lockID, retryAt, handleErr.Error())
		if markErr != nil {
			return true, errors.Join(handleErr, markErr)
		}

		return true, nil
	}

	if err := p.repository.MarkProcessed(workCtx, event.ID, lockID, p.now().UTC()); err != nil {
		return true, err
	}

	return true, nil
}

func retryDelay(attempt int) time.Duration {
	base := retryBaseDelay(attempt)

	const jitter = 0.25

	minDelay := float64(base) * (1 - jitter)
	spread := float64(base) * (2 * jitter)

	return time.Duration(minDelay + rand.Float64()*spread)
}

func retryBaseDelay(attempt int) time.Duration {
	switch {
	case attempt <= 1:
		return 5 * time.Second
	case attempt == 2:
		return 15 * time.Second
	case attempt == 3:
		return 30 * time.Second
	case attempt == 4:
		return time.Minute
	default:
		return 5 * time.Minute
	}
}
