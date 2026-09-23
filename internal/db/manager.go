package db

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type txKey struct{}

type TxManager struct {
	pool *pgxpool.Pool
}

func NewTxManager(pool *pgxpool.Pool) *TxManager {
	return &TxManager{pool: pool}
}

func (m *TxManager) Do(ctx context.Context, fn func(context.Context) error) (err error) {
	_, ind := ctx.Value(txKey{}).(pgx.Tx)
	if ind {
		return fn(ctx)
	}

	tx, err := m.pool.Begin(ctx)
	if err != nil {
		return err
	}

	ctxN := context.WithValue(ctx, txKey{}, tx)

	defer func() {
		if recovered := recover(); recovered != nil {
			_ = tx.Rollback(ctx)
			panic(recovered)
		}

		if err != nil {
			_ = tx.Rollback(ctx)
			return
		}

		err = tx.Commit(ctx)
	}()

	err = fn(ctxN)

	return err
}
