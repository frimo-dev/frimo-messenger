package outbox

import (
	"encoding/json"
	"time"
	"uuid"
)

type Event struct {
	ID          uuid.UUID
	Type        string
	Payload     json.RawMessage
	CreatedAt   time.Time
	AvailableAt time.Time
	Attempts    int
}
