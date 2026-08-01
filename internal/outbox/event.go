package outbox

import (
	"encoding/json"
	"time"
)

type Event struct {
	ID          string
	Type        string
	Payload     json.RawMessage
	CreatedAt   time.Time
	AvailableAt time.Time
	Attempts    int
}
