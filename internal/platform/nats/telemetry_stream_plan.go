package nats

import (
	"time"

	natssrv "github.com/nats-io/nats.go"
)

// TelemetryStreamPlan describes effective JetStream limits for one telemetry stream after normalization.
type TelemetryStreamPlan struct {
	Name       string
	Subjects   []string
	MaxBytes   int64
	MaxAge     time.Duration
	Retention  natssrv.RetentionPolicy
	Discard    natssrv.DiscardPolicy
	Storage    natssrv.StorageType
	Duplicates time.Duration
}

// TelemetryStreamRetentionPlan returns the bounded stream definitions mqtt-ingest applies (idempotent ensure).
func TelemetryStreamRetentionPlan(lim TelemetryBrokerLimits) []TelemetryStreamPlan {
	lim = normalizeTelemetryBrokerLimits(lim)
	b := lim.StreamMaxAgeBaseline
	dupShort := 30 * time.Second
	dupMed := 2 * time.Minute
	dupLong := 5 * time.Minute
	return []TelemetryStreamPlan{
		{
			Name:       StreamTelemetryHeartbeat,
			Subjects:   []string{SubjectTelemetryPrefix + "heartbeat.>"},
			MaxBytes:   lim.StreamMaxBytes,
			MaxAge:     streamMaxAge(b, 2),
			Retention:  natssrv.LimitsPolicy,
			Discard:    natssrv.DiscardOld,
			Storage:    natssrv.FileStorage,
			Duplicates: dupShort,
		},
		{
			Name:       StreamTelemetryState,
			Subjects:   []string{SubjectTelemetryPrefix + "state.>"},
			MaxBytes:   lim.StreamMaxBytes,
			MaxAge:     streamMaxAge(b, 6),
			Retention:  natssrv.LimitsPolicy,
			Discard:    natssrv.DiscardOld,
			Storage:    natssrv.FileStorage,
			Duplicates: dupShort,
		},
		{
			Name:       StreamTelemetryMetrics,
			Subjects:   []string{SubjectTelemetryPrefix + "metrics.>"},
			MaxBytes:   lim.StreamMaxBytes,
			MaxAge:     streamMaxAge(b, 6),
			Retention:  natssrv.LimitsPolicy,
			Discard:    natssrv.DiscardOld,
			Storage:    natssrv.FileStorage,
			Duplicates: dupShort,
		},
		{
			Name:       StreamTelemetryIncidents,
			Subjects:   []string{SubjectTelemetryPrefix + "incident.>"},
			MaxBytes:   lim.StreamMaxBytes,
			MaxAge:     streamMaxAge(b, 24),
			Retention:  natssrv.LimitsPolicy,
			Discard:    natssrv.DiscardOld,
			Storage:    natssrv.FileStorage,
			Duplicates: dupMed,
		},
		{
			Name:       StreamTelemetryCommandReceipts,
			Subjects:   []string{SubjectTelemetryPrefix + "command_receipt.>"},
			MaxBytes:   lim.StreamMaxBytes,
			MaxAge:     streamMaxAge(b, 72),
			Retention:  natssrv.LimitsPolicy,
			Discard:    natssrv.DiscardOld,
			Storage:    natssrv.FileStorage,
			Duplicates: dupMed,
		},
		{
			Name:       StreamTelemetryDiagnosticBundleReady,
			Subjects:   []string{SubjectTelemetryPrefix + "diagnostic_bundle_ready.>"},
			MaxBytes:   lim.StreamMaxBytes,
			MaxAge:     streamMaxAge(b, 168),
			Retention:  natssrv.LimitsPolicy,
			Discard:    natssrv.DiscardOld,
			Storage:    natssrv.FileStorage,
			Duplicates: dupLong,
		},
	}
}

func planToStreamSpec(p TelemetryStreamPlan) streamSpec {
	return streamSpec{
		Name:       p.Name,
		Subjects:   p.Subjects,
		Retention:  p.Retention,
		MaxAge:     p.MaxAge,
		MaxBytes:   p.MaxBytes,
		Discard:    p.Discard,
		Storage:    p.Storage,
		Duplicates: p.Duplicates,
	}
}
