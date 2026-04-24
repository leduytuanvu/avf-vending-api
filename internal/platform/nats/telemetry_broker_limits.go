package nats

import "time"

// TelemetryStreamLegacyDefaultMaxBytes is the historical per-stream MaxBytes (256 MiB) when TELEMETRY_STREAM_MAX_BYTES is unset.
const TelemetryStreamLegacyDefaultMaxBytes int64 = 256 << 20

// TelemetryBrokerLimits configures JetStream telemetry stream retention and pull consumer fetch tuning.
// Zero values in EnsureTelemetryStreams / EnsureTelemetryDurableConsumers are replaced with
// DefaultTelemetryBrokerLimits() (including legacy 256 MiB per-stream bytes). Production processes should set
// explicit TELEMETRY_* env vars so limits are not accidental; mqtt-ingest and worker log effective values at startup.
type TelemetryBrokerLimits struct {
	StreamMaxBytes        int64
	StreamMaxAgeBaseline  time.Duration
	ConsumerMaxAckPending int
	ConsumerAckWait       time.Duration
	ConsumerMaxDeliver    int
	ConsumerFetchBatch    int
	ConsumerFetchMaxWait  time.Duration
}

// DefaultTelemetryBrokerLimits matches the historical hardcoded telemetry profile (conservative VPS).
func DefaultTelemetryBrokerLimits() TelemetryBrokerLimits {
	return TelemetryBrokerLimits{
		StreamMaxBytes:        TelemetryStreamLegacyDefaultMaxBytes,
		StreamMaxAgeBaseline:  168 * time.Hour, // 7d — longest stream (diagnostic)
		ConsumerMaxAckPending: 1024,
		ConsumerAckWait:       30 * time.Second,
		ConsumerMaxDeliver:    12,
		ConsumerFetchBatch:    32,
		ConsumerFetchMaxWait:  2 * time.Second,
	}
}

// NormalizeTelemetryBrokerLimits fills zero fields with defaults.
func NormalizeTelemetryBrokerLimits(l TelemetryBrokerLimits) TelemetryBrokerLimits {
	return normalizeTelemetryBrokerLimits(l)
}

func normalizeTelemetryBrokerLimits(l TelemetryBrokerLimits) TelemetryBrokerLimits {
	d := DefaultTelemetryBrokerLimits()
	if l.StreamMaxBytes <= 0 {
		l.StreamMaxBytes = d.StreamMaxBytes
	}
	if l.StreamMaxAgeBaseline <= 0 {
		l.StreamMaxAgeBaseline = d.StreamMaxAgeBaseline
	}
	if l.ConsumerMaxAckPending <= 0 {
		l.ConsumerMaxAckPending = d.ConsumerMaxAckPending
	}
	if l.ConsumerAckWait <= 0 {
		l.ConsumerAckWait = d.ConsumerAckWait
	}
	if l.ConsumerMaxDeliver <= 0 {
		l.ConsumerMaxDeliver = d.ConsumerMaxDeliver
	}
	if l.ConsumerFetchBatch <= 0 {
		l.ConsumerFetchBatch = d.ConsumerFetchBatch
	}
	if l.ConsumerFetchMaxWait <= 0 {
		l.ConsumerFetchMaxWait = d.ConsumerFetchMaxWait
	}
	return l
}

// streamMaxAge scales MaxAge relative to the longest stream (7d baseline = 168h).
// Ratios match the pre-config defaults: 2h,6h,6h,24h,72h,168h.
func streamMaxAge(baseline time.Duration, fractionOf168h float64) time.Duration {
	if fractionOf168h <= 0 {
		return 5 * time.Minute
	}
	age := time.Duration(float64(baseline) * fractionOf168h / 168.0)
	if age < 5*time.Minute {
		return 5 * time.Minute
	}
	return age
}
