package grpcserver

import (
	"context"
	"strings"
	"testing"

	plauth "github.com/avf/avf-vending-api/internal/platform/auth"
	machinev1 "github.com/avf/avf-vending-api/proto/avf/machine/v1"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestP23_SubmitTelemetryBatch_rejectsOverConfiguredEventCap(t *testing.T) {
	t.Parallel()

	pool := machineGRPCTestPool(t)
	ctx := context.Background()
	orgID := uuid.New()
	siteID := uuid.New()
	machineID := uuid.New()
	require.NoError(t, insertMachineReplayLedgerFixture(ctx, pool, orgID, siteID, machineID))

	cfg := testMachineGRPCConfig()
	cfg.Capacity.MaxTelemetryGRPCBatchEvents = 2
	cfg.Capacity.MaxTelemetryGRPCBatchBytes = 1 << 20
	deps := offlineSyncIntegrationDeps(t, pool)
	deps.Config = cfg
	srv := &machineTelemetryServer{deps: deps}

	claims := plauth.MachineAccessClaims{OrganizationID: orgID, MachineID: machineID, CredentialVersion: 1}
	ctxClaims := plauth.WithMachineAccessClaims(ctx, claims)

	req := &machinev1.SubmitTelemetryBatchRequest{
		Context: &machinev1.IdempotencyContext{
			IdempotencyKey:  "p23-tel-batch",
			ClientEventId:   "p23-tel-client",
			ClientCreatedAt: timestamppb.Now(),
		},
		Events: []*machinev1.TelemetryEvent{
			{EventType: "a", OccurredAt: timestamppb.Now(), EventId: "e1"},
			{EventType: "b", OccurredAt: timestamppb.Now(), EventId: "e2"},
			{EventType: "c", OccurredAt: timestamppb.Now(), EventId: "e3"},
		},
	}
	_, err := srv.SubmitTelemetryBatch(ctxClaims, req)
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	require.Equal(t, codes.InvalidArgument, st.Code())
	require.True(t, strings.Contains(st.Message(), "too many events"))
}

func TestP23_PushOfflineEvents_rejectsOverConfiguredBatchCap(t *testing.T) {
	t.Parallel()

	pool := machineGRPCTestPool(t)
	ctx := context.Background()
	orgID := uuid.New()
	siteID := uuid.New()
	machineID := uuid.New()
	require.NoError(t, insertMachineReplayLedgerFixture(ctx, pool, orgID, siteID, machineID))

	deps := offlineSyncIntegrationDeps(t, pool)
	deps.Config.Capacity.MaxOfflineEventsPerRequest = 2
	srv := &machineOfflineSyncServer{deps: deps}

	claims := plauth.MachineAccessClaims{OrganizationID: orgID, MachineID: machineID, CredentialVersion: 1}
	ctxClaims := plauth.WithMachineAccessClaims(ctx, claims)

	req := &machinev1.SyncOfflineEventsRequest{
		Meta: &machinev1.MachineRequestMeta{IdempotencyKey: "p23-off-sync", RequestId: "p23-req"},
		Events: []*machinev1.OfflineEvent{
			{Meta: &machinev1.MachineRequestMeta{OfflineSequence: 1, IdempotencyKey: "o1", ClientEventId: "c1", OccurredAt: timestamppb.Now()}, EventType: "noop.a"},
			{Meta: &machinev1.MachineRequestMeta{OfflineSequence: 2, IdempotencyKey: "o2", ClientEventId: "c2", OccurredAt: timestamppb.Now()}, EventType: "noop.b"},
			{Meta: &machinev1.MachineRequestMeta{OfflineSequence: 3, IdempotencyKey: "o3", ClientEventId: "c3", OccurredAt: timestamppb.Now()}, EventType: "noop.c"},
		},
	}
	_, err := srv.PushOfflineEvents(ctxClaims, req)
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	require.Equal(t, codes.InvalidArgument, st.Code())
	require.True(t, strings.Contains(st.Message(), "too many offline events"))
}
