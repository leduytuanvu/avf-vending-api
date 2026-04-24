package nats

import (
	"fmt"

	"go.uber.org/zap"
)

// LogTelemetryJetStreamRetention logs normalized stream retention and pull-consumer tuning at process startup.
// Call once per process before EnsureTelemetryStreams (mqtt-ingest, worker).
func LogTelemetryJetStreamRetention(log *zap.Logger, process string, appEnv string, lim TelemetryBrokerLimits) {
	if log == nil {
		return
	}
	lim = normalizeTelemetryBrokerLimits(lim)
	log.Info("jetstream_telemetry_retention_effective",
		zap.String("process", process),
		zap.String("app_env", appEnv),
		zap.Int64("stream_max_bytes_per_stream", lim.StreamMaxBytes),
		zap.Duration("stream_max_age_baseline", lim.StreamMaxAgeBaseline),
		zap.String("stream_discard_policy", "old"),
		zap.Int("consumer_max_ack_pending", lim.ConsumerMaxAckPending),
		zap.Duration("consumer_ack_wait", lim.ConsumerAckWait),
		zap.Int("consumer_max_deliver", lim.ConsumerMaxDeliver),
		zap.Int("consumer_fetch_batch_size", lim.ConsumerFetchBatch),
		zap.Duration("consumer_fetch_max_wait", lim.ConsumerFetchMaxWait),
	)
	for _, p := range TelemetryStreamRetentionPlan(lim) {
		log.Info("jetstream_telemetry_stream_limits",
			zap.String("process", process),
			zap.String("stream", p.Name),
			zap.Int64("max_bytes", p.MaxBytes),
			zap.Duration("max_age", p.MaxAge),
			zap.String("subjects", fmt.Sprintf("%v", p.Subjects)),
		)
	}
	if appEnv == "production" && lim.StreamMaxBytes <= TelemetryStreamLegacyDefaultMaxBytes {
		log.Warn("jetstream_telemetry_stream_max_bytes_at_or_below_legacy_default",
			zap.String("process", process),
			zap.Int64("stream_max_bytes", lim.StreamMaxBytes),
			zap.Int64("legacy_default_bytes", TelemetryStreamLegacyDefaultMaxBytes),
			zap.String("hint", "raise TELEMETRY_STREAM_MAX_BYTES for fleets above pilot and ensure JetStream volume headroom (see telemetry-jetstream-resilience.md)"),
		)
	}
}
