package clickhouse

import (
	"encoding/base64"
	"encoding/json"
	"time"

	domaincommerce "github.com/avf/avf-vending-api/internal/domain/commerce"
)

// outboxMirrorRow is the JSONEachRow payload for analytics mirror inserts.
type outboxMirrorRow struct {
	OutboxID       int64  `json:"outbox_id"`
	Topic          string `json:"topic"`
	EventType      string `json:"event_type"`
	AggregateType  string `json:"aggregate_type"`
	AggregateID    string `json:"aggregate_id"`
	OrganizationID string `json:"organization_id"`
	PayloadBase64  string `json:"payload_base64"`
	PublishedAt    string `json:"published_at"`
	IngestedAt     string `json:"ingested_at"`
	IdempotencyKey string `json:"idempotency_key"`
}

func mirrorRowFromOutboxEvent(ev domaincommerce.OutboxEvent) (line []byte, err error) {
	org := ""
	if ev.OrganizationID != nil {
		org = ev.OrganizationID.String()
	}
	pub := ""
	if ev.PublishedAt != nil {
		pub = ev.PublishedAt.UTC().Format(time.RFC3339Nano)
	}
	idem := ""
	if ev.IdempotencyKey != nil {
		idem = *ev.IdempotencyKey
	}
	r := outboxMirrorRow{
		OutboxID:       ev.ID,
		Topic:          ev.Topic,
		EventType:      ev.EventType,
		AggregateType:  ev.AggregateType,
		AggregateID:    ev.AggregateID.String(),
		OrganizationID: org,
		PayloadBase64:  base64.StdEncoding.EncodeToString(ev.Payload),
		PublishedAt:    pub,
		IngestedAt:     time.Now().UTC().Format(time.RFC3339Nano),
		IdempotencyKey: idem,
	}
	b, err := json.Marshal(r)
	if err != nil {
		return nil, err
	}
	// JSONEachRow: one object per line
	return append(b, '\n'), nil
}
