package telemetryapp

import "testing"

func TestTopicKind(t *testing.T) {
	t.Parallel()
	if g := topicKind("org/mach/telemetry"); g != "telemetry" {
		t.Fatalf("telemetry: got %q", g)
	}
	if g := topicKind("p/mid/shadow/reported"); g != "shadow_reported" {
		t.Fatalf("shadow: got %q", g)
	}
	if g := topicKind("p/mid/commands/receipt"); g != "command_receipt" {
		t.Fatalf("receipt: got %q", g)
	}
	if g := topicKind("x/y/z"); g != "other" {
		t.Fatalf("other: got %q", g)
	}
}

func TestNewIngestHooks(t *testing.T) {
	t.Parallel()
	h := NewIngestHooks()
	if h == nil || h.OnDispatchOutcome == nil || h.OnIngressRejected == nil {
		t.Fatal("expected non-nil hooks")
	}
	h.OnDispatchOutcome(true, "pre/mid/telemetry", 10)
	h.OnDispatchOutcome(false, "pre/mid/telemetry", 0)
	h.OnIngressRejected("pre/mid/telemetry", "payload_too_large", 99)
}
