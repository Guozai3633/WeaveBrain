package repository

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Conn is a minimal abstraction over pgx operations shared between pgxpool.Pool and pgx.Tx.
type Conn interface {
	Query(ctx context.Context, query string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, query string, args ...any) pgx.Row
	Exec(ctx context.Context, query string, args ...any) (tag any, err error)
}

// PgConn wraps a pgxpool.Pool so repositories can use it uniformly.
type PgConn struct {
	pool *pgxpool.Pool
}

func NewPgConn(pool *pgxpool.Pool) *PgConn {
	return &PgConn{pool: pool}
}

func (p *PgConn) Query(ctx context.Context, query string, args ...any) (pgx.Rows, error) {
	return p.pool.Query(ctx, query, args...)
}

func (p *PgConn) QueryRow(ctx context.Context, query string, args ...any) pgx.Row {
	return p.pool.QueryRow(ctx, query, args...)
}

func (p *PgConn) Exec(ctx context.Context, query string, args ...any) (any, error) {
	return p.pool.Exec(ctx, query, args...)
}

// TxWrapper wraps a pgx.Tx so it implements the Conn interface.
type TxWrapper struct {
	tx pgx.Tx
}

func NewTxWrapper(tx pgx.Tx) *TxWrapper {
	return &TxWrapper{tx: tx}
}

func (t *TxWrapper) Query(ctx context.Context, query string, args ...any) (pgx.Rows, error) {
	return t.tx.Query(ctx, query, args...)
}

func (t *TxWrapper) QueryRow(ctx context.Context, query string, args ...any) pgx.Row {
	return t.tx.QueryRow(ctx, query, args...)
}

func (t *TxWrapper) Exec(ctx context.Context, query string, args ...any) (any, error) {
	return t.tx.Exec(ctx, query, args...)
}
