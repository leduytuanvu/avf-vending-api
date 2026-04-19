package nats

import (
	"fmt"
	"time"

	natssrv "github.com/nats-io/nats.go"
	"github.com/google/uuid"

	"github.com/avf/avf-vending-api/internal/platform/telemetry"
)

// Telemetry stream names and subject prefix.
const (
	StreamTelemetryHeartbeat             = "AVF_TELEMETRY_HEARTBEAT"
	StreamTelemetryState                 = "AVF_TELEMETRY_STATE"
	StreamTelemetryMetrics               = "AVF_TELEMETRY_METRICS"
	StreamTelemetryIncidents             = "AVF_TELEMETRY_INCIDENTS"
	StreamTelemetryCommandReceipts       = "AVF_TELEMETRY_COMMAND_RECEIPTS"
	StreamTelemetryDiagnosticBundleReady = "AVF_TELEMETRY_DIAGNOSTIC_READY"

	SubjectTelemetryPrefix = "avf.telemetry."
)

// TelemetryStreamName returns the JetStream stream backing a telemetry class.
func TelemetryStreamName(c telemetry.Class) (string, error) {
	switch c {
	case telemetry.ClassHeartbeat:
		return StreamTelemetryHeartbeat, nil
	case telemetry.ClassState:
		return StreamTelemetryState, nil
	case telemetry.ClassMetrics:
		return StreamTelemetryMetrics, nil
	case telemetry.ClassIncident:
		return StreamTelemetryIncidents, nil
	case telemetry.ClassCommandReceipt:
		return StreamTelemetryCommandReceipts, nil
	case telemetry.ClassDiagnosticBundleReady:
		return StreamTelemetryDiagnosticBundleReady, nil
	default:
		return "", fmt.Errorf("nats: unsupported telemetry class %q", c)
	}
}

// TelemetrySubject builds a subject for a class + machine (wildcard stream still matches).
func TelemetrySubject(c telemetry.Class, machineID uuid.UUID) (string, error) {
	if machineID == uuid.Nil {
		return "", fmt.Errorf("nats: machine_id required for telemetry subject")
	}
	switch c {
	case telemetry.ClassHeartbeat:
		return fmt.Sprintf("%sheartbeat.%s", SubjectTelemetryPrefix, machineID.String()), nil
	case telemetry.ClassState:
		return fmt.Sprintf("%sstate.%s", SubjectTelemetryPrefix, machineID.String()), nil
	case telemetry.ClassMetrics:
		return fmt.Sprintf("%smetrics.%s", SubjectTelemetryPrefix, machineID.String()), nil
	case telemetry.ClassIncident:
		return fmt.Sprintf("%sincident.%s", SubjectTelemetryPrefix, machineID.String()), nil
	case telemetry.ClassCommandReceipt:
		return fmt.Sprintf("%scommand_receipt.%s", SubjectTelemetryPrefix, machineID.String()), nil
	case telemetry.ClassDiagnosticBundleReady:
		return fmt.Sprintf("%sdiagnostic_bundle_ready.%s", SubjectTelemetryPrefix, machineID.String()), nil
	default:
		return "", fmt.Errorf("nats: unsupported telemetry class %q", c)
	}
}

// EnsureTelemetryStreams creates or updates bounded telemetry streams (idempotent).
func EnsureTelemetryStreams(js natssrv.JetStreamContext, lim TelemetryBrokerLimits) error {
	if js == nil {
		return fmt.Errorf("nats: nil jetstream context")
	}
	lim = normalizeTelemetryBrokerLimits(lim)
	b := lim.StreamMaxAgeBaseline
	base := []streamSpec{
		{
			Name:       StreamTelemetryHeartbeat,
			Subjects:   []string{SubjectTelemetryPrefix + "heartbeat.>"},
			Retention:  natssrv.LimitsPolicy,
			MaxAge:     streamMaxAge(b, 2),
			MaxBytes:   lim.StreamMaxBytes,
			Discard:    natssrv.DiscardOld,
			Storage:    natssrv.FileStorage,
			Duplicates: 30 * time.Second,
		},
		{
			Name:       StreamTelemetryState,
			Subjects:   []string{SubjectTelemetryPrefix + "state.>"},
			Retention:  natssrv.LimitsPolicy,
			MaxAge:     streamMaxAge(b, 6),
			MaxBytes:   lim.StreamMaxBytes,
			Discard:    natssrv.DiscardOld,
			Storage:    natssrv.FileStorage,
			Duplicates: 30 * time.Second,
		},
		{
			Name:       StreamTelemetryMetrics,
			Subjects:   []string{SubjectTelemetryPrefix + "metrics.>"},
			Retention:  natssrv.LimitsPolicy,
			MaxAge:     streamMaxAge(b, 6),
			MaxBytes:   lim.StreamMaxBytes,
			Discard:    natssrv.DiscardOld,
			Storage:    natssrv.FileStorage,
			Duplicates: 30 * time.Second,
		},
		{
			Name:       StreamTelemetryIncidents,
			Subjects:   []string{SubjectTelemetryPrefix + "incident.>"},
			Retention:  natssrv.LimitsPolicy,
			MaxAge:     streamMaxAge(b, 24),
			MaxBytes:   lim.StreamMaxBytes,
			Discard:    natssrv.DiscardOld,
			Storage:    natssrv.FileStorage,
			Duplicates: 2 * time.Minute,
		},
		{
			Name:       StreamTelemetryCommandReceipts,
			Subjects:   []string{SubjectTelemetryPrefix + "command_receipt.>"},
			Retention:  natssrv.LimitsPolicy,
			MaxAge:     streamMaxAge(b, 72),
			MaxBytes:   lim.StreamMaxBytes,
			Discard:    natssrv.DiscardOld,
			Storage:    natssrv.FileStorage,
			Duplicates: 2 * time.Minute,
		},
		{
			Name:       StreamTelemetryDiagnosticBundleReady,
			Subjects:   []string{SubjectTelemetryPrefix + "diagnostic_bundle_ready.>"},
			Retention:  natssrv.LimitsPolicy,
			MaxAge:     streamMaxAge(b, 168),
			MaxBytes:   lim.StreamMaxBytes,
			Discard:    natssrv.DiscardOld,
			Storage:    natssrv.FileStorage,
			Duplicates: 5 * time.Minute,
		},
	}
	for _, s := range base {
		if err := ensureStream(js, s); err != nil {
			return err
		}
	}
	return nil
}

// PublishTelemetry publishes a JSON envelope to the stream for class with optional dedupe header.
func PublishTelemetry(js natssrv.JetStreamContext, c telemetry.Class, machineID uuid.UUID, body []byte, dedupe string) error {
	if js == nil {
		return fmt.Errorf("nats: nil jetstream context")
	}
	subj, err := TelemetrySubject(c, machineID)
	if err != nil {
		return err
	}
	opts := []natssrv.PubOpt{}
	if dedupe != "" {
		opts = append(opts, natssrv.MsgId(dedupe))
	}
	_, err = js.Publish(subj, body, opts...)
	return err
}
