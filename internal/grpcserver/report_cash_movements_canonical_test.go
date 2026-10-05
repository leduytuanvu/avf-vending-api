package grpcserver

import (
	"testing"

	machinev1 "github.com/avf/avf-vending-api/proto/avf/machine/v1"
	"google.golang.org/protobuf/encoding/protojson"
)

// Golden JSON shape produced by the Android OutboxCanonicalMapper after remapping snake_case events.
func TestReportCashMovementsRequest_protojsonUnmarshal_canonicalBillCredit(t *testing.T) {
	t.Parallel()
	const payload = `{
  "context": {
    "idempotencyKey": "cash_movement:01a0a7e5-3c68-7895-b526-bcb6504bccfb:230004",
    "clientEventId": "ev-bill-230004"
  },
  "events": [{
    "kind": "bill_credit",
    "deviceEventId": "bill:230004",
    "occurredAt": "2023-11-14T22:13:20.123Z",
    "denominationMinor": 10000,
    "rawRecordHex": "4:10000:230004",
    "currency": "VND"
  }]
}`
	var req machinev1.ReportCashMovementsRequest
	if err := protojson.Unmarshal([]byte(payload), &req); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(req.Events) != 1 {
		t.Fatalf("events: got %d", len(req.Events))
	}
	if req.Events[0].DenominationMinor != 10000 {
		t.Fatalf("denomination: got %d", req.Events[0].DenominationMinor)
	}
	if req.Events[0].RawRecordHex != "4:10000:230004" {
		t.Fatalf("rawRecordHex: got %q", req.Events[0].RawRecordHex)
	}
}

func TestReportCashMovementsRequest_protojsonUnmarshal_rejectsSnakeCaseEvents(t *testing.T) {
	t.Parallel()
	const payload = `{
  "context": {"idempotencyKey": "k", "clientEventId": "c"},
  "events": [{
    "kind": "bill_credit",
    "device_event_id": "bill:230004",
    "occurred_at_millis": 1700000000123,
    "denomination_minor": 10000
  }]
}`
	var req machinev1.ReportCashMovementsRequest
	if err := protojson.Unmarshal([]byte(payload), &req); err != nil {
		return
	}
	if len(req.Events) == 1 && req.Events[0].DenominationMinor == 10000 && req.Events[0].DeviceEventId == "bill:230004" {
		t.Fatal("expected snake_case event fields not to map under protojson")
	}
}
