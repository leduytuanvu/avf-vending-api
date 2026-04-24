package config

import (
	"errors"
	"fmt"
	"time"

	platformnats "github.com/avf/avf-vending-api/internal/platform/nats"
)

const (
	envTelemetryStreamMaxBytes          = "TELEMETRY_STREAM_MAX_BYTES"
	envTelemetryStreamMaxAge            = "TELEMETRY_STREAM_MAX_AGE"
	envTelemetryConsumerMaxAckPending   = "TELEMETRY_CONSUMER_MAX_ACK_PENDING"
	envTelemetryConsumerAckWait         = "TELEMETRY_CONSUMER_ACK_WAIT"
	envTelemetryConsumerMaxDeliver      = "TELEMETRY_CONSUMER_MAX_DELIVER"
	envTelemetryConsumerBatchSize       = "TELEMETRY_CONSUMER_BATCH_SIZE"
	envTelemetryConsumerPullTimeout     = "TELEMETRY_CONSUMER_PULL_TIMEOUT"
	envTelemetryProjectionMaxConcurrent = "TELEMETRY_PROJECTION_MAX_CONCURRENCY"
	envTelemetryProjectionDedupeLRUSize = "TELEMETRY_PROJECTION_DEDUPE_LRU_SIZE"
	envTelemetryReadinessMaxPending     = "TELEMETRY_READINESS_MAX_PENDING"
	envTelemetryReadinessMaxFailStreak  = "TELEMETRY_READINESS_MAX_PROJECTION_FAIL_STREAK"
	envTelemetryConsumerLagPollInterval = "TELEMETRY_CONSUMER_LAG_POLL_INTERVAL"
)

// TelemetryJetStreamConfig bounds JetStream telemetry streams, durable consumers, and worker projection behavior.
type TelemetryJetStreamConfig struct {
	StreamMaxBytes int64
	// StreamMaxAgeBaseline is the maximum age for the longest-lived stream (diagnostic); other streams scale by fixed ratios.
	StreamMaxAgeBaseline     time.Duration
	ConsumerMaxAckPending    int
	ConsumerAckWait          time.Duration
	ConsumerMaxDeliver       int
	ConsumerBatchSize        int
	ConsumerPullTimeout      time.Duration
	ProjectionMaxConcurrency int
	ProjectionDedupeLRUSize  int
	// ReadinessMaxPending: if >0, worker /health/ready returns 503 when any telemetry consumer NumPending exceeds this.
	ReadinessMaxPending int64
	// ReadinessMaxProjectionFailStreak: if >0, worker /health/ready returns 503 when any durable's consecutive projection failures reach this.
	ReadinessMaxProjectionFailStreak int
	ConsumerLagPollInterval          time.Duration
}

func loadTelemetryJetStreamConfig() TelemetryJetStreamConfig {
	baseline := mustParseDuration(envTelemetryStreamMaxAge, getenv(envTelemetryStreamMaxAge, "168h"))
	return TelemetryJetStreamConfig{
		StreamMaxBytes:                   getenvInt64(envTelemetryStreamMaxBytes, platformnats.TelemetryStreamLegacyDefaultMaxBytes),
		StreamMaxAgeBaseline:             baseline,
		ConsumerMaxAckPending:            getenvInt(envTelemetryConsumerMaxAckPending, 1024),
		ConsumerAckWait:                  mustParseDuration(envTelemetryConsumerAckWait, getenv(envTelemetryConsumerAckWait, "30s")),
		ConsumerMaxDeliver:               getenvInt(envTelemetryConsumerMaxDeliver, 12),
		ConsumerBatchSize:                getenvInt(envTelemetryConsumerBatchSize, 32),
		ConsumerPullTimeout:              mustParseDuration(envTelemetryConsumerPullTimeout, getenv(envTelemetryConsumerPullTimeout, "2s")),
		ProjectionMaxConcurrency:         getenvInt(envTelemetryProjectionMaxConcurrent, 6),
		ProjectionDedupeLRUSize:          getenvInt(envTelemetryProjectionDedupeLRUSize, 100_000),
		ReadinessMaxPending:              getenvInt64(envTelemetryReadinessMaxPending, 0),
		ReadinessMaxProjectionFailStreak: getenvInt(envTelemetryReadinessMaxFailStreak, 0),
		ConsumerLagPollInterval:          mustParseDuration(envTelemetryConsumerLagPollInterval, getenv(envTelemetryConsumerLagPollInterval, "15s")),
	}
}

