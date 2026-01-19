package service

import (
	"context"

	"github.com/aelhady03/sumflow/adder/internal/outbox"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AdderService struct {
	pool       *pgxpool.Pool
	outboxRepo *outbox.Repository
}

func NewAdderService(pool *pgxpool.Pool, outboxRepo *outbox.Repository) *AdderService {
	return &AdderService{
		pool:       pool,
		outboxRepo: outboxRepo,
	}
}

func (a *AdderService) Add(ctx context.Context, x, y int) (int, error) {
	sum := x + y

	tx, err := a.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	event, err := outbox.NewSumCalculatedEvent(x, y, sum)
	if err != nil {
		return 0, err
	}

	if err := a.outboxRepo.InsertInTx(ctx, tx, event); err != nil {
		return 0, err
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}

	return sum, nil
}
