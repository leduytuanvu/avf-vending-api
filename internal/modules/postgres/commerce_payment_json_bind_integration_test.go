package postgres_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	appcommerce "github.com/avf/avf-vending-api/internal/app/commerce"
	"github.com/avf/avf-vending-api/internal/domain/commerce"
	"github.com/avf/avf-vending-api/internal/modules/postgres"
	"github.com/avf/avf-vending-api/internal/testfixtures"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func commerceJSONTestDSN(t *testing.T) string {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping integration tests in -short mode")
	}
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	return dsn
}

func commerceJSONPool(t *testing.T, mode pgx.QueryExecMode) *pgxpool.Pool {
	t.Helper()
	dsn := commerceJSONTestDSN(t)
	testfixtures.EnsureTestMigrations(t, dsn)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pcfg, err := pgxpool.ParseConfig(dsn)
	require.NoError(t, err)
	pcfg.ConnConfig.DefaultQueryExecMode = mode
	pool, err := pgxpool.NewWithConfig(ctx, pcfg)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return pool
}

func pgErrCode(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code
	}
	return ""
}

func createCommercePaymentFixture(t *testing.T, store *postgres.Store) (uuid.UUID, uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	idem := "json-bind-" + uuid.NewString()
	or, err := store.CreateOrderWithVendSession(ctx, commerce.CreateOrderVendInput{
		MachineID:      testfixtures.DevMachineID,
		ProductID:      testfixtures.DevProductWater,
		SlotIndex:      2,
		Currency:       "VND",
		SubtotalMinor:  1000,
		TaxMinor:       0,
		TotalMinor:     1000,
		IdempotencyKey: idem,
		OrderStatus:    "created",
		VendState:      "pending",
	})
	require.NoError(t, err)
	payRes, err := store.CreatePaymentWithOutbox(ctx, commerce.PaymentOutboxInput{
		OrderID:              or.Order.ID,
		Provider:             "momo",
		PaymentState:         "created",
		AmountMinor:          1000,
		Currency:             "VND",
		IdempotencyKey:       idem + ":pay",
		OutboxTopic:          "commerce.payments",
		OutboxEventType:      "payment.session_started",
		OutboxPayload:        []byte(`{}`),
		OutboxAggregateType:  "payment",
		OutboxAggregateID:    or.Order.ID,
		OutboxIdempotencyKey: idem + ":out",
	})
	require.NoError(t, err)
	return or.Order.ID, payRes.Payment.ID
}

func TestInsertPaymentAttempt_TextJSONBind_ExecAndCacheDescribe(t *testing.T) {
	modes := []pgx.QueryExecMode{pgx.QueryExecModeExec, pgx.QueryExecModeCacheDescribe}
	for _, mode := range modes {
		t.Run(mode.String(), func(t *testing.T) {
			pool := commerceJSONPool(t, mode)
			store := postgres.NewStore(pool)
			_, paymentID := createCommercePaymentFixture(t, store)
			ref := "momo-ref-" + uuid.NewString()[:8]
			payload, err := json.Marshal(map[string]any{
				"provider":           "momo",
				"provider_reference": ref,
				"qr_code_url":        "https://example.test/qr",
				"result_code":        0,
			})
			require.NoError(t, err)
			_, err = store.InsertPaymentAttempt(context.Background(), appcommerce.InsertPaymentAttemptParams{
				PaymentID:         paymentID,
				State:             "created",
				ProviderReference: &ref,
				Payload:           payload,
			})
			require.NoError(t, err)
		})
	}
}

func TestInsertPaymentAttempt_UncastByteSliceJSON_Returns22P02(t *testing.T) {
	pool := commerceJSONPool(t, pgx.QueryExecModeExec)
	ctx := context.Background()
	store := postgres.NewStore(pool)
	_, paymentID := createCommercePaymentFixture(t, store)

	// Direct []byte bind bypassing pgjson (legacy failure mode).
	_, err := pool.Exec(ctx, `
INSERT INTO payment_attempts (payment_id, provider_reference, state, payload)
VALUES ($1, $2, $3, $4)`,
		paymentID,
		"legacy-bytea",
		"created",
		[]byte(`{"provider":"momo"}`),
	)
	require.Error(t, err)
	require.Equal(t, "22P02", pgErrCode(err))
}

func TestApplyPaymentProviderWebhook_ExecMode_PersistsJSON(t *testing.T) {
	pool := commerceJSONPool(t, pgx.QueryExecModeExec)
	ctx := context.Background()
	store := postgres.NewStore(pool)
	orderID, paymentID := createCommercePaymentFixture(t, store)
	provRef := "ipn-ref-" + uuid.NewString()[:8]
	_, err := store.ApplyPaymentProviderWebhook(ctx, appcommerce.ApplyPaymentProviderWebhookInput{
		OrderID:                 orderID,
		PaymentID:               paymentID,
		Provider:                "momo",
		ProviderReference:       provRef,
		WebhookEventID:          "evt-" + uuid.NewString(),
		EventType:               "momo.ipn",
		NormalizedPaymentState:  "captured",
		Payload:                 []byte(`{"resultCode":0,"partnerCode":"AVF"}`),
		WebhookValidationStatus: "provider_native_verified",
		ProviderMetadata:        []byte(`{"delivery":{"mode":"provider_native"}}`),
	})
	require.NoError(t, err)
}