func (t TelemetryJetStreamConfig) validate() error {
	if t.StreamMaxBytes < 1<<20 || t.StreamMaxBytes > 1<<40 {
		return fmt.Errorf("config: %s must be between 1MiB and 1TiB", envTelemetryStreamMaxBytes)
	}
	if t.StreamMaxAgeBaseline < time.Hour || t.StreamMaxAgeBaseline > 30*24*time.Hour {
		return fmt.Errorf("config: %s must be between 1h and 720h", envTelemetryStreamMaxAge)
	}
	if t.ConsumerMaxAckPending < 64 || t.ConsumerMaxAckPending > 100_000 {
		return fmt.Errorf("config: %s out of range [64,100000]", envTelemetryConsumerMaxAckPending)
	}
	if t.ConsumerAckWait < 5*time.Second || t.ConsumerAckWait > 10*time.Minute {
		return fmt.Errorf("config: %s out of range", envTelemetryConsumerAckWait)
	}
	if t.ConsumerMaxDeliver < 2 || t.ConsumerMaxDeliver > 100 {
		return fmt.Errorf("config: %s out of range [2,100]", envTelemetryConsumerMaxDeliver)
	}
	if t.ConsumerBatchSize < 1 || t.ConsumerBatchSize > 512 {
		return fmt.Errorf("config: %s out of range [1,512]", envTelemetryConsumerBatchSize)
	}
	if t.ConsumerPullTimeout < 250*time.Millisecond || t.ConsumerPullTimeout > 60*time.Second {
		return fmt.Errorf("config: %s out of range", envTelemetryConsumerPullTimeout)
	}
	if t.ProjectionMaxConcurrency < 1 || t.ProjectionMaxConcurrency > 128 {
		return fmt.Errorf("config: %s out of range [1,128]", envTelemetryProjectionMaxConcurrent)
	}
	if t.ProjectionDedupeLRUSize < 1024 || t.ProjectionDedupeLRUSize > 5_000_000 {
		return fmt.Errorf("config: %s out of range [1024,5000000]", envTelemetryProjectionDedupeLRUSize)
	}
	if t.ReadinessMaxPending < 0 {
		return errors.New("config: TELEMETRY_READINESS_MAX_PENDING must be >= 0")
	}
	if t.ReadinessMaxProjectionFailStreak < 0 {
		return errors.New("config: TELEMETRY_READINESS_MAX_PROJECTION_FAIL_STREAK must be >= 0")
	}
	if t.ConsumerLagPollInterval < time.Second || t.ConsumerLagPollInterval > 5*time.Minute {
		return fmt.Errorf("config: %s out of range", envTelemetryConsumerLagPollInterval)
	}
	return nil
}

// NATSBrokerLimits maps this config into JetStream ensure helpers (cmd/worker, mqtt-ingest).
func (t TelemetryJetStreamConfig) NATSBrokerLimits() platformnats.TelemetryBrokerLimits {
	return platformnats.TelemetryBrokerLimits{
		StreamMaxBytes:        t.StreamMaxBytes,
		StreamMaxAgeBaseline:  t.StreamMaxAgeBaseline,
		ConsumerMaxAckPending: t.ConsumerMaxAckPending,
		ConsumerAckWait:       t.ConsumerAckWait,
		ConsumerMaxDeliver:    t.ConsumerMaxDeliver,
		ConsumerFetchBatch:    t.ConsumerBatchSize,
		ConsumerFetchMaxWait:  t.ConsumerPullTimeout,
	}
}
