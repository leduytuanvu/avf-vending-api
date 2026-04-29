package clickhouse

import (
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"

	domaincommerce "github.com/avf/avf-vending-api/internal/domain/commerce"
	"github.com/google/uuid"
)

func TestProjectionRowFromOutboxEvent_MapsRequiredProjectionTypes(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		event domaincommerce.OutboxEvent
		want  string
	}{
		{name: "sales", event: domaincommerce.OutboxEvent{EventType: "order.completed"}, want: ProjectionTypeSales},
		{name: "payment", event: domaincommerce.OutboxEvent{EventType: "payment.captured"}, want: ProjectionTypePayment},
		{name: "vend", event: domaincommerce.OutboxEvent{EventType: "vend.succeeded"}, want: ProjectionTypeVend},
		{name: "inventory", event: domaincommerce.OutboxEvent{EventType: "inventory.stock_delta"}, want: ProjectionTypeInventoryDelta},
		{name: "telemetry", event: domaincommerce.OutboxEvent{EventType: "telemetry.summary"}, want: ProjectionTypeMachineTelemetrySummary},
		{name: "command", event: domaincommerce.OutboxEvent{EventType: "machine.command.acked"}, want: ProjectionTypeCommandLifecycle},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, ok := classifyProjection(tc.event)
			if !ok || got != tc.want {
				t.Fatalf("projection=%q ok=%v want=%q", got, ok, tc.want)
			}
		})
	}
}

func TestProjectionRowFromOutboxEvent_StablePayloadAndOccurredAt(t *testing.T) {
	t.Parallel()
	orgID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	aggID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	idem := "payment-captured-1"
	created := time.Date(2026, 4, 29, 1, 2, 3, 0, time.UTC)
	published := created.Add(time.Minute)
	payload := []byte(`{"occurred_at":"2026-04-29T01:01:59Z","amount_minor":1200}`)
	line, err := projectionRowFromOutboxEvent(domaincommerce.OutboxEvent{
		ID:             42,
		OrganizationID: &orgID,
		Topic:          "commerce.payments",
		EventType:      "payment.captured",
		AggregateType:  "payment",
		AggregateID:    aggID,
		IdempotencyKey: &idem,
		Payload:        payload,
		CreatedAt:      created,
		PublishedAt:    &published,
	}, ProjectionTypePayment)
	if err != nil {
		t.Fatal(err)
	}
	var row ProjectionRow
	if err := json.Unmarshal(line, &row); err != nil {
		t.Fatal(err)
	}
	if row.ProjectionID != "outbox:42:payment_event" {
		t.Fatalf("projection_id=%q", row.ProjectionID)
	}
	if row.PayloadBase64 != base64.StdEncoding.EncodeToString(payload) {
		t.Fatalf("payload_base64=%q", row.PayloadBase64)
	}
	if row.OccurredAt != "2026-04-29T01:01:59Z" {
		t.Fatalf("occurred_at=%q", row.OccurredAt)
	}
	if row.OrganizationID != orgID.String() || row.AggregateID != aggID.String() || row.IdempotencyKey != idem {
		t.Fatalf("unexpected row: %+v", row)
	}
}
