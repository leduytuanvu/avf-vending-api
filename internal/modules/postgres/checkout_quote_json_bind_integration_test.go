package postgres_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	appcommerce "github.com/avf/avf-vending-api/internal/app/commerce"
	"github.com/avf/avf-vending-api/internal/modules/postgres"
	"github.com/avf/avf-vending-api/internal/testfixtures"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

func localPricingSnapshotJSON(t *testing.T) []byte {
	raw, err := json.Marshal(map[string]any{
		"schema_version":         2,
		"snapshot_id":            "8c1f3e01-b629-4f48-9135-68edf8bd8488",
		"subtotal_minor":         2000,
		"tax_minor":              0,
		"total_minor":            2000,
		"unit_price_minor":       2000,
		"local_pricing_revision": 2,
		"pricing_fingerprint":    "fp-test",
	})
	require.NoError(t, err)
	return raw
}

func TestInsertCheckoutQuote_UncastByteSliceJSON_Returns22P02(t *testing.T) {
	pool := commerceJSONPool(t, pgx.QueryExecModeExec)
	ctx := context.Background()
	machineID := testfixtures.DevMachineID
	_, err := pool.Exec(ctx, `
INSERT INTO checkout_quotes (
    machine_id, currency, payment_method, subtotal_minor, discount_minor, payable_minor,
    state, expires_at, pricing_source, machine_pricing_snapshot
) VALUES (
    $1, 'VND', 'cash', 2000, 0, 2000, 'active', NOW() + interval '15 minutes',
    'machine_local_verified', $2
)`, machineID, []byte(`{"total_minor":2000}`))
	require.Error(t, err)
	require.Equal(t, "22P02", pgErrCode(err))
}

func TestCreateQuoteWithLines_LocalPricingSnapshot_ExecAndSimpleProtocol(t *testing.T) {
	modes := []pgx.QueryExecMode{pgx.QueryExecModeExec, pgx.QueryExecModeSimpleProtocol}
	for _, mode := range modes {
		t.Run(mode.String(), func(t *testing.T) {
			pool := commerceJSONPool(t, mode)
			ctx := context.Background()
			store := postgres.NewStore(pool)
			const payableMinor int64 = 2000
			rev := int64(2)
			serverRef := int64(15000)
			snap := localPricingSnapshotJSON(t)
			machineUnit := payableMinor
			serverUnit := serverRef

			quoteKey := "quote-json-" + mode.String() + "-" + uuid.NewString()
			quote, err := store.CreateQuoteWithLines(ctx, appcommerce.PersistQuoteInput{
				MachineID:                   testfixtures.DevMachineID,
				Currency:                    "VND",
				PaymentMethod:               "cash",
				SubtotalMinor:               payableMinor,
				DiscountMinor:               0,
				PayableMinor:                payableMinor,
				IdempotencyKey:              quoteKey,
				ExpiresAt:                   time.Now().UTC().Add(15 * time.Minute),
				PricingSource:               appcommerce.PricingSourceMachineLocalVerified,
				MachinePricingRevision:      &rev,
				MachinePricingSnapshot:      snap,
				ServerReferencePayableMinor: &serverRef,
				Lines: []appcommerce.PersistQuoteLineInput{
					{
						LineSequence:                  1,
						ProductID:                     testfixtures.DevProductWater,
						CabinetCode:                   "A",
						SlotCode:                      "1",
						SlotIndex:                     2,
						Quantity:                      1,
						UnitPriceMinor:                payableMinor,
						LineSubtotalMinor:             payableMinor,
						MachineUnitPriceMinor:         &machineUnit,
						ServerReferenceUnitPriceMinor: &serverUnit,
					},
				},
			})
			require.NoError(t, err)
			require.False(t, quote.Replay)
			require.Equal(t, payableMinor, quote.PayableMinor)
			require.Equal(t, appcommerce.PricingSourceMachineLocalVerified, quote.PricingSource)

			var jsonType string
			var revision int64
			var serverRefPayable *int64
			var machineUnitLine *int64
			var serverUnitLine *int64
			err = pool.QueryRow(ctx, `
				SELECT jsonb_typeof(machine_pricing_snapshot), machine_pricing_revision,
				       server_reference_payable_minor, machine_unit_price_minor, server_reference_unit_price_minor
				FROM checkout_quotes cq
				JOIN checkout_quote_lines cql ON cql.quote_id = cq.id
				WHERE cq.id = $1`, quote.QuoteID).Scan(&jsonType, &revision, &serverRefPayable, &machineUnitLine, &serverUnitLine)
			require.NoError(t, err)
			require.Equal(t, "object", jsonType)
			require.Equal(t, rev, revision)
			require.NotNil(t, serverRefPayable)
			require.Equal(t, serverRef, *serverRefPayable)
			require.NotNil(t, machineUnitLine)
			require.Equal(t, payableMinor, *machineUnitLine)
			require.NotNil(t, serverUnitLine)
			require.Equal(t, serverRef, *serverUnitLine)

			replayed, err := store.CreateQuoteWithLines(ctx, appcommerce.PersistQuoteInput{
				MachineID:      testfixtures.DevMachineID,
				Currency:       "VND",
				PaymentMethod:  "cash",
				SubtotalMinor:  payableMinor,
				DiscountMinor:  0,
				PayableMinor:   payableMinor,
				IdempotencyKey: quoteKey,
				ExpiresAt:      time.Now().UTC().Add(15 * time.Minute),
				Lines: []appcommerce.PersistQuoteLineInput{
					{
						LineSequence:      1,
						ProductID:         testfixtures.DevProductWater,
						CabinetCode:       "A",
						SlotCode:          "1",
						SlotIndex:         2,
						Quantity:          1,
						UnitPriceMinor:    payableMinor,
						LineSubtotalMinor: payableMinor,
					},
				},
			})
			require.NoError(t, err)
			require.True(t, replayed.Replay)
			require.Equal(t, quote.QuoteID, replayed.QuoteID)

			orderKey := "order-json-" + mode.String() + "-" + uuid.NewString()
			created, err := store.CreateOrderFromQuoteWithVendSessions(ctx, appcommerce.PersistOrderFromQuoteInput{
				Quote:          quote,
				IdempotencyKey: orderKey,
			})
			require.NoError(t, err)
			require.False(t, created.Replay)
			require.Equal(t, payableMinor, created.Order.TotalMinor)

			replayedOrder, err := store.CreateOrderFromQuoteWithVendSessions(ctx, appcommerce.PersistOrderFromQuoteInput{
				Quote:          quote,
				IdempotencyKey: orderKey,
			})
			require.NoError(t, err)
			require.True(t, replayedOrder.Replay)
			require.Equal(t, created.Order.ID, replayedOrder.Order.ID)
		})
	}
}

