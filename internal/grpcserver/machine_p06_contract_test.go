package grpcserver

import (
	"testing"

	machinev1 "github.com/avf/avf-vending-api/proto/avf/machine/v1"
)

func TestP06MachineMethodsRequireMachineJWT(t *testing.T) {
	t.Parallel()

	methods := []string{
		machinev1.MachineInventoryService_GetPlanogram_FullMethodName,
		machinev1.MachineInventoryService_SubmitStockSnapshot_FullMethodName,
		machinev1.MachineInventoryService_SubmitFillResult_FullMethodName,
		machinev1.MachineInventoryService_SubmitFillReport_FullMethodName,
		machinev1.MachineInventoryService_SubmitStockAdjustment_FullMethodName,
		machinev1.MachineTelemetryService_CheckIn_FullMethodName,
		machinev1.MachineTelemetryService_SubmitTelemetryBatch_FullMethodName,
		machinev1.MachineTelemetryService_ReconcileEvents_FullMethodName,
		machinev1.MachineTelemetryService_GetEventStatus_FullMethodName,
	}
	for _, method := range methods {
		method := method
		t.Run(method, func(t *testing.T) {
			t.Parallel()
			if !requiresMachineAccessJWT(method) {
				t.Fatalf("%s must require machine JWT", method)
			}
		})
	}
}

func TestTelemetryEventDedupeKey(t *testing.T) {
	t.Parallel()

	if got := telemetryEventDedupeKey("batch-1", "event-1", 0); got != "batch-1:event-1" {
		t.Fatalf("event id key = %q", got)
	}
	if got := telemetryEventDedupeKey("batch-1", "", 2); got != "batch-1:2" {
		t.Fatalf("index key = %q", got)
	}
}
