package mqtt

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
)

type rejectIngest struct{}

func (rejectIngest) IngestTelemetry(ctx context.Context, in TelemetryIngest) error {
	return fmt.Errorf("unexpected telemetry ingest")
}

func (rejectIngest) IngestShadowReported(ctx context.Context, in ShadowReportedIngest) error {
	return fmt.Errorf("unexpected shadow ingest")
}

func (rejectIngest) IngestCommandReceipt(ctx context.Context, in CommandReceiptIngest) error {
	return fmt.Errorf("unexpected receipt ingest")
}

func TestDispatch_rejectsOversizePayload(t *testing.T) {
	t.Parallel()
	mid := uuid.New()
	topic := fmt.Sprintf("pre/%s/telemetry", mid)
	payload := []byte(`{"event_type":"x","payload":{}}`)
	lim := &TelemetryIngressLimits{
		MaxPayloadBytes: 10,
		MaxPoints:       512,
		MaxTags:         128,
	}
	var gotReason string
	hooks := &IngestHooks{
		OnIngressRejected: func(_ string, reason string, _ int) {
			gotReason = reason
		},
	}
	err := Dispatch(context.Background(), "pre", topic, payload, rejectIngest{}, lim, hooks)
	if err == nil {
		t.Fatal("expected error")
	}
	if gotReason != "payload_too_large" {
		t.Fatalf("reason: got %q", gotReason)
	}
}
