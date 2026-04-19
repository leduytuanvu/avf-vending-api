package nats

import (
	"testing"
	"time"

	natssrv "github.com/nats-io/nats.go"
)

func TestTelemetryConsumerConfigMatches(t *testing.T) {
	t.Parallel()
	a := &natssrv.ConsumerConfig{
		Durable:       "d",
		FilterSubject: "sub.>",
		AckPolicy:     natssrv.AckExplicitPolicy,
		DeliverPolicy: natssrv.DeliverAllPolicy,
		AckWait:       30 * time.Second,
		MaxAckPending: 1024,
		MaxDeliver:    12,
	}
	b := &natssrv.ConsumerConfig{
		Durable:       "d",
		FilterSubject: "sub.>",
		AckPolicy:     natssrv.AckExplicitPolicy,
		DeliverPolicy: natssrv.DeliverAllPolicy,
		AckWait:       30 * time.Second,
		MaxAckPending: 1024,
		MaxDeliver:    12,
	}
	if !telemetryConsumerConfigMatches(a, b) {
		t.Fatal("expected match")
	}
	b.MaxAckPending = 512
	if telemetryConsumerConfigMatches(a, b) {
		t.Fatal("expected mismatch")
	}
}
