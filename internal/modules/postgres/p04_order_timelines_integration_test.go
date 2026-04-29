package postgres_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/avf/avf-vending-api/internal/domain/commerce"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/avf/avf-vending-api/internal/modules/postgres"
	"github.com/avf/avf-vending-api/internal/testfixtures"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
)

func TestOrderTimeline_insertAndList(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	store := postgres.NewStore(pool)
	q := db.New(pool)

	idem := "tl-order-" + uuid.NewString()
	or, err := store.CreateOrderWithVendSession(ctx, commerce.CreateOrderVendInput{
		OrganizationID: testfixtures.DevOrganizationID,
		MachineID:      testfixtures.DevMachineID,
		ProductID:      testfixtures.DevProductWater,
		SlotIndex:      2,
		Currency:       "USD",
		SubtotalMinor:  100,
		TaxMinor:       0,
		TotalMinor:     100,
		IdempotencyKey: idem,
		OrderStatus:    "paid",
		VendState:      "failed",
	})
	require.NoError(t, err)

	orderID := or.Order.ID
	payload, _ := json.Marshal(map[string]any{"test": true})
	err = q.InsertOrderTimelineEvent(ctx, db.InsertOrderTimelineEventParams{
		OrganizationID: testfixtures.DevOrganizationID,
		OrderID:        orderID,
		EventType:      "test.event",
		ActorType:      "system",
		ActorID:        pgtype.Text{},
		Payload:        payload,
		OccurredAt:     time.Now().UTC(),
	})
	require.NoError(t, err)

	rows, err := q.CommerceAdminListOrderTimeline(ctx, db.CommerceAdminListOrderTimelineParams{
		OrganizationID: testfixtures.DevOrganizationID,
		OrderID:        orderID,
		Limit:          50,
		Offset:         0,
	})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "test.event", rows[0].EventType)
}

func TestRefundRequests_insertAndList(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	store := postgres.NewStore(pool)
	q := db.New(pool)

	idem := "rr-order-" + uuid.NewString()
	or, err := store.CreateOrderWithVendSession(ctx, commerce.CreateOrderVendInput{
		OrganizationID: testfixtures.DevOrganizationID,
		MachineID:      testfixtures.DevMachineID,
		ProductID:      testfixtures.DevProductWater,
		SlotIndex:      2,
		Currency:       "USD",
		SubtotalMinor:  150,
		TaxMinor:       0,
		TotalMinor:     150,
		IdempotencyKey: idem,
		OrderStatus:    "paid",
		VendState:      "failed",
	})
	require.NoError(t, err)

	payIDem := idem + ":pay"
	outIDem := idem + ":out:" + or.Order.ID.String()
	payRes, err := store.CreatePaymentWithOutbox(ctx, commerce.PaymentOutboxInput{
		OrganizationID:       testfixtures.DevOrganizationID,
		OrderID:              or.Order.ID,
		Provider:             "psp_fixture",
		PaymentState:         "captured",
		AmountMinor:          150,
		Currency:             "USD",
		IdempotencyKey:       payIDem,
		OutboxTopic:          "commerce.payments",
		OutboxEventType:      "payment.session_started",
		OutboxPayload:        []byte(`{}`),
		OutboxAggregateType:  "payment",
		OutboxAggregateID:    or.Order.ID,
		OutboxIdempotencyKey: outIDem,
	})
	require.NoError(t, err)

	reqRow, err := q.CommerceAdminInsertRefundRequest(ctx, db.CommerceAdminInsertRefundRequestParams{
		OrganizationID: testfixtures.DevOrganizationID,
		OrderID:        or.Order.ID,
		PaymentID:      pgtype.UUID{Bytes: payRes.Payment.ID, Valid: true},
		AmountMinor:    150,
		Currency:       "USD",
		Reason:         pgtype.Text{String: "integration", Valid: true},
		Status:         "requested",
		RequestedBy:    pgtype.UUID{},
		IdempotencyKey: pgtype.Text{String: "idem-" + idem, Valid: true},
	})
	require.NoError(t, err)

	list, err := q.CommerceAdminListRefundRequests(ctx, db.CommerceAdminListRefundRequestsParams{
		OrganizationID: testfixtures.DevOrganizationID,
		Column2:        false,
		Column3:        "",
		Limit:          50,
		Offset:         0,
	})
	require.NoError(t, err)
	found := false
	for _, r := range list {
		if r.ID == reqRow.ID {
			found = true
			break
		}
	}
	require.True(t, found)
}
