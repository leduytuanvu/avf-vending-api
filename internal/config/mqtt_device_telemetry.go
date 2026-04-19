package config

import (
	"errors"
	"fmt"
	"os"
	"strings"

	platformnats "github.com/avf/avf-vending-api/internal/platform/nats"
)

const (
	envTelemetryMaxPayloadBytes       = "TELEMETRY_MAX_PAYLOAD_BYTES"
	envTelemetryMaxPointsPerMessage   = "TELEMETRY_MAX_POINTS_PER_MESSAGE"
	envTelemetryMaxTagsPerMessage     = "TELEMETRY_MAX_TAGS_PER_MESSAGE"
	envTelemetryPerMachineMsgsPerSec  = "TELEMETRY_PER_MACHINE_MSGS_PER_SEC"
	envTelemetryPerMachineBurst       = "TELEMETRY_PER_MACHINE_BURST"
	envTelemetryGlobalMaxInflight     = "TELEMETRY_GLOBAL_MAX_INFLIGHT"
	envTelemetryWorkerConcurrency     = "TELEMETRY_WORKER_CONCURRENCY"
	envTelemetryDropOnBackpressure    = "TELEMETRY_DROP_ON_BACKPRESSURE"
	envTelemetryLegacyPostgresIngest  = "TELEMETRY_LEGACY_POSTGRES_INGEST"
	envTelemetrySubmitWaitMs          = "TELEMETRY_SUBMIT_WAIT_MS"
)

// MQTTDeviceTelemetryConfig bounds high-frequency device MQTT → NATS/OLTP ingress (cmd/mqtt-ingest).
// OpenTelemetry SDK settings remain on TelemetryConfig; this struct is device-pipeline only.
type MQTTDeviceTelemetryConfig struct {
	MaxPayloadBytes       int
	MaxPointsPerMessage   int
	MaxTagsPerMessage     int
	PerMachineMsgsPerSec  float64
	PerMachineBurst       int
	GlobalMaxInflight     int
	WorkerConcurrency     int
	DropOnBackpressure    bool
	LegacyPostgresIngest  bool
	SubmitWaitMs          int
}

func loadMQTTDeviceTelemetryConfig() MQTTDeviceTelemetryConfig {
	return MQTTDeviceTelemetryConfig{
		MaxPayloadBytes:      getenvInt(envTelemetryMaxPayloadBytes, 65536),
		MaxPointsPerMessage:  getenvInt(envTelemetryMaxPointsPerMessage, 512),
		MaxTagsPerMessage:    getenvInt(envTelemetryMaxTagsPerMessage, 128),
		PerMachineMsgsPerSec: getenvFloat64(envTelemetryPerMachineMsgsPerSec, 25),
		PerMachineBurst:      getenvInt(envTelemetryPerMachineBurst, 50),
		GlobalMaxInflight:    getenvInt(envTelemetryGlobalMaxInflight, 256),
		WorkerConcurrency:    getenvInt(envTelemetryWorkerConcurrency, 8),
		DropOnBackpressure:   getenvBool(envTelemetryDropOnBackpressure, true),
		LegacyPostgresIngest: getenvBool(envTelemetryLegacyPostgresIngest, false),
		SubmitWaitMs:         getenvInt(envTelemetrySubmitWaitMs, 2000),
	}
}

func (m MQTTDeviceTelemetryConfig) validate() error {
	if m.MaxPayloadBytes < 1024 || m.MaxPayloadBytes > 512*1024 {
		return fmt.Errorf("config: %s must be between 1024 and 524288", envTelemetryMaxPayloadBytes)
	}
	if m.MaxPointsPerMessage < 1 || m.MaxPointsPerMessage > 20000 {
		return fmt.Errorf("config: %s out of range", envTelemetryMaxPointsPerMessage)
	}
	if m.MaxTagsPerMessage < 1 || m.MaxTagsPerMessage > 5000 {
		return fmt.Errorf("config: %s out of range", envTelemetryMaxTagsPerMessage)
	}
	if m.PerMachineMsgsPerSec <= 0 || m.PerMachineMsgsPerSec > 5000 {
		return fmt.Errorf("config: %s must be in (0,5000]", envTelemetryPerMachineMsgsPerSec)
	}
	if m.PerMachineBurst < 1 || m.PerMachineBurst > 100000 {
		return fmt.Errorf("config: %s out of range", envTelemetryPerMachineBurst)
	}
	if m.GlobalMaxInflight < 8 || m.GlobalMaxInflight > 100000 {
		return fmt.Errorf("config: %s must be between 8 and 100000", envTelemetryGlobalMaxInflight)
	}
	if m.WorkerConcurrency < 1 || m.WorkerConcurrency > 256 {
		return fmt.Errorf("config: %s must be between 1 and 256", envTelemetryWorkerConcurrency)
	}
	if m.SubmitWaitMs < 1 || m.SubmitWaitMs > 120000 {
		return fmt.Errorf("config: %s must be between 1 and 120000", envTelemetrySubmitWaitMs)
	}
	return nil
}

// validateProductionTelemetryNATS enforces JetStream-backed telemetry in production.
func (c *Config) validateProductionTelemetryNATS() error {
	if c == nil {
		return errors.New("config: nil")
	}
	if c.AppEnv != AppEnvProduction {
		return nil
	}
	if strings.TrimSpace(os.Getenv(platformnats.EnvNATSURL)) == "" {
		return fmt.Errorf("config: APP_ENV=production requires non-empty %s (NATS/JetStream is mandatory for telemetry, outbox, and worker consumers; direct MQTT→Postgres telemetry hot path is disabled in production)", platformnats.EnvNATSURL)
	}
	if c.MQTTDeviceTelemetry.LegacyPostgresIngest {
		return fmt.Errorf("config: APP_ENV=production forbids %s=true (high-frequency telemetry must use the NATS bridge)", envTelemetryLegacyPostgresIngest)
	}
	return nil
}
