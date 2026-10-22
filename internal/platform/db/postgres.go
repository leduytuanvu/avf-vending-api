package db

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/avf/avf-vending-api/internal/config"
	"github.com/avf/avf-vending-api/internal/platform/pgxutil"
	"github.com/jackc/pgx/v5"
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
	pcfg.MinConns = cfg.MinConnsForProcess(processName)
	pcfg.MaxConnIdleTime = cfg.MaxConnIdleTime
	pcfg.MaxConnLifetime = cfg.MaxConnLifetime
	if tr := NewSlowQueryTracer(cfg.SlowQueryLogThresholdMS, slog.Default()); tr != nil {
		pcfg.ConnConfig.Tracer = tr
	}
	applyPgBouncerCompat(pcfg, cfg.URL)
	pgxutil.ApplyUUIDArrayCodecsToPoolConfig(pcfg)
	return pcfg, nil
}

// shouldUsePgBouncerSimpleProtocol is true for the production transaction pooler
// (Docker hostname pgbouncer, or host port 6432). Direct Postgres :5432 and
// local dev stay on pgx defaults so prepared statements still work.
func shouldUsePgBouncerSimpleProtocol(host, port string) bool {
	if strings.EqualFold(strings.TrimSpace(host), "pgbouncer") {
		return true
	}
	return strings.TrimSpace(port) == "6432"
}

func dsnSetsQueryExecMode(dsn string) bool {
	u, err := url.Parse(dsn)
	if err == nil && u.RawQuery != "" {
		if _, ok := u.Query()["default_query_exec_mode"]; ok {
			return true
		}
	}
	return strings.Contains(strings.ToLower(dsn), "default_query_exec_mode=")
}

// applyPgBouncerCompat disables pgx statement-cache PREPARE when talking to
// PgBouncer transaction pooling. Prepared names (stmtcache_*) do not survive
// backend reassignment and surface as SQLSTATE 26000. An explicit
// default_query_exec_mode in DATABASE_URL is left unchanged.
func applyPgBouncerCompat(pcfg *pgxpool.Config, dsn string) bool {
	if pcfg == nil || pcfg.ConnConfig == nil {
		return false
	}
	host := pcfg.ConnConfig.Host
	port := fmt.Sprintf("%d", pcfg.ConnConfig.Port)
	if !shouldUsePgBouncerSimpleProtocol(host, port) {
		return false
	}
	if dsnSetsQueryExecMode(dsn) {
		return false
	}
	pcfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
	slog.Info("postgres_query_exec_mode",
		"mode", "simple_protocol",
		"reason", "pgbouncer_transaction_pool",
		"host", host,
		"port", pcfg.ConnConfig.Port,
	)
	return true
}

func detectProcessName() string {
	name := strings.TrimSpace(filepath.Base(os.Args[0]))
	name = strings.TrimSuffix(name, filepath.Ext(name))
	return name
}
