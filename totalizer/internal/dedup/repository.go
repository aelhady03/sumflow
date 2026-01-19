package dedup

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrEventAlreadyProcessed = errors.New("event already processed")

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// CheckAndMarkInTx checks if an event has been processed and marks it if not.
// Must be called within a transaction to ensure atomicity.
// Returns ErrEventAlreadyProcessed if the event was already processed.
func (r *Repository) CheckAndMarkInTx(ctx context.Context, tx pgx.Tx, eventID uuid.UUID, aggregateType, eventType string) error {
	// Try to insert the event. If it already exists (duplicate key), the event was already processed.
	query := `
		INSERT INTO processed_events (event_id, aggregate_type, event_type)
		VALUES ($1, $2, $3)
		ON CONFLICT (event_id) DO NOTHING
	`
	result, err := tx.Exec(ctx, query, eventID, aggregateType, eventType)
	if err != nil {
		return err
	}

	// If no rows were affected, the event was already processed
	if result.RowsAffected() == 0 {
		return ErrEventAlreadyProcessed
	}

	return nil
}

// IsProcessed checks if an event has already been processed
func (r *Repository) IsProcessed(ctx context.Context, eventID uuid.UUID) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM processed_events WHERE event_id = $1)`
	var exists bool
	err := r.pool.QueryRow(ctx, query, eventID).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

// CleanupOldEvents removes processed events older than the retention period
func (r *Repository) CleanupOldEvents(ctx context.Context, retentionDays int) (int64, error) {
	query := `
		DELETE FROM processed_events
		WHERE processed_at < NOW() - INTERVAL '1 day' * $1
	`
	result, err := r.pool.Exec(ctx, query, retentionDays)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected(), nil
}