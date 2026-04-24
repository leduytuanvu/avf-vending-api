package nats

import (
	"fmt"

	"github.com/google/uuid"
	natssrv "github.com/nats-io/nats.go"

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
	for _, p := range TelemetryStreamRetentionPlan(lim) {
		if err := ensureStream(js, planToStreamSpec(p)); err != nil {
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
