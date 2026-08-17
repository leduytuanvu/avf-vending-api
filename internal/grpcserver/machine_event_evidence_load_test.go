package grpcserver

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/avf/avf-vending-api/internal/platform/id"
	plauth "github.com/avf/avf-vending-api/internal/platform/auth"
	machinev1 "github.com/avf/avf-vending-api/proto/avf/machine/v1"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// TestSubmitEventEvidenceBatch_realLocalLoadMultiMachineRebatchReconnect exercises a
// real non-production Postgres-backed evidence load (not MQTT dry-run):
// small multi-machine batches, rebatch/response-loss duplicates, and a reconnect wave.
// Scale is intentionally modest for workstation safety; financial mutations are not exercised.
func TestSubmitEventEvidenceBatch_realLocalLoadMultiMachineRebatchReconnect(t *testing.T) {
	pool := machineGRPCTestPool(t)
	ctx := context.Background()
	deps := offlineSyncIntegrationDeps(t, pool)
	srv := &machineTelemetryServer{deps: deps}

	const (
		machineCount   = 20
		eventsPerMachine = 50 // total logical events = 1000
		batchSize      = 25
	)

	type machineFixture struct {
		id     uuid.UUID
		claims plauth.MachineAccessClaims
		ctx    context.Context
	}
	machines := make([]machineFixture, 0, machineCount)
	for i := 0; i < machineCount; i++ {
		siteID := id.NewUUIDV7()
		machineID := id.NewUUIDV7()
		require.NoError(t, insertMachineReplayLedgerFixture(ctx, pool, siteID, machineID))
		claims := plauth.MachineAccessClaims{MachineID: machineID, CredentialVersion: 1}
		machines = append(machines, machineFixture{
			id:     machineID,
			claims: claims,
			ctx:    plauth.WithMachineAccessClaims(ctx, claims),
		})
	}

	makeEv := func(machineIdx, eventIdx int) *machinev1.EventEvidence {
		return &machinev1.EventEvidence{
			EventId:    fmt.Sprintf("load-m%d-e%d", machineIdx, eventIdx),
			EventType:  "vend_result",
			OccurredAt: timestamppb.Now(),
			Category:   "business_critical",
			Severity:   "info",
			Source:     "device",
			StreamId:   fmt.Sprintf("stream-%d", machineIdx),
		}
	}

	submitBatches := func(batchKeyPrefix string, expectDuplicate bool) (accepted, duplicates int) {
		for mi, m := range machines {
			for start := 1; start <= eventsPerMachine; start += batchSize {
				end := start + batchSize - 1
				if end > eventsPerMachine {
					end = eventsPerMachine
				}
				events := make([]*machinev1.EventEvidence, 0, end-start+1)
				for ei := start; ei <= end; ei++ {
					events = append(events, makeEv(mi, ei))
				}
				batchID := fmt.Sprintf("%s-m%d-%d-%d", batchKeyPrefix, mi, start, end)
				out, err := srv.SubmitEventEvidenceBatch(m.ctx, &machinev1.SubmitEventEvidenceBatchRequest{
					Context: &machinev1.IdempotencyContext{
						IdempotencyKey:  batchID,
						ClientEventId:   batchID,
						ClientCreatedAt: timestamppb.Now(),
					},
					Events: events,
				})
				require.NoError(t, err)
				require.Len(t, out.GetResults(), len(events))
				for _, r := range out.GetResults() {
					switch r.GetStatus() {
					case machinev1.EventEvidenceResultStatus_EVENT_EVIDENCE_RESULT_STATUS_ACCEPTED:
						accepted++
						require.False(t, expectDuplicate, "unexpected ACCEPTED on rebatch for %s", r.GetEventId())
					case machinev1.EventEvidenceResultStatus_EVENT_EVIDENCE_RESULT_STATUS_DUPLICATE:
						duplicates++
						require.True(t, expectDuplicate, "unexpected DUPLICATE on first delivery for %s", r.GetEventId())
					default:
						t.Fatalf("unexpected status %v for %s reason=%s", r.GetStatus(), r.GetEventId(), r.GetReason())
					}
				}
			}
		}
		return accepted, duplicates
	}

	start := time.Now()
	accepted1, dup1 := submitBatches("wave1", false)
	require.Equal(t, machineCount*eventsPerMachine, accepted1)
	require.Equal(t, 0, dup1)

	// Rebatch / response-loss equivalent: different batch IDs, same event identities.
	accepted2, dup2 := submitBatches("wave2-rebatch", true)
	require.Equal(t, 0, accepted2)
	require.Equal(t, machineCount*eventsPerMachine, dup2)

	// DB authority: exactly one logical row per event identity for this run's machines.
	machineIDs := make([]uuid.UUID, 0, len(machines))
	for _, m := range machines {
		machineIDs = append(machineIDs, m.id)
	}
	var totalRows int
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM machine_event_evidence
		WHERE machine_id = ANY($1) AND event_id LIKE 'load-m%'
	`, machineIDs).Scan(&totalRows))
	require.Equal(t, machineCount*eventsPerMachine, totalRows)

	var dupLogical int
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM (
			SELECT machine_id, event_id FROM machine_event_evidence
			WHERE machine_id = ANY($1) AND event_id LIKE 'load-m%'
			GROUP BY machine_id, event_id
			HAVING COUNT(*) > 1
		) d
	`, machineIDs).Scan(&dupLogical))
	require.Equal(t, 0, dupLogical)

	// Reconnect wave: concurrent machines submit a small additional unique set.
	const reconnectExtra = 5
	var (
		wg           sync.WaitGroup
		acceptedAtom atomic.Int64
		errAtom      atomic.Int64
	)
	for mi, m := range machines {
		wg.Add(1)
		go func(mi int, m machineFixture) {
			defer wg.Done()
			events := make([]*machinev1.EventEvidence, 0, reconnectExtra)
			for ei := eventsPerMachine + 1; ei <= eventsPerMachine+reconnectExtra; ei++ {
				events = append(events, makeEv(mi, ei))
			}
			batchID := fmt.Sprintf("reconnect-m%d", mi)
			out, err := srv.SubmitEventEvidenceBatch(m.ctx, &machinev1.SubmitEventEvidenceBatchRequest{
				Context: &machinev1.IdempotencyContext{
					IdempotencyKey:  batchID,
					ClientEventId:   batchID,
					ClientCreatedAt: timestamppb.Now(),
				},
				Events: events,
			})
			if err != nil {
				errAtom.Add(1)
				t.Errorf("reconnect machine %d: %v", mi, err)
				return
			}
			for _, r := range out.GetResults() {
				if r.GetStatus() == machinev1.EventEvidenceResultStatus_EVENT_EVIDENCE_RESULT_STATUS_ACCEPTED {
					acceptedAtom.Add(1)
				} else {
					errAtom.Add(1)
					t.Errorf("reconnect unexpected status %v for %s", r.GetStatus(), r.GetEventId())
				}
			}
		}(mi, m)
	}
	wg.Wait()
	require.Equal(t, int64(0), errAtom.Load())
	require.Equal(t, int64(machineCount*reconnectExtra), acceptedAtom.Load())

	expectedFinal := machineCount*eventsPerMachine + machineCount*reconnectExtra
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM machine_event_evidence
		WHERE machine_id = ANY($1) AND event_id LIKE 'load-m%'
	`, machineIDs).Scan(&totalRows))
	require.Equal(t, expectedFinal, totalRows)

	t.Logf("REAL_EVIDENCE_LOAD machines=%d events_per_machine=%d batch_size=%d logical_final=%d duration=%s accepted_first=%d rebatch_duplicates=%d reconnect_accepted=%d",
		machineCount, eventsPerMachine, batchSize, expectedFinal, time.Since(start), accepted1, dup2, acceptedAtom.Load())
}
