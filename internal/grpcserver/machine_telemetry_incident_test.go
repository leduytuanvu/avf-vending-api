package grpcserver

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/avf/avf-vending-api/internal/app/alerts"
	"github.com/avf/avf-vending-api/internal/modules/postgres"
	plauth "github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/avf/avf-vending-api/internal/platform/id"
	machinev1 "github.com/avf/avf-vending-api/proto/avf/machine/v1"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// failThenSucceedProjector fails the first ProjectMachineIncident call, then delegates.
type failThenSucceedProjector struct {
	mu       sync.Mutex
	failures int
	inner    alerts.IncidentProjector
	calls    int
}

func (p *failThenSucceedProjector) ProjectMachineIncident(ctx context.Context, in alerts.ProjectInput, policy alerts.Policy) (alerts.ProjectResult, error) {
	p.mu.Lock()
	p.calls++
	fail := p.failures > 0
	if fail {
		p.failures--
	}
	p.mu.Unlock()
	if fail {
		return alerts.ProjectResult{}, errors.New("injected projection failure")
	}
	return p.inner.ProjectMachineIncident(ctx, in, policy)
}

type recordingProjector struct {
	mu    sync.Mutex
	calls []alerts.ProjectInput
}

func (p *recordingProjector) ProjectMachineIncident(ctx context.Context, in alerts.ProjectInput, policy alerts.Policy) (alerts.ProjectResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls = append(p.calls, in)
	return alerts.ProjectResult{NewOccurrence: true, AlertQueued: true, OccurrenceCount: 1}, nil
}

func TestSubmitTelemetryBatch_ProjectsIncidentWithoutMQTT(t *testing.T) {
	pool := machineGRPCTestPool(t)
	ctx := context.Background()
	siteID := id.NewUUIDV7()
	machineID := id.NewUUIDV7()
	require.NoError(t, insertMachineReplayLedgerFixture(ctx, pool, siteID, machineID))

	store := postgres.NewStore(pool)
	deps := offlineSyncIntegrationDeps(t, pool)
	deps.TelemetryStore = store
	deps.IncidentProjector = nil // use real TelemetryStore projection path
	deps.Config = testMachineGRPCConfig()
	srv := &machineTelemetryServer{deps: deps}

	claims := plauth.MachineAccessClaims{MachineID: machineID, CredentialVersion: 1}
	ctxClaims := plauth.WithMachineAccessClaims(ctx, claims)
	occ := "incident_runtime_error:audit-" + machineID.String()
	fp := "fp-telegram-audit-" + machineID.String()
	// device_telemetry_events.dedupe_key is globally unique — include machine id so shared test DBs stay isolated.
	batchKey := "tel-inc-no-mqtt-" + machineID.String()
	req := &machinev1.SubmitTelemetryBatchRequest{
		Context: &machinev1.IdempotencyContext{
			IdempotencyKey:  batchKey,
			ClientEventId:   "client-inc-1",
			ClientCreatedAt: timestamppb.Now(),
		},
		Events: []*machinev1.TelemetryEvent{
			{
				EventType:  "incident_runtime_error",
				EventId:    occ,
				OccurredAt: timestamppb.Now(),
				Attributes: map[string]string{
					"severity":    "high",
					"fingerprint": fp,
					"title":       "runtime error",
				},
			},
		},
	}
	resp, err := srv.SubmitTelemetryBatch(ctxClaims, req)
	require.NoError(t, err)
	require.True(t, resp.GetAccepted())
	require.Equal(t, int32(1), resp.GetAcceptedCount())

	var occCount int
	require.NoError(t, pool.QueryRow(ctx, `
SELECT COUNT(*) FROM machine_incident_occurrences WHERE machine_id = $1 AND occurrence_id = $2`, machineID, occ).Scan(&occCount))
	require.Equal(t, 1, occCount)

	var groupCount int64
	require.NoError(t, pool.QueryRow(ctx, `
SELECT occurrence_count FROM machine_incidents WHERE machine_id = $1 AND dedupe_key = $2`, machineID, fp).Scan(&groupCount))
	require.Equal(t, int64(1), groupCount)

	key := alerts.TelegramAppIdempotencyKey(machineID.String(), occ)
	var intentCount int
	require.NoError(t, pool.QueryRow(ctx, `
SELECT COUNT(*) FROM outbox_events WHERE topic = 'notification.telegram' AND idempotency_key = $1`, key).Scan(&intentCount))
	require.Equal(t, 1, intentCount)
	var payload []byte
	require.NoError(t, pool.QueryRow(ctx, `
SELECT payload FROM outbox_events WHERE topic = 'notification.telegram' AND idempotency_key = $1 LIMIT 1`, key).Scan(&payload))
	var parsed map[string]any
	require.NoError(t, json.Unmarshal(payload, &parsed))
	require.Equal(t, "app", parsed["source"])
	require.Equal(t, occ, parsed["occurrence_id"])
	require.Equal(t, machineID.String(), parsed["machine_id"])
}

