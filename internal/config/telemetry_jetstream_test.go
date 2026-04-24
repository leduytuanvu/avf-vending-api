package config

import (
	"testing"
	"time"

	platformnats "github.com/avf/avf-vending-api/internal/platform/nats"
)

func TestTelemetryJetStreamConfig_NATSBrokerLimits(t *testing.T) {
	t.Parallel()
	c := TelemetryJetStreamConfig{
		StreamMaxBytes:        64 << 20,
		StreamMaxAgeBaseline:  72 * time.Hour,
		ConsumerMaxAckPending: 512,
		ConsumerAckWait:       20 * time.Second,
		ConsumerMaxDeliver:    10,
		ConsumerBatchSize:     16,
		ConsumerPullTimeout:   1500 * time.Millisecond,
	}
	lim := c.NATSBrokerLimits()
	if lim.StreamMaxBytes != 64<<20 {
		t.Fatalf("bytes: %d", lim.StreamMaxBytes)
	}
	if lim.ConsumerMaxAckPending != 512 {
		t.Fatalf("maxack: %d", lim.ConsumerMaxAckPending)
	}
	if lim.ConsumerFetchBatch != 16 {
		t.Fatalf("batch: %d", lim.ConsumerFetchBatch)
	}
}

func TestTelemetryJetStreamConfig_validate_StreamMaxBytes(t *testing.T) {
	t.Parallel()
	base := TelemetryJetStreamConfig{
		StreamMaxBytes:                   platformnats.TelemetryStreamLegacyDefaultMaxBytes,
		StreamMaxAgeBaseline:             168 * time.Hour,
		ConsumerMaxAckPending:            1024,
		ConsumerAckWait:                  30 * time.Second,
		ConsumerMaxDeliver:               12,
		ConsumerBatchSize:                32,
		ConsumerPullTimeout:              2 * time.Second,
		ProjectionMaxConcurrency:         6,
		ProjectionDedupeLRUSize:          100_000,
		ReadinessMaxPending:              0,
		ReadinessMaxProjectionFailStreak: 0,
		ConsumerLagPollInterval:          15 * time.Second,
	}
	if err := base.validate(); err != nil {
		t.Fatal(err)
	}
	bad := base
	bad.StreamMaxBytes = 512 * 1024 // < 1 MiB
	if err := bad.validate(); err == nil {
		t.Fatal("expected error for stream max bytes below minimum")
	}
}

func TestTelemetryJetStreamConfig_validate_ConsumerOutOfRange(t *testing.T) {
	t.Parallel()
	base := TelemetryJetStreamConfig{
		StreamMaxBytes:                   64 << 20,
		StreamMaxAgeBaseline:             168 * time.Hour,
		ConsumerMaxAckPending:            1024,
		ConsumerAckWait:                  30 * time.Second,
		ConsumerMaxDeliver:               12,
		ConsumerBatchSize:                32,
		ConsumerPullTimeout:              2 * time.Second,
		ProjectionMaxConcurrency:         6,
		ProjectionDedupeLRUSize:          100_000,
		ReadinessMaxPending:              0,
		ReadinessMaxProjectionFailStreak: 0,
		ConsumerLagPollInterval:          15 * time.Second,
	}
	if err := base.validate(); err != nil {
		t.Fatal(err)
	}
	bad := base
	bad.ConsumerMaxAckPending = 32
	if err := bad.validate(); err == nil {
		t.Fatal("expected error for max ack pending below minimum")
	}
}
