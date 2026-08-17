package grpcserver

import (
	"context"
	"testing"

	"github.com/avf/avf-vending-api/internal/platform/id"
	plauth "github.com/avf/avf-vending-api/internal/platform/auth"
	machinev1 "github.com/avf/avf-vending-api/proto/avf/machine/v1"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestSubmitEventEvidenceBatch_acceptsDuplicateAndConflict(t *testing.T) {
	t.Parallel()

	pool := machineGRPCTestPool(t)
	ctx := context.Background()
	siteID := id.NewUUIDV7()
	machineID := id.NewUUIDV7()
	require.NoError(t, insertMachineReplayLedgerFixture(ctx, pool, siteID, machineID))

	deps := offlineSyncIntegrationDeps(t, pool)
	srv := &machineTelemetryServer{deps: deps}
	claims := plauth.MachineAccessClaims{MachineID: machineID, CredentialVersion: 1}
	ctxClaims := plauth.WithMachineAccessClaims(ctx, claims)

	payload, err := structpb.NewStruct(map[string]any{"order_id": "ord-1"})
	require.NoError(t, err)
	ev := &machinev1.EventEvidence{
		EventId:     "ev-1",
		EventType:   "vend_result",
		OccurredAt:  timestamppb.Now(),
		Category:    "business_critical",
		Severity:    "info",
		Source:      "device",
		StreamId:    "install-1",
		OrderId:     "ord-1",
		Payload:     payload,
	}
	req := &machinev1.SubmitEventEvidenceBatchRequest{
		Context: &machinev1.IdempotencyContext{IdempotencyKey: "ev-batch-1", ClientEventId: "ev-batch-1", ClientCreatedAt: timestamppb.Now()},
		Events:  []*machinev1.EventEvidence{ev},
	}
	out, err := srv.SubmitEventEvidenceBatch(ctxClaims, req)
	require.NoError(t, err)
	require.Equal(t, machinev1.EventEvidenceResultStatus_EVENT_EVIDENCE_RESULT_STATUS_ACCEPTED, out.GetResults()[0].GetStatus())

	dup, err := srv.SubmitEventEvidenceBatch(ctxClaims, req)
	require.NoError(t, err)
	require.Equal(t, machinev1.EventEvidenceResultStatus_EVENT_EVIDENCE_RESULT_STATUS_DUPLICATE, dup.GetResults()[0].GetStatus())

	conflictPayload, err := structpb.NewStruct(map[string]any{"order_id": "ord-other"})
	require.NoError(t, err)
	conflictReq := &machinev1.SubmitEventEvidenceBatchRequest{
		Context: &machinev1.IdempotencyContext{IdempotencyKey: "ev-batch-2", ClientEventId: "ev-batch-2", ClientCreatedAt: timestamppb.Now()},
		Events: []*machinev1.EventEvidence{{
			EventId:    "ev-1",
			EventType:  "vend_result",
			OccurredAt: timestamppb.Now(),
			Payload:    conflictPayload,
		}},
	}
	conflict, err := srv.SubmitEventEvidenceBatch(ctxClaims, conflictReq)
	require.NoError(t, err)
	require.Equal(t, machinev1.EventEvidenceResultStatus_EVENT_EVIDENCE_RESULT_STATUS_CONFLICT, conflict.GetResults()[0].GetStatus())
}

func TestSubmitEventEvidenceBatch_rejectsMissingEventID(t *testing.T) {
	t.Parallel()

	pool := machineGRPCTestPool(t)
	ctx := context.Background()
	siteID := id.NewUUIDV7()
	machineID := id.NewUUIDV7()
	require.NoError(t, insertMachineReplayLedgerFixture(ctx, pool, siteID, machineID))

	deps := offlineSyncIntegrationDeps(t, pool)
	srv := &machineTelemetryServer{deps: deps}
	claims := plauth.MachineAccessClaims{MachineID: machineID, CredentialVersion: 1}
	ctxClaims := plauth.WithMachineAccessClaims(ctx, claims)

	out, err := srv.SubmitEventEvidenceBatch(ctxClaims, &machinev1.SubmitEventEvidenceBatchRequest{
		Context: &machinev1.IdempotencyContext{IdempotencyKey: "ev-bad", ClientEventId: "ev-bad", ClientCreatedAt: timestamppb.Now()},
		Events: []*machinev1.EventEvidence{{
			EventType:  "vend_result",
			OccurredAt: timestamppb.Now(),
		}},
	})
	require.NoError(t, err)
	require.Equal(t, machinev1.EventEvidenceResultStatus_EVENT_EVIDENCE_RESULT_STATUS_REJECTED, out.GetResults()[0].GetStatus())
}

func TestSubmitEventEvidenceBatch_resourceExhaustedOnOversize(t *testing.T) {
	t.Parallel()

	pool := machineGRPCTestPool(t)
	ctx := context.Background()
	siteID := id.NewUUIDV7()
	machineID := id.NewUUIDV7()
	require.NoError(t, insertMachineReplayLedgerFixture(ctx, pool, siteID, machineID))

	cfg := testMachineGRPCConfig()
	cfg.Capacity.MaxTelemetryGRPCBatchEvents = 1
	deps := offlineSyncIntegrationDeps(t, pool)
	deps.Config = cfg
	srv := &machineTelemetryServer{deps: deps}
	claims := plauth.MachineAccessClaims{MachineID: machineID, CredentialVersion: 1}
	ctxClaims := plauth.WithMachineAccessClaims(ctx, claims)

	_, err := srv.SubmitEventEvidenceBatch(ctxClaims, &machinev1.SubmitEventEvidenceBatchRequest{
		Context: &machinev1.IdempotencyContext{IdempotencyKey: "ev-cap", ClientEventId: "ev-cap", ClientCreatedAt: timestamppb.Now()},
		Events: []*machinev1.EventEvidence{
			{EventId: "a", EventType: "vend_result", OccurredAt: timestamppb.Now()},
			{EventId: "b", EventType: "vend_result", OccurredAt: timestamppb.Now()},
		},
	})
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	require.Equal(t, codes.ResourceExhausted, st.Code())
}

func TestSubmitEventEvidenceBatch_batchIndependentIdentityAndSplitRetry(t *testing.T) {
	t.Parallel()

	pool := machineGRPCTestPool(t)
	ctx := context.Background()
	siteID := id.NewUUIDV7()
	machineID := id.NewUUIDV7()
	require.NoError(t, insertMachineReplayLedgerFixture(ctx, pool, siteID, machineID))

	deps := offlineSyncIntegrationDeps(t, pool)
	srv := &machineTelemetryServer{deps: deps}
	claims := plauth.MachineAccessClaims{MachineID: machineID, CredentialVersion: 1}
	ctxClaims := plauth.WithMachineAccessClaims(ctx, claims)

	makeEv := func(id string) *machinev1.EventEvidence {
		return &machinev1.EventEvidence{
			EventId:    id,
			EventType:  "vend_result",
			OccurredAt: timestamppb.Now(),
			Category:   "business_critical",
		}
	}
	// First delivery: events 1..3 under batch A.
	first, err := srv.SubmitEventEvidenceBatch(ctxClaims, &machinev1.SubmitEventEvidenceBatchRequest{
		Context: &machinev1.IdempotencyContext{IdempotencyKey: "batch-A", ClientEventId: "batch-A", ClientCreatedAt: timestamppb.Now()},
		Events:  []*machinev1.EventEvidence{makeEv("e1"), makeEv("e2"), makeEv("e3")},
	})
	require.NoError(t, err)
	require.Len(t, first.GetResults(), 3)

	// Simulated response loss: resend 1..2 then 3.. under new batch IDs.
	retry1, err := srv.SubmitEventEvidenceBatch(ctxClaims, &machinev1.SubmitEventEvidenceBatchRequest{
		Context: &machinev1.IdempotencyContext{IdempotencyKey: "batch-B", ClientEventId: "batch-B", ClientCreatedAt: timestamppb.Now()},
		Events:  []*machinev1.EventEvidence{makeEv("e1"), makeEv("e2")},
	})
	require.NoError(t, err)
	require.Equal(t, machinev1.EventEvidenceResultStatus_EVENT_EVIDENCE_RESULT_STATUS_DUPLICATE, retry1.GetResults()[0].GetStatus())
	require.Equal(t, machinev1.EventEvidenceResultStatus_EVENT_EVIDENCE_RESULT_STATUS_DUPLICATE, retry1.GetResults()[1].GetStatus())

	retry2, err := srv.SubmitEventEvidenceBatch(ctxClaims, &machinev1.SubmitEventEvidenceBatchRequest{
		Context: &machinev1.IdempotencyContext{IdempotencyKey: "batch-C", ClientEventId: "batch-C", ClientCreatedAt: timestamppb.Now()},
		Events:  []*machinev1.EventEvidence{makeEv("e3")},
	})
	require.NoError(t, err)
	require.Equal(t, machinev1.EventEvidenceResultStatus_EVENT_EVIDENCE_RESULT_STATUS_DUPLICATE, retry2.GetResults()[0].GetStatus())

	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM machine_event_evidence WHERE machine_id = $1`, machineID).Scan(&count))
	require.Equal(t, 3, count)
}

func TestSubmitEventEvidenceBatch_crossMachineSameEventIDAllowed(t *testing.T) {
	t.Parallel()

	pool := machineGRPCTestPool(t)
	ctx := context.Background()
	site1 := id.NewUUIDV7()
	site2 := id.NewUUIDV7()
	m1 := id.NewUUIDV7()
	m2 := id.NewUUIDV7()
	require.NoError(t, insertMachineReplayLedgerFixture(ctx, pool, site1, m1))
	require.NoError(t, insertMachineReplayLedgerFixture(ctx, pool, site2, m2))

	deps := offlineSyncIntegrationDeps(t, pool)
	srv := &machineTelemetryServer{deps: deps}
	ev := &machinev1.EventEvidence{EventId: "shared-id", EventType: "vend_result", OccurredAt: timestamppb.Now()}

	out1, err := srv.SubmitEventEvidenceBatch(plauth.WithMachineAccessClaims(ctx, plauth.MachineAccessClaims{MachineID: m1, CredentialVersion: 1}), &machinev1.SubmitEventEvidenceBatchRequest{
		Context: &machinev1.IdempotencyContext{IdempotencyKey: "xm1", ClientEventId: "xm1", ClientCreatedAt: timestamppb.Now()},
		Events:  []*machinev1.EventEvidence{ev},
	})
	require.NoError(t, err)
	require.Equal(t, machinev1.EventEvidenceResultStatus_EVENT_EVIDENCE_RESULT_STATUS_ACCEPTED, out1.GetResults()[0].GetStatus())

	out2, err := srv.SubmitEventEvidenceBatch(plauth.WithMachineAccessClaims(ctx, plauth.MachineAccessClaims{MachineID: m2, CredentialVersion: 1}), &machinev1.SubmitEventEvidenceBatchRequest{
		Context: &machinev1.IdempotencyContext{IdempotencyKey: "xm2", ClientEventId: "xm2", ClientCreatedAt: timestamppb.Now()},
		Events:  []*machinev1.EventEvidence{ev},
	})
	require.NoError(t, err)
	require.Equal(t, machinev1.EventEvidenceResultStatus_EVENT_EVIDENCE_RESULT_STATUS_ACCEPTED, out2.GetResults()[0].GetStatus())
}

func TestSubmitEventEvidenceBatch_unknownTypePreserved(t *testing.T) {
	t.Parallel()

	pool := machineGRPCTestPool(t)
	ctx := context.Background()
	siteID := id.NewUUIDV7()
	machineID := id.NewUUIDV7()
	require.NoError(t, insertMachineReplayLedgerFixture(ctx, pool, siteID, machineID))

	deps := offlineSyncIntegrationDeps(t, pool)
	srv := &machineTelemetryServer{deps: deps}
	claims := plauth.MachineAccessClaims{MachineID: machineID, CredentialVersion: 1}
	ctxClaims := plauth.WithMachineAccessClaims(ctx, claims)

	out, err := srv.SubmitEventEvidenceBatch(ctxClaims, &machinev1.SubmitEventEvidenceBatchRequest{
		Context: &machinev1.IdempotencyContext{IdempotencyKey: "unk", ClientEventId: "unk", ClientCreatedAt: timestamppb.Now()},
		Events: []*machinev1.EventEvidence{{
			EventId:    "future-1",
			EventType:  "future.semantic.event.v9",
			OccurredAt: timestamppb.Now(),
		}},
	})
	require.NoError(t, err)
	require.Equal(t, machinev1.EventEvidenceResultStatus_EVENT_EVIDENCE_RESULT_STATUS_ACCEPTED, out.GetResults()[0].GetStatus())

	var status string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT processing_status FROM machine_event_evidence WHERE machine_id = $1 AND event_id = $2`,
		machineID, "future-1",
	).Scan(&status))
	require.Equal(t, "unrecognized", status)
}

