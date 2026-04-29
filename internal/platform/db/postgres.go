package db

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/avf/avf-vending-api/internal/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPool returns a PostgreSQL pool when DATABASE_URL is configured.
func NewPool(ctx context.Context, cfg *config.PostgresConfig) (*pgxpool.Pool, error) {
	processName := detectProcessName()
	pcfg, err := newPoolConfig(cfg, processName)
	if err != nil {
		return nil, err
	}
	if pcfg == nil {
		return nil, nil
	}

	pool, err := pgxpool.NewWithConfig(ctx, pcfg)
	if err != nil {
		return nil, fmt.Errorf("db: connect postgres: %w", err)
	}

	sum := cfg.PoolSummaryForProcess(processName)
	slog.Info("postgres_pool_effective",
		"process", sum.ProcessName,
		"max_conns", sum.MaxConns,
		"min_conns", sum.MinConns,
		"max_conn_idle_time", sum.MaxConnIdleTime.String(),
		"max_conn_lifetime", sum.MaxConnLifetime.String(),
	)

	return pool, nil
}

func newPoolConfig(cfg *config.PostgresConfig, processName string) (*pgxpool.Config, error) {
	if cfg == nil || cfg.URL == "" {
		return nil, nil
	}

	pcfg, err := pgxpool.ParseConfig(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("db: parse postgres url: %w", err)
	}

	pcfg.MaxConns = cfg.MaxConnsForProcess(processName)
	pcfg.MinConns = cfg.MinConns
	pcfg.MaxConnIdleTime = cfg.MaxConnIdleTime
	pcfg.MaxConnLifetime = cfg.MaxConnLifetime
	if tr := NewSlowQueryTracer(cfg.SlowQueryLogThresholdMS, slog.Default()); tr != nil {
		pcfg.ConnConfig.Tracer = tr
	}
	return pcfg, nil
}

func detectProcessName() string {
	name := strings.TrimSpace(filepath.Base(os.Args[0]))
	name = strings.TrimSuffix(name, filepath.Ext(name))
	return name
}
