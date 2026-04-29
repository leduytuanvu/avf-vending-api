package clickhouse

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	domaincommerce "github.com/avf/avf-vending-api/internal/domain/commerce"
)

const (
	ProjectionTypeSales                   = "sales_event"
	ProjectionTypePayment                 = "payment_event"
	ProjectionTypeVend                    = "vend_event"
	ProjectionTypeInventoryDelta          = "inventory_delta"
	ProjectionTypeMachineTelemetrySummary = "machine_telemetry_summary"
	ProjectionTypeCommandLifecycle        = "command_lifecycle_event"
)

// ProjectionRow is the JSONEachRow payload for typed fleet/sales analytics projections.
type ProjectionRow struct {
	ProjectionID   string `json:"projection_id"`
	ProjectionType string `json:"projection_type"`
	Source         string `json:"source"`
	OutboxID       int64  `json:"outbox_id"`
	Topic          string `json:"topic"`
	EventType      string `json:"event_type"`
	AggregateType  string `json:"aggregate_type"`
	AggregateID    string `json:"aggregate_id"`
	OrganizationID string `json:"organization_id"`
	OccurredAt     string `json:"occurred_at"`
	PublishedAt    string `json:"published_at"`
	IngestedAt     string `json:"ingested_at"`
	IdempotencyKey string `json:"idempotency_key"`
	PayloadBase64  string `json:"payload_base64"`
}

func projectionRowsFromOutboxEvent(ev domaincommerce.OutboxEvent) ([][]byte, error) {
	projectionType, ok := classifyProjection(ev)
	if !ok {
		projectionSkipped.WithLabelValues("unclassified").Inc()
		return nil, nil
	}
	line, err := projectionRowFromOutboxEvent(ev, projectionType)
	if err != nil {
		return nil, err
	}
	return [][]byte{line}, nil
}

func projectionRowFromOutboxEvent(ev domaincommerce.OutboxEvent, projectionType string) ([]byte, error) {
	if strings.TrimSpace(projectionType) == "" {
		return nil, fmt.Errorf("clickhouse: projection type is required")
	}
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
		idem = strings.TrimSpace(*ev.IdempotencyKey)
	}
	occurredAt := eventTimeFromPayload(ev.Payload)
	if occurredAt == "" && !ev.CreatedAt.IsZero() {
		occurredAt = ev.CreatedAt.UTC().Format(time.RFC3339Nano)
	}
	row := ProjectionRow{
		ProjectionID:   projectionID(ev, projectionType),
		ProjectionType: projectionType,
		Source:         "outbox",
		OutboxID:       ev.ID,
		Topic:          ev.Topic,
		EventType:      ev.EventType,
		AggregateType:  ev.AggregateType,
		AggregateID:    ev.AggregateID.String(),
		OrganizationID: org,
		OccurredAt:     occurredAt,
		PublishedAt:    pub,
		IngestedAt:     time.Now().UTC().Format(time.RFC3339Nano),
		IdempotencyKey: idem,
		PayloadBase64:  base64.StdEncoding.EncodeToString(ev.Payload),
	}
	b, err := json.Marshal(row)
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

func projectionID(ev domaincommerce.OutboxEvent, projectionType string) string {
	if ev.ID != 0 {
		return fmt.Sprintf("outbox:%d:%s", ev.ID, projectionType)
	}
	if ev.IdempotencyKey != nil && strings.TrimSpace(*ev.IdempotencyKey) != "" {
		return fmt.Sprintf("outbox:%s:%s", strings.TrimSpace(*ev.IdempotencyKey), projectionType)
	}
	return fmt.Sprintf("outbox:%s:%s:%s", ev.AggregateType, ev.AggregateID.String(), projectionType)
}

func classifyProjection(ev domaincommerce.OutboxEvent) (string, bool) {
	text := strings.ToLower(strings.Join([]string{ev.EventType, ev.AggregateType, ev.Topic}, " "))
	switch {
	case containsAny(text, "command", "machine.command"):
		return ProjectionTypeCommandLifecycle, true
	case containsAny(text, "telemetry", "heartbeat", "snapshot", "machine_state", "machine.state"):
		return ProjectionTypeMachineTelemetrySummary, true
	case containsAny(text, "inventory", "stock", "refill"):
		return ProjectionTypeInventoryDelta, true
	case containsAny(text, "vend", "dispense", "fulfillment"):
		return ProjectionTypeVend, true
	case containsAny(text, "payment", "refund", "charge", "settlement"):
		return ProjectionTypePayment, true
	case containsAny(text, "sale", "order", "checkout"):
		return ProjectionTypeSales, true
	default:
		return "", false
	}
}

func containsAny(s string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(s, needle) {
			return true
		}
	}
	return false
}

func eventTimeFromPayload(payload []byte) string {
	var body map[string]any
	if err := json.Unmarshal(payload, &body); err != nil {
		return ""
	}
	for _, key := range []string{"occurred_at", "occurredAt", "created_at", "createdAt", "event_time", "eventTime", "timestamp"} {
		if raw, ok := body[key]; ok {
			if s, ok := raw.(string); ok {
				s = strings.TrimSpace(s)
				if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
					return t.UTC().Format(time.RFC3339Nano)
				}
			}
		}
	}
	return ""
}
