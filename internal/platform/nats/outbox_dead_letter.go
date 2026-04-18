package nats

import (
	"context"
	"fmt"
	"strings"

	domaincommerce "github.com/avf/avf-vending-api/internal/domain/commerce"
	natssrv "github.com/nats-io/nats.go"
)

const outboxDLQHeaderErrMax = 512

func truncateDLQHeaderErr(s string) string {
	s = strings.TrimSpace(s)
	if len(s) <= outboxDLQHeaderErrMax {
		return s
	}
	return s[:outboxDLQHeaderErrMax]
}

// PublishOutboxWorkerDeadLetter publishes a terminal copy to the internal DLQ stream after Postgres
// has quarantined the row (dead_lettered_at set). Uses a fresh Nats-Msg-Id so JetStream dedupe on the
// primary outbox stream is unchanged; DLQ stream has its own dedupe window.
func PublishOutboxWorkerDeadLetter(ctx context.Context, js natssrv.JetStreamContext, ev domaincommerce.OutboxEvent, lastPublishError string) error {
	if js == nil {
		return fmt.Errorf("nats: nil jetstream for outbox dead letter")
	}
	h := outboxHeaders(ev)
	// Distinct from live outbox publishes (publisher_outbox.go) so DLQ replays do not collide with AVF_INTERNAL_OUTBOX dedupe.
	h.Set("Nats-Msg-Id", fmt.Sprintf("outbox-dlq-%d", ev.ID))
	h.Set("X-Outbox-Dead-Letter", "true")
	h.Set("X-Outbox-Last-Error", truncateDLQHeaderErr(lastPublishError))
	return PublishDLQ(ctx, js, "outbox_publish_exhausted", h, ev.Payload)
}

// OutboxDeadLetterJetStream publishes quarantined outbox rows to AVF_INTERNAL_DLQ (cmd/worker hook).
type OutboxDeadLetterJetStream struct {
	JS natssrv.JetStreamContext
}

// NewOutboxDeadLetterJetStream returns nil when js is nil.
func NewOutboxDeadLetterJetStream(js natssrv.JetStreamContext) *OutboxDeadLetterJetStream {
	if js == nil {
		return nil
	}
	return &OutboxDeadLetterJetStream{JS: js}
}

// PublishOutboxDeadLettered implements the worker OutboxDeadLetterPublisher contract.
func (p *OutboxDeadLetterJetStream) PublishOutboxDeadLettered(ctx context.Context, ev domaincommerce.OutboxEvent, lastPublishError string) error {
	if p == nil || p.JS == nil {
		return fmt.Errorf("nats: nil outbox dead letter publisher")
	}
	return PublishOutboxWorkerDeadLetter(ctx, p.JS, ev, lastPublishError)
}
