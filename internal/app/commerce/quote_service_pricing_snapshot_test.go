package commerce

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	domaincommerce "github.com/avf/avf-vending-api/internal/domain/commerce"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestValidateMachinePricingSnapshotMultiLine_acceptsConsistentLines(t *testing.T) {
	t.Parallel()
	productID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	err := validateMachinePricingSnapshotMultiLine(MachinePricingSnapshotInput{
		SubtotalMinor: 5000,
		TaxMinor:      0,
		TotalMinor:    5000,
		Lines: []MachinePricingSnapshotLineInput{
			{LineSequence: 1, ProductID: productID, SlotCode: "A1", UnitPriceMinor: 2000, LineSubtotalMinor: 2000, Quantity: 1},
			{LineSequence: 2, ProductID: productID, SlotCode: "A2", UnitPriceMinor: 3000, LineSubtotalMinor: 3000, Quantity: 1},
		},
	}, 2)
	require.NoError(t, err)
}

func TestValidateMachinePricingSnapshotMultiLine_rejectsLineCountMismatch(t *testing.T) {
	t.Parallel()
	productID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	err := validateMachinePricingSnapshotMultiLine(MachinePricingSnapshotInput{
		SubtotalMinor: 2000,
		TaxMinor:      0,
		TotalMinor:    2000,
		Lines: []MachinePricingSnapshotLineInput{
			{LineSequence: 1, ProductID: productID, UnitPriceMinor: 2000, LineSubtotalMinor: 2000, Quantity: 1},
		},
	}, 2)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrInvalidArgument)
}

func TestValidateMachinePricingSnapshotMultiLine_rejectsSubtotalMismatch(t *testing.T) {
	t.Parallel()
	productID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	err := validateMachinePricingSnapshotMultiLine(MachinePricingSnapshotInput{
		SubtotalMinor: 9999,
		TaxMinor:      0,
		TotalMinor:    9999,
		Lines: []MachinePricingSnapshotLineInput{
			{LineSequence: 1, ProductID: productID, UnitPriceMinor: 2000, LineSubtotalMinor: 2000, Quantity: 1},
		},
	}, 1)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrInvalidArgument)
}

func TestValidateMachinePricingSnapshotMultiLine_rejectsDuplicateLineSequence(t *testing.T) {
	t.Parallel()
	productID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	err := validateMachinePricingSnapshotMultiLine(MachinePricingSnapshotInput{
		SubtotalMinor: 4000,
		TaxMinor:      0,
		TotalMinor:    4000,
		Lines: []MachinePricingSnapshotLineInput{
			{LineSequence: 1, ProductID: productID, UnitPriceMinor: 2000, LineSubtotalMinor: 2000, Quantity: 1},
			{LineSequence: 1, ProductID: productID, UnitPriceMinor: 2000, LineSubtotalMinor: 2000, Quantity: 1},
		},
	}, 2)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrInvalidArgument)
}

func TestClassifyMachineLocalPricingSourceFromMirror_verifiedWhenMirrorMatches(t *testing.T) {
	t.Parallel()
	productID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	slotsJSON, err := json.Marshal([]mirrorSlotPrice{
		{SlotCode: "A1", ProductID: productID.String(), PriceMinor: 2000},
	})
	require.NoError(t, err)
	source := classifyMachineLocalPricingSourceFromMirror(
		MachinePricingSnapshotInput{
			SubtotalMinor:        2000,
			TaxMinor:             0,
			TotalMinor:           2000,
			LocalPricingRevision: 2,
			Lines: []MachinePricingSnapshotLineInput{
				{LineSequence: 1, ProductID: productID, SlotCode: "A1", UnitPriceMinor: 2000, LineSubtotalMinor: 2000, Quantity: 1},
			},
		},
		LocalLayoutMirror{Revision: 2, SlotsJSON: slotsJSON},
	)
	require.Equal(t, PricingSourceMachineLocalVerified, source)
}

func TestClassifyMachineLocalPricingSourceFromMirror_unverifiedWhenMirrorPriceDiffers(t *testing.T) {
	t.Parallel()
	productID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	slotsJSON, err := json.Marshal([]mirrorSlotPrice{
		{SlotCode: "A1", ProductID: productID.String(), PriceMinor: 15000},
	})
	require.NoError(t, err)
	source := classifyMachineLocalPricingSourceFromMirror(
		MachinePricingSnapshotInput{
			SubtotalMinor: 2000,
			TaxMinor:      0,
			TotalMinor:    2000,
			Lines: []MachinePricingSnapshotLineInput{
				{LineSequence: 1, ProductID: productID, SlotCode: "A1", UnitPriceMinor: 2000, LineSubtotalMinor: 2000, Quantity: 1},
			},
		},
		LocalLayoutMirror{Revision: 1, SlotsJSON: slotsJSON},
	)
	require.Equal(t, PricingSourceMachineLocalUnverified, source)
}

