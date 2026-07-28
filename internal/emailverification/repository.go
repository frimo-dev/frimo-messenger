package emailverification

import (
	"context"
	"time"
)

type Repository interface {
	Confirm(ctx context.Context, tokenHash []byte, confirmedAt time.Time) error
}
