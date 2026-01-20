package outbox

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

const (
	AggregateTypeSum   = "sum"
	EventTypeSumCalculated = "sum.calculated"
)

type Event struct {
	ID            uuid.UUID       `json:"event_id"`
	AggregateType string          `json:"aggregate_type"`
	AggregateID   string          `json:"aggregate_id"`
	EventType     string          `json:"event_type"`
	Payload       json.RawMessage `json:"payload"`
	CreatedAt     time.Time       `json:"created_at"`
	PublishedAt   *time.Time      `json:"published_at,omitempty"`
	RetryCount    int             `json:"-"`
	LastError     *string         `json:"-"`
}

type SumCalculatedPayload struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Result int `json:"result"`
}

func NewSumCalculatedEvent(x, y, result int) (*Event, error) {
	payload := SumCalculatedPayload{
		X:      x,
		Y:      y,
		Result: result,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	eventID := uuid.New()

	return &Event{
		ID:            eventID,
		AggregateType: AggregateTypeSum,
		AggregateID:   eventID.String(),
		EventType:     EventTypeSumCalculated,
		Payload:       payloadBytes,
		CreatedAt:     time.Now().UTC(),
	}, nil
}

// ToJSON converts the event to JSON for publishing to Kafka
func (e *Event) ToJSON() ([]byte, error) {
	return json.Marshal(e)
}