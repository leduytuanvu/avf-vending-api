package config

import (
	"os"
	"strings"
)

// applyPostgresProductionServiceDefaults sets per-process pool caps when APP_ENV=production
// and the operator has not set the corresponding env var (explicit deployment values win).
func applyPostgresProductionServiceDefaults(cfg *PostgresConfig, appEnv AppEnvironment) {
	if cfg == nil || strings.TrimSpace(cfg.URL) == "" || appEnv != AppEnvProduction {
		return
	}
	setInt32PtrIfEnvUnset("API_DATABASE_MAX_CONNS", &cfg.APIMaxConns, 25)
	setInt32PtrIfEnvUnset("WORKER_DATABASE_MAX_CONNS", &cfg.WorkerMaxConns, 8)
	setInt32PtrIfEnvUnset("MQTT_INGEST_DATABASE_MAX_CONNS", &cfg.MQTTIngestMaxConns, 8)
	setInt32PtrIfEnvUnset("RECONCILER_DATABASE_MAX_CONNS", &cfg.ReconcilerMaxConns, 4)
	setInt32PtrIfEnvUnset("TEMPORAL_WORKER_DATABASE_MAX_CONNS", &cfg.TemporalWorkerMaxConns, 4)

	setInt32PtrIfEnvUnset("API_DATABASE_MIN_CONNS", &cfg.APIMinConns, 4)
	setInt32PtrIfEnvUnset("WORKER_DATABASE_MIN_CONNS", &cfg.WorkerMinConns, 1)
	setInt32PtrIfEnvUnset("MQTT_INGEST_DATABASE_MIN_CONNS", &cfg.MQTTIngestMinConns, 1)
	setInt32PtrIfEnvUnset("RECONCILER_DATABASE_MIN_CONNS", &cfg.ReconcilerMinConns, 1)
	setInt32PtrIfEnvUnset("TEMPORAL_WORKER_DATABASE_MIN_CONNS", &cfg.TemporalWorkerMinConns, 1)
}

func setInt32PtrIfEnvUnset(envKey string, dst **int32, value int32) {
	if _, ok := os.LookupEnv(envKey); ok {
		return
	}
	if dst != nil && *dst != nil {
		return
	}
	v := value
	if dst == nil {
		return
	}
	*dst = &v
}
