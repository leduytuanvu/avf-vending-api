package telemetryapp

import (
	"context"
	"testing"

	tel "github.com/avf/avf-vending-api/internal/platform/telemetry"
	"github.com/google/uuid"
)

func TestHandleDeviceHealthSnapshot_emptyCustomFieldsIsNoop(t *testing.T) {
	t.Parallel()
	w := &JetStreamWorkers{}
	err := w.handleDeviceHealthSnapshot(context.Background(), tel.Envelope{
		MachineID:   uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"),
		SourceEvent: "telemetry.snapshot",
		Payload:     []byte(`{"event_type":"telemetry.snapshot","data":{"timestamp":1,"syncPendingCount":9}}`),
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestHandleMetrics_routesSnapshotAwayFromGenericRollup(t *testing.T) {
	t.Parallel()
	w := &JetStreamWorkers{}
	err := w.handleMetrics(context.Background(), tel.Envelope{
		MachineID:   uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"),
		SourceEvent: "telemetry.snapshot",
		Payload:     []byte(`{"event_type":"telemetry.snapshot","data":{"timestamp":99,"customFields":{}}}`),
	})
	if err != nil {
		t.Fatal(err)
	}
}
