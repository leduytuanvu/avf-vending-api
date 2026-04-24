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

func TestStreamMaxAgeScaling(t *testing.T) {
	t.Parallel()
	base := 168 * time.Hour
	if g := streamMaxAge(base, 2); g < 5*time.Minute || g > 3*time.Hour {
		t.Fatalf("heartbeat age: %v", g)
	}
	if g := streamMaxAge(base, 168); g != base {
		t.Fatalf("diagnostic age want %v got %v", base, g)
	}
}

func TestTelemetryStreamRetentionPlan_usesConfiguredMaxBytes(t *testing.T) {
	t.Parallel()
	wantBytes := int64(4 << 30)
	lim := TelemetryBrokerLimits{StreamMaxBytes: wantBytes, StreamMaxAgeBaseline: 168 * time.Hour}
	plan := TelemetryStreamRetentionPlan(lim)
	if len(plan) != 6 {
		t.Fatalf("plan len: %d", len(plan))
	}
	for _, p := range plan {
		if p.MaxBytes != wantBytes {
			t.Fatalf("stream %s max bytes: got %d want %d", p.Name, p.MaxBytes, wantBytes)
		}
		if p.MaxAge < 5*time.Minute {
			t.Fatalf("stream %s max age too small: %v", p.Name, p.MaxAge)
		}
	}
}
