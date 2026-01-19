package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"time"

	"github.com/aelhady03/sumflow/totalizer/internal/dedup"
	"github.com/aelhady03/sumflow/totalizer/internal/storage"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	kafka "github.com/segmentio/kafka-go"
)

// Event represents a Kafka message from the outbox
type Event struct {
	EventID       uuid.UUID       `json:"event_id"`
	AggregateType string          `json:"aggregate_type"`
	AggregateID   string          `json:"aggregate_id"`
	EventType     string          `json:"event_type"`
	Payload       json.RawMessage `json:"payload"`
	CreatedAt     time.Time       `json:"created_at"`
}

// SumCalculatedPayload represents the payload for sum.calculated events
type SumCalculatedPayload struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Result int `json:"result"`
}

type ConsumerConfig struct {
	Brokers []string
	Topic   string
	GroupID string
}

type Consumer struct {
	reader    *kafka.Reader
	pool      *pgxpool.Pool
	dedupRepo *dedup.Repository
	storage   *storage.PostgresStorage
	stopCh    chan struct{}
}

func NewConsumer(cfg ConsumerConfig, pool *pgxpool.Pool, dedupRepo *dedup.Repository, storage *storage.PostgresStorage) *Consumer {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        cfg.Brokers,
		Topic:          cfg.Topic,
		GroupID:        cfg.GroupID,
		MinBytes:       10e3, // 10KB
		MaxBytes:       10e6, // 10MB
		CommitInterval: time.Second,
		StartOffset:    kafka.FirstOffset,
	})

	return &Consumer{
		reader:    reader,
		pool:      pool,
		dedupRepo: dedupRepo,
		storage:   storage,
		stopCh:    make(chan struct{}),
	}
}

// Start begins consuming messages
func (c *Consumer) Start(ctx context.Context) {
	go c.consumeLoop(ctx)
}

// Stop signals the consumer to stop
func (c *Consumer) Stop() error {
	close(c.stopCh)
	return c.reader.Close()
}

func (c *Consumer) consumeLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-c.stopCh:
			return
		default:
			msg, err := c.reader.FetchMessage(ctx)
			if err != nil {
				if errors.Is(err, context.Canceled) {
					return
				}
				log.Printf("error fetching message: %v", err)
				continue
			}

			if err := c.processMessage(ctx, msg); err != nil {
				log.Printf("error processing message: %v", err)
				// Continue processing - don't commit the message so it will be retried
				continue
			}

			if err := c.reader.CommitMessages(ctx, msg); err != nil {
				log.Printf("error committing message: %v", err)
			}
		}
	}
}

func (c *Consumer) processMessage(ctx context.Context, msg kafka.Message) error {
	var event Event
	if err := json.Unmarshal(msg.Value, &event); err != nil {
		log.Printf("error unmarshaling event: %v", err)
		return nil // Skip malformed messages
	}

	// Start transaction
	tx, err := c.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Check idempotency and mark as processed
	err = c.dedupRepo.CheckAndMarkInTx(ctx, tx, event.EventID, event.AggregateType, event.EventType)
	if errors.Is(err, dedup.ErrEventAlreadyProcessed) {
		log.Printf("event %s already processed, skipping", event.EventID)
		return nil // Already processed, skip
	}
	if err != nil {
		return err
	}

	// Process the event based on type
	if err := c.handleEvent(ctx, tx, &event); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (c *Consumer) handleEvent(ctx context.Context, tx pgx.Tx, event *Event) error {
	switch event.EventType {
	case "sum.calculated":
		return c.handleSumCalculated(ctx, tx, event)
	default:
		log.Printf("unknown event type: %s", event.EventType)
		return nil
	}
}

func (c *Consumer) handleSumCalculated(ctx context.Context, tx pgx.Tx, event *Event) error {
	var payload SumCalculatedPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return err
	}

	log.Printf("processing sum.calculated event: %d + %d = %d", payload.X, payload.Y, payload.Result)

	return c.storage.AddToTotalInTx(ctx, tx, payload.Result)
}