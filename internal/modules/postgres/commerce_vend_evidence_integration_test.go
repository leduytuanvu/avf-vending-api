package postgres_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/avf/avf-vending-api/internal/domain/commerce"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/avf/avf-vending-api/internal/modules/postgres"
	"github.com/avf/avf-vending-api/internal/testfixtures"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
)

func TestVendHardwareEvidence_InsertDedupeKeyIdempotent(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	q := db.New(pool)
	store := postgres.NewStore(pool)

	orderRes, err := store.CreateOrderWithVendSession(ctx, commerce.CreateOrderVendInput{
		MachineID:      testfixtures.DevMachineID,
		ProductID:      testfixtures.DevProductCola,
		SlotIndex:      0,
		Currency:       "USD",
		SubtotalMinor:  150,
		TaxMinor:       0,
		TotalMinor:     150,
		IdempotencyKey: "evidence-dedupe-order-" + uuid.NewString(),
		OrderStatus:    "created",
		VendState:      "pending",
	})
	require.NoError(t, err)
	orderID := orderRes.Order.ID
	vendSessionID := orderRes.Vend.ID

	dedupe := "test-evidence-dedupe-" + uuid.NewString()
	params := db.InsertVendHardwareEvidenceParams{
		OrderID:        orderID,
		VendSessionID:  vendSessionID,
		MachineID:      testfixtures.DevMachineID,
		SlotIndex:      0,
		VendAttemptID:  uuid.New(),
		CorrelationID:  uuid.New(),
		CommandID:      "cmd-test-1",
		EvidenceDigest: "digest-abc",
		Raw:            json.RawMessage(`{"source":"integration_test"}`),
		DedupeKey:      dedupe,
	}
	_, err = q.InsertVendHardwareEvidence(ctx, params)
	require.NoError(t, err)

	_, err = q.InsertVendHardwareEvidence(ctx, params)
	require.Error(t, err)

	var count int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM vend_hardware_evidence WHERE dedupe_key = $1`, dedupe,
	).Scan(&count))
	require.Equal(t, 1, count)
}

func TestOutbox_InsertOutboxEventIdempotent(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	q := db.New(pool)

	topic := "commerce.vends"
	idem := "test-outbox-idem-" + uuid.NewString()
	orderID := uuid.New()
	payload := json.RawMessage(`{"order_id":"` + orderID.String() + `"}`)
	idemText := pgtype.Text{String: idem, Valid: true}

	p := db.InsertOutboxEventIdempotentParams{
		Topic:          topic,
		EventType:      "vend.succeeded",
		Payload:        payload,
		AggregateType:  "order",
		AggregateID:    orderID,
		IdempotencyKey: idemText,
	}
	row1, err := q.InsertOutboxEventIdempotent(ctx, p)
	require.NoError(t, err)
	require.NotZero(t, row1.ID)

	_, err = q.InsertOutboxEventIdempotent(ctx, p)
	require.Error(t, err)

	got, err := q.GetOutboxEventByTopicIdempotencyKey(ctx, db.GetOutboxEventByTopicIdempotencyKeyParams{
		Topic:          topic,
		IdempotencyKey: idemText,
	})
	require.NoError(t, err)
	require.Equal(t, row1.ID, got.ID)

	var count int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM outbox_events WHERE topic = $1 AND idempotency_key = $2`, topic, idem,
	).Scan(&count))
	require.Equal(t, 1, count)
}
