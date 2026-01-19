package outbox

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// InsertInTx inserts an event into the outbox within an existing transaction
func (r *Repository) InsertInTx(ctx context.Context, tx pgx.Tx, event *Event) error {
	query := `
		INSERT INTO outbox (aggregate_type, aggregate_id, event_type, payload, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`
	_, err := tx.Exec(ctx, query,
		event.AggregateType,
		event.AggregateID,
		event.EventType,
		event.Payload,
		event.CreatedAt,
	)
	return err
}

// FetchUnpublished retrieves unpublished events ordered by creation time
func (r *Repository) FetchUnpublished(ctx context.Context, limit int) ([]*Event, error) {
	query := `
		SELECT id, aggregate_type, aggregate_id, event_type, payload, created_at, retry_count, last_error
		FROM outbox
		WHERE published_at IS NULL
		ORDER BY created_at ASC
		LIMIT $1
		FOR UPDATE SKIP LOCKED
	`
	rows, err := r.pool.Query(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []*Event
	for rows.Next() {
		var e Event
		var payload []byte
		err := rows.Scan(
			&e.ID,
			&e.AggregateType,
			&e.AggregateID,
			&e.EventType,
			&payload,
			&e.CreatedAt,
			&e.RetryCount,
			&e.LastError,
		)
		if err != nil {
			return nil, err
		}
		e.Payload = json.RawMessage(payload)
		events = append(events, &e)
	}

	return events, rows.Err()
}

// MarkPublished marks an event as successfully published
func (r *Repository) MarkPublished(ctx context.Context, id uuid.UUID) error {
	query := `
		UPDATE outbox
		SET published_at = $1
		WHERE id = $2
	`
	_, err := r.pool.Exec(ctx, query, time.Now().UTC(), id)
	return err
}

// MarkFailed increments retry count and records the error
func (r *Repository) MarkFailed(ctx context.Context, id uuid.UUID, errMsg string) error {
	query := `
		UPDATE outbox
		SET retry_count = retry_count + 1, last_error = $1
		WHERE id = $2
	`
	_, err := r.pool.Exec(ctx, query, errMsg, id)
	return err
}

// CleanupOldEvents deletes published events older than the retention period
func (r *Repository) CleanupOldEvents(ctx context.Context, retention time.Duration) (int64, error) {
	query := `
		DELETE FROM outbox
		WHERE published_at IS NOT NULL
		AND published_at < $1
	`
	cutoff := time.Now().UTC().Add(-retention)
	result, err := r.pool.Exec(ctx, query, cutoff)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected(), nil
}

// GetFailedEvents retrieves events that have exceeded retry limit
func (r *Repository) GetFailedEvents(ctx context.Context, maxRetries int) ([]*Event, error) {
	query := `
		SELECT id, aggregate_type, aggregate_id, event_type, payload, created_at, retry_count, last_error
		FROM outbox
		WHERE published_at IS NULL AND retry_count >= $1
		ORDER BY created_at ASC
	`
	rows, err := r.pool.Query(ctx, query, maxRetries)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []*Event
	for rows.Next() {
		var e Event
		var payload []byte
		err := rows.Scan(
			&e.ID,
			&e.AggregateType,
			&e.AggregateID,
			&e.EventType,
			&payload,
			&e.CreatedAt,
			&e.RetryCount,
			&e.LastError,
		)
		if err != nil {
			return nil, err
		}
		e.Payload = json.RawMessage(payload)
		events = append(events, &e)
	}

	return events, rows.Err()
}