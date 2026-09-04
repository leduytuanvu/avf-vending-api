package commerce

import (
	"context"
	"testing"
	"time"

	domaincommerce "github.com/avf/avf-vending-api/internal/domain/commerce"
	machinev1 "github.com/avf/avf-vending-api/proto/avf/machine/v1"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestClassifyMachineLocalPricingSource_verifiedWhenServerMatches(t *testing.T) {
	t.Parallel()
	source := classifyMachineLocalPricingSource(
		MachinePricingSnapshotInput{UnitPriceMinor: 2000, SubtotalMinor: 2000, TotalMinor: 2000},
		ResolvedSaleLine{PriceMinor: 2000, SubtotalMinor: 2000, TotalMinor: 2000},
	)
	require.Equal(t, PricingSourceMachineLocalVerified, source)
}

func TestClassifyMachineLocalPricingSource_unverifiedWhenServerDiffers(t *testing.T) {
	t.Parallel()
	source := classifyMachineLocalPricingSource(
		MachinePricingSnapshotInput{UnitPriceMinor: 2000, SubtotalMinor: 2000, TotalMinor: 2000},
		ResolvedSaleLine{PriceMinor: 15000, SubtotalMinor: 15000, TotalMinor: 15000},
	)
	require.Equal(t, PricingSourceMachineLocalUnverified, source)
}

func TestValidateMachinePricingSnapshot_acceptsConsistentTotals(t *testing.T) {
	t.Parallel()
	err := validateMachinePricingSnapshot(MachinePricingSnapshotInput{
		SubtotalMinor:  2000,
		TaxMinor:       0,
		TotalMinor:     2000,
		UnitPriceMinor: 2000,
	})
	require.NoError(t, err)
}

func TestValidateMachinePricingSnapshot_rejectsMismatchedTotal(t *testing.T) {
	t.Parallel()
	err := validateMachinePricingSnapshot(MachinePricingSnapshotInput{
		SubtotalMinor:  2000,
		TaxMinor:       100,
		TotalMinor:     2000,
		UnitPriceMinor: 2000,
	})
	require.Error(t, err)
	require.ErrorIs(t, err, ErrInvalidArgument)
}

func TestValidateMachinePricingSnapshot_rejectsUnitSubtotalMismatch(t *testing.T) {
	t.Parallel()
	err := validateMachinePricingSnapshot(MachinePricingSnapshotInput{
		SubtotalMinor:  2000,
		TaxMinor:       0,
		TotalMinor:     2000,
		UnitPriceMinor: 1500,
	})
	require.Error(t, err)
}

func TestMachinePricingSnapshotFromProto_mapsFields(t *testing.T) {
	t.Parallel()
	captured := time.Date(2026, 3, 5, 12, 0, 0, 0, time.UTC)
	snap, err := machinePricingSnapshotFromProto(&machinev1.MachinePricingSnapshot{
		SubtotalMinor:        2000,
		TaxMinor:             0,
		TotalMinor:           2000,
		UnitPriceMinor:       2000,
		LocalPricingRevision: 3,
		PricingFingerprint:   "fp-abc",
		CapturedAt:           timestamppb.New(captured),
	})
	require.NoError(t, err)
	require.Equal(t, int64(2000), snap.TotalMinor)
	require.Equal(t, int64(3), snap.LocalPricingRevision)
	require.Equal(t, "fp-abc", snap.PricingFingerprint)
	require.Equal(t, captured, snap.CapturedAt)
}

func TestCreateOrder_withPricingSnapshot_skipsServerRepricing(t *testing.T) {
	t.Parallel()
	machineID := uuidNew()
	productID := uuidNew()
	slotID := uuidNew()
	resolver := &recordingSaleLineResolver{
		line: ResolvedSaleLine{
			SlotConfigID:  slotID,
			CabinetCode:   "A",
			SlotCode:      "1",
			SlotIndex:     1,
			PriceMinor:    15000,
			SubtotalMinor: 15000,
			TaxMinor:      0,
			TotalMinor:    15000,
		},
	}
	orders := &captureOrderVendWorkflow{}
	svc := NewService(Deps{
		OrderVend: orders,
		SaleLines: resolver,
	})
	out, err := svc.CreateOrder(t.Context(), CreateOrderInput{
		MachineID:      machineID,
		ProductID:      productID,
		SlotIndex:      ptrInt32(1),
		Currency:       "VND",
		IdempotencyKey: "offline-sale-1",
		PricingSnapshot: &MachinePricingSnapshotInput{
			SubtotalMinor:        2000,
			TaxMinor:             0,
			TotalMinor:           2000,
			UnitPriceMinor:       2000,
			LocalPricingRevision: 7,
		},
	})
	require.NoError(t, err)
	require.True(t, resolver.called, "slot identity should still be resolved")
	require.Equal(t, int64(2000), out.Order.TotalMinor)
	require.Equal(t, PricingSourceMachineLocalUnverified, orders.lastInput.PricingSource)
	require.NotNil(t, orders.lastInput.MachinePricingRevision)
	require.Equal(t, int64(7), *orders.lastInput.MachinePricingRevision)
	require.NotEmpty(t, orders.lastInput.MachinePricingSnapshot)
}

func TestCreateOrder_withPricingSnapshot_marksVerifiedWhenServerPriceMatches(t *testing.T) {
	t.Parallel()
	machineID := uuidNew()
	productID := uuidNew()
	slotID := uuidNew()
	resolver := &recordingSaleLineResolver{
		line: ResolvedSaleLine{
			SlotConfigID:  slotID,
			CabinetCode:   "A",
			SlotCode:      "1",
			SlotIndex:     1,
			PriceMinor:    2000,
			SubtotalMinor: 2000,
			TaxMinor:      0,
			TotalMinor:    2000,
		},
	}
	orders := &captureOrderVendWorkflow{}
	svc := NewService(Deps{
		OrderVend: orders,
		SaleLines: resolver,
	})
	_, err := svc.CreateOrder(t.Context(), CreateOrderInput{
		MachineID:      machineID,
		ProductID:      productID,
		SlotIndex:      ptrInt32(1),
		Currency:       "VND",
		IdempotencyKey: "offline-sale-verified",
		PricingSnapshot: &MachinePricingSnapshotInput{
			SubtotalMinor:        2000,
			TaxMinor:             0,
			TotalMinor:           2000,
			UnitPriceMinor:       2000,
			LocalPricingRevision: 7,
		},
	})
	require.NoError(t, err)
	require.Equal(t, PricingSourceMachineLocalVerified, orders.lastInput.PricingSource)
}

func TestCreateOrder_withPricingSnapshot_rejectsTamperedTotal(t *testing.T) {
	t.Parallel()
	svc := NewService(Deps{
		OrderVend: &captureOrderVendWorkflow{},
		SaleLines: &recordingSaleLineResolver{
			line: ResolvedSaleLine{SlotIndex: 1, SubtotalMinor: 15000, TotalMinor: 15000, PriceMinor: 15000},
		},
	})
	_, err := svc.CreateOrder(t.Context(), CreateOrderInput{
		MachineID:      uuidNew(),
		ProductID:      uuidNew(),
		SlotIndex:      ptrInt32(1),
		Currency:       "VND",
		IdempotencyKey: "offline-sale-bad",
		PricingSnapshot: &MachinePricingSnapshotInput{
			SubtotalMinor:  2000,
			TaxMinor:       0,
			TotalMinor:     2500,
			UnitPriceMinor: 2000,
		},
	})
	require.Error(t, err)
	require.ErrorIs(t, err, ErrInvalidArgument)
}

func TestCreateOrder_offlineReplayPreservesDeclaredPrice(t *testing.T) {
	t.Parallel()
	machineID := uuidNew()
	orderID := uuidNew()
	vendID := uuidNew()
	replay := domainCreateOrderVendResult(machineID, orderID, vendID, 2000, PricingSourceMachineLocalUnverified)
	orders := &replayOrderVendWorkflow{replay: replay}
	svc := NewService(Deps{
		OrderVend: orders,
		SaleLines: &recordingSaleLineResolver{},
	})
	out, err := svc.CreateOrder(t.Context(), CreateOrderInput{
		MachineID:      machineID,
		ProductID:      replay.Vend.ProductID,
		SlotIndex:      ptrInt32(replay.Vend.SlotIndex),
		Currency:       "VND",
		IdempotencyKey: "idem-replay",
		PricingSnapshot: &MachinePricingSnapshotInput{
			SubtotalMinor:  2000,
			TaxMinor:       0,
			TotalMinor:     2000,
			UnitPriceMinor: 2000,
		},
	})
	require.NoError(t, err)
	require.True(t, out.Replay)
	require.Equal(t, int64(2000), out.Order.TotalMinor)
	require.Equal(t, PricingSourceMachineLocalUnverified, out.Order.PricingSource)
}

func TestConfirmCashPayment_allocatedMustMatchOrderTotal(t *testing.T) {
	t.Parallel()
	orderID := uuidNew()
	machineID := uuidNew()
	life := &cashConfirmLifecycle{
		order: domainOrder(orderID, machineID, 2000, PricingSourceMachineLocalUnverified),
	}
	payments := &cashConfirmPayments{}
	svc := NewService(Deps{
		OrderVend:     stubOrderVendWorkflow{},
		PaymentOutbox: payments,
		Lifecycle:     life,
		SaleLines:     stubSaleLineResolver{},
	})
	_, err := svc.ConfirmCashPayment(t.Context(), ConfirmCashPaymentInput{
		OrderID:        orderID,
		MachineID:      machineID,
		IdempotencyKey: "cash-1",
		AllocatedMinor: 15000,
		GrossAcceptedMinor: 15000,
		ConsentSource:  "explicit_confirm",
		Currency:       "VND",
		OutboxTopic:    "commerce",
		OutboxEventType: "cash",
		OutboxAggregateType: "order",
	})
	require.Error(t, err)
	require.ErrorIs(t, err, ErrInvalidArgument)
}

func TestConfirmCashPayment_acceptsMatchingAllocatedForMachineLocalOrder(t *testing.T) {
	t.Parallel()
	orderID := uuidNew()
	machineID := uuidNew()
	life := &cashConfirmLifecycle{
		order: domainOrder(orderID, machineID, 2000, PricingSourceMachineLocalUnverified),
	}
	payments := &cashConfirmPayments{}
	svc := NewService(Deps{
		OrderVend:     stubOrderVendWorkflow{},
		PaymentOutbox: payments,
		Lifecycle:     life,
		SaleLines:     stubSaleLineResolver{},
	})
	life.payments = payments
	res, err := svc.ConfirmCashPayment(t.Context(), ConfirmCashPaymentInput{
		OrderID:        orderID,
		MachineID:      machineID,
		IdempotencyKey: "cash-2",
		AllocatedMinor: 2000,
		GrossAcceptedMinor: 2000,
		ConsentSource:  "explicit_confirm",
		Currency:       "VND",
		OutboxTopic:    "commerce",
		OutboxEventType: "cash",
		OutboxAggregateType: "order",
	})
	require.NoError(t, err)
	require.Equal(t, int64(2000), res.Payment.AmountMinor)
}

// --- test helpers ---

func ptrInt32(v int32) *int32 { return &v }

func uuidNew() uuid.UUID { return uuid.MustParse("11111111-1111-1111-1111-111111111111") }

type recordingSaleLineResolver struct {
	called bool
	line   ResolvedSaleLine
}

func (r *recordingSaleLineResolver) ResolveSaleLine(context.Context, ResolveSaleLineInput) (ResolvedSaleLine, error) {
	r.called = true
	return r.line, nil
}

func (r *recordingSaleLineResolver) LookupSlotDisplay(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, int32) (ResolvedSaleLine, error) {
	return r.line, nil
}

type captureOrderVendWorkflow struct {
	lastInput domaincommerce.CreateOrderVendInput
}

func (c *captureOrderVendWorkflow) CreateOrderWithVendSession(_ context.Context, in domaincommerce.CreateOrderVendInput) (domaincommerce.CreateOrderVendResult, error) {
	c.lastInput = in
	return domaincommerce.CreateOrderVendResult{
		Order: domaincommerce.Order{
			ID:            uuidNew(),
			MachineID:     in.MachineID,
			Status:        in.OrderStatus,
			Currency:      in.Currency,
			SubtotalMinor: in.SubtotalMinor,
			TaxMinor:      in.TaxMinor,
			TotalMinor:    in.TotalMinor,
			PricingSource: in.PricingSource,
		},
		Vend: domaincommerce.VendSession{SlotIndex: in.SlotIndex, ProductID: in.ProductID},
	}, nil
}

func (c *captureOrderVendWorkflow) TryReplayCreateOrderWithVend(context.Context, uuid.UUID, string) (domaincommerce.CreateOrderVendResult, bool, error) {
	return domaincommerce.CreateOrderVendResult{}, false, nil
}

type replayOrderVendWorkflow struct {
	replay domaincommerce.CreateOrderVendResult
}

func (r *replayOrderVendWorkflow) CreateOrderWithVendSession(context.Context, domaincommerce.CreateOrderVendInput) (domaincommerce.CreateOrderVendResult, error) {
	return domaincommerce.CreateOrderVendResult{}, nil
}

func (r *replayOrderVendWorkflow) TryReplayCreateOrderWithVend(context.Context, uuid.UUID, string) (domaincommerce.CreateOrderVendResult, bool, error) {
	return r.replay, true, nil
}

func domainCreateOrderVendResult(machineID, orderID, vendID uuid.UUID, total int64, pricingSource string) domaincommerce.CreateOrderVendResult {
	productID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	return domaincommerce.CreateOrderVendResult{
		Order: domaincommerce.Order{
			ID:            orderID,
			MachineID:     machineID,
			Status:        "created",
			Currency:      "VND",
			SubtotalMinor: total,
			TotalMinor:    total,
			PricingSource: pricingSource,
		},
		Vend: domaincommerce.VendSession{
			ID:        vendID,
			OrderID:   orderID,
			MachineID: machineID,
			ProductID: productID,
			SlotIndex: 1,
			State:     "pending",
		},
		Replay: true,
	}
}

func domainOrder(orderID, machineID uuid.UUID, total int64, pricingSource string) domaincommerce.Order {
	return domaincommerce.Order{
		ID:            orderID,
		MachineID:     machineID,
		Status:        "created",
		Currency:      "VND",
		TotalMinor:    total,
		SubtotalMinor: total,
		PricingSource: pricingSource,
	}
}

type cashConfirmLifecycle struct {
	order    domaincommerce.Order
	payments *cashConfirmPayments
}

func (c *cashConfirmLifecycle) GetOrderByID(context.Context, uuid.UUID) (domaincommerce.Order, error) {
	return c.order, nil
}

func (c *cashConfirmLifecycle) UpdateOrderStatus(context.Context, uuid.UUID, uuid.UUID, string) (domaincommerce.Order, error) {
	c.order.Status = "paid"
	return c.order, nil
}

func (c *cashConfirmLifecycle) GetVendSessionByOrderAndSlot(context.Context, uuid.UUID, int32) (domaincommerce.VendSession, error) {
	return domaincommerce.VendSession{}, ErrNotFound
}

func (c *cashConfirmLifecycle) GetVendSessionByOrderAndLineSequence(context.Context, uuid.UUID, int32) (domaincommerce.VendSession, error) {
	return domaincommerce.VendSession{}, ErrNotFound
}

func (c *cashConfirmLifecycle) ListVendSessionsForOrder(context.Context, uuid.UUID) ([]domaincommerce.VendSession, error) {
	return nil, nil
}

func (c *cashConfirmLifecycle) UpdateVendSessionState(context.Context, UpdateVendSessionParams) (domaincommerce.VendSession, error) {
	return domaincommerce.VendSession{}, ErrNotConfigured
}

func (c *cashConfirmLifecycle) GetLatestPaymentForOrder(context.Context, uuid.UUID) (domaincommerce.Payment, error) {
	if c.payments != nil && c.payments.lastPayment.ID != uuid.Nil {
		return c.payments.lastPayment, nil
	}
	return domaincommerce.Payment{}, ErrNotFound
}

func (c *cashConfirmLifecycle) GetPaymentByID(context.Context, uuid.UUID) (domaincommerce.Payment, error) {
	return domaincommerce.Payment{}, ErrNotFound
}

func (c *cashConfirmLifecycle) GetLatestPaymentAttemptProviderReference(context.Context, uuid.UUID) (string, error) {
	return "", nil
}

func (c *cashConfirmLifecycle) GetLatestPaymentAttemptPayload(context.Context, uuid.UUID) ([]byte, error) {
	return nil, nil
}

func (c *cashConfirmLifecycle) InsertPaymentAttempt(context.Context, InsertPaymentAttemptParams) (PaymentAttemptView, error) {
	return PaymentAttemptView{}, ErrNotConfigured
}

func (c *cashConfirmLifecycle) InsertRefundRow(context.Context, InsertRefundRowInput) (RefundRowView, error) {
	return RefundRowView{}, ErrNotConfigured
}

func (c *cashConfirmLifecycle) ListRefundsForOrder(context.Context, uuid.UUID) ([]RefundRowView, error) {
	return nil, nil
}

func (c *cashConfirmLifecycle) GetRefundByIDForOrder(context.Context, uuid.UUID, uuid.UUID) (RefundRowView, error) {
	return RefundRowView{}, ErrNotFound
}

func (c *cashConfirmLifecycle) GetRefundByOrderIdempotency(context.Context, uuid.UUID, string) (RefundRowView, error) {
	return RefundRowView{}, ErrNotFound
}

func (c *cashConfirmLifecycle) SumNonFailedRefundAmountForPayment(context.Context, uuid.UUID) (int64, error) {
	return 0, nil
}

func (c *cashConfirmLifecycle) FulfillSuccessfulVendAtomically(context.Context, FulfillSuccessfulVendInput) (FulfillSuccessfulVendResult, error) {
	return FulfillSuccessfulVendResult{}, ErrNotConfigured
}

func (c *cashConfirmLifecycle) FulfillFailedVendAtomically(context.Context, FulfillFailedVendInput) (FulfillFailedVendResult, error) {
	return FulfillFailedVendResult{}, ErrNotConfigured
}

type cashConfirmPayments struct {
	lastPayment domaincommerce.Payment
}

func (c *cashConfirmPayments) CreatePaymentWithOutbox(_ context.Context, in domaincommerce.PaymentOutboxInput) (domaincommerce.PaymentOutboxResult, error) {
	c.lastPayment = domaincommerce.Payment{
		ID:          uuid.MustParse("33333333-3333-3333-3333-333333333333"),
		OrderID:     in.OrderID,
		State:       in.PaymentState,
		AmountMinor: in.AmountMinor,
		Currency:    in.Currency,
	}
	return domaincommerce.PaymentOutboxResult{Payment: c.lastPayment}, nil
}