func TestSubmitTelemetryBatch_HealProjectionAfterTelemetryDuplicate(t *testing.T) {
	pool := machineGRPCTestPool(t)
	ctx := context.Background()
	siteID := id.NewUUIDV7()
	machineID := id.NewUUIDV7()
	require.NoError(t, insertMachineReplayLedgerFixture(ctx, pool, siteID, machineID))

	store := postgres.NewStore(pool)
	wrapper := &failThenSucceedProjector{failures: 1, inner: store}
	deps := offlineSyncIntegrationDeps(t, pool)
	deps.TelemetryStore = store
	deps.IncidentProjector = wrapper
	deps.Config = testMachineGRPCConfig()
	srv := &machineTelemetryServer{deps: deps}

	claims := plauth.MachineAccessClaims{MachineID: machineID, CredentialVersion: 1}
	ctxClaims := plauth.WithMachineAccessClaims(ctx, claims)
	occ := "incident_runtime_error:heal-" + machineID.String()
	fp := "fp-heal-" + machineID.String()
	fixedOccurred := timestamppb.New(time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC))
	req := &machinev1.SubmitTelemetryBatchRequest{
		Context: &machinev1.IdempotencyContext{
			IdempotencyKey:  "tel-inc-heal-" + machineID.String(),
			ClientEventId:   "client-heal-1",
			ClientCreatedAt: fixedOccurred,
		},
		Events: []*machinev1.TelemetryEvent{
			{
				EventType:  "incident_runtime_error",
				EventId:    occ,
				OccurredAt: fixedOccurred,
				Attributes: map[string]string{
					"severity":    "high",
					"fingerprint": fp,
					"title":       "heal me",
				},
			},
		},
	}

	_, err := srv.SubmitTelemetryBatch(ctxClaims, req)
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	require.Equal(t, codes.Internal, st.Code())

	resp, err := srv.SubmitTelemetryBatch(ctxClaims, req)
	require.NoError(t, err)
	require.True(t, resp.GetAccepted())
	require.Contains(t, resp.GetDuplicateEventIds(), occ)

	var occCount int
	require.NoError(t, pool.QueryRow(ctx, `
SELECT COUNT(*) FROM machine_incident_occurrences WHERE machine_id = $1 AND occurrence_id = $2`, machineID, occ).Scan(&occCount))
	require.Equal(t, 1, occCount)

	var groupCount int64
	require.NoError(t, pool.QueryRow(ctx, `
SELECT occurrence_count FROM machine_incidents WHERE machine_id = $1 AND dedupe_key = $2`, machineID, fp).Scan(&groupCount))
	require.Equal(t, int64(1), groupCount)

	var intentCount int
	key := alerts.TelegramAppIdempotencyKey(machineID.String(), occ)
	require.NoError(t, pool.QueryRow(ctx, `
SELECT COUNT(*) FROM outbox_events WHERE topic = 'notification.telegram' AND idempotency_key = $1`, key).Scan(&intentCount))
	require.Equal(t, 1, intentCount)
	require.Equal(t, 2, wrapper.calls)
}

func TestSubmitTelemetryBatch_NativeGRPCPersistsOccurrence(t *testing.T) {
	pool := machineGRPCTestPool(t)
	ctx := context.Background()
	siteID := id.NewUUIDV7()
	machineID := id.NewUUIDV7()
	require.NoError(t, insertMachineReplayLedgerFixture(ctx, pool, siteID, machineID))

	deps := offlineSyncIntegrationDeps(t, pool)
	deps.Config = testMachineGRPCConfig()
	srv := &machineTelemetryServer{deps: deps}

	claims := plauth.MachineAccessClaims{MachineID: machineID, CredentialVersion: 1}
	ctxClaims := plauth.WithMachineAccessClaims(ctx, claims)
	occ := "incident_runtime_error:native-" + machineID.String()
	req := &machinev1.SubmitTelemetryBatchRequest{
		Context: &machinev1.IdempotencyContext{
			IdempotencyKey:  "tel-inc-native-" + machineID.String(),
			ClientEventId:   "client-native-1",
			ClientCreatedAt: timestamppb.Now(),
		},
		Events: []*machinev1.TelemetryEvent{
			{
				EventType:  "incident_runtime_error",
				EventId:    occ,
				OccurredAt: timestamppb.Now(),
				Attributes: map[string]string{
					"severity":    "high",
					"fingerprint": "fp-native-" + machineID.String(),
					"title":       "native",
					"password":    "my-secret",
				},
			},
		},
	}
	_, err := srv.SubmitTelemetryBatch(ctxClaims, req)
	require.NoError(t, err)

	var occCount int
	require.NoError(t, pool.QueryRow(ctx, `
SELECT COUNT(*) FROM machine_incident_occurrences WHERE machine_id = $1 AND occurrence_id = $2`, machineID, occ).Scan(&occCount))
	require.Equal(t, 1, occCount)

	key := alerts.TelegramAppIdempotencyKey(machineID.String(), occ)
	var payload []byte
	require.NoError(t, pool.QueryRow(ctx, `
SELECT payload FROM outbox_events WHERE topic = 'notification.telegram' AND idempotency_key = $1`, key).Scan(&payload))
	var parsed map[string]any
	require.NoError(t, json.Unmarshal(payload, &parsed))
	require.Equal(t, "app", parsed["source"])
	require.NotContains(t, string(payload), "my-secret")
}

