package storage

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresStorage struct {
	pool *pgxpool.Pool
}

func NewPostgresStorage(pool *pgxpool.Pool) *PostgresStorage {
	return &PostgresStorage{pool: pool}
}

func (p *PostgresStorage) Save(total int) error {
	return p.SaveContext(context.Background(), total)
}

func (p *PostgresStorage) SaveContext(ctx context.Context, total int) error {
	query := `UPDATE totals SET total = $1, updated_at = NOW() WHERE id = 1`
	_, err := p.pool.Exec(ctx, query, total)
	return err
}

func (p *PostgresStorage) Load() (int, error) {
	return p.LoadContext(context.Background())
}

func (p *PostgresStorage) LoadContext(ctx context.Context) (int, error) {
	var total int
	query := `SELECT total FROM totals WHERE id = 1`
	err := p.pool.QueryRow(ctx, query).Scan(&total)
	if err != nil {
		return 0, err
	}
	return total, nil
}

// AddToTotalInTx atomically adds a value to the total within a transaction
func (p *PostgresStorage) AddToTotalInTx(ctx context.Context, tx pgx.Tx, value int) error {
	query := `UPDATE totals SET total = total + $1, updated_at = NOW() WHERE id = 1`
	_, err := tx.Exec(ctx, query, value)
	return err
}

// GetPool returns the underlying connection pool for transaction management
func (p *PostgresStorage) GetPool() *pgxpool.Pool {
	return p.pool
}