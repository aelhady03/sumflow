# Phase 1: Outbox Pattern Implementation

## Overview

Replace direct Kafka publishing with the transactional outbox pattern for reliable, exactly-once message delivery.

## Problem Statement

Current `AdderService.Add()` directly publishes to Kafka. If Kafka fails after calculating the sum, the system enters an inconsistent state where the operation succeeded but the event was lost.

## Solution Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        Adder Service                            │
│  ┌─────────────┐    ┌──────────────────────────────────────┐   │
│  │ gRPC Server │───>│ BEGIN TRANSACTION                    │   │
│  │             │    │   1. Calculate sum                   │   │
│  │ SumNumbers()│    │   2. INSERT INTO outbox              │   │
│  │             │    │ COMMIT                               │   │
│  └─────────────┘    └──────────────────────────────────────┘   │
│                                   │                             │
│                     ┌─────────────▼─────────────┐               │
│                     │     Outbox Relay          │               │
│                     │  (Background Goroutine)   │               │
│                     │  - Poll unpublished       │               │
│                     │  - Publish to Kafka       │               │
│                     │  - Mark as published      │               │
│                     └─────────────┬─────────────┘               │
└───────────────────────────────────┼─────────────────────────────┘
                                    │
                                    ▼
                              ┌───────────┐
                              │   Kafka   │
                              │  "sums"   │
                              └─────┬─────┘
                                    │
┌───────────────────────────────────┼─────────────────────────────┐
│                        Totalizer Service                        │
│                     ┌─────────────▼─────────────┐               │
│                     │     Kafka Consumer        │               │
│                     │  - Consume message        │               │
│                     │  - Check dedup table      │               │
│                     │  - Process if new         │               │
│                     │  - Mark as processed      │               │
│                     └───────────────────────────┘               │
└─────────────────────────────────────────────────────────────────┘
```

## Database Schemas

### Adder Service (PostgreSQL)

```sql
-- Outbox table
CREATE TABLE outbox (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    aggregate_type  TEXT NOT NULL,
    aggregate_id    TEXT NOT NULL,
    event_type      TEXT NOT NULL,
    payload         JSONB NOT NULL,
    created_at      TIMESTAMPTZ DEFAULT NOW() NOT NULL,
    published_at    TIMESTAMPTZ,
    retry_count     INTEGER DEFAULT 0,
    last_error      TEXT
);

CREATE INDEX idx_outbox_unpublished ON outbox(created_at)
    WHERE published_at IS NULL;
```

### Totalizer Service (PostgreSQL)

```sql
-- Deduplication table
CREATE TABLE processed_events (
    event_id        UUID PRIMARY KEY,
    aggregate_type  VARCHAR(255) NOT NULL,
    event_type      VARCHAR(255) NOT NULL,
    processed_at    TIMESTAMPTZ DEFAULT NOW() NOT NULL
);

-- Replace file storage with database
CREATE TABLE totals (
    id          INTEGER PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    total       BIGINT NOT NULL DEFAULT 0,
    updated_at  TIMESTAMPTZ DEFAULT NOW() NOT NULL
);
```

## Files to Create/Modify

### Adder Service

| File                            | Action | Description                          |
| ------------------------------- | ------ | ------------------------------------ |
| `internal/database/database.go` | Create | PostgreSQL connection pool using pgx |
| `internal/outbox/event.go`      | Create | Event type definitions               |
| `internal/outbox/repository.go` | Create | Outbox CRUD operations               |
| `internal/outbox/relay.go`      | Create | Background publisher component       |
| `internal/service/service.go`   | Modify | Use transaction + outbox insert      |
| `internal/kafka/producer.go`    | Modify | Add `PublishWithKey()` method        |
| `cmd/grpc/main.go`              | Modify | Initialize DB, start relay           |

### Totalizer Service

| File                            | Action | Description                       |
| ------------------------------- | ------ | --------------------------------- |
| `internal/database/database.go` | Create | PostgreSQL connection pool        |
| `internal/dedup/repository.go`  | Create | Deduplication tracking            |
| `internal/kafka/consumer.go`    | Create | Kafka consumer with idempotency   |
| `internal/storage/postgres.go`  | Create | PostgreSQL storage implementation |
| `internal/service/service.go`   | Modify | Add `AddToTotal()` method         |
| `cmd/api/main.go`               | Modify | Initialize DB, start consumer     |

## Key Code Changes

### Modified AdderService.Add()

```go
func (a *AdderService) Add(ctx context.Context, x, y int) (int, error) {
    sum := x + y

    tx, err := a.pool.Begin(ctx)
    if err != nil {
        return 0, err
    }
    defer tx.Rollback(ctx)

    event, _ := outbox.NewSumCalculatedEvent(x, y, sum)
    if err := a.outboxRepo.InsertInTx(ctx, tx, event); err != nil {
        return 0, err
    }

    return sum, tx.Commit(ctx)
}
```

### Outbox Relay Loop

```go
func (r *Relay) processBatch(ctx context.Context) error {
    events, _ := r.repo.FetchUnpublished(ctx, r.config.BatchSize)

    for _, event := range events {
        if err := r.producer.PublishWithKey(event.AggregateID, event); err != nil {
            r.repo.MarkFailed(ctx, event.ID, err.Error())
            continue
        }
        r.repo.MarkPublished(ctx, event.ID)
    }
    return nil
}
```

### Consumer Deduplication

```go
func (c *Consumer) processMessage(ctx context.Context, msg kafka.Message) error {
    tx, _ := c.pool.Begin(ctx)
    defer tx.Rollback(ctx)

    // Idempotency check
    err := c.dedupRepo.CheckAndMark(ctx, tx, eventID, aggregateType, eventType)
    if errors.Is(err, dedup.ErrEventAlreadyProcessed) {
        return nil // Skip, already processed
    }

    // Process the event
    c.handler.HandleSumCalculated(ctx, eventID, payload)

    return tx.Commit(ctx)
}
```

## Kafka Message Format

```json
{
  "event_id": "550e8400-e29b-41d4-a716-446655440000",
  "aggregate_type": "sum",
  "aggregate_id": "550e8400-e29b-41d4-a716-446655440000",
  "event_type": "sum.calculated",
  "payload": {
    "x": 5,
    "y": 3,
    "result": 8
  },
  "created_at": "2024-01-15T10:30:00Z"
}
```

## Configuration

### Adder Service Flags

```
--db-dsn          PostgreSQL connection string
--relay-interval  Outbox polling interval (default: 100ms)
--relay-batch     Batch size per poll (default: 100)
--retry-limit     Max retries per event (default: 5)
```

### Totalizer Service Flags

```
--db-dsn          PostgreSQL connection string
--kafka-brokers   Kafka broker addresses
--kafka-topic     Topic to consume (default: "sums")
--kafka-group-id  Consumer group ID
```

## Testing Checklist

- [ ] Unit tests for outbox repository
- [ ] Unit tests for dedup repository
- [ ] Integration test: gRPC call → outbox → Kafka → consumer → total updated
- [ ] Test idempotency: same event ID processed twice → total incremented once
- [ ] Test relay retry: simulate Kafka failure, verify retry with backoff
- [ ] Test cleanup: verify old published events are deleted

## Cleanup Strategy

- Relay runs cleanup every hour (configurable)
- Default retention: 7 days for published events
- Dedup table: 30 days retention
