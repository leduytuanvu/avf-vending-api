package postgres_test

import (
	"context"
	"testing"
	"time"

	appcommerce "github.com/avf/avf-vending-api/internal/app/commerce"
	"github.com/avf/avf-vending-api/internal/modules/postgres"
	"github.com/avf/avf-vending-api/internal/testfixtures"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestCreateOrderFromQuoteWithVendSessions_persistsServerPricedAt2000(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	store := postgres.NewStore(pool)

	const payableMinor int64 = 2000
	quoteKey := "quote-pricing-" + uuid.NewString()
	orderKey := "order-pricing-" + uuid.NewString()

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
	require.False(t, quote.Replay)

	created, err := store.CreateOrderFromQuoteWithVendSessions(ctx, appcommerce.PersistOrderFromQuoteInput{
		Quote:          quote,
		IdempotencyKey: orderKey,
	})
	require.NoError(t, err)
	require.False(t, created.Replay)
	require.Equal(t, payableMinor, created.Order.TotalMinor)
	require.Equal(t, payableMinor, created.Order.SubtotalMinor)
	require.Equal(t, appcommerce.PricingSourceServerPriced, created.Order.PricingSource)
	require.Len(t, created.Lines, 1)

	var pricingSource string
	var subtotalMinor int64
	var totalMinor int64
	err = pool.QueryRow(ctx, `
		SELECT pricing_source, subtotal_minor, total_minor
		FROM orders
		WHERE id = $1
	`, created.Order.ID).Scan(&pricingSource, &subtotalMinor, &totalMinor)
	require.NoError(t, err)
	require.Equal(t, appcommerce.PricingSourceServerPriced, pricingSource)
	require.Equal(t, payableMinor, subtotalMinor)
	require.Equal(t, payableMinor, totalMinor)

	replayed, err := store.CreateOrderFromQuoteWithVendSessions(ctx, appcommerce.PersistOrderFromQuoteInput{
		Quote:          quote,
		IdempotencyKey: orderKey,
	})
	require.NoError(t, err)
	require.True(t, replayed.Replay)
	require.Equal(t, created.Order.ID, replayed.Order.ID)
}
