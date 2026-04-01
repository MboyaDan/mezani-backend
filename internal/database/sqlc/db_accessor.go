package db

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// BeginTx starts a transaction on the underlying pool.
// Services use this to open transactions without holding
// a direct reference to *pgxpool.Pool.
func (q *Queries) BeginTx(ctx context.Context, opts pgx.TxOptions) (pgx.Tx, error) {
	return q.db.(*pgxpool.Pool).BeginTx(ctx, opts)
}
