package device

import (
	"testing"

	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestMapAttemptTransportState(t *testing.T) {
	cases := map[string]string{
		"pending":     "queued",
		"sent":        "published",
		"completed":   "acknowledged",
		"failed":      "failed",
		"nack":        "failed",
		"ack_timeout": "timed_out",
		"duplicate":   "superseded",
		"late":        "superseded",
	}
	for in, want := range cases {
		if got := MapAttemptTransportState(in); got != want {
			t.Fatalf("%q: got %q want %q", in, got, want)
		}
	}
}

func TestIsPublishFailure(t *testing.T) {
	reason := "mqtt_publish: broker refused"
	if !isPublishFailure(db.MachineCommandAttempt{TimeoutReason: pgtype.Text{String: reason, Valid: true}}) {
		t.Fatal("expected publish failure")
	}
	if isPublishFailure(db.MachineCommandAttempt{}) {
		t.Fatal("expected false")
	}
}
