package config

import (
	"os"
	"testing"
)

func TestApplyPostgresProductionServiceDefaults_respectsExplicitEnv(t *testing.T) {
	t.Setenv("API_DATABASE_MAX_CONNS", "9")
	cfg := &PostgresConfig{URL: "postgres://localhost/db"}
	applyPostgresProductionServiceDefaults(cfg, AppEnvProduction)
	if cfg.APIMaxConns != nil {
		t.Fatalf("expected nil API max when env set explicitly")
	}
}

func TestApplyPostgresProductionServiceDefaults_setsWhenUnset(t *testing.T) {
	keys := []string{
		"API_DATABASE_MAX_CONNS", "WORKER_DATABASE_MAX_CONNS",
		"MQTT_INGEST_DATABASE_MAX_CONNS", "RECONCILER_DATABASE_MAX_CONNS",
		"TEMPORAL_WORKER_DATABASE_MAX_CONNS",
		"API_DATABASE_MIN_CONNS", "WORKER_DATABASE_MIN_CONNS",
	}
	for _, k := range keys {
		_ = os.Unsetenv(k)
	}
	cfg := &PostgresConfig{URL: "postgres://localhost/db", MaxConns: 40, MinConns: 4}
	applyPostgresProductionServiceDefaults(cfg, AppEnvProduction)
	if cfg.APIMaxConns == nil || *cfg.APIMaxConns != 25 {
		t.Fatalf("API max: %+v", cfg.APIMaxConns)
	}
	if cfg.WorkerMaxConns == nil || *cfg.WorkerMaxConns != 8 {
		t.Fatalf("worker max: %+v", cfg.WorkerMaxConns)
	}
	if cfg.APIMinConns == nil || *cfg.APIMinConns != 4 {
		t.Fatalf("API min: %+v", cfg.APIMinConns)
	}
}

func TestApplyPostgresProductionServiceDefaults_clampsToGlobalMax(t *testing.T) {
	for _, k := range []string{
		"API_DATABASE_MAX_CONNS", "WORKER_DATABASE_MAX_CONNS",
		"MQTT_INGEST_DATABASE_MAX_CONNS", "RECONCILER_DATABASE_MAX_CONNS",
		"TEMPORAL_WORKER_DATABASE_MAX_CONNS",
	} {
		_ = os.Unsetenv(k)
	}
	cfg := &PostgresConfig{URL: "postgres://localhost/db", MaxConns: 8, MinConns: 1}
	applyPostgresProductionServiceDefaults(cfg, AppEnvProduction)
	if cfg.APIMaxConns == nil || *cfg.APIMaxConns != 8 {
		t.Fatalf("API max should clamp to global 8, got %+v", cfg.APIMaxConns)
	}
}
