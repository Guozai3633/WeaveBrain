package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresRepository wraps the connection pool.
type PostgresRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresRepository creates a new connection pool.
func NewPostgresRepository(dsn string) (*PostgresRepository, error) {
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		return nil, err
	}

	if err := pool.Ping(context.Background()); err != nil {
		return nil, err
	}

	return &PostgresRepository{pool: pool}, nil
}

// Pool returns the underlying pgxpool.Pool.
func (r *PostgresRepository) Pool() *pgxpool.Pool {
	return r.pool
}

// ConnPool returns a connection from the pool that implements sql.DB-like methods.
func (r *PostgresRepository) ConnPool() *pgxpool.Pool {
	return r.pool
}

// Close closes all connections in the pool.
func (r *PostgresRepository) Close() {
	if r.pool != nil {
		r.pool.Close()
	}
}
