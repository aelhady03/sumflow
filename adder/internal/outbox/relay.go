package outbox

import (
	"context"
	"log"
	"time"
)

type Publisher interface {
	PublishEvent(ctx context.Context, event *Event) error
}

type RelayConfig struct {
	PollInterval     time.Duration
	BatchSize        int
	MaxRetries       int
	CleanupInterval  time.Duration
	RetentionPeriod  time.Duration
}

func DefaultRelayConfig() RelayConfig {
	return RelayConfig{
		PollInterval:     100 * time.Millisecond,
		BatchSize:        100,
		MaxRetries:       5,
		CleanupInterval:  time.Hour,
		RetentionPeriod:  7 * 24 * time.Hour, // 7 days
	}
}

type Relay struct {
	repo      *Repository
	publisher Publisher
	config    RelayConfig
	stopCh    chan struct{}
}

func NewRelay(repo *Repository, publisher Publisher, config RelayConfig) *Relay {
	return &Relay{
		repo:      repo,
		publisher: publisher,
		config:    config,
		stopCh:    make(chan struct{}),
	}
}

// Start begins the relay background processing
func (r *Relay) Start(ctx context.Context) {
	go r.runPublishLoop(ctx)
	go r.runCleanupLoop(ctx)
}

// Stop signals the relay to stop processing
func (r *Relay) Stop() {
	close(r.stopCh)
}

func (r *Relay) runPublishLoop(ctx context.Context) {
	ticker := time.NewTicker(r.config.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-r.stopCh:
			return
		case <-ticker.C:
			if err := r.processBatch(ctx); err != nil {
				log.Printf("outbox relay error: %v", err)
			}
		}
	}
}

func (r *Relay) processBatch(ctx context.Context) error {
	events, err := r.repo.FetchUnpublished(ctx, r.config.BatchSize)
	if err != nil {
		return err
	}

	for _, event := range events {
		if event.RetryCount >= r.config.MaxRetries {
			log.Printf("outbox event %s exceeded max retries, skipping", event.ID)
			continue
		}

		if err := r.publisher.PublishEvent(ctx, event); err != nil {
			log.Printf("failed to publish event %s: %v", event.ID, err)
			if markErr := r.repo.MarkFailed(ctx, event.ID, err.Error()); markErr != nil {
				log.Printf("failed to mark event as failed: %v", markErr)
			}
			continue
		}

		if err := r.repo.MarkPublished(ctx, event.ID); err != nil {
			log.Printf("failed to mark event as published: %v", err)
		}
	}

	return nil
}

func (r *Relay) runCleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(r.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-r.stopCh:
			return
		case <-ticker.C:
			deleted, err := r.repo.CleanupOldEvents(ctx, r.config.RetentionPeriod)
			if err != nil {
				log.Printf("outbox cleanup error: %v", err)
			} else if deleted > 0 {
				log.Printf("outbox cleanup: deleted %d old events", deleted)
			}
		}
	}
}