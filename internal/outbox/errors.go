package outbox

import (
	"errors"
)

var ErrNonRetryable = errors.New("non-retryable outbox error")
var ErrLeaseLost = errors.New("outbox lease lost")
