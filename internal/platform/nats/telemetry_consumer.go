package nats

import (
	"errors"
	"fmt"
	"time"

	natssrv "github.com/nats-io/nats.go"
)

// telemetryConsumerSpec registers a durable pull consumer per telemetry stream.
type telemetryConsumerSpec struct {
	Stream   string
	Durable  string
	Filter   string
	AckWait  int // seconds
	MaxAck   int
	MaxDeliv int
}

// EnsureTelemetryDurableConsumers registers pull consumers for all telemetry streams (idempotent).
func EnsureTelemetryDurableConsumers(js natssrv.JetStreamContext) error {
	if js == nil {
		return fmt.Errorf("nats: nil jetstream context")
	}
	specs := []telemetryConsumerSpec{
		{Stream: StreamTelemetryHeartbeat, Durable: "avf-w-telemetry-heartbeat", Filter: SubjectTelemetryPrefix + "heartbeat.>", AckWait: 30, MaxAck: 2048, MaxDeliv: 12},
		{Stream: StreamTelemetryState, Durable: "avf-w-telemetry-state", Filter: SubjectTelemetryPrefix + "state.>", AckWait: 30, MaxAck: 2048, MaxDeliv: 12},
		{Stream: StreamTelemetryMetrics, Durable: "avf-w-telemetry-metrics", Filter: SubjectTelemetryPrefix + "metrics.>", AckWait: 30, MaxAck: 4096, MaxDeliv: 12},
		{Stream: StreamTelemetryIncidents, Durable: "avf-w-telemetry-incidents", Filter: SubjectTelemetryPrefix + "incident.>", AckWait: 30, MaxAck: 1024, MaxDeliv: 12},
		{Stream: StreamTelemetryCommandReceipts, Durable: "avf-w-telemetry-command-receipts", Filter: SubjectTelemetryPrefix + "command_receipt.>", AckWait: 30, MaxAck: 2048, MaxDeliv: 12},
		{Stream: StreamTelemetryDiagnosticBundleReady, Durable: "avf-w-telemetry-diagnostic", Filter: SubjectTelemetryPrefix + "diagnostic_bundle_ready.>", AckWait: 60, MaxAck: 512, MaxDeliv: 8},
	}
	for _, s := range specs {
		cfg := &natssrv.ConsumerConfig{
			Durable:       s.Durable,
			AckPolicy:     natssrv.AckExplicitPolicy,
			AckWait:       time.Duration(s.AckWait) * time.Second,
			MaxAckPending: uint64(s.MaxAck),
			MaxDeliver:    s.MaxDeliv,
			FilterSubject: s.Filter,
			DeliverPolicy: natssrv.DeliverAllPolicy,
		}
		if _, err := js.ConsumerInfo(s.Stream, s.Durable); err == nil {
			continue
		} else if !errors.Is(err, natssrv.ErrConsumerNotFound) {
			return fmt.Errorf("nats: consumer info %s/%s: %w", s.Stream, s.Durable, err)
		}
		if _, err := js.AddConsumer(s.Stream, cfg); err != nil {
			return fmt.Errorf("nats: add consumer %s/%s: %w", s.Stream, s.Durable, err)
		}
	}
	return nil
}