func TestClassifyMachineLocalPricingSourceFromMirror_unverifiedWhenRevisionStale(t *testing.T) {
	t.Parallel()
	productID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	slotsJSON, err := json.Marshal([]mirrorSlotPrice{
		{SlotCode: "A1", ProductID: productID.String(), PriceMinor: 2000},
	})
	require.NoError(t, err)
	source := classifyMachineLocalPricingSourceFromMirror(
		MachinePricingSnapshotInput{
			SubtotalMinor:        2000,
			TaxMinor:             0,
			TotalMinor:           2000,
			LocalPricingRevision: 1,
			Lines: []MachinePricingSnapshotLineInput{
				{LineSequence: 1, ProductID: productID, SlotCode: "A1", UnitPriceMinor: 2000, LineSubtotalMinor: 2000, Quantity: 1},
			},
		},
		LocalLayoutMirror{Revision: 5, SlotsJSON: slotsJSON},
	)
	require.Equal(t, PricingSourceMachineLocalUnverified, source)
}

func TestCreateQuote_adoptsSnapshotPayableWhenPresent(t *testing.T) {
	t.Parallel()
	machineID := uuidNew()
	productID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	slotID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	quotes := &captureQuoteStore{}
	svc := NewService(Deps{
		OrderVend: quotes,
		SaleLines: &recordingSaleLineResolver{
			line: ResolvedSaleLine{
				SlotConfigID:  slotID,
				CabinetCode:   "A",
				SlotCode:      "1",
				SlotIndex:     1,
				PriceMinor:    15000,
				SubtotalMinor: 15000,
				TotalMinor:    15000,
			},
		},
	})
	out, err := svc.CreateQuote(t.Context(), CreateQuoteInput{
		MachineID:      machineID,
		Currency:       "VND",
		IdempotencyKey: "quote-snapshot-1",
		Lines: []QuoteLineInput{{
			ProductID: productID,
			SlotCode:  "1",
		}},
		PricingSnapshot: &MachinePricingSnapshotInput{
			SubtotalMinor:        2000,
			TaxMinor:             0,
			TotalMinor:           2000,
			UnitPriceMinor:       2000,
			LocalPricingRevision: 3,
			CapturedAt:           time.Now().UTC(),
		},
	})
	require.NoError(t, err)
	require.Equal(t, int64(2000), out.PayableMinor)
	require.Equal(t, int64(2000), out.SubtotalMinor)
	require.Equal(t, int64(15000), out.ServerReferencePayableMinor)
	require.Equal(t, PricingSourceMachineLocalUnverified, out.PricingSource)
	require.Equal(t, int64(2000), quotes.lastInput.PayableMinor)
	require.Equal(t, int64(2000), quotes.lastInput.SubtotalMinor)
	require.NotNil(t, quotes.lastInput.ServerReferencePayableMinor)
	require.Equal(t, int64(15000), *quotes.lastInput.ServerReferencePayableMinor)
}

type captureQuoteStore struct {
	lastInput PersistQuoteInput
}

func (c *captureQuoteStore) CreateQuoteWithLines(_ context.Context, in PersistQuoteInput) (PersistQuoteResult, error) {
	c.lastInput = in
	return PersistQuoteResult{
		QuoteID:                     uuidNew(),
		MachineID:                   in.MachineID,
		Currency:                    in.Currency,
		SubtotalMinor:               in.SubtotalMinor,
		DiscountMinor:               in.DiscountMinor,
		PayableMinor:                in.PayableMinor,
		ExpiresAt:                   in.ExpiresAt,
		State:                       "active",
		PricingSource:               in.PricingSource,
		ServerReferencePayableMinor: derefInt64(in.ServerReferencePayableMinor),
		Lines:                       in.Lines,
	}, nil
}

func (c *captureQuoteStore) TryReplayQuoteByIdempotency(context.Context, uuid.UUID, string) (PersistQuoteResult, bool, error) {
	return PersistQuoteResult{}, false, nil
}

func (c *captureQuoteStore) GetQuoteWithLines(context.Context, uuid.UUID) (PersistQuoteResult, error) {
	return PersistQuoteResult{}, ErrNotFound
}

func (c *captureQuoteStore) CreateOrderFromQuoteWithVendSessions(context.Context, PersistOrderFromQuoteInput) (PersistOrderFromQuoteResult, error) {
	return PersistOrderFromQuoteResult{}, nil
}

func (c *captureQuoteStore) TryReplayOrderFromQuote(context.Context, uuid.UUID, string) (PersistOrderFromQuoteResult, bool, error) {
	return PersistOrderFromQuoteResult{}, false, nil
}

func (c *captureQuoteStore) CreateOrderWithVendSession(context.Context, domaincommerce.CreateOrderVendInput) (domaincommerce.CreateOrderVendResult, error) {
	return domaincommerce.CreateOrderVendResult{}, nil
}

func (c *captureQuoteStore) TryReplayCreateOrderWithVend(context.Context, uuid.UUID, string) (domaincommerce.CreateOrderVendResult, bool, error) {
	return domaincommerce.CreateOrderVendResult{}, false, nil
}

func derefInt64(v *int64) int64 {
	if v == nil {
		return 0
	}
	return *v
}
