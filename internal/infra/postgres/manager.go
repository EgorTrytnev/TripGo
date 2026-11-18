package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type txKey struct{}

// TxManager runs a callback in a PostgreSQL transaction.
type TxManager struct {
	pool         *pgxpool.Pool
	queryTimeout time.Duration
}

// NewTxManager creates a transaction manager for a connection pool.
func NewTxManager(pool *pgxpool.Pool, queryTimeout time.Duration) *TxManager {
	return &TxManager{pool: pool, queryTimeout: queryTimeout}
}

// Do runs fn in a transaction and commits or rolls back its changes.
func (m *TxManager) Do(ctx context.Context, fn func(context.Context) error) (err error) {
	_, ind := TxFromContext(ctx)
	if ind {
		return fn(ctx)
	}

	txCtx, cancel := context.WithTimeout(ctx, m.queryTimeout)
	defer cancel()

	tx, err := m.pool.BeginTx(txCtx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return err
	}

	txCtx = context.WithValue(txCtx, txKey{}, tx)

	defer func() {
		if recovered := recover(); recovered != nil {
			rollbackCtx, rollbackCancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
			_ = tx.Rollback(rollbackCtx)
			rollbackCancel()
			panic(recovered)
		}

		if err != nil {
			rollbackCtx, rollbackCancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
			_ = tx.Rollback(rollbackCtx)
			rollbackCancel()
			return
		}

		err = tx.Commit(txCtx)
	}()

	err = fn(txCtx)

	return err
}

// TxFromContext returns the transaction stored in ctx, if one exists.
func TxFromContext(ctx context.Context) (pgx.Tx, bool) {
	tx, ok := ctx.Value(txKey{}).(pgx.Tx)
	return tx, ok
}
