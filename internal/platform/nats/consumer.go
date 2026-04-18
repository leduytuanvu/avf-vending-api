package nats

import (
	"errors"
	"fmt"
	"time"

	natssrv "github.com/nats-io/nats.go"
)

// ConsumerRetryDefaults is a DLQ-friendly pull-consumer profile (explicit ack + bounded redelivery).
type ConsumerRetryDefaults struct {
	AckWait    time.Duration
	MaxDeliver int
	Backoff    []time.Duration
}

// DefaultConsumerRetryDefaults returns conservative redelivery tuning.
func DefaultConsumerRetryDefaults() ConsumerRetryDefaults {
	return ConsumerRetryDefaults{
		AckWait:    30 * time.Second,
		MaxDeliver: 8,
		Backoff: []time.Duration{
			1 * time.Second,
			5 * time.Second,
			30 * time.Second,
			2 * time.Minute,
		},
	}
}

// EnsureOutboxPullConsumer registers a durable pull consumer on the internal outbox stream.
func EnsureOutboxPullConsumer(js natssrv.JetStreamContext, durableName string, d ConsumerRetryDefaults) error {
	if js == nil {
		return fmt.Errorf("nats: nil jetstream")
	}
	if durableName == "" {
		return fmt.Errorf("nats: durable name required")
	}
	if d.AckWait <= 0 {
		d = DefaultConsumerRetryDefaults()
	}
	if d.MaxDeliver <= 0 {
		d.MaxDeliver = 8
	}
	if len(d.Backoff) == 0 {
		d.Backoff = DefaultConsumerRetryDefaults().Backoff
	}
	cfg := &natssrv.ConsumerConfig{
		Durable:       durableName,
		AckPolicy:     natssrv.AckExplicitPolicy,
		AckWait:       d.AckWait,
		MaxDeliver:    d.MaxDeliver,
		BackOff:       d.Backoff,
		FilterSubject: SubjectPatternOutbox,
		DeliverPolicy: natssrv.DeliverAllPolicy,
	}
	if _, err := js.ConsumerInfo(StreamOutbox, durableName); err == nil {
		return nil
	} else if !errors.Is(err, natssrv.ErrConsumerNotFound) {
		return fmt.Errorf("nats: consumer info %q: %w", durableName, err)
	}
	_, err := js.AddConsumer(StreamOutbox, cfg)
	if err != nil {
		return fmt.Errorf("nats: add outbox consumer %q: %w", durableName, err)
	}
	return nil
}

// BindOutboxPull returns a pull subscription bound to the outbox stream (caller Fetch/Ack's explicitly).
func BindOutboxPull(nc *natssrv.Conn, durableName string) (*natssrv.Subscription, error) {
	if nc == nil {
		return nil, fmt.Errorf("nats: nil connection")
	}
	js, err := nc.JetStream()
	if err != nil {
		return nil, fmt.Errorf("nats: jetstream: %w", err)
	}
	return js.PullSubscribe(SubjectPatternOutbox, durableName, natssrv.BindStream(StreamOutbox))
}
