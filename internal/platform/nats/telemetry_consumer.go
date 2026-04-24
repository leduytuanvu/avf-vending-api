package nats

import (
	"errors"
	"fmt"

	natssrv "github.com/nats-io/nats.go"
)

// telemetryConsumerSpec registers a durable pull consumer per telemetry stream.
type telemetryConsumerSpec struct {
	Stream  string
	Durable string
	Filter  string
}

// EnsureTelemetryDurableConsumers registers or updates pull consumers for all telemetry streams.
// Consumer AckWait, MaxAckPending, and MaxDeliver come from lim (TELEMETRY_CONSUMER_ACK_WAIT,
// TELEMETRY_CONSUMER_MAX_ACK_PENDING, TELEMETRY_CONSUMER_MAX_DELIVER); fetch batch size / max wait are used by the worker pull loop.
func EnsureTelemetryDurableConsumers(js natssrv.JetStreamContext, lim TelemetryBrokerLimits) error {
	if js == nil {
		return fmt.Errorf("nats: nil jetstream context")
	}
	lim = normalizeTelemetryBrokerLimits(lim)
	specs := []telemetryConsumerSpec{
		{Stream: StreamTelemetryHeartbeat, Durable: "avf-w-telemetry-heartbeat", Filter: SubjectTelemetryPrefix + "heartbeat.>"},
		{Stream: StreamTelemetryState, Durable: "avf-w-telemetry-state", Filter: SubjectTelemetryPrefix + "state.>"},
		{Stream: StreamTelemetryMetrics, Durable: "avf-w-telemetry-metrics", Filter: SubjectTelemetryPrefix + "metrics.>"},
		{Stream: StreamTelemetryIncidents, Durable: "avf-w-telemetry-incidents", Filter: SubjectTelemetryPrefix + "incident.>"},
		{Stream: StreamTelemetryCommandReceipts, Durable: "avf-w-telemetry-command-receipts", Filter: SubjectTelemetryPrefix + "command_receipt.>"},
		{Stream: StreamTelemetryDiagnosticBundleReady, Durable: "avf-w-telemetry-diagnostic", Filter: SubjectTelemetryPrefix + "diagnostic_bundle_ready.>"},
	}
	for _, s := range specs {
		cfg := &natssrv.ConsumerConfig{
			Durable:       s.Durable,
			AckPolicy:     natssrv.AckExplicitPolicy,
			AckWait:       lim.ConsumerAckWait,
			MaxAckPending: lim.ConsumerMaxAckPending,
			MaxDeliver:    lim.ConsumerMaxDeliver,
			FilterSubject: s.Filter,
			DeliverPolicy: natssrv.DeliverAllPolicy,
		}
		if err := ensureTelemetryConsumer(js, s.Stream, cfg); err != nil {
			return err
		}
	}
	return nil
}

func ensureTelemetryConsumer(js natssrv.JetStreamContext, stream string, want *natssrv.ConsumerConfig) error {
	info, err := js.ConsumerInfo(stream, want.Durable)
	if err != nil {
		if errors.Is(err, natssrv.ErrConsumerNotFound) {
			_, err = js.AddConsumer(stream, want)
			if err != nil {
				return fmt.Errorf("nats: add consumer %s/%s: %w", stream, want.Durable, err)
			}
			return nil
		}
		return fmt.Errorf("nats: consumer info %s/%s: %w", stream, want.Durable, err)
	}
	if telemetryConsumerConfigMatches(&info.Config, want) {
		return nil
	}
	if _, err := js.UpdateConsumer(stream, want); err != nil {
		return fmt.Errorf("nats: update consumer %s/%s: %w", stream, want.Durable, err)
	}
	return nil
}

func telemetryConsumerConfigMatches(have, want *natssrv.ConsumerConfig) bool {
	if have == nil || want == nil {
		return false
	}
	return have.Durable == want.Durable &&
		have.FilterSubject == want.FilterSubject &&
		have.AckPolicy == want.AckPolicy &&
		have.DeliverPolicy == want.DeliverPolicy &&
		have.AckWait == want.AckWait &&
		have.MaxAckPending == want.MaxAckPending &&
		have.MaxDeliver == want.MaxDeliver
}
