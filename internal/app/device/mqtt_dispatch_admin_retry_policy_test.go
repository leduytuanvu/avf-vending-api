package device

import "testing"

func TestAdminRetryableLatestAttemptStatus(t *testing.T) {
	t.Parallel()
	terminal := []string{"completed", "nack", "duplicate", "late", " COMPLETED "}
	for _, s := range terminal {
		if adminRetryableLatestAttemptStatus(s) {
			t.Fatalf("expected non-retryable for %q", s)
		}
	}
	if !adminRetryableLatestAttemptStatus("failed") {
		t.Fatal("failed should allow admin retry")
	}
	if !adminRetryableLatestAttemptStatus("ack_timeout") {
		t.Fatal("ack_timeout should allow admin retry")
	}
}