func TestCreateQuoteWithLines_ServerPricedWithoutSnapshot_ExecMode(t *testing.T) {
	pool := commerceJSONPool(t, pgx.QueryExecModeExec)
	ctx := context.Background()
	store := postgres.NewStore(pool)
	const payableMinor int64 = 2000
	quoteKey := "quote-server-" + uuid.NewString()
	quote, err := store.CreateQuoteWithLines(ctx, appcommerce.PersistQuoteInput{
		MachineID:      testfixtures.DevMachineID,
		Currency:       "VND",
		PaymentMethod:  "cash",
		SubtotalMinor:  payableMinor,
		DiscountMinor:  0,
		PayableMinor:   payableMinor,
		IdempotencyKey: quoteKey,
		ExpiresAt:      time.Now().UTC().Add(15 * time.Minute),
		Lines: []appcommerce.PersistQuoteLineInput{
			{
				LineSequence:      1,
				ProductID:         testfixtures.DevProductWater,
				CabinetCode:       "A",
				SlotCode:          "1",
				SlotIndex:         2,
				Quantity:          1,
				UnitPriceMinor:    payableMinor,
				LineSubtotalMinor: payableMinor,
			},
		},
	})
	require.NoError(t, err)
	require.Equal(t, appcommerce.PricingSourceServerPriced, quote.PricingSource)

	var snapIsNull bool
	err = pool.QueryRow(ctx, `
		SELECT machine_pricing_snapshot IS NULL FROM checkout_quotes WHERE id = $1`, quote.QuoteID).Scan(&snapIsNull)
	require.NoError(t, err)
	require.True(t, snapIsNull)
}

func TestCreateQuoteWithLines_MultiLineLocalPricing_ExecMode(t *testing.T) {
	pool := commerceJSONPool(t, pgx.QueryExecModeExec)
	ctx := context.Background()
	store := postgres.NewStore(pool)
	snap, err := json.Marshal(map[string]any{
		"schema_version":         2,
		"subtotal_minor":         5000,
		"tax_minor":              0,
		"total_minor":            5000,
		"local_pricing_revision": 2,
		"lines": []map[string]any{
			{"line_sequence": 1, "unit_price_minor": 2000, "line_subtotal_minor": 2000, "quantity": 1},
			{"line_sequence": 2, "unit_price_minor": 3000, "line_subtotal_minor": 3000, "quantity": 1},
		},
	})
	require.NoError(t, err)
	rev := int64(2)
	quote, err := store.CreateQuoteWithLines(ctx, appcommerce.PersistQuoteInput{
		MachineID:              testfixtures.DevMachineID,
		Currency:               "VND",
		PaymentMethod:          "cash",
		SubtotalMinor:          5000,
		DiscountMinor:          0,
		PayableMinor:           5000,
		IdempotencyKey:         "quote-multi-" + uuid.NewString(),
		ExpiresAt:              time.Now().UTC().Add(15 * time.Minute),
		PricingSource:          appcommerce.PricingSourceMachineLocalUnverified,
		MachinePricingRevision: &rev,
		MachinePricingSnapshot: snap,
		Lines: []appcommerce.PersistQuoteLineInput{
			{LineSequence: 1, ProductID: testfixtures.DevProductWater, CabinetCode: "A", SlotCode: "1", SlotIndex: 2, Quantity: 1, UnitPriceMinor: 2000, LineSubtotalMinor: 2000},
			{LineSequence: 2, ProductID: testfixtures.DevProductCola, CabinetCode: "A", SlotCode: "2", SlotIndex: 3, Quantity: 1, UnitPriceMinor: 3000, LineSubtotalMinor: 3000},
		},
	})
	require.NoError(t, err)
	require.Equal(t, int64(5000), quote.PayableMinor)

	var lineCount int
	err = pool.QueryRow(ctx, `SELECT COUNT(*) FROM checkout_quote_lines WHERE quote_id = $1`, quote.QuoteID).Scan(&lineCount)
	require.NoError(t, err)
	require.Equal(t, 2, lineCount)
}
