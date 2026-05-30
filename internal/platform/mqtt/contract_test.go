package mqtt

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestDevicePublishTopicStrict_enterpriseAllChannels(t *testing.T) {
	t.Parallel()
	prefix := "avf/production"
	mid := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	for _, rel := range EnterpriseDevicePublishRelPaths() {
		got, err := DevicePublishTopicStrict(TopicLayoutEnterprise, prefix, mid, rel)
		if err != nil {
			t.Fatalf("rel %q: %v", rel, err)
		}
		want, err := EnterpriseDeviceTopicStrict(prefix, mid, rel)
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Fatalf("rel %q: got %q want %q", rel, got, want)
		}
		if rel == RelTopicCommandsAck {
			wantAck := fmt.Sprintf("%s/machines/%s/commands/ack", prefix, mid)
			if got != wantAck {
				t.Fatalf("ack topic: got %q want %q", got, wantAck)
			}
		}
	}
}

func TestDevicePublishTopicStrict_legacyChannels(t *testing.T) {
	t.Parallel()
	prefix := "avf/devices"
	mid := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	for _, rel := range LegacyDevicePublishRelPaths() {
		got, err := DevicePublishTopicStrict(TopicLayoutLegacy, prefix, mid, rel)
		if err != nil {
			t.Fatalf("rel %q: %v", rel, err)
		}
		want := DeviceTopic(prefix, mid, rel)
		if got != want {
			t.Fatalf("rel %q: got %q want %q", rel, got, want)
		}
	}
}

func TestParseDeviceTopicWithLayout_rejectsCrossLayoutTopics(t *testing.T) {
	t.Parallel()
	prefix := "avf/staging"
	mid := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	legacyTopic := fmt.Sprintf("%s/%s/telemetry", prefix, mid)
	enterpriseTopic := fmt.Sprintf("%s/machines/%s/telemetry", prefix, mid)

	if _, _, err := ParseDeviceTopicWithLayout(TopicLayoutEnterprise, prefix, legacyTopic); err == nil {
		t.Fatal("expected enterprise layout to reject legacy topic path")
	}
	if _, _, err := ParseDeviceTopicWithLayout(TopicLayoutLegacy, prefix, enterpriseTopic); err == nil {
		t.Fatal("expected legacy layout to reject enterprise topic path")
	}
}

func TestParseDeviceTopic_rejectsWildcardPublishTopic(t *testing.T) {
	t.Parallel()
	prefix := "avf/devices"
	mid := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	_, _, err := ParseDeviceTopic(prefix, fmt.Sprintf("%s/+/telemetry", prefix))
	if err == nil {
		t.Fatal("expected wildcard topic rejection")
	}
	_, _, err = ParseDeviceTopicWithLayout(TopicLayoutEnterprise, prefix, prefix+"/machines/"+mid.String()+"/commands/#")
	if err == nil {
		t.Fatal("expected wildcard publish rejection")
	}
}

func TestDispatch_commandsAck_requiresErrorReasonOnFailure(t *testing.T) {
	t.Parallel()
	mid := uuid.MustParse("55555555-5555-5555-5555-555555555555")
	cmdID := uuid.MustParse("aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee")
	topic := fmt.Sprintf("pre/%s/commands/ack", mid)
	payload := []byte(fmt.Sprintf(
		`{"machine_id":%q,"command_id":%q,"occurred_at":"2020-04-01T12:00:00Z","sequence":1,"status":"failed","payload":{},"dedupe_key":"d-fail"}`,
		mid.String(), cmdID.String(),
	))
	err := Dispatch(context.Background(), TopicLayoutLegacy, "pre", topic, payload, &captureIngest{}, nil, nil)
	if err == nil {
		t.Fatal("expected missing error_reason rejection")
	}

	payload = []byte(fmt.Sprintf(
		`{"machine_id":%q,"command_id":%q,"occurred_at":"2020-04-01T12:00:00Z","sequence":1,"status":"failed","payload":{},"dedupe_key":"d-fail2","error_reason":"motor jam"}`,
		mid.String(), cmdID.String(),
	))
	var cap captureIngest
	if err := Dispatch(context.Background(), TopicLayoutLegacy, "pre", topic, payload, &cap, nil, nil); err != nil {
		t.Fatal(err)
	}
	if cap.lastReceipt == nil || cap.lastReceipt.ErrorReason != "motor jam" {
		t.Fatalf("receipt: %+v", cap.lastReceipt)
	}
}

