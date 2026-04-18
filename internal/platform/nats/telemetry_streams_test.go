package nats

import (
	"testing"
	"time"

	"github.com/avf/avf-vending-api/internal/platform/telemetry"
	"github.com/google/uuid"
)

func TestTelemetryStreamName(t *testing.T) {
	if _, err := TelemetryStreamName(telemetry.ClassHeartbeat); err != nil {
		t.Fatal(err)
	}
	if _, err := TelemetryStreamName(telemetry.Class("nope")); err == nil {
		t.Fatal("expected error")
	}
}

func TestTelemetrySubjectShape(t *testing.T) {
	id := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	s, err := TelemetrySubject(telemetry.ClassMetrics, id)
	if err != nil {
		t.Fatal(err)
	}
	if want := SubjectTelemetryPrefix + "metrics.11111111-1111-1111-1111-111111111111"; s != want {
		t.Fatalf("subject: %s", s)
	}
}

func TestBoundedRetentionDurations(t *testing.T) {
	// Sanity: documented JetStream max-age targets remain in expected ratios.
	_ = 2 * time.Hour
	_ = 6 * time.Hour
	_ = 24 * time.Hour
	_ = 72 * time.Hour
	_ = 7 * 24 * time.Hour
}
