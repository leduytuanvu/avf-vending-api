package grpcserver

import "testing"

func TestOfflineLedgerRejectedIsNotApplied(t *testing.T) {
	t.Parallel()
	if offlineLedgerAppliedStatus("rejected") {
		t.Fatal("rejected must not count as applied")
	}
	if offlineLedgerTerminalStatus("rejected") {
		t.Fatal("rejected must not be successful terminal replay")
	}
	if !offlineLedgerRejectedStatus("rejected") {
		t.Fatal("rejected must remain rejected")
	}
	if !offlineLedgerAppliedStatus("processed") {
		t.Fatal("processed is applied")
	}
}

func TestOfflineCursorStreamName(t *testing.T) {
	t.Parallel()
	if got := offlineCursorStreamName(""); got != "offline" {
		t.Fatalf("legacy stream = %q", got)
	}
	if got := offlineCursorStreamName("install-1"); got != "offline:install-1" {
		t.Fatalf("named stream = %q", got)
	}
}

func TestOfflineContentConflicts(t *testing.T) {
	t.Parallel()
	fpA := offlinePayloadFingerprint("commerce.create_order", []byte(`{"a":"1"}`))
	fpB := offlinePayloadFingerprint("commerce.cancel_order", []byte(`{"a":"1"}`))
	if fpA == fpB {
		t.Fatal("different operations must fingerprint differently")
	}
	if !offlineContentConflicts("commerce.create_order", fpA, []byte(`{"a":"1"}`), "commerce.cancel_order", fpB, []byte(`{"a":"1"}`)) {
		t.Fatal("expected operation conflict")
	}
	if offlineContentConflicts("commerce.create_order", fpA, []byte(`{"a":"1"}`), "commerce.create_order", fpA, []byte(`{"a":"1"}`)) {
		t.Fatal("same content must not conflict")
	}
}
