package grpcserver

import (
	"testing"

	"github.com/avf/avf-vending-api/internal/app/machineruntime"
	machinev1 "github.com/avf/avf-vending-api/proto/avf/machine/v1"
)

func TestOfflineProvenReplay_requiresProcessedAndMatchingHash(t *testing.T) {
	payload := []byte(`{"orderId":"456c0923-a516-4002-9842-dc69d1fbc7d9"}`)
	other := []byte(`{"orderId":"501e15b3-6768-4a84-ac8f-1cfb67df22d1"}`)
	if !offlineProvenReplay("processed", payload, payload) {
		t.Fatal("same payload + processed must be a proven replay")
	}
	if offlineProvenReplay("processed", payload, other) {
		t.Fatal("new client payload must not skip dispatch")
	}
	if offlineProvenReplay("rejected", payload, payload) {
		t.Fatal("rejected ledger rows must redispatch")
	}
	if offlineProvenReplay("failed_terminal", payload, payload) {
		t.Fatal("failed_terminal ledger rows must redispatch")
	}
}

func TestOfflineEventStreamKind_splitsCashFromCommerce(t *testing.T) {
	if got := offlineEventStreamKind("commerce.report_cash_movements"); got != machineruntime.OfflineStreamKindCash {
		t.Fatalf("cash kind=%q", got)
	}
	if got := offlineEventStreamKind("commerce.offline_sale"); got != machineruntime.OfflineStreamKindCommerce {
		t.Fatalf("sale kind=%q", got)
	}
	ev := &machinev1.OfflineEvent{
		Meta:      &machinev1.MachineRequestMeta{StreamId: "install-1"},
		EventType: "commerce.report_cash_movements",
	}
	if got := resolveOfflineStreamNameForEvent(nil, ev); got != "install-1:cash" {
		t.Fatalf("stream=%q", got)
	}
}

func TestStaleOfflineSkipReason_newClientEventOccupiesSlot(t *testing.T) {
	if got := staleOfflineSkipReason("", false); got != "offline sequence already synced" {
		t.Fatalf("empty client event id: %q", got)
	}
	if got := staleOfflineSkipReason("be6f4c3d-45d8-43e7-b8d5-52b6971a37e0", false); got != "offline sequence slot occupied" {
		t.Fatalf("new client event id: %q", got)
	}
}

func TestShouldAdvanceOfflineStreamCursor_skipsForeignSlotOccupancy(t *testing.T) {
	accepted := &machinev1.OfflineEventResult{
		Status: machinev1.MachineResponseStatus_MACHINE_RESPONSE_STATUS_ACCEPTED,
	}
	if !shouldAdvanceOfflineStreamCursor(accepted) {
		t.Fatal("accepted must advance cursor")
	}
	proven := &machinev1.OfflineEventResult{
		Status: machinev1.MachineResponseStatus_MACHINE_RESPONSE_STATUS_REPLAYED,
		Reason: "offline event replayed",
	}
	if !shouldAdvanceOfflineStreamCursor(proven) {
		t.Fatal("proven replay must advance cursor")
	}
	occupied := &machinev1.OfflineEventResult{
		Status: machinev1.MachineResponseStatus_MACHINE_RESPONSE_STATUS_REPLAYED,
		Reason: "offline sequence slot occupied",
	}
	if shouldAdvanceOfflineStreamCursor(occupied) {
		t.Fatal("slot occupied must not advance cursor")
	}
	alreadySynced := &machinev1.OfflineEventResult{
		Status: machinev1.MachineResponseStatus_MACHINE_RESPONSE_STATUS_REPLAYED,
		Reason: "offline sequence already synced",
	}
	if shouldAdvanceOfflineStreamCursor(alreadySynced) {
		t.Fatal("already synced of a different event must not advance cursor")
	}
	rejected := &machinev1.OfflineEventResult{
		Status: machinev1.MachineResponseStatus_MACHINE_RESPONSE_STATUS_REJECTED,
		Reason: "invalid cash movements payload",
	}
	if shouldAdvanceOfflineStreamCursor(rejected) {
		t.Fatal("rejected must not advance cursor")
	}
}

func TestUnmarshalOfflineCashPayload_discardsUnknown(t *testing.T) {
	raw := []byte(`{"events":[{"kind":"bill_credit","deviceEventId":"090204","denominationMinor":10000,"rawRecordHex":"4:10000:probe_rehydrate","probe_rehydrate":true,"currency":"VND"}]}`)
	var req machinev1.ReportCashMovementsRequest
	if err := unmarshalOfflineCashPayload(raw, &req); err != nil {
		t.Fatalf("discard unknown: %v", err)
	}
	if len(req.GetEvents()) != 1 || req.GetEvents()[0].GetDeviceEventId() != "090204" {
		t.Fatalf("events=%v", req.GetEvents())
	}
}
