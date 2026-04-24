package db

import (
	"testing"
	"time"

	"github.com/avf/avf-vending-api/internal/config"
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
