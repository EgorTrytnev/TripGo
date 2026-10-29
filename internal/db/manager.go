package db

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type txKey struct{}

type TxManager struct {
	pool         *pgxpool.Pool
	queryTimeout time.Duration
}

func NewTxManager(pool *pgxpool.Pool, queryTimeout time.Duration) *TxManager {
	return &TxManager{pool: pool, queryTimeout: queryTimeout}
}

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

func TxFromContext(ctx context.Context) (pgx.Tx, bool) {
	tx, ok := ctx.Value(txKey{}).(pgx.Tx)
	return tx, ok
}
