package mqtt

import (
	"testing"

	"github.com/google/uuid"
)

func TestOutboundCommandDispatchTopic(t *testing.T) {
	mid := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	cases := []struct {
		prefix string
		want   string
	}{
		{"avf/devices", "avf/devices/11111111-1111-1111-1111-111111111111/commands/dispatch"},
		{"avf/devices/", "avf/devices/11111111-1111-1111-1111-111111111111/commands/dispatch"},
		{"  custom/prefix/  ", "custom/prefix/11111111-1111-1111-1111-111111111111/commands/dispatch"},
	}
	for _, tc := range cases {
		got := OutboundCommandDispatchTopic(tc.prefix, mid)
		if got != tc.want {
			t.Fatalf("prefix %q: got %q want %q", tc.prefix, got, tc.want)
		}
	}
}

func TestOutboundCommandDownTopic(t *testing.T) {
	mid := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	got := OutboundCommandDownTopic("avf/devices/", mid)
	want := "avf/devices/11111111-1111-1111-1111-111111111111/commands/down"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestDeviceTopicHelpers(t *testing.T) {
	mid := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	prefix := "avf/devices"
	cases := []struct {
		rel  string
		want string
	}{
		{RelTopicPresence, "avf/devices/22222222-2222-2222-2222-222222222222/presence"},
		{RelTopicStateHeartbeat, "avf/devices/22222222-2222-2222-2222-222222222222/state/heartbeat"},
		{RelTopicTelemetrySnapshot, "avf/devices/22222222-2222-2222-2222-222222222222/telemetry/snapshot"},
		{RelTopicTelemetryIncident, "avf/devices/22222222-2222-2222-2222-222222222222/telemetry/incident"},
		{RelTopicEventsVend, "avf/devices/22222222-2222-2222-2222-222222222222/events/vend"},
		{RelTopicEventsCash, "avf/devices/22222222-2222-2222-2222-222222222222/events/cash"},
		{RelTopicEventsInventory, "avf/devices/22222222-2222-2222-2222-222222222222/events/inventory"},
		{RelTopicShadowReported, "avf/devices/22222222-2222-2222-2222-222222222222/shadow/reported"},
		{RelTopicShadowDesired, "avf/devices/22222222-2222-2222-2222-222222222222/shadow/desired"},
		{RelTopicCommandsAck, "avf/devices/22222222-2222-2222-2222-222222222222/commands/ack"},
		{RelTopicCommandsReceipt, "avf/devices/22222222-2222-2222-2222-222222222222/commands/receipt"},
	}
	for _, tc := range cases {
		if got := DeviceTopic(prefix, mid, tc.rel); got != tc.want {
			t.Fatalf("rel %q: got %q want %q", tc.rel, got, tc.want)
		}
	}
}

func TestInboundDeviceTopicPatterns(t *testing.T) {
	pats := InboundDeviceTopicPatterns("  avf/devices/  ")
	if len(pats) != 12 {
		t.Fatalf("expected 12 subscribe patterns, got %d: %#v", len(pats), pats)
	}
	want := []string{
		"avf/devices/+/telemetry",
		"avf/devices/+/presence",
		"avf/devices/+/state/heartbeat",
		"avf/devices/+/telemetry/snapshot",
		"avf/devices/+/telemetry/incident",
		"avf/devices/+/events/vend",
		"avf/devices/+/events/cash",
		"avf/devices/+/events/inventory",
		"avf/devices/+/shadow/reported",
		"avf/devices/+/shadow/desired",
		"avf/devices/+/commands/receipt",
		"avf/devices/+/commands/ack",
	}
	for _, w := range want {
		found := false
		for _, p := range pats {
			if p == w {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("missing pattern %q in %#v", w, pats)
		}
	}
}
