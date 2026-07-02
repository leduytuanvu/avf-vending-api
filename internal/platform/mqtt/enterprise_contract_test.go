package mqtt

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
)

func TestEnterpriseRequiredTopicPaths_matchAndroidContract(t *testing.T) {
	t.Parallel()
	prefix := "avf/production"
	mid := uuid.MustParse("11111111-1111-1111-1111-111111111111")

	sub, err := OutboundCommandPublishTopicStrict(TopicLayoutEnterprise, prefix, mid)
	if err != nil {
		t.Fatal(err)
	}
	wantSub := prefix + "/machines/" + mid.String() + "/commands"
	if sub != wantSub {
		t.Fatalf("subscribe topic: got %q want %q", sub, wantSub)
	}

	wantPublish := map[string]string{
		"commands/ack":       prefix + "/machines/" + mid.String() + "/commands/ack",
		"commands/receipt":   prefix + "/machines/" + mid.String() + "/commands/receipt",
		"presence":           prefix + "/machines/" + mid.String() + "/presence",
		"state/heartbeat":    prefix + "/machines/" + mid.String() + "/state/heartbeat",
		"telemetry":          prefix + "/machines/" + mid.String() + "/telemetry",
		"telemetry/snapshot": prefix + "/machines/" + mid.String() + "/telemetry/snapshot",
		"telemetry/incident": prefix + "/machines/" + mid.String() + "/telemetry/incident",
		"events":             prefix + "/machines/" + mid.String() + "/events",
		"events/vend":        prefix + "/machines/" + mid.String() + "/events/vend",
		"events/cash":        prefix + "/machines/" + mid.String() + "/events/cash",
		"events/inventory":   prefix + "/machines/" + mid.String() + "/events/inventory",
		"shadow/reported":    prefix + "/machines/" + mid.String() + "/shadow/reported",
	}
	for rel, want := range wantPublish {
		got, err := DevicePublishTopicStrict(TopicLayoutEnterprise, prefix, mid, rel)
		if err != nil {
			t.Fatalf("rel %q: %v", rel, err)
		}
		if got != want {
			t.Fatalf("rel %q: got %q want %q", rel, got, want)
		}
	}
}

func TestParseDeviceTopicWithLayout_rejectsWrongPrefix(t *testing.T) {
	t.Parallel()
	mid := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	topic := fmt.Sprintf("wrong/prefix/machines/%s/telemetry", mid)
	_, _, err := ParseDeviceTopicWithLayout(TopicLayoutEnterprise, "avf/production", topic)
	if err == nil {
		t.Fatal("expected wrong prefix rejection")
	}
}

func TestParseDeviceTopicWithLayout_rejectsWrongMachineSegment(t *testing.T) {
	t.Parallel()
	prefix := "avf/production"
	topic := prefix + "/machines/not-a-uuid/telemetry"
	_, _, err := ParseDeviceTopicWithLayout(TopicLayoutEnterprise, prefix, topic)
	if err == nil {
		t.Fatal("expected invalid machine id rejection")
	}
}

func TestDispatch_eventsCashInventoryVend_storedAsTelemetry(t *testing.T) {
	t.Parallel()
	prefix := "avf/production"
	mid := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	cases := []struct {
		rel       string
		eventType string
	}{
		{RelTopicEventsVend, "events.vend"},
		{RelTopicEventsCash, "events.cash"},
		{RelTopicEventsInventory, "events.inventory"},
	}
	for _, tc := range cases {
		topic, err := EnterpriseDeviceTopicStrict(prefix, mid, tc.rel)
		if err != nil {
			t.Fatal(err)
		}
		var cap captureIngest
		payload := []byte(`{"payload":{"order_id":"` + uuid.NewString() + `"}}`)
		if err := Dispatch(context.Background(), TopicLayoutEnterprise, prefix, topic, payload, &cap, nil, nil); err != nil {
			t.Fatalf("rel %s: %v", tc.rel, err)
		}
		if cap.lastTelemetry == nil || cap.lastTelemetry.EventType != tc.eventType {
			t.Fatalf("rel %s: telemetry %+v", tc.rel, cap.lastTelemetry)
		}
	}
}

func TestDispatch_commandsAck_acceptsIdempotencyKeyAndErrorAliases(t *testing.T) {
	t.Parallel()
	mid := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	cmdID := uuid.MustParse("aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee")
	topic, err := EnterpriseDeviceTopicStrict("avf/staging", mid, RelTopicCommandsAck)
	if err != nil {
		t.Fatal(err)
	}
	payload := []byte(fmt.Sprintf(
		`{"machine_id":%q,"command_id":%q,"occurred_at":"2020-04-01T12:00:00Z","sequence":2,"status":"nacked","payload":{},"idempotency_key":"idem-ack-1","error_code":"UNKNOWN_COMMAND","error_message":"unsupported command_type"}`,
		mid.String(), cmdID.String(),
	))
	var cap captureIngest
	if err := Dispatch(context.Background(), TopicLayoutEnterprise, "avf/staging", topic, payload, &cap, nil, nil); err != nil {
		t.Fatal(err)
	}
	if cap.lastReceipt == nil {
		t.Fatal("expected receipt ingest")
	}
	if cap.lastReceipt.DedupeKey != "idem-ack-1" {
		t.Fatalf("dedupe: %q", cap.lastReceipt.DedupeKey)
	}
	if cap.lastReceipt.Status != "nacked" {
		t.Fatalf("status: %q", cap.lastReceipt.Status)
	}
	if cap.lastReceipt.ErrorReason != "unsupported command_type" {
		t.Fatalf("error reason: %q", cap.lastReceipt.ErrorReason)
	}
}

func TestDispatch_commandsAck_rejectsInvalidStatus(t *testing.T) {
	t.Parallel()
	mid := uuid.MustParse("55555555-5555-5555-5555-555555555555")
	topic := fmt.Sprintf("pre/%s/commands/ack", mid)
	payload := []byte(fmt.Sprintf(
		`{"machine_id":%q,"command_id":"aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee","occurred_at":"2020-04-01T12:00:00Z","sequence":1,"status":"unknown","payload":{},"dedupe_key":"d1"}`,
		mid.String(),
	))
	err := Dispatch(context.Background(), TopicLayoutLegacy, "pre", topic, payload, &captureIngest{}, nil, nil)
	if err == nil {
		t.Fatal("expected invalid status rejection")
	}
}

func TestBootstrapTLSRequired_andMachineClientID(t *testing.T) {
	t.Parallel()
	if !BootstrapTLSRequired("tls://broker.example:8883", false, "production") {
		t.Fatal("expected tls URL to require TLS")
	}
	if !BootstrapTLSRequired("tcp://broker.example:1883", false, "production") {
		t.Fatal("expected plain tcp in production without TLS flag to still mark tls_required")
	}
	if BootstrapTLSRequired("tcp://localhost:1883", false, "development") {
		t.Fatal("expected dev localhost tcp to not require TLS")
	}
	mid := uuid.MustParse("66666666-6666-6666-6666-666666666666")
	if got := MachineClientID(mid); got != "avf-machine-"+mid.String() {
		t.Fatalf("client id: %q", got)
	}
}
