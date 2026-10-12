package commerce

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestResolveCreateQuoteSaleLines_usesSnapshotTopologyWithoutBatchPricing(t *testing.T) {
	t.Parallel()
	machineID := uuidNew()
	productID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	resolver := &recordingSaleLineResolver{
		line: ResolvedSaleLine{
			SlotConfigID:  uuid.MustParse("33333333-3333-3333-3333-333333333333"),
			CabinetCode:   "A",
			SlotCode:      "A1",
			SlotIndex:     1,
			PriceMinor:    99000,
			SubtotalMinor: 99000,
			TotalMinor:    99000,
		},
	}
	svc := NewService(Deps{
		OrderVend: stubOrderVendWorkflow{},
		SaleLines: resolver,
	})
	lines, err := svc.resolveCreateQuoteSaleLines(context.Background(), CreateQuoteInput{
		MachineID: machineID,
		Lines: []QuoteLineInput{{
			ProductID: productID,
			SlotCode:  "A1",
			Quantity:  1,
		}},
		PricingSnapshot: &MachinePricingSnapshotInput{
			SubtotalMinor:        2000,
			TaxMinor:             0,
			TotalMinor:           2000,
			UnitPriceMinor:       2000,
			LocalPricingRevision: 1,
			CapturedAt:           time.Now().UTC(),
			Lines: []MachinePricingSnapshotLineInput{
				{
					LineSequence:      1,
					ProductID:         productID,
					SlotCode:          "A1",
					Quantity:          1,
					UnitPriceMinor:    2000,
					LineSubtotalMinor: 2000,
				},
			},
		},
	})
	require.NoError(t, err)
	require.Len(t, lines, 1)
	require.Equal(t, int64(99000), lines[0].PriceMinor)
	require.False(t, resolver.called, "ResolveSaleLine must not run when snapshot topology path succeeds")
}
