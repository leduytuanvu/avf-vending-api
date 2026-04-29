package nats

import (
	"testing"
	"time"

	domaincommerce "github.com/avf/avf-vending-api/internal/domain/commerce"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func validOutboxEventForTest() domaincommerce.OutboxEvent {
	idem := "payment-session-started:order-1"
	lastErr := "prior broker error"
	lastAttempt := time.Date(2026, 4, 29, 1, 2, 3, 0, time.UTC)
	publishedAt := time.Date(2026, 4, 29, 1, 3, 0, 0, time.UTC)
	return domaincommerce.OutboxEvent{
		ID:                   42,
		Topic:                "commerce.payments",
		EventType:            "payment.session_started",
		Payload:              []byte(`{"schema_version":1,"payment_id":"pay_1"}`),
		AggregateType:        "payment",
		AggregateID:          uuid.New(),
		IdempotencyKey:       &idem,
		CreatedAt:            time.Date(2026, 4, 29, 1, 0, 0, 0, time.UTC),
		PublishedAt:          &publishedAt,
		PublishAttemptCount:  2,
		LastPublishError:     &lastErr,
		LastPublishAttemptAt: &lastAttempt,
	}
}

func TestValidateOutboxEventRequiresProductionContractFields(t *testing.T) {
	ev := validOutboxEventForTest()
	ev.EventType = ""
	ev.Payload = []byte(`not-json`)

	err := ValidateOutboxEvent(ev)
	require.Error(t, err)
	require.Contains(t, err.Error(), "event_type")
	require.Contains(t, err.Error(), "payload schema version")
}

func TestOutboxPayloadSchemaVersionDefaultsLegacyJSONPayloads(t *testing.T) {
	require.Equal(t, "1", OutboxPayloadSchemaVersion([]byte(`{"payment_id":"pay_1"}`)))
	require.Equal(t, "", OutboxPayloadSchemaVersion([]byte(`not-json`)))
}

func TestOutboxHeadersCarryEventKeyAndPublishMetadata(t *testing.T) {
	ev := validOutboxEventForTest()
	h := outboxHeaders(ev)

	require.Equal(t, "payment-session-started:order-1", h.Get("Nats-Msg-Id"))
	require.Equal(t, "payment.session_started", h.Get("X-Event-Type"))
	require.Equal(t, "payment", h.Get("X-Aggregate-Type"))
	require.Equal(t, ev.AggregateID.String(), h.Get("X-Aggregate-Id"))
	require.Equal(t, "1", h.Get("X-Payload-Schema-Version"))
	require.Equal(t, "2", h.Get("X-Outbox-Publish-Attempts"))
	require.Equal(t, "prior broker error", h.Get("X-Outbox-Last-Error"))
	require.NotEmpty(t, h.Get("X-Outbox-Created-At"))
	require.NotEmpty(t, h.Get("X-Outbox-Published-At"))
}

func TestOutboxEventKeyFallsBackToOutboxID(t *testing.T) {
	ev := validOutboxEventForTest()
	ev.IdempotencyKey = nil

	require.Equal(t, "outbox-42", OutboxEventKey(ev))
	require.NoError(t, ValidateOutboxEvent(ev))
}
