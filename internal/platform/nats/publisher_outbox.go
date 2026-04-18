package nats

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	domaincommerce "github.com/avf/avf-vending-api/internal/domain/commerce"
	"github.com/google/uuid"
	natssrv "github.com/nats-io/nats.go"
)

// JetStreamOutboxPublisher publishes outbox rows to JetStream with correlation headers.
//
// P0: keep Nats-Msg-Id stable for a logical outbox event; changing the header strategy breaks
// broker-side dedupe and can double-charge downstream consumers after outbox worker retries.
type JetStreamOutboxPublisher struct {
	JS natssrv.JetStreamContext
}

// NewJetStreamOutboxPublisher builds a commerce OutboxPublisher backed by JetStream.
func NewJetStreamOutboxPublisher(js natssrv.JetStreamContext) *JetStreamOutboxPublisher {
	return &JetStreamOutboxPublisher{JS: js}
}

var _ domaincommerce.OutboxPublisher = (*JetStreamOutboxPublisher)(nil)

// Publish implements domaincommerce.OutboxPublisher.
//
// Transport-level idempotency: Nats-Msg-Id is set from the outbox idempotency key when present, otherwise
// "outbox-<id>". That stabilizes JetStream dedupe if Postgres marks published_at late or a worker retries
// after a partial failure (publish ok, mark not persisted).
func (p *JetStreamOutboxPublisher) Publish(ctx context.Context, ev domaincommerce.OutboxEvent) error {
	if p == nil || p.JS == nil {
		return fmt.Errorf("nats: nil jetstream outbox publisher")
	}
	subj := OutboxPublishSubject(ev)
	msg := &natssrv.Msg{
		Subject: subj,
		Data:    ev.Payload,
		Header:  outboxHeaders(ev),
	}
	opts := []natssrv.PubOpt{}
	if ctx != nil {
		opts = append(opts, natssrv.Context(ctx))
	}
	_, err := p.JS.PublishMsg(msg, opts...)
	if err != nil {
		return fmt.Errorf("nats: jetstream publish %s: %w", subj, err)
	}
	return nil
}

func outboxHeaders(ev domaincommerce.OutboxEvent) natssrv.Header {
	h := natssrv.Header{}
	h.Set("X-Outbox-Id", strconv.FormatInt(ev.ID, 10))
	h.Set("X-Event-Type", strings.TrimSpace(ev.EventType))
	h.Set("X-Aggregate-Type", strings.TrimSpace(ev.AggregateType))
	h.Set("X-Aggregate-Id", ev.AggregateID.String())
	if ev.OrganizationID != nil {
		h.Set("X-Organization-Id", ev.OrganizationID.String())
	} else {
		h.Set("X-Organization-Id", uuid.Nil.String())
	}
	h.Set("X-Correlation-Id", correlationID(ev))
	if ev.IdempotencyKey != nil && strings.TrimSpace(*ev.IdempotencyKey) != "" {
		h.Set("Nats-Msg-Id", strings.TrimSpace(*ev.IdempotencyKey))
	} else {
		h.Set("Nats-Msg-Id", fmt.Sprintf("outbox-%d", ev.ID))
	}
	if strings.TrimSpace(ev.Topic) != "" {
		h.Set("X-Outbox-Topic", strings.TrimSpace(ev.Topic))
	}
	return h
}

func correlationID(ev domaincommerce.OutboxEvent) string {
	base := fmt.Sprintf("outbox-%d", ev.ID)
	if ev.IdempotencyKey != nil && strings.TrimSpace(*ev.IdempotencyKey) != "" {
		return base + "|idem=" + strings.TrimSpace(*ev.IdempotencyKey)
	}
	if ev.AggregateID != uuid.Nil {
		return base + "|agg=" + ev.AggregateID.String()
	}
	return base
}