func TestDispatch_enterpriseGenericEventsWithEventType(t *testing.T) {
	t.Parallel()
	mid := uuid.MustParse("66666666-6666-6666-6666-666666666666")
	topic := fmt.Sprintf("avf/staging/machines/%s/events", mid)
	var cap captureIngest
	payload := []byte(`{"event_type":"custom.event","payload":{"k":"v"}}`)
	if err := Dispatch(context.Background(), TopicLayoutEnterprise, "avf/staging", topic, payload, &cap, nil, nil); err != nil {
		t.Fatal(err)
	}
	if cap.lastTelemetry == nil || cap.lastTelemetry.EventType != "custom.event" {
		t.Fatalf("telemetry: %+v", cap.lastTelemetry)
	}
}

func TestDispatch_enterpriseTelemetryRoot(t *testing.T) {
	t.Parallel()
	mid := uuid.MustParse("77777777-7777-7777-7777-777777777777")
	topic, err := EnterpriseDeviceTopicStrict("avf/production", mid, RelTopicTelemetry)
	if err != nil {
		t.Fatal(err)
	}
	var cap captureIngest
	payload := []byte(`{"event_type":"telemetry.metrics","payload":{"temperature_c":21.5}}`)
	if err := Dispatch(context.Background(), TopicLayoutEnterprise, "avf/production", topic, payload, &cap, nil, nil); err != nil {
		t.Fatal(err)
	}
	if cap.lastTelemetry == nil || cap.lastTelemetry.EventType != "telemetry.metrics" {
		t.Fatalf("telemetry: %+v", cap.lastTelemetry)
	}
}

type countingReceiptIngest struct {
	captureIngest
	receiptCalls int
}

func (c *countingReceiptIngest) IngestCommandReceipt(ctx context.Context, in CommandReceiptIngest) error {
	c.receiptCalls++
	c.lastReceipt = &in
	return nil
}

func TestDispatch_commandsAck_duplicateDedupeKeyReachesIngestTwice(t *testing.T) {
	t.Parallel()
	mid := uuid.MustParse("88888888-8888-8888-8888-888888888888")
	cmdID := uuid.MustParse("aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee")
	topic := fmt.Sprintf("pre/%s/commands/ack", mid)
	payload := []byte(fmt.Sprintf(
		`{"machine_id":%q,"command_id":%q,"occurred_at":"2020-04-01T12:00:00Z","sequence":1,"status":"ack","payload":{},"dedupe_key":"dup-key-1"}`,
		mid.String(), cmdID.String(),
	))
	var ing countingReceiptIngest
	for i := 0; i < 2; i++ {
		if err := Dispatch(context.Background(), TopicLayoutLegacy, "pre", topic, payload, &ing, nil, nil); err != nil {
			t.Fatalf("dispatch %d: %v", i, err)
		}
	}
	if ing.receiptCalls != 2 {
		t.Fatalf("router passes through duplicate ACKs; idempotency is OLTP responsibility (calls=%d)", ing.receiptCalls)
	}
}

func TestValidateEdgeCommandReceipt_rejectsZeroOccurredAt(t *testing.T) {
	t.Parallel()
	mid := uuid.MustParse("99999999-9999-9999-9999-999999999999")
	cmdID := uuid.MustParse("aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee")
	if err := ValidateEdgeCommandReceipt(mid, cmdID, mid, time.Time{}); err == nil {
		t.Fatal("expected zero occurred_at rejection")
	}
}
