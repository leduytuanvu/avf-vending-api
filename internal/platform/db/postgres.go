package db

import (
	"context"
	"fmt"

	"github.com/avf/avf-vending-api/internal/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPool returns a PostgreSQL pool when DATABASE_URL is configured.
func NewPool(ctx context.Context, cfg *config.PostgresConfig) (*pgxpool.Pool, error) {
	if cfg == nil || cfg.URL == "" {
		return nil, nil
	}

	pcfg, err := pgxpool.ParseConfig(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("db: parse postgres url: %w", err)
	}

	pcfg.MaxConns = cfg.MaxConns
	pcfg.MinConns = cfg.MinConns
	pcfg.MaxConnIdleTime = cfg.MaxConnIdleTime
	pcfg.MaxConnLifetime = cfg.MaxConnLifetime

	pool, err := pgxpool.NewWithConfig(ctx, pcfg)
	if err != nil {
		return nil, fmt.Errorf("db: connect postgres: %w", err)
	}

	return pool, nil
}
