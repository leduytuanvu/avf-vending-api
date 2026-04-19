package config

import (
	"testing"
	"time"
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
