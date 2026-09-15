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

func TestValidateMachinePricingSnapshot_acceptsMultiLineWhenUnitDiffersFromSubtotal(t *testing.T) {
	t.Parallel()
	productA := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	productB := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	err := validateMachinePricingSnapshot(MachinePricingSnapshotInput{
		SubtotalMinor:  100000,
		TaxMinor:       0,
		TotalMinor:     100000,
		UnitPriceMinor: 50000, // legacy first-line unit price, not the aggregate
		Lines: []MachinePricingSnapshotLineInput{
			{LineSequence: 1, ProductID: productA, SlotCode: "A1", Quantity: 1, UnitPriceMinor: 50000, LineSubtotalMinor: 50000},
			{LineSequence: 2, ProductID: productB, SlotCode: "A2", Quantity: 1, UnitPriceMinor: 50000, LineSubtotalMinor: 50000},
		},
	})
	require.NoError(t, err)
}

func TestValidateMachinePricingSnapshot_acceptsSingleLineQuantityGreaterThanOne(t *testing.T) {
	t.Parallel()
	productID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	err := validateMachinePricingSnapshot(MachinePricingSnapshotInput{
		SubtotalMinor:  40000,
		TaxMinor:       0,
		TotalMinor:     40000,
		UnitPriceMinor: 20000,
		Lines: []MachinePricingSnapshotLineInput{
			{LineSequence: 1, ProductID: productID, SlotCode: "A1", Quantity: 2, UnitPriceMinor: 20000, LineSubtotalMinor: 40000},
		},
	})
	require.NoError(t, err)
}

func TestValidateMachinePricingSnapshot_legacyNoLinesStillRequiresUnitEqualsSubtotal(t *testing.T) {
	t.Parallel()
	err := validateMachinePricingSnapshot(MachinePricingSnapshotInput{
		SubtotalMinor:  2000,
		TaxMinor:       0,
		TotalMinor:     2000,
		UnitPriceMinor: 1500,
	})
	require.Error(t, err)
	require.ErrorIs(t, err, ErrInvalidArgument)
	require.Contains(t, err.Error(), "unit_price_minor must match subtotal_minor for single-line order")
}

func TestValidateMachinePricingSnapshotMultiLine_rejectsZeroQuantity(t *testing.T) {
	t.Parallel()
	productID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	err := validateMachinePricingSnapshotMultiLine(MachinePricingSnapshotInput{
		SubtotalMinor: 2000,
		TaxMinor:      0,
		TotalMinor:    2000,
		Lines: []MachinePricingSnapshotLineInput{
			{LineSequence: 1, ProductID: productID, SlotCode: "A1", Quantity: 0, UnitPriceMinor: 2000, LineSubtotalMinor: 2000},
		},
	}, 1)
	require.Error(t, err)
	require.Contains(t, err.Error(), "quantity must be positive")
}

func TestValidateMachinePricingSnapshotMultiLine_rejectsDuplicateLineIdentity(t *testing.T) {
	t.Parallel()
	productID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	err := validateMachinePricingSnapshotMultiLine(MachinePricingSnapshotInput{
		SubtotalMinor: 4000,
		TaxMinor:      0,
		TotalMinor:    4000,
		Lines: []MachinePricingSnapshotLineInput{
			{LineSequence: 1, ProductID: productID, SlotCode: "A1", Quantity: 1, UnitPriceMinor: 2000, LineSubtotalMinor: 2000},
			{LineSequence: 2, ProductID: productID, SlotCode: "A1", Quantity: 1, UnitPriceMinor: 2000, LineSubtotalMinor: 2000},
		},
	}, 2)
	require.Error(t, err)
	require.Contains(t, err.Error(), "duplicate line identity")
}

func TestValidateMachinePricingSnapshotMultiLine_rejectsTamperedLineSubtotal(t *testing.T) {
	t.Parallel()
	productID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	err := validateMachinePricingSnapshotMultiLine(MachinePricingSnapshotInput{
		SubtotalMinor: 9999,
		TaxMinor:      0,
		TotalMinor:    9999,
		Lines: []MachinePricingSnapshotLineInput{
			{LineSequence: 1, ProductID: productID, SlotCode: "A1", Quantity: 1, UnitPriceMinor: 2000, LineSubtotalMinor: 9999},
		},
	}, 1)
	require.Error(t, err)
	require.Contains(t, err.Error(), "line subtotal mismatch")
}

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

