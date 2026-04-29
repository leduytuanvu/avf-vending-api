package config

// RetentionWorkerConfig gates scheduled retention jobs in cmd/worker and optional global dry-run overrides.
type RetentionWorkerConfig struct {
	// Enabled is ENABLE_RETENTION_WORKER — master switch for telemetry + enterprise retention tickers (cmd/worker).
	// When false, retention hooks are never scheduled regardless of TELEMETRY_CLEANUP_ENABLED / ENTERPRISE_RETENTION_CLEANUP_ENABLED.
	Enabled bool
	// GlobalDryRun is RETENTION_DRY_RUN — when true, telemetry and enterprise retention jobs behave as dry-run (no DELETE rows).
	GlobalDryRun bool
}

func loadRetentionWorkerConfig(appEnv AppEnvironment) RetentionWorkerConfig {
	workerDefault := appEnv != AppEnvDevelopment && appEnv != AppEnvTest
	return RetentionWorkerConfig{
		Enabled:      getenvBool("ENABLE_RETENTION_WORKER", workerDefault),
		GlobalDryRun: getenvBool("RETENTION_DRY_RUN", false),
	}
}

// EffectiveRetentionDryRun merges subsystem dry-run flags with RETENTION_DRY_RUN (global wins).
func EffectiveRetentionDryRun(globalDryRun bool, subsystemDryRun bool) bool {
	return globalDryRun || subsystemDryRun
}

// DestructiveRetentionAllowed reports whether Postgres DELETE retention may run for this deployment (blocks dev/test unless opted in).
func DestructiveRetentionAllowed(cfg *Config) bool {
	if cfg == nil {
		return false
	}
	if cfg.AppEnv != AppEnvDevelopment && cfg.AppEnv != AppEnvTest {
		return true
	}
	return cfg.RetentionAllowDestructiveLocal
}
