package mqtt

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	tel "github.com/avf/avf-vending-api/internal/platform/telemetry"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

const contractMachineID = "55555555-5555-5555-5555-555555555555"

// criticalIdentityIngest applies the same stable-identity rules as mqtt-ingest’s JetStream bridge
// for critical_no_drop telemetry, without Postgres or NATS.
type criticalIdentityIngest struct {
	telemetry []TelemetryIngest
	receipts  []CommandReceiptIngest
}

func (c *criticalIdentityIngest) IngestTelemetry(ctx context.Context, in TelemetryIngest) error {
	if tel.CriticalityForEventType(in.EventType) != tel.CriticalityCriticalNoDrop {
		c.telemetry = append(c.telemetry, in)
		return nil
	}
	_, err := tel.StableCriticalIdempotencyKey(in.MachineID, in.EventType, tel.CriticalIngestIdentity{
		DedupeKey: in.DedupeKey,
		EventID:   in.EventID,
		BootID:    in.BootID,
		SeqNo:     in.SeqNo,
	})
	if err != nil {
		return err
	}
	c.telemetry = append(c.telemetry, in)
	return nil
}

func (c *criticalIdentityIngest) IngestShadowReported(ctx context.Context, in ShadowReportedIngest) error {
	return nil
}

func (c *criticalIdentityIngest) IngestShadowDesired(ctx context.Context, in ShadowDesiredIngest) error {
	return nil
}

func (c *criticalIdentityIngest) IngestCommandReceipt(ctx context.Context, in CommandReceiptIngest) error {
	c.receipts = append(c.receipts, in)
	return nil
}

func testdataTelemetryPath(t *testing.T, name string) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	// internal/platform/mqtt -> repo root is ../..
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
	return filepath.Join(repoRoot, "testdata", "telemetry", name)
}

func readSample(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(testdataTelemetryPath(t, name))
	require.NoError(t, err)
	return b
}

func deviceTopic(t *testing.T, prefix, channel string) string {
	t.Helper()
	mid := uuid.MustParse(contractMachineID)
	switch channel {
	case "telemetry":
		return prefix + "/" + mid.String() + "/telemetry"
	case "events/vend":
		return prefix + "/" + mid.String() + "/events/vend"
	case "events/cash":
		return prefix + "/" + mid.String() + "/events/cash"
	case "events/inventory":
		return prefix + "/" + mid.String() + "/events/inventory"
	case "state/heartbeat":
		return prefix + "/" + mid.String() + "/state/heartbeat"
	case "commands/ack":
		return prefix + "/" + mid.String() + "/commands/ack"
	default:
		t.Fatalf("unknown channel %q", channel)
		return ""
	}
}

func TestOfflineReplayContract_validSamplesAccepted(t *testing.T) {
	t.Parallel()
	prefix := "avf"
	ctx := context.Background()

	cases := []struct {
		file    string
		channel string
	}{
		{"valid_vend_success.json", "events/vend"},
		{"valid_vend_failed.json", "events/vend"},
		{"valid_payment_success.json", "telemetry"},
		{"valid_cash_inserted.json", "events/cash"},
		{"valid_inventory_delta.json", "events/inventory"},
		{"valid_heartbeat_metrics.json", "state/heartbeat"},
	}
	for _, tc := range cases {
		t.Run(tc.file, func(t *testing.T) {
			t.Parallel()
			ing := &criticalIdentityIngest{}
			err := Dispatch(ctx, prefix, deviceTopic(t, prefix, tc.channel), readSample(t, tc.file), ing, nil, nil)
			require.NoError(t, err)
		})
	}

	t.Run("valid_command_ack", func(t *testing.T) {
		t.Parallel()
		ing := &criticalIdentityIngest{}
		err := Dispatch(ctx, prefix, deviceTopic(t, prefix, "commands/ack"), readSample(t, "valid_command_ack.json"), ing, nil, nil)
		require.NoError(t, err)
		require.Len(t, ing.receipts, 1)
		require.Equal(t, "cmd-ack:55555555-5555-5555-5555-555555555555:42:01JR8CMD", ing.receipts[0].DedupeKey)
		require.Equal(t, int64(42), ing.receipts[0].Sequence)
	})

	t.Run("command_receipt_missing_dedupe_rejected", func(t *testing.T) {
		t.Parallel()
		ing := &criticalIdentityIngest{}
		var rejects []string
		hooks := &IngestHooks{
			OnIngressRejected: func(_ string, reason string, _ int) {
				rejects = append(rejects, reason)
			},
		}
		bad := []byte(`{"sequence":1,"status":"acked","payload":{}}`)
		err := Dispatch(ctx, prefix, deviceTopic(t, prefix, "commands/ack"), bad, ing, nil, hooks)
		require.Error(t, err)
		require.Contains(t, rejects, "receipt_missing_dedupe")
		require.Empty(t, ing.receipts)
	})
}