func TestValidateMachinePricingSnapshotMultiLine_acceptsCapturedAtWithinFutureSkew(t *testing.T) {
	t.Parallel()
	productID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	err := validateMachinePricingSnapshotMultiLine(MachinePricingSnapshotInput{
		SubtotalMinor: 2000,
		TaxMinor:      0,
		TotalMinor:    2000,
		CapturedAt:    time.Now().UTC().Add(2 * time.Minute),
		Lines: []MachinePricingSnapshotLineInput{
			{LineSequence: 1, ProductID: productID, UnitPriceMinor: 2000, LineSubtotalMinor: 2000, Quantity: 1},
		},
	}, 1)
	require.NoError(t, err)
}

func TestValidateMachinePricingSnapshotMultiLine_rejectsCapturedAtBeyondFutureSkew(t *testing.T) {
	t.Parallel()
	productID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	err := validateMachinePricingSnapshotMultiLine(MachinePricingSnapshotInput{
		SubtotalMinor: 2000,
		TaxMinor:      0,
		TotalMinor:    2000,
		CapturedAt:    time.Now().UTC().Add(10 * time.Minute),
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
		{SlotCode: "A1", ProductID: productID.String(), PriceMinor: 2000, LocalPricingRevision: 2},
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
		{SlotCode: "A1", ProductID: productID.String(), PriceMinor: 15000, LocalPricingRevision: 1},
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
		{SlotCode: "A1", ProductID: productID.String(), PriceMinor: 2000, LocalPricingRevision: 1},
	})
	require.NoError(t, err)
	source := classifyMachineLocalPricingSourceFromMirror(
		MachinePricingSnapshotInput{
			SubtotalMinor:        2000,
			TaxMinor:             0,
			TotalMinor:           2000,
			LocalPricingRevision: 5,
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

func TestCreateQuote_acceptsMultiLineSnapshotWhenUnitDiffersFromSubtotal(t *testing.T) {
	t.Parallel()
	machineID := uuidNew()
	productA := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	productB := uuid.MustParse("44444444-4444-4444-4444-444444444444")
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
				PriceMinor:    50000,
				SubtotalMinor: 50000,
				TotalMinor:    50000,
			},
		},
	})
	out, err := svc.CreateQuote(t.Context(), CreateQuoteInput{
		MachineID:      machineID,
		Currency:       "VND",
		IdempotencyKey: "quote-snapshot-multi",
		Lines: []QuoteLineInput{
			{ProductID: productA, SlotCode: "A1", Quantity: 1},
			{ProductID: productB, SlotCode: "A2", Quantity: 1},
		},
		PricingSnapshot: &MachinePricingSnapshotInput{
			SubtotalMinor:        100000,
			TaxMinor:             0,
			TotalMinor:           100000,
			UnitPriceMinor:       50000,
			LocalPricingRevision: 3,
			CapturedAt:           time.Now().UTC(),
			Lines: []MachinePricingSnapshotLineInput{
				{LineSequence: 1, ProductID: productA, SlotCode: "A1", Quantity: 1, UnitPriceMinor: 50000, LineSubtotalMinor: 50000},
				{LineSequence: 2, ProductID: productB, SlotCode: "A2", Quantity: 1, UnitPriceMinor: 50000, LineSubtotalMinor: 50000},
			},
		},
	})
	require.NoError(t, err)
	require.Equal(t, int64(100000), out.PayableMinor)
	require.Equal(t, int64(100000), out.SubtotalMinor)
	require.Len(t, out.Lines, 2)
	require.Equal(t, int64(50000), out.Lines[0].UnitPriceMinor)
	require.Equal(t, int64(50000), out.Lines[1].UnitPriceMinor)
	require.Equal(t, int64(50000), out.Lines[0].LineSubtotalMinor)
	require.Equal(t, int64(50000), out.Lines[1].LineSubtotalMinor)
}