func TestSubmitEventEvidenceBatch_mixedValidAndInvalid(t *testing.T) {
	t.Parallel()

	pool := machineGRPCTestPool(t)
	ctx := context.Background()
	siteID := id.NewUUIDV7()
	machineID := id.NewUUIDV7()
	require.NoError(t, insertMachineReplayLedgerFixture(ctx, pool, siteID, machineID))

	deps := offlineSyncIntegrationDeps(t, pool)
	srv := &machineTelemetryServer{deps: deps}
	claims := plauth.MachineAccessClaims{MachineID: machineID, CredentialVersion: 1}
	ctxClaims := plauth.WithMachineAccessClaims(ctx, claims)

	out, err := srv.SubmitEventEvidenceBatch(ctxClaims, &machinev1.SubmitEventEvidenceBatchRequest{
		Context: &machinev1.IdempotencyContext{IdempotencyKey: "mix", ClientEventId: "mix", ClientCreatedAt: timestamppb.Now()},
		Events: []*machinev1.EventEvidence{
			{EventId: "ok-1", EventType: "vend_result", OccurredAt: timestamppb.Now()},
			{EventType: "vend_result", OccurredAt: timestamppb.Now()}, // missing event_id
			{EventId: "ok-2", EventType: "vend_result", OccurredAt: timestamppb.Now()},
		},
	})
	require.NoError(t, err)
	require.Len(t, out.GetResults(), 3)
	require.Equal(t, machinev1.EventEvidenceResultStatus_EVENT_EVIDENCE_RESULT_STATUS_ACCEPTED, out.GetResults()[0].GetStatus())
	require.Equal(t, machinev1.EventEvidenceResultStatus_EVENT_EVIDENCE_RESULT_STATUS_REJECTED, out.GetResults()[1].GetStatus())
	require.Equal(t, machinev1.EventEvidenceResultStatus_EVENT_EVIDENCE_RESULT_STATUS_ACCEPTED, out.GetResults()[2].GetStatus())

	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM machine_event_evidence WHERE machine_id = $1`, machineID).Scan(&count))
	require.Equal(t, 2, count)
}
