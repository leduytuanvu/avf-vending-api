package postgres_test

import (
	"bytes"
	"context"
	"testing"
	"time"

	appcommerce "github.com/avf/avf-vending-api/internal/app/commerce"
	apppayments "github.com/avf/avf-vending-api/internal/app/payments"
	"github.com/avf/avf-vending-api/internal/domain/commerce"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/avf/avf-vending-api/internal/modules/postgres"
	"github.com/avf/avf-vending-api/internal/testfixtures"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
)

func TestPaymentP12_webhookStoresIngressAndOrg(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	store := postgres.NewStore(pool)
	q := db.New(pool)

	orderIDem := "p12-wh-" + uuid.NewString()
	orderRes, err := store.CreateOrderWithVendSession(ctx, commerce.CreateOrderVendInput{
		OrganizationID: testfixtures.DevOrganizationID,
		MachineID:      testfixtures.DevMachineID,
		ProductID:      testfixtures.DevProductWater,
		SlotIndex:      2,
		Currency:       "USD",
		SubtotalMinor:  200,
		TaxMinor:       0,
		TotalMinor:     200,
		IdempotencyKey: orderIDem,
		OrderStatus:    "created",
		VendState:      "pending",
	})
	require.NoError(t, err)

	payIDem := orderIDem + ":pay"
	outIDem := orderIDem + ":out:" + orderRes.Order.ID.String()
	payRes, err := store.CreatePaymentWithOutbox(ctx, commerce.PaymentOutboxInput{
		OrganizationID:       testfixtures.DevOrganizationID,
		OrderID:              orderRes.Order.ID,
		Provider:             "psp_fixture",
		PaymentState:         "created",
		AmountMinor:          200,
		Currency:             "USD",
		IdempotencyKey:       payIDem,
		OutboxTopic:          "commerce.payments",
		OutboxEventType:      "payment.session_started",
		OutboxPayload:        []byte(`{}`),
		OutboxAggregateType:  "payment",
		OutboxAggregateID:    orderRes.Order.ID,
		OutboxIdempotencyKey: outIDem,
	})
	require.NoError(t, err)

	evID := "evt-p12-" + uuid.NewString()
	in := appcommerce.ApplyPaymentProviderWebhookInput{
		OrganizationID:          testfixtures.DevOrganizationID,
		OrderID:                 orderRes.Order.ID,
		PaymentID:               payRes.Payment.ID,
		Provider:                "psp_fixture",
		ProviderReference:       "prov-p12-1",
		WebhookEventID:          evID,
		EventType:               "payment.captured",
		NormalizedPaymentState:  "captured",
		Payload:                 []byte(`{"nested":{"card_number":"4242424242424242"}}`),
		WebhookValidationStatus: "hmac_verified",
	}
	_, err = store.ApplyPaymentProviderWebhook(ctx, in)
	require.NoError(t, err)

	row, err := q.GetPaymentProviderEventByWebhookEventID(ctx, db.GetPaymentProviderEventByWebhookEventIDParams{
		Provider:       "psp_fixture",
		WebhookEventID: pgtype.Text{String: evID, Valid: true},
	})
	require.NoError(t, err)
	require.True(t, row.SignatureValid)
	require.True(t, row.OrganizationID.Valid)
	require.Equal(t, testfixtures.DevOrganizationID, uuid.UUID(row.OrganizationID.Bytes))
	require.Equal(t, "applied", row.IngressStatus)
	require.True(t, row.AppliedAt.Valid)
	require.NotContains(t, string(row.Payload), "4242424242424242")
}

