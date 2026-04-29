package grpcserver

import (
	"testing"

	machinev1 "github.com/avf/avf-vending-api/proto/avf/machine/v1"
)

func TestCanonicalMachineMutationOperation_ReconcileIsIdentity(t *testing.T) {
	t.Parallel()
	if got := canonicalMachineMutationOperation(machinev1.MachineTelemetryService_ReconcileEvents_FullMethodName); got != machinev1.MachineTelemetryService_ReconcileEvents_FullMethodName {
		t.Fatalf("ReconcileEvents: got %q", got)
	}
}

func TestIsMachineIdempotentMutation_includesReconcileEvents(t *testing.T) {
	t.Parallel()
	if !isMachineIdempotentMutation(machinev1.MachineTelemetryService_ReconcileEvents_FullMethodName) {
		t.Fatal("expected ReconcileEvents to use idempotency ledger")
	}
}

func TestCanonicalMachineMutationOperation_MapsAliases(t *testing.T) {
	t.Parallel()
	if got := canonicalMachineMutationOperation(machinev1.MachineCommerceService_CreateCashCheckout_FullMethodName); got != machinev1.MachineCommerceService_ConfirmCashPayment_FullMethodName {
		t.Fatalf("CreateCashCheckout: got %q", got)
	}
	if got := canonicalMachineMutationOperation(machinev1.MachineCommerceService_AttachPaymentResult_FullMethodName); got != machinev1.MachineCommerceService_CreatePaymentSession_FullMethodName {
		t.Fatalf("AttachPaymentResult: got %q", got)
	}
	if got := canonicalMachineMutationOperation(machinev1.MachineInventoryService_SubmitFillReport_FullMethodName); got != machinev1.MachineInventoryService_SubmitFillResult_FullMethodName {
		t.Fatalf("SubmitFillReport: got %q", got)
	}
	if got := canonicalMachineMutationOperation(machinev1.MachineInventoryService_SubmitStockAdjustment_FullMethodName); got != machinev1.MachineInventoryService_SubmitInventoryAdjustment_FullMethodName {
		t.Fatalf("SubmitStockAdjustment: got %q", got)
	}
	if got := canonicalMachineMutationOperation(machinev1.MachineCommerceService_CreateOrder_FullMethodName); got != machinev1.MachineCommerceService_CreateOrder_FullMethodName {
		t.Fatalf("non-alias should be identity, got %q", got)
	}
}

func TestIsMachineIdempotentMutation_includesHeartbeatOperatorSession(t *testing.T) {
	t.Parallel()
	if !isMachineIdempotentMutation(machinev1.MachineOperatorService_HeartbeatOperatorSession_FullMethodName) {
		t.Fatal("expected HeartbeatOperatorSession to use idempotency ledger")
	}
}
