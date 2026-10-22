package db

import (
	"fmt"
	"testing"
	"time"

	"github.com/avf/avf-vending-api/internal/config"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestNewPoolConfig_UsesGlobalPostgresPoolByDefault(t *testing.T) {
	cfg := &config.PostgresConfig{
		URL:             "postgres://user:pass@localhost:5432/db?sslmode=disable",
		MaxConns:        3,
		MinConns:        0,
		MaxConnIdleTime: 5 * time.Minute,
		MaxConnLifetime: 30 * time.Minute,
	}

	pcfg, err := newPoolConfig(cfg, "api")
	if err != nil {
		t.Fatal(err)
	}
	if pcfg.MaxConns != 3 {
		t.Fatalf("max conns: got %d want 3", pcfg.MaxConns)
	}
	if pcfg.MinConns != 0 {
		t.Fatalf("min conns: got %d want 0", pcfg.MinConns)
	}
}

func TestNewPoolConfig_UsesPerProcessOverride(t *testing.T) {
	workerMax := int32(1)
	cfg := &config.PostgresConfig{
		URL:             "postgres://user:pass@localhost:5432/db?sslmode=disable",
		MaxConns:        3,
		MinConns:        0,
		MaxConnIdleTime: 5 * time.Minute,
		MaxConnLifetime: 30 * time.Minute,
		WorkerMaxConns:  &workerMax,
	}

	pcfg, err := newPoolConfig(cfg, "worker")
	if err != nil {
		t.Fatal(err)
	}
	if pcfg.MaxConns != 1 {
		t.Fatalf("worker max conns: got %d want 1", pcfg.MaxConns)
	}
}

func TestNewPoolConfig_PgBouncerHostnameUsesSimpleProtocol(t *testing.T) {
	pcfg := mustPoolConfig(t, "postgres://user:pass@pgbouncer:5432/db?sslmode=disable")
	if pcfg.ConnConfig.DefaultQueryExecMode != pgx.QueryExecModeSimpleProtocol {
		t.Fatalf("mode: got %v want simple_protocol", pcfg.ConnConfig.DefaultQueryExecMode)
	}
}

func TestNewPoolConfig_HostPort6432UsesSimpleProtocol(t *testing.T) {
	pcfg := mustPoolConfig(t, "postgres://user:pass@72.62.244.94:6432/db?sslmode=disable")
	if pcfg.ConnConfig.DefaultQueryExecMode != pgx.QueryExecModeSimpleProtocol {
		t.Fatalf("mode: got %v want simple_protocol", pcfg.ConnConfig.DefaultQueryExecMode)
	}
}

func TestNewPoolConfig_LocalhostKeepsDefaultExecMode(t *testing.T) {
	pcfg := mustPoolConfig(t, "postgres://user:pass@localhost:5432/db?sslmode=disable")
	if pcfg.ConnConfig.DefaultQueryExecMode != pgx.QueryExecModeCacheStatement {
		t.Fatalf("mode: got %v want cache_statement default", pcfg.ConnConfig.DefaultQueryExecMode)
	}
}

func TestNewPoolConfig_ExplicitExecModeNotOverridden(t *testing.T) {
	pcfg := mustPoolConfig(t, "postgres://user:pass@pgbouncer:5432/db?sslmode=disable&default_query_exec_mode=exec")
	if pcfg.ConnConfig.DefaultQueryExecMode != pgx.QueryExecModeExec {
		t.Fatalf("mode: got %v want exec", pcfg.ConnConfig.DefaultQueryExecMode)
	}
}

func TestNewPoolConfig_ExplicitSimpleProtocolKept(t *testing.T) {
	pcfg := mustPoolConfig(t, "postgres://user:pass@72.62.244.94:6432/db?sslmode=disable&default_query_exec_mode=simple_protocol")
	if pcfg.ConnConfig.DefaultQueryExecMode != pgx.QueryExecModeSimpleProtocol {
		t.Fatalf("mode: got %v want simple_protocol", pcfg.ConnConfig.DefaultQueryExecMode)
	}
}

func TestShouldUsePgBouncerSimpleProtocol(t *testing.T) {
	t.Parallel()
	cases := []struct {
		host string
		port string
		want bool
	}{
		{host: "pgbouncer", port: "5432", want: true},
		{host: "PgBouncer", port: "5432", want: true},
		{host: "72.62.244.94", port: "6432", want: true},
		{host: "localhost", port: "5432", want: false},
		{host: "postgres", port: "5432", want: false},
		{host: "stgpool.example.com", port: "5432", want: false},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("%s:%s", tc.host, tc.port), func(t *testing.T) {
			t.Parallel()
			got := shouldUsePgBouncerSimpleProtocol(tc.host, tc.port)
			if got != tc.want {
				t.Fatalf("got %v want %v", got, tc.want)
			}
		})
	}
}

func mustPoolConfig(t *testing.T, dsn string) *pgxpool.Config {
	t.Helper()
	cfg := &config.PostgresConfig{
		URL:             dsn,
		MaxConns:        3,
		MinConns:        0,
		MaxConnIdleTime: 5 * time.Minute,
		MaxConnLifetime: 30 * time.Minute,
	}
	pcfg, err := newPoolConfig(cfg, "api")
	if err != nil {
		t.Fatal(err)
	}
	if pcfg == nil {
		t.Fatal("expected pool config")
	}
	return pcfg
}