func TestPaymentP12_settlementImport_idempotentAndMismatch(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	store := postgres.NewStore(pool)
	q := db.New(pool)
	adm, err := apppayments.NewAdminService(pool, q, nil)
	require.NoError(t, err)

	orderIDem := "p12-settle-" + uuid.NewString()
	orderRes, err := store.CreateOrderWithVendSession(ctx, commerce.CreateOrderVendInput{
		OrganizationID: testfixtures.DevOrganizationID,
		MachineID:      testfixtures.DevMachineID,
		ProductID:      testfixtures.DevProductWater,
		SlotIndex:      2,
		Currency:       "USD",
		SubtotalMinor:  500,
		TaxMinor:       0,
		TotalMinor:     500,
		IdempotencyKey: orderIDem,
		OrderStatus:    "created",
		VendState:      "pending",
	})
	require.NoError(t, err)
	payIDem := orderIDem + ":pay"
	outIDem := orderIDem + ":out:" + orderRes.Order.ID.String()
	payRes, err := store.CreatePaymentWithOutbox(ctx, commerce.PaymentOutboxInput{
		OrganizationID:       testfixtures.DevOrganizationID,
		OrderID:              orderRes.Order.ID,
		Provider:             "psp_fixture",
		PaymentState:         "created",
		AmountMinor:          500,
		Currency:             "USD",
		IdempotencyKey:       payIDem,
		OutboxTopic:          "commerce.payments",
		OutboxEventType:      "payment.session_started",
		OutboxPayload:        []byte(`{}`),
		OutboxAggregateType:  "payment",
		OutboxAggregateID:    orderRes.Order.ID,
		OutboxIdempotencyKey: outIDem,
	})
	require.NoError(t, err)
	wIn := appcommerce.ApplyPaymentProviderWebhookInput{
		OrganizationID:          testfixtures.DevOrganizationID,
		OrderID:                 orderRes.Order.ID,
		PaymentID:               payRes.Payment.ID,
		Provider:                "psp_fixture",
		ProviderReference:       "pi_settle_1",
		WebhookEventID:          "evt-settle-1-" + uuid.NewString(),
		EventType:               "payment.captured",
		NormalizedPaymentState:  "captured",
		Payload:                 []byte(`{}`),
		WebhookValidationStatus: "hmac_verified",
	}
	_, err = store.ApplyPaymentProviderWebhook(ctx, wIn)
	require.NoError(t, err)

	settleID := "set_" + uuid.NewString()
	item := apppayments.SettlementImportItem{
		ProviderSettlementID: settleID,
		GrossAmountMinor:     500,
		FeeAmountMinor:       0,
		NetAmountMinor:       500,
		Currency:             "USD",
		SettlementDate:       time.Now().UTC().Format("2006-01-02"),
		TransactionRefs:      []string{"pi_settle_1"},
	}
	r1, err := adm.ImportSettlements(ctx, testfixtures.DevOrganizationID, "psp_fixture", []apppayments.SettlementImportItem{item})
	require.NoError(t, err)
	require.Len(t, r1.Results, 1)
	require.True(t, r1.Results[0].Matched)
	require.Equal(t, "reconciled", r1.Results[0].Settlement.Status)

	r2, err := adm.ImportSettlements(ctx, testfixtures.DevOrganizationID, "psp_fixture", []apppayments.SettlementImportItem{item})
	require.NoError(t, err)
	require.Len(t, r2.Results, 1)
	var cnt int64
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM payment_provider_settlements WHERE organization_id = $1 AND provider_settlement_id = $2`,
		testfixtures.DevOrganizationID, settleID).Scan(&cnt))
	require.EqualValues(t, 1, cnt)
	require.Equal(t, r1.Results[0].Settlement.ID, r2.Results[0].Settlement.ID)

	bad := item
	bad.GrossAmountMinor = 1
	bad.ProviderSettlementID = "set_bad_" + uuid.NewString()
	_, err = adm.ImportSettlements(ctx, testfixtures.DevOrganizationID, "psp_fixture", []apppayments.SettlementImportItem{bad})
	require.NoError(t, err)
	var caseCnt int64
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM commerce_reconciliation_cases WHERE organization_id = $1 AND case_type = 'settlement_amount_mismatch' AND correlation_key = $2`,
		testfixtures.DevOrganizationID, "settlement:psp_fixture:"+bad.ProviderSettlementID).Scan(&caseCnt))
	require.EqualValues(t, 1, caseCnt)
}

