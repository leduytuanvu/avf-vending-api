package nats

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

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

// Health verifies the JetStream API used by the publisher is reachable.
func (p *JetStreamOutboxPublisher) Health(ctx context.Context) error {
	if p == nil || p.JS == nil {
		return fmt.Errorf("nats: nil jetstream outbox publisher")
	}
	_, err := p.JS.StreamInfo(StreamOutbox, natssrv.Context(ctx))
	if err != nil {
		return fmt.Errorf("nats: outbox stream health: %w", err)
	}
	return nil
}

// Publish implements domaincommerce.OutboxPublisher.
//
// Transport-level idempotency: Nats-Msg-Id is set from the outbox idempotency key when present, otherwise
// "outbox-<id>". That stabilizes JetStream dedupe if Postgres marks published_at late or a worker retries
// after a partial failure (publish ok, mark not persisted).
func (p *JetStreamOutboxPublisher) Publish(ctx context.Context, ev domaincommerce.OutboxEvent) error {
	if p == nil || p.JS == nil {
		return fmt.Errorf("nats: nil jetstream outbox publisher")
	}
	if err := ValidateOutboxEvent(ev); err != nil {
		return err
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
	h.Set("X-Outbox-Created-At", ev.CreatedAt.UTC().Format(time.RFC3339Nano))
	h.Set("X-Outbox-Publish-Attempts", strconv.FormatInt(int64(ev.PublishAttemptCount), 10))
	h.Set("X-Payload-Schema-Version", OutboxPayloadSchemaVersion(ev.Payload))
	if ev.PublishedAt != nil {
		h.Set("X-Outbox-Published-At", ev.PublishedAt.UTC().Format(time.RFC3339Nano))
	}
	if ev.LastPublishAttemptAt != nil {
		h.Set("X-Outbox-Last-Publish-Attempt-At", ev.LastPublishAttemptAt.UTC().Format(time.RFC3339Nano))
	}
	if ev.LastPublishError != nil && strings.TrimSpace(*ev.LastPublishError) != "" {
		h.Set("X-Outbox-Last-Error", truncateDLQHeaderErr(*ev.LastPublishError))
	}
	if ev.OrganizationID != nil {
		h.Set("X-Organization-Id", ev.OrganizationID.String())
	} else {
		h.Set("X-Organization-Id", uuid.Nil.String())
	}
	h.Set("X-Correlation-Id", correlationID(ev))
	h.Set("Nats-Msg-Id", OutboxEventKey(ev))
	if strings.TrimSpace(ev.Topic) != "" {
		h.Set("X-Outbox-Topic", strings.TrimSpace(ev.Topic))
	}
	return h
}

// OutboxEventKey returns the stable event key used for JetStream idempotent publish dedupe.
func OutboxEventKey(ev domaincommerce.OutboxEvent) string {
	if ev.IdempotencyKey != nil && strings.TrimSpace(*ev.IdempotencyKey) != "" {
		return strings.TrimSpace(*ev.IdempotencyKey)
	}
	return fmt.Sprintf("outbox-%d", ev.ID)
}

// OutboxPayloadSchemaVersion extracts the payload schema version from JSON payloads.
// Legacy JSON payloads predate an explicit version field, so they publish as schema version 1.
func OutboxPayloadSchemaVersion(payload []byte) string {
	var body map[string]any
	if err := json.Unmarshal(payload, &body); err != nil {
		return ""
	}
	for _, key := range []string{"schema_version", "schemaVersion", "payload_schema_version", "payloadSchemaVersion"} {
		if v, ok := body[key]; ok {
			switch x := v.(type) {
			case string:
				return strings.TrimSpace(x)
			case float64:
				if x == float64(int64(x)) {
					return strconv.FormatInt(int64(x), 10)
				}
			}
		}
	}
	return "1"
}

// ValidateOutboxEvent enforces the production outbox contract before any broker publish.
func ValidateOutboxEvent(ev domaincommerce.OutboxEvent) error {
	var missing []string
	if strings.TrimSpace(ev.AggregateType) == "" {
		missing = append(missing, "aggregate_type")
	}
	if ev.AggregateID == uuid.Nil {
		missing = append(missing, "aggregate_id")
	}
	if strings.TrimSpace(ev.EventType) == "" {
		missing = append(missing, "event_type")
	}
	if strings.TrimSpace(OutboxEventKey(ev)) == "" {
		missing = append(missing, "idempotency/event key")
	}
	if ev.CreatedAt.IsZero() {
		missing = append(missing, "created_at")
	}
	if strings.TrimSpace(OutboxPayloadSchemaVersion(ev.Payload)) == "" {
		missing = append(missing, "payload schema version")
	}
	if len(missing) > 0 {
		return fmt.Errorf("nats: invalid outbox event contract: missing %s", strings.Join(missing, ", "))
	}
	return nil
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
