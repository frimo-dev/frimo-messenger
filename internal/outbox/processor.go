package outbox

import (
	"context"
	"errors"
	"time"

	"go.uber.org/zap"
)

type Handler interface {
	Handle(ctx context.Context, event Event) error
}

type Clock func() time.Time

type Processor struct {
	repository Repository
	handler    Handler
	logger     *zap.Logger

	pollInterval time.Duration
	lockLease    time.Duration
	maxAttempts  int
	now          Clock
}

func NewProcessor(repository Repository, handler Handler, logger *zap.Logger, now Clock) *Processor {
	if now == nil {
		now = time.Now
	}

	return &Processor{
		repository:   repository,
		handler:      handler,
		logger:       logger,
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
				p.logger.Error("process outbox event", zap.Error(err))
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

	event, err := p.repository.ClaimNext(ctx, now, now.Add(-p.lockLease))
	if err != nil {
		if errors.Is(err, ErrNoEvents) {
			return false, nil
		}

		return false, err
	}

	handleErr := p.handler.Handle(ctx, event)
	if handleErr != nil {
		if event.Attempts >= p.maxAttempts {
			markErr := p.repository.MarkFailed(ctx, event.ID, p.now().UTC(), handleErr.Error())
			if markErr != nil {
				return true, errors.Join(handleErr, markErr)
			}

			p.logger.Error(
				"outbox event permanently failed",
				zap.String("event_id", event.ID),
				zap.String("event_type", event.Type),
				zap.Int("attempts", event.Attempts),
				zap.Error(handleErr),
			)

			return true, nil
		}

		retryAt := p.now().UTC().Add(retryDelay(event.Attempts))

		markErr := p.repository.ScheduleRetry(ctx, event.ID, retryAt, handleErr.Error())
		if markErr != nil {
			return true, errors.Join(handleErr, markErr)
		}

		return true, nil
	}

	if err := p.repository.MarkProcessed(ctx, event.ID, p.now().UTC()); err != nil {
		return true, err
	}

	return true, nil
}

func retryDelay(attempt int) time.Duration {
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
