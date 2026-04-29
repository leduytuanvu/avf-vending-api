package device

import (
	"context"
	"errors"
	"testing"

	domainfleet "github.com/avf/avf-vending-api/internal/domain/fleet"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/google/uuid"
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
		"expired":     "expired",
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

type stubMachineStatusReader struct {
	status string
}

func (s stubMachineStatusReader) GetMachine(ctx context.Context, machineID uuid.UUID) (domainfleet.Machine, error) {
	_ = ctx
	return domainfleet.Machine{ID: machineID, Status: s.status}, nil
}

func TestEnsureMachineCommandableRejectsSuspendedAndCompromised(t *testing.T) {
	t.Parallel()
	mid := uuid.New()
	for _, status := range []string{"suspended", "compromised", "retired", "maintenance"} {
		d := &MQTTCommandDispatcher{machines: stubMachineStatusReader{status: status}}
		err := d.ensureMachineCommandable(context.Background(), mid)
		if !errors.Is(err, ErrMachineNotCommandable) {
			t.Fatalf("status %q: got %v want ErrMachineNotCommandable", status, err)
		}
	}
}

func TestMQTTLedgerRouteMeta_roundTrip(t *testing.T) {
	t.Parallel()
	topic := "avf/devices/00000000-0000-0000-0000-000000000001/commands/dispatch"
	wire := []byte(`{"command_id":"550e8400-e29b-41d4-a716-446655440001","machine_id":"00000000-0000-0000-0000-000000000001"}`)
	meta, sha := mqttLedgerRouteMeta(topic, wire)
	if sha == "" {
		t.Fatal("expected payload sha256")
	}
	topic2, sha2 := ledgerMQTTMetaFromRouteKey(pgtype.Text{String: meta, Valid: true})
	if topic2 != topic || sha2 != sha {
		t.Fatalf("round-trip: topic %q %q sha %q %q", topic, topic2, sha, sha2)
	}
}

func TestAttemptLifecycleStatus_mapsTerminalStatuses(t *testing.T) {
	t.Parallel()
	if got := AttemptLifecycleStatus("duplicate", ptrStr("")); got != "acked" {
		t.Fatalf("duplicate: %q", got)
	}
	if got := AttemptLifecycleStatus("completed", ptrStr("")); got != "acked" {
		t.Fatalf("completed: %q", got)
	}
	if got := AttemptLifecycleStatus("ack_timeout", ptrStr("")); got != "timeout" {
		t.Fatalf("ack_timeout: %q", got)
	}
	if got := AttemptLifecycleStatus("failed", ptrStr("admin_cancel")); got != "cancelled" {
		t.Fatalf("admin cancel failed: %q", got)
	}
}

func ptrStr(s string) *string {
	return &s
}

func TestEnsureMachineCommandableAllowsActive(t *testing.T) {
	t.Parallel()
	d := &MQTTCommandDispatcher{machines: stubMachineStatusReader{status: "active"}}
	if err := d.ensureMachineCommandable(context.Background(), uuid.New()); err != nil {
		t.Fatalf("active machine should be commandable: %v", err)
	}
}