func TestPaymentP12_financeExportOtherOrgEmpty(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	q := db.New(pool)
	adm, err := apppayments.NewAdminService(pool, q, nil)
	require.NoError(t, err)
	from := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	otherOrg := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	var buf bytes.Buffer
	require.NoError(t, adm.WriteFinanceExportCSV(ctx, &buf, otherOrg, from, to))
	require.Contains(t, buf.String(), "payment_id")
	n, err := q.ListPaymentsFinanceExportForOrg(ctx, db.ListPaymentsFinanceExportForOrgParams{
		OrganizationID: otherOrg,
		CreatedAt:      from,
		CreatedAt_2:    to,
	})
	require.NoError(t, err)
	require.Empty(t, n)
}

func TestPaymentP12_disputeResolve(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	q := db.New(pool)
	adm, err := apppayments.NewAdminService(pool, q, nil)
	require.NoError(t, err)

	orderIDem := "p12-disp-" + uuid.NewString()
	store := postgres.NewStore(pool)
	orderRes, err := store.CreateOrderWithVendSession(ctx, commerce.CreateOrderVendInput{
		OrganizationID: testfixtures.DevOrganizationID,
		MachineID:      testfixtures.DevMachineID,
		ProductID:      testfixtures.DevProductWater,
		SlotIndex:      2,
		Currency:       "USD",
		SubtotalMinor:  100,
		TaxMinor:       0,
		TotalMinor:     100,
		IdempotencyKey: orderIDem,
		OrderStatus:    "created",
		VendState:      "pending",
	})
	require.NoError(t, err)
	payRes, err := store.CreatePaymentWithOutbox(ctx, commerce.PaymentOutboxInput{
		OrganizationID:       testfixtures.DevOrganizationID,
		OrderID:              orderRes.Order.ID,
		Provider:             "psp_fixture",
		PaymentState:         "created",
		AmountMinor:          100,
		Currency:             "USD",
		IdempotencyKey:       orderIDem + ":pay",
		OutboxTopic:          "commerce.payments",
		OutboxEventType:      "payment.session_started",
		OutboxPayload:        []byte(`{}`),
		OutboxAggregateType:  "payment",
		OutboxAggregateID:    orderRes.Order.ID,
		OutboxIdempotencyKey: orderIDem + ":out",
	})
	require.NoError(t, err)

	ext := "dp_" + uuid.NewString()
	_, err = q.InsertPaymentDispute(ctx, db.InsertPaymentDisputeParams{
		OrganizationID:    testfixtures.DevOrganizationID,
		Provider:          "psp_fixture",
		ProviderDisputeID: ext,
		AmountMinor:       100,
		Currency:          "USD",
		Metadata:          []byte(`{}`),
		PaymentID:         pgtype.UUID{Bytes: payRes.Payment.ID, Valid: true},
		OrderID:           pgtype.UUID{Bytes: orderRes.Order.ID, Valid: true},
		Reason:            pgtype.Text{String: "integration", Valid: true},
		Status:            pgtype.Text{String: "opened", Valid: true},
	})
	require.NoError(t, err)

	list, err := adm.ListDisputes(ctx, testfixtures.DevOrganizationID, 20, 0)
	require.NoError(t, err)
	require.NotEmpty(t, list.Items)
	did := uuid.MustParse(list.Items[0].ID)
	out, err := adm.ResolveDispute(ctx, apppayments.ResolveDisputeInput{
		OrganizationID: testfixtures.DevOrganizationID,
		DisputeID:      did,
		Status:         "lost",
		Note:           "unit test",
		ResolvedBy:     uuid.Nil,
	})
	require.NoError(t, err)
	require.Equal(t, "lost", out.Status)
	require.NotNil(t, out.ResolvedAt)
}