func TestCreateQuote_acceptsThreeLineMixedQuantities(t *testing.T) {
	t.Parallel()
	machineID := uuidNew()
	productA := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	productB := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	productC := uuid.MustParse("55555555-5555-5555-5555-555555555555")
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
				PriceMinor:    20000,
				SubtotalMinor: 20000,
				TotalMinor:    20000,
			},
		},
	})
	out, err := svc.CreateQuote(t.Context(), CreateQuoteInput{
		MachineID:      machineID,
		Currency:       "VND",
		IdempotencyKey: "quote-snapshot-three-mixed",
		Lines: []QuoteLineInput{
			{ProductID: productA, SlotCode: "A1", Quantity: 1},
			{ProductID: productB, SlotCode: "A2", Quantity: 2},
			{ProductID: productC, SlotCode: "A3", Quantity: 1},
		},
		PricingSnapshot: &MachinePricingSnapshotInput{
			SubtotalMinor:        80000,
			TaxMinor:             0,
			TotalMinor:           80000,
			UnitPriceMinor:       20000,
			LocalPricingRevision: 3,
			CapturedAt:           time.Now().UTC(),
			Lines: []MachinePricingSnapshotLineInput{
				{LineSequence: 1, ProductID: productA, SlotCode: "A1", Quantity: 1, UnitPriceMinor: 20000, LineSubtotalMinor: 20000},
				{LineSequence: 2, ProductID: productB, SlotCode: "A2", Quantity: 2, UnitPriceMinor: 20000, LineSubtotalMinor: 40000},
				{LineSequence: 3, ProductID: productC, SlotCode: "A3", Quantity: 1, UnitPriceMinor: 20000, LineSubtotalMinor: 20000},
			},
		},
	})
	require.NoError(t, err)
	require.Equal(t, int64(80000), out.PayableMinor)
	require.Len(t, out.Lines, 3)
	require.Equal(t, int32(2), out.Lines[1].Quantity)
	require.Equal(t, int64(40000), out.Lines[1].LineSubtotalMinor)
}

func TestValidateMachinePricingSnapshot_rejectsNegativeUnitPrice(t *testing.T) {
	t.Parallel()
	productID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	err := validateMachinePricingSnapshot(MachinePricingSnapshotInput{
		SubtotalMinor:  2000,
		TaxMinor:       0,
		TotalMinor:     2000,
		UnitPriceMinor: -1,
		Lines: []MachinePricingSnapshotLineInput{
			{LineSequence: 1, ProductID: productID, SlotCode: "A1", Quantity: 1, UnitPriceMinor: -1, LineSubtotalMinor: -1},
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "unit_price_minor out of range")
}

func TestValidateMachinePricingSnapshot_rejectsAggregateTotalMismatch(t *testing.T) {
	t.Parallel()
	productA := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	productB := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	err := validateMachinePricingSnapshot(MachinePricingSnapshotInput{
		SubtotalMinor:  100000,
		TaxMinor:       0,
		TotalMinor:     99999,
		UnitPriceMinor: 50000,
		Lines: []MachinePricingSnapshotLineInput{
			{LineSequence: 1, ProductID: productA, SlotCode: "A1", Quantity: 1, UnitPriceMinor: 50000, LineSubtotalMinor: 50000},
			{LineSequence: 2, ProductID: productB, SlotCode: "A2", Quantity: 1, UnitPriceMinor: 50000, LineSubtotalMinor: 50000},
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "line sum does not match total_minor")
}

func TestCreateQuote_rejectsMultiLineSnapshotWhenSubtotalDoesNotMatchLineSum(t *testing.T) {
	t.Parallel()
	machineID := uuidNew()
	productA := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	productB := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	svc := NewService(Deps{
		OrderVend: &captureQuoteStore{},
		SaleLines: &recordingSaleLineResolver{
			line: ResolvedSaleLine{PriceMinor: 50000, SubtotalMinor: 50000, TotalMinor: 50000},
		},
	})
	_, err := svc.CreateQuote(t.Context(), CreateQuoteInput{
		MachineID:      machineID,
		Currency:       "VND",
		IdempotencyKey: "quote-snapshot-tampered-subtotal",
		Lines: []QuoteLineInput{
			{ProductID: productA, SlotCode: "A1", Quantity: 1},
			{ProductID: productB, SlotCode: "A2", Quantity: 1},
		},
		PricingSnapshot: &MachinePricingSnapshotInput{
			SubtotalMinor:  999999,
			TaxMinor:       0,
			TotalMinor:     999999,
			UnitPriceMinor: 50000,
			CapturedAt:     time.Now().UTC(),
			Lines: []MachinePricingSnapshotLineInput{
				{LineSequence: 1, ProductID: productA, SlotCode: "A1", Quantity: 1, UnitPriceMinor: 50000, LineSubtotalMinor: 50000},
				{LineSequence: 2, ProductID: productB, SlotCode: "A2", Quantity: 1, UnitPriceMinor: 50000, LineSubtotalMinor: 50000},
			},
		},
	})
	require.Error(t, err)
	require.ErrorIs(t, err, ErrInvalidArgument)
	require.Contains(t, err.Error(), "subtotal does not match line sum")
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