func TestOfflineReplayContract_invalidCriticalRejected(t *testing.T) {
	t.Parallel()
	prefix := "avf"
	ctx := context.Background()

	t.Run("missing_identity", func(t *testing.T) {
		t.Parallel()
		ing := &criticalIdentityIngest{}
		err := Dispatch(ctx, prefix, deviceTopic(t, prefix, "events/vend"), readSample(t, "invalid_critical_missing_identity.json"), ing, nil, nil)
		require.ErrorIs(t, err, tel.ErrCriticalTelemetryMissingIdentity)
	})

	t.Run("wrong_machine_id", func(t *testing.T) {
		t.Parallel()
		ing := &criticalIdentityIngest{}
		var rejects []string
		hooks := &IngestHooks{
			OnIngressRejected: func(_ string, reason string, _ int) {
				rejects = append(rejects, reason)
			},
		}
		err := Dispatch(ctx, prefix, deviceTopic(t, prefix, "events/vend"), readSample(t, "invalid_critical_wrong_machine_id.json"), ing, nil, hooks)
		require.Error(t, err)
		require.Contains(t, rejects, "machine_id_mismatch")
	})

	t.Run("malformed_occurred_at", func(t *testing.T) {
		t.Parallel()
		ing := &criticalIdentityIngest{}
		var rejects []string
		hooks := &IngestHooks{
			OnIngressRejected: func(_ string, reason string, _ int) {
				rejects = append(rejects, reason)
			},
		}
		err := Dispatch(ctx, prefix, deviceTopic(t, prefix, "events/vend"), readSample(t, "invalid_occurred_at_malformed.json"), ing, nil, hooks)
		require.Error(t, err)
		require.Contains(t, rejects, "json_decode")
	})
}

func TestOfflineReplayContract_duplicateReplaySameIdempotency(t *testing.T) {
	t.Parallel()
	prefix := "avf"
	ctx := context.Background()
	ing := &criticalIdentityIngest{}
	payload := readSample(t, "duplicate_replay_vend.json")
	require.NoError(t, Dispatch(ctx, prefix, deviceTopic(t, prefix, "events/vend"), payload, ing, nil, nil))
	require.NoError(t, Dispatch(ctx, prefix, deviceTopic(t, prefix, "events/vend"), payload, ing, nil, nil))
	require.Len(t, ing.telemetry, 2)
	k1, err := tel.StableCriticalIdempotencyKey(ing.telemetry[0].MachineID, ing.telemetry[0].EventType, tel.CriticalIngestIdentity{
		DedupeKey: ing.telemetry[0].DedupeKey,
		EventID:   ing.telemetry[0].EventID,
		BootID:    ing.telemetry[0].BootID,
		SeqNo:     ing.telemetry[0].SeqNo,
	})
	require.NoError(t, err)
	k2, err := tel.StableCriticalIdempotencyKey(ing.telemetry[1].MachineID, ing.telemetry[1].EventType, tel.CriticalIngestIdentity{
		DedupeKey: ing.telemetry[1].DedupeKey,
		EventID:   ing.telemetry[1].EventID,
		BootID:    ing.telemetry[1].BootID,
		SeqNo:     ing.telemetry[1].SeqNo,
	})
	require.NoError(t, err)
	require.Equal(t, k1, k2)
	require.Equal(t, "vend:55555555-5555-5555-5555-555555555555:slot-3:2026-04-24T12:00:05Z:01JR8VEND", k1)
}

func TestOfflineReplayContract_droppableHeartbeatWithoutDedupe(t *testing.T) {
	t.Parallel()
	prefix := "avf"
	ctx := context.Background()
	ing := &criticalIdentityIngest{}
	b := []byte(`{"occurred_at":"2026-04-24T12:00:00Z","payload":{"ping":true}}`)
	require.NoError(t, Dispatch(ctx, prefix, deviceTopic(t, prefix, "state/heartbeat"), b, ing, nil, nil))
	require.Len(t, ing.telemetry, 1)
	require.Equal(t, "state.heartbeat", ing.telemetry[0].EventType)
	require.Equal(t, tel.CriticalityDroppableMetrics, tel.CriticalityForEventType(ing.telemetry[0].EventType))
}
