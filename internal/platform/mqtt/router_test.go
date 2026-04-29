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

func (rejectIngest) IngestShadowDesired(ctx context.Context, in ShadowDesiredIngest) error {
	return fmt.Errorf("unexpected shadow desired ingest")
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
	err := Dispatch(context.Background(), TopicLayoutLegacy, "pre", topic, payload, rejectIngest{}, lim, hooks)
	if err == nil {
		t.Fatal("expected error")
	}
	if gotReason != "payload_too_large" {
		t.Fatalf("reason: got %q", gotReason)
	}
}

type captureIngest struct {
	lastTelemetry *TelemetryIngest
	lastReceipt   *CommandReceiptIngest
}

func (c *captureIngest) IngestTelemetry(ctx context.Context, in TelemetryIngest) error {
	c.lastTelemetry = &in
	return nil
}

func (c *captureIngest) IngestShadowReported(ctx context.Context, in ShadowReportedIngest) error {
	return fmt.Errorf("unexpected shadow")
}

func (c *captureIngest) IngestShadowDesired(ctx context.Context, in ShadowDesiredIngest) error {
	return fmt.Errorf("unexpected shadow desired")
}

func (c *captureIngest) IngestCommandReceipt(ctx context.Context, in CommandReceiptIngest) error {
	c.lastReceipt = &in
	return nil
}

func TestParseDeviceTopicWithLayout_enterprise(t *testing.T) {
	t.Parallel()
	prefix := "avf/staging"
	mid := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	topic := prefix + "/machines/" + mid.String() + "/commands/ack"
	gotMid, ch, err := ParseDeviceTopicWithLayout(TopicLayoutEnterprise, prefix, topic)
	if err != nil {
		t.Fatal(err)
	}
	if gotMid != mid || ch != "commands/ack" {
		t.Fatalf("mid=%s ch=%q", gotMid, ch)
	}
}

func TestParseDeviceTopic_newChannels(t *testing.T) {
	t.Parallel()
	prefix := "avf/devices"
	mid := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	cases := []struct {
		topic   string
		channel string
	}{
		{fmt.Sprintf("%s/%s/presence", prefix, mid), "presence"},
		{fmt.Sprintf("%s/%s/state/heartbeat", prefix, mid), "state/heartbeat"},
		{fmt.Sprintf("%s/%s/telemetry/snapshot", prefix, mid), "telemetry/snapshot"},
		{fmt.Sprintf("%s/%s/events/inventory", prefix, mid), "events/inventory"},
		{fmt.Sprintf("%s/%s/commands/ack", prefix, mid), "commands/ack"},
	}
	for _, tc := range cases {
		gotMid, ch, err := ParseDeviceTopic(prefix, tc.topic)
		if err != nil {
			t.Fatalf("topic %s: %v", tc.topic, err)
		}
		if gotMid != mid || ch != tc.channel {
			t.Fatalf("topic %s: mid=%s ch=%q want ch=%q", tc.topic, gotMid, ch, tc.channel)
		}
	}
}

func TestDispatch_stateHeartbeat_derivesEventType(t *testing.T) {
	t.Parallel()
	mid := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	topic := fmt.Sprintf("pre/%s/state/heartbeat", mid)
	var cap captureIngest
	err := Dispatch(context.Background(), TopicLayoutLegacy, "pre", topic, []byte(`{"boot_id":"55555555-5555-5555-5555-555555555555","seq_no":7,"payload":{}}`), &cap, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if cap.lastTelemetry == nil {
		t.Fatal("expected telemetry ingest")
	}
	if cap.lastTelemetry.EventType != "state.heartbeat" {
		t.Fatalf("event type: %q", cap.lastTelemetry.EventType)
	}
	if cap.lastTelemetry.BootID == nil || *cap.lastTelemetry.BootID != uuid.MustParse("55555555-5555-5555-5555-555555555555") {
		t.Fatalf("boot_id not set")
	}
	if cap.lastTelemetry.SeqNo == nil || *cap.lastTelemetry.SeqNo != 7 {
		t.Fatalf("seq_no not set")
	}
}

func TestDispatch_commandsAck_normalizesAckAlias(t *testing.T) {
	t.Parallel()
	mid := uuid.MustParse("66666666-6666-6666-6666-666666666666")
	cmdID := uuid.MustParse("aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee")
	topic := fmt.Sprintf("pre/%s/commands/ack", mid)
	var cap captureIngest
	payload := []byte(fmt.Sprintf(
		`{"machine_id":%q,"command_id":%q,"occurred_at":"2020-04-01T12:00:00Z","sequence":1,"status":"ack","payload":{},"dedupe_key":"d1"}`,
		mid.String(), cmdID.String(),
	))
	if err := Dispatch(context.Background(), TopicLayoutLegacy, "pre", topic, payload, &cap, nil, nil); err != nil {
		t.Fatal(err)
	}
	if cap.lastReceipt == nil || cap.lastReceipt.Status != "acked" {
		t.Fatalf("got %+v", cap.lastReceipt)
	}
}

func TestDispatch_commandsAckRejectsBodyMachineMismatch(t *testing.T) {
	t.Parallel()
	topicMid := uuid.MustParse("66666666-6666-6666-6666-666666666666")
	bodyMid := uuid.MustParse("77777777-7777-7777-7777-777777777777")
	topic := fmt.Sprintf("pre/%s/commands/ack", topicMid)
	var cap captureIngest
	payload := []byte(fmt.Sprintf(
		`{"machine_id":%q,"command_id":"aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee","occurred_at":"2020-04-01T12:00:00Z","sequence":1,"status":"ack","payload":{},"dedupe_key":"d1"}`,
		bodyMid.String(),
	))
	err := Dispatch(context.Background(), TopicLayoutLegacy, "pre", topic, payload, &cap, nil, nil)
	if err == nil {
		t.Fatal("expected machine mismatch rejection")
	}
	if cap.lastReceipt != nil {
		t.Fatal("mismatched ACK must not reach persistence ingest")
	}
}

func TestTelemetryIdempotencyKey_bootSeq(t *testing.T) {
	t.Parallel()
	mid := uuid.MustParse("77777777-7777-7777-7777-777777777777")
	b := uuid.MustParse("88888888-8888-8888-8888-888888888888")
	seq := int64(99)
	got := TelemetryIdempotencyKey(mid, TelemetryIngest{MachineID: mid, BootID: &b, SeqNo: &seq})
	want := mid.String() + ":" + b.String() + ":99"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