func TestUnaryServerAlertInterceptor_PagesInternalNotClientErrors(t *testing.T) {
	t.Cleanup(func() { setGRPCServerAlertTestHook(nil) })
	var reports []alerts.ServerAlert
	setGRPCServerAlertTestHook(func(a alerts.ServerAlert) { reports = append(reports, a) })

	interceptor := unaryServerAlertInterceptor()
	info := &grpc.UnaryServerInfo{FullMethod: "/test.Service/Do"}

	_, err := interceptor(context.Background(), nil, info, func(ctx context.Context, req any) (any, error) {
		return nil, status.Error(codes.InvalidArgument, "bad")
	})
	require.Error(t, err)
	require.Empty(t, reports)

	_, err = interceptor(context.Background(), nil, info, func(ctx context.Context, req any) (any, error) {
		return nil, status.Error(codes.Internal, "boom")
	})
	require.Error(t, err)
	require.Len(t, reports, 1)
	require.Equal(t, "grpc_server_error", reports[0].Code)

	_, err = interceptor(context.Background(), nil, info, func(ctx context.Context, req any) (any, error) {
		return nil, status.Error(codes.Unauthenticated, "nope")
	})
	require.Error(t, err)
	require.Len(t, reports, 1)

	_, err = interceptor(context.Background(), nil, info, func(ctx context.Context, req any) (any, error) {
		return nil, status.Error(codes.PermissionDenied, "no")
	})
	require.Error(t, err)
	require.Len(t, reports, 1)
}

func TestUnaryServerAlertInterceptor_MachineInternalQueuesAppIncident(t *testing.T) {
	t.Cleanup(func() {
		setGRPCServerAlertTestHook(nil)
		grpcAppIncidentTestHook = nil
	})
	var reports []alerts.ServerAlert
	setGRPCServerAlertTestHook(func(a alerts.ServerAlert) { reports = append(reports, a) })
	var app []alerts.ProjectInput
	grpcAppIncidentTestHook = func(in alerts.ProjectInput) { app = append(app, in) }

	machineID := id.NewUUIDV7()
	ctx := plauth.WithMachineAccessClaims(context.Background(), plauth.MachineAccessClaims{MachineID: machineID})
	interceptor := unaryServerAlertInterceptor()
	info := &grpc.UnaryServerInfo{FullMethod: machinev1.MachineRuntimeSessionService_StartRuntimeSession_FullMethodName}
	_, err := interceptor(ctx, nil, info, func(ctx context.Context, req any) (any, error) {
		return nil, status.Error(codes.Internal, "start runtime session: stage=insert_runtime_session sql=StartMachineRuntimeAppSession sqlstate=22P02 msg=invalid input syntax for type json")
	})
	require.Error(t, err)
	require.Len(t, reports, 1)
	require.Equal(t, "22P02", reports[0].Detail["sqlstate"])
	require.Equal(t, "insert_runtime_session", reports[0].Detail["stage"])
	require.Equal(t, machineID.String(), reports[0].Detail["machine_id"])
	require.Contains(t, reports[0].Detail["next_action"], "jsonb")
	require.Len(t, app, 1)
	require.Equal(t, machineID.String(), app[0].MachineID)
	require.Equal(t, "incident_runtime_error", app[0].EventType)
	require.NotContains(t, string(app[0].Detail), "Bearer")
}

func TestShouldPageGRPCCode(t *testing.T) {
	require.True(t, shouldPageGRPCCode(codes.Internal))
	require.True(t, shouldPageGRPCCode(codes.Unknown))
	require.True(t, shouldPageGRPCCode(codes.DataLoss))
	require.True(t, shouldPageGRPCCode(codes.Unavailable))
	require.False(t, shouldPageGRPCCode(codes.InvalidArgument))
	require.False(t, shouldPageGRPCCode(codes.Unauthenticated))
	require.False(t, shouldPageGRPCCode(codes.PermissionDenied))
	require.False(t, shouldPageGRPCCode(codes.NotFound))
}

func TestUnaryRecoveryInterceptor_PagesPanic(t *testing.T) {
	t.Cleanup(func() { setGRPCServerAlertTestHook(nil) })
	var reports []alerts.ServerAlert
	setGRPCServerAlertTestHook(func(a alerts.ServerAlert) { reports = append(reports, a) })

	interceptor := unaryRecoveryInterceptor(nil)
	info := &grpc.UnaryServerInfo{FullMethod: "/test.Service/Panic"}
	_, err := interceptor(context.Background(), nil, info, func(ctx context.Context, req any) (any, error) {
		panic("boom")
	})
	require.Error(t, err)
	require.Equal(t, codes.Internal, status.Code(err))
	require.Len(t, reports, 1)
	require.Equal(t, "grpc_panic", reports[0].Code)
}
