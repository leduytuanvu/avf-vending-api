package machineruntime

import (
	"testing"

	machinev1 "github.com/avf/avf-vending-api/proto/avf/machine/v1"
)

func TestOfflineStreamName_legacyWhenEmpty(t *testing.T) {
	if OfflineStreamName(nil) != LegacyOfflineStreamName {
		t.Fatalf("expected legacy stream")
	}
	if OfflineStreamName(&machinev1.MachineRequestMeta{}) != LegacyOfflineStreamName {
		t.Fatalf("expected legacy stream for empty meta")
	}
}

func TestOfflineStreamName_usesStreamId(t *testing.T) {
	meta := &machinev1.MachineRequestMeta{StreamId: "install-epoch-1"}
	if OfflineStreamName(meta) != "install-epoch-1" {
		t.Fatalf("got %q", OfflineStreamName(meta))
	}
}

func TestQualifyOfflineStreamName_appendsKind(t *testing.T) {
	got := QualifyOfflineStreamName("install-epoch-1", OfflineStreamKindCash)
	if got != "install-epoch-1:cash" {
		t.Fatalf("got %q", got)
	}
	got = QualifyOfflineStreamName("install-epoch-1:cash", OfflineStreamKindCommerce)
	if got != "install-epoch-1:commerce" {
		t.Fatalf("got %q", got)
	}
}
