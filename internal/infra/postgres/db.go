// Package postgres manages PostgreSQL connections and transactions.
package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/EgorTrytnev/TripGo/internal/config"
)

// NewPool creates a configured PostgreSQL connection pool.
func NewPool(ctx context.Context, cfg config.Config, tracer pgx.QueryTracer) (*pgxpool.Pool, error) {
	if cfg.DatabaseURL == "" {
		return nil, errors.New("URL is empty")
	}

	poolCfg, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}

	poolCfg.MaxConns = cfg.DatabaseMaxConns
	poolCfg.MinConns = cfg.DatabaseMinConns
	poolCfg.MaxConnLifetime = cfg.DatabaseMaxConnLife
	poolCfg.ConnConfig.ConnectTimeout = cfg.DatabaseConnectTimeout
	poolCfg.ConnConfig.Tracer = tracer

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("pool create: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, cfg.DatabaseQueryTimeout)
	defer cancel()

	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping db: %w", err)
	}

	return pool, nil
}
