package grpcserver

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/avf/avf-vending-api/internal/app/activation"
	"github.com/avf/avf-vending-api/internal/app/api"
	appaudit "github.com/avf/avf-vending-api/internal/app/audit"
	appcommerce "github.com/avf/avf-vending-api/internal/app/commerce"
	"github.com/avf/avf-vending-api/internal/app/machineidempotency"
	"github.com/avf/avf-vending-api/internal/app/salecatalog"
	"github.com/avf/avf-vending-api/internal/app/workfloworch"
	"github.com/avf/avf-vending-api/internal/config"
	"github.com/avf/avf-vending-api/internal/modules/postgres"
	plauth "github.com/avf/avf-vending-api/internal/platform/auth"
	platformpayments "github.com/avf/avf-vending-api/internal/platform/payments"
	"github.com/avf/avf-vending-api/internal/testfixtures"
	machinev1 "github.com/avf/avf-vending-api/proto/avf/machine/v1"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func machineOfflineCashTestServer(t *testing.T, pool *pgxpool.Pool, cfg *config.Config) (*Server, *plauth.SessionIssuer) {
	t.Helper()
	if cfg == nil {
		cfg = testMachineGRPCConfig()
	}
	issuer, err := plauth.NewSessionIssuerFromHTTPAuth(cfg.HTTPAuth)
	require.NoError(t, err)
	pepper := plauth.TrimSecret(cfg.HTTPAuth.JWTSecret)
	act := activation.NewService(pool, issuer, pepper, nil)
	store := postgres.NewStore(pool)
	auditSvc := appaudit.NewService(pool)
	payReg := platformpayments.NewRegistry(cfg)
	commerceSvc := appcommerce.NewService(appcommerce.Deps{
		OrderVend:              store,
		PaymentOutbox:          store,
		Lifecycle:              store,
		WebhookPersist:         store,
		SaleLines:              store,
		WorkflowOrchestration:  workfloworch.NewDisabled(),
		EnterpriseAudit:        auditSvc,
		PaymentSessionRegistry: payReg,
		CashForensic:           store,
	})
	machineQueries := api.NewInternalMachineQueryService(store, api.NewSQLMachineShadow(pool))
	replayLedger := NewMachineReplayLedger(pool, auditSvc)
	srv, err := NewServer(cfg, zap.NewNop(), nil, nil, nil, replayLedger, nil, nil, RegisterMachineGRPCServices(MachineGRPCServicesDeps{
		Activation:      act,
		MachineQueries:  machineQueries,
		SaleCatalog:     salecatalog.NewService(pool),
		Pool:            pool,
		MQTTBrokerURL:   "tcp://example.invalid",
		MQTTTopicPrefix: "avf/devices",
		Config:          cfg,
		InventoryLedger: postgres.NewInventoryRepository(pool),
		Commerce:        commerceSvc,
		TelemetryStore:  store,
		EnterpriseAudit: auditSvc,
		PaymentRuntime:  payReg,
	}))
	require.NoError(t, err)
	return srv, issuer
}

func TestPushOfflineEvents_cashRetryWithAbandonedStillIngests(t *testing.T) {
	pool := machineGRPCTestPool(t)
	ctx := context.Background()
	cfg := testMachineGRPCConfig()
	srv, issuer := machineOfflineCashTestServer(t, pool, cfg)
	conn := dialMachineCommerceServer(t, srv)
	md := machineAccessMD(t, pool, issuer, testfixtures.DevMachineID, testfixtures.DevSiteID)
	cli := machinev1.NewMachineOfflineSyncServiceClient(conn)

	_, err := pool.Exec(ctx, `DELETE FROM machine_offline_events WHERE machine_id = $1`, testfixtures.DevMachineID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `DELETE FROM machine_sync_cursors WHERE machine_id = $1 AND stream_name = 'offline'`,
		testfixtures.DevMachineID)
	require.NoError(t, err)

	deviceEventID := "D20204-" + uuid.NewString()
	cashKey := "cash_movement:" + testfixtures.DevMachineID.String() + ":boot:" + deviceEventID + ":bill_credit"
	payload := reportCashOfflinePayload(t, cashKey, deviceEventID)
	clientEventID := "cli-" + uuid.NewString()

	event := &machinev1.OfflineEvent{
		Meta: &machinev1.MachineRequestMeta{
			OfflineSequence: 1,
			IdempotencyKey:  cashKey,
			RequestId:       "req-" + uuid.NewString(),
			ClientEventId:   clientEventID,
			OccurredAt:      timestamppb.Now(),
		},
		EventType: "commerce.report_cash_movements",
		Payload:   payload,
	}

	first, err := cli.PushOfflineEvents(md, &machinev1.SyncOfflineEventsRequest{
		Meta:   &machinev1.MachineRequestMeta{IdempotencyKey: cashKey, RequestId: "sync-1"},
		Events: []*machinev1.OfflineEvent{event},
	})
	require.NoError(t, err)
	require.Len(t, first.GetResults(), 1)
	require.Equal(t, machinev1.MachineResponseStatus_MACHINE_RESPONSE_STATUS_ACCEPTED, first.GetResults()[0].GetStatus(), first.GetResults()[0].GetReason())

	retry, err := cli.PushOfflineEvents(md, &machinev1.SyncOfflineEventsRequest{
		Meta: &machinev1.MachineRequestMeta{IdempotencyKey: cashKey, RequestId: "sync-2"},
		Events: []*machinev1.OfflineEvent{{
			Meta: &machinev1.MachineRequestMeta{
				OfflineSequence: 1,
				IdempotencyKey:  cashKey,
				RequestId:       "req-retry",
				ClientEventId:   clientEventID,
				OccurredAt:      timestamppb.Now(),
			},
			EventType: "commerce.report_cash_movements",
			Payload:   payload,
		}},
		Abandoned: []*machinev1.AbandonedOfflineSequence{{
			OfflineSequence: 99,
			ClientEventId:   "abandoned-" + uuid.NewString(),
			Reason:          "device_dead_letter",
		}},
	})
	require.NoError(t, err, "retry with abandoned[] must not trip outer idempotency_payload_mismatch")
	require.NotEqual(t, codes.FailedPrecondition, status.Code(err))
	require.GreaterOrEqual(t, len(retry.GetResults()), 1)

	var count int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM cash_acceptance_events WHERE machine_id = $1 AND device_event_id = $2`,
		testfixtures.DevMachineID, deviceEventID,
	).Scan(&count))
	require.Equal(t, 1, count)
}

func TestPushOfflineEvents_abandonedClientEventIdCollisionStillIngestsLiveCash(t *testing.T) {
	pool := machineGRPCTestPool(t)
	ctx := context.Background()
	cfg := testMachineGRPCConfig()
	cfg.OfflineSync.GapTolerant = true
	srv, issuer := machineOfflineCashTestServer(t, pool, cfg)
	conn := dialMachineCommerceServer(t, srv)
	md := machineAccessMD(t, pool, issuer, testfixtures.DevMachineID, testfixtures.DevSiteID)
	cli := machinev1.NewMachineOfflineSyncServiceClient(conn)
	resetMachineOfflineLedger(t, ctx, pool)
	seedCashSyncCursor(t, ctx, pool, 316)

	collidingID := "cash_movement:seed-322:" + uuid.NewString()
	_, err := pool.Exec(ctx, `
INSERT INTO machine_offline_events (
    machine_id, offline_sequence, event_type, event_id, client_event_id,
    occurred_at, payload, processing_status, processing_error, idempotency_key
) VALUES (
    $1, 50, 'stream.abandoned', '', $2,
    now(), '{}'::jsonb, 'rejected', 'seed-collision', ''
)`, testfixtures.DevMachineID, collidingID)
	require.NoError(t, err)

	deviceEventID := "D20204-" + uuid.NewString()
	cashKey := "cash_movement:" + testfixtures.DevMachineID.String() + ":boot:" + deviceEventID + ":bill_credit"
	live := cashOfflineEvent(t, 317, cashKey, deviceEventID)

	resp, err := cli.PushOfflineEvents(md, &machinev1.SyncOfflineEventsRequest{
		Meta:   &machinev1.MachineRequestMeta{IdempotencyKey: cashKey, RequestId: "sync-collide"},
		Events: []*machinev1.OfflineEvent{live},
		Abandoned: []*machinev1.AbandonedOfflineSequence{{
			OfflineSequence: 322,
			ClientEventId:   collidingID,
			Reason:          "device_dead_letter",
		}},
	})
	require.NoError(t, err, "colliding abandoned client_event_id must not abort PushOfflineEvents")
	require.NotEqual(t, codes.Unknown, status.Code(err))
	requireLiveCashAccepted(t, resp, cashKey)

	var count int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM cash_acceptance_events WHERE machine_id = $1 AND device_event_id = $2`,
		testfixtures.DevMachineID, deviceEventID,
	).Scan(&count))
	require.Equal(t, 1, count)

	var eventType, clientEventID string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT event_type, client_event_id FROM machine_offline_events WHERE machine_id = $1 AND offline_sequence = 322`,
		testfixtures.DevMachineID,
	).Scan(&eventType, &clientEventID))
	require.Equal(t, "stream.abandoned", eventType)
	require.NotEqual(t, collidingID, clientEventID, "retry must not reuse the colliding client_event_id")
}

func TestPushOfflineEvents_abandonedSameSequenceRetryReplayed(t *testing.T) {
	pool := machineGRPCTestPool(t)
	ctx := context.Background()
	cfg := testMachineGRPCConfig()
	cfg.OfflineSync.GapTolerant = true
	srv, issuer := machineOfflineCashTestServer(t, pool, cfg)
	conn := dialMachineCommerceServer(t, srv)
	md := machineAccessMD(t, pool, issuer, testfixtures.DevMachineID, testfixtures.DevSiteID)
	cli := machinev1.NewMachineOfflineSyncServiceClient(conn)
	resetMachineOfflineLedger(t, ctx, pool)

	deviceEventID := "D20204-" + uuid.NewString()
	cashKey := "cash_movement:" + testfixtures.DevMachineID.String() + ":boot:" + deviceEventID + ":bill_credit"
	live := cashOfflineEvent(t, 1, cashKey, deviceEventID)
	abandonedID := "abandoned-seq2-" + uuid.NewString()

	first, err := cli.PushOfflineEvents(md, &machinev1.SyncOfflineEventsRequest{
		Meta:   &machinev1.MachineRequestMeta{IdempotencyKey: cashKey, RequestId: "sync-abandon-1"},
		Events: []*machinev1.OfflineEvent{live},
		Abandoned: []*machinev1.AbandonedOfflineSequence{{
			OfflineSequence: 2,
			ClientEventId:   abandonedID,
			Reason:          "device_dead_letter",
		}},
	})
	require.NoError(t, err)
	requireLiveCashAccepted(t, first, cashKey)

	retry, err := cli.PushOfflineEvents(md, &machinev1.SyncOfflineEventsRequest{
		Meta: &machinev1.MachineRequestMeta{IdempotencyKey: cashKey + ":retry", RequestId: "sync-abandon-2"},
		Abandoned: []*machinev1.AbandonedOfflineSequence{{
			OfflineSequence: 2,
			ClientEventId:   abandonedID,
			Reason:          "device_dead_letter",
		}},
	})
	require.NoError(t, err)
	require.NotEmpty(t, retry.GetResults())
	foundReplay := false
	for _, result := range retry.GetResults() {
		if result.GetOfflineSequence() != 2 {
			continue
		}
		foundReplay = true
		require.Equal(t, machinev1.MachineResponseStatus_MACHINE_RESPONSE_STATUS_REPLAYED, result.GetStatus(), result.GetReason())
	}
	require.True(t, foundReplay, "retry of abandoned seq 2 must be REPLAYED")

	var count int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM cash_acceptance_events WHERE machine_id = $1 AND device_event_id = $2`,
		testfixtures.DevMachineID, deviceEventID,
	).Scan(&count))
	require.Equal(t, 1, count, "abandoned retry must not duplicate cash ingest")

	var abandonedCount int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM machine_offline_events WHERE machine_id = $1 AND offline_sequence = 2 AND event_type = 'stream.abandoned'`,
		testfixtures.DevMachineID,
	).Scan(&abandonedCount))
	require.Equal(t, 1, abandonedCount)
}

func TestPushOfflineEvents_abandonedJumpDoesNotSkipLowerLiveCash(t *testing.T) {
	pool := machineGRPCTestPool(t)
	ctx := context.Background()
	cfg := testMachineGRPCConfig()
	cfg.OfflineSync.GapTolerant = true
	srv, issuer := machineOfflineCashTestServer(t, pool, cfg)
	conn := dialMachineCommerceServer(t, srv)
	md := machineAccessMD(t, pool, issuer, testfixtures.DevMachineID, testfixtures.DevSiteID)
	cli := machinev1.NewMachineOfflineSyncServiceClient(conn)
	resetMachineOfflineLedger(t, ctx, pool)
	seedCashSyncCursor(t, ctx, pool, 316)

	deviceEventID := "D20204-" + uuid.NewString()
	cashKey := "cash_movement:" + testfixtures.DevMachineID.String() + ":boot:" + deviceEventID + ":bill_credit"
	live := cashOfflineEvent(t, 317, cashKey, deviceEventID)

	resp, err := cli.PushOfflineEvents(md, &machinev1.SyncOfflineEventsRequest{
		Meta:   &machinev1.MachineRequestMeta{IdempotencyKey: cashKey, RequestId: "sync-jump"},
		Events: []*machinev1.OfflineEvent{live},
		Abandoned: []*machinev1.AbandonedOfflineSequence{{
			OfflineSequence: 322,
			ClientEventId:   "abandoned-322-" + uuid.NewString(),
			Reason:          "device_dead_letter",
		}},
	})
	require.NoError(t, err)
	requireLiveCashAccepted(t, resp, cashKey)
	for _, result := range resp.GetResults() {
		if result.GetIdempotencyKey() == cashKey {
			require.NotContains(t, strings.ToLower(result.GetReason()), "slot occupied")
		}
	}

	var count int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM cash_acceptance_events WHERE machine_id = $1 AND device_event_id = $2`,
		testfixtures.DevMachineID, deviceEventID,
	).Scan(&count))
	require.Equal(t, 1, count, "live seq 317 must ingest even when abandoned 322 is in the same request")

	var cashCursor int64
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT last_sequence FROM machine_sync_cursors WHERE machine_id = $1 AND stream_name = 'offline:cash'`,
		testfixtures.DevMachineID,
	).Scan(&cashCursor))
	require.Equal(t, int64(317), cashCursor, "abandoned 322 must not jump the cash cursor past live 317")
}

func resetMachineOfflineLedger(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	_, err := pool.Exec(ctx, `DELETE FROM machine_offline_events WHERE machine_id = $1`, testfixtures.DevMachineID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `DELETE FROM machine_sync_cursors WHERE machine_id = $1`, testfixtures.DevMachineID)
	require.NoError(t, err)
}

func seedCashSyncCursor(t *testing.T, ctx context.Context, pool *pgxpool.Pool, lastSequence int64) {
	t.Helper()
	_, err := pool.Exec(ctx, `
INSERT INTO machine_sync_cursors (machine_id, stream_name, last_sequence, last_synced_at, updated_at)
VALUES ($1, 'offline:cash', $2, now(), now())
`, testfixtures.DevMachineID, lastSequence)
	require.NoError(t, err)
}

func cashOfflineEvent(t *testing.T, seq int64, cashKey, deviceEventID string) *machinev1.OfflineEvent {
	t.Helper()
	return &machinev1.OfflineEvent{
		Meta: &machinev1.MachineRequestMeta{
			OfflineSequence: seq,
			IdempotencyKey:  cashKey,
			RequestId:       "req-" + uuid.NewString(),
			ClientEventId:   "cli-" + uuid.NewString(),
			OccurredAt:      timestamppb.Now(),
		},
		EventType: "commerce.report_cash_movements",
		Payload:   reportCashOfflinePayload(t, cashKey, deviceEventID),
	}
}

func requireLiveCashAccepted(t *testing.T, resp *machinev1.SyncOfflineEventsResponse, cashKey string) {
	t.Helper()
	require.NotNil(t, resp)
	found := false
	for _, result := range resp.GetResults() {
		if result.GetIdempotencyKey() != cashKey {
			continue
		}
		found = true
		require.Equal(t, machinev1.MachineResponseStatus_MACHINE_RESPONSE_STATUS_ACCEPTED, result.GetStatus(), result.GetReason())
	}
	require.True(t, found, "live cash result missing for %s", cashKey)
}

func TestHashMutationRequest_abandonedChangesPushOfflineEventsFingerprint(t *testing.T) {
	t.Parallel()
	event := &machinev1.OfflineEvent{
		Meta: &machinev1.MachineRequestMeta{
			OfflineSequence: 43,
			IdempotencyKey:  "cash_movement:m:D20204:bill_credit",
			ClientEventId:   "cli-1",
		},
		EventType: "commerce.report_cash_movements",
	}
	first := &machinev1.SyncOfflineEventsRequest{
		Meta:   &machinev1.MachineRequestMeta{IdempotencyKey: event.GetMeta().GetIdempotencyKey()},
		Events: []*machinev1.OfflineEvent{event},
	}
	second := &machinev1.SyncOfflineEventsRequest{
		Meta:   &machinev1.MachineRequestMeta{IdempotencyKey: event.GetMeta().GetIdempotencyKey()},
		Events: []*machinev1.OfflineEvent{event},
		Abandoned: []*machinev1.AbandonedOfflineSequence{{
			OfflineSequence: 42,
			Reason:          "stale",
		}},
	}
	h1, err := machineidempotency.HashMutationRequest(first)
	require.NoError(t, err)
	h2, err := machineidempotency.HashMutationRequest(second)
	require.NoError(t, err)
	require.NotEqual(t, h1, h2, "abandoned[] must change the outer ledger hash; PushOfflineEvents must not use that ledger")
}

func reportCashOfflinePayload(t *testing.T, cashKey, deviceEventID string) *structpb.Struct {
	t.Helper()
	req := &machinev1.ReportCashMovementsRequest{
		Context: &machinev1.IdempotencyContext{
			IdempotencyKey:  cashKey,
			ClientEventId:   "cli-" + deviceEventID,
			ClientCreatedAt: timestamppb.New(time.Now().UTC()),
		},
		Events: []*machinev1.CashMovementEvent{{
			Kind:              "bill_credit",
			DeviceEventId:     deviceEventID,
			OccurredAt:        timestamppb.Now(),
			DenominationMinor: 10000,
			CreditSource:      "ict_bill",
			Currency:          "VND",
			RawRecordHex:      "D20204",
		}},
	}
	b, err := protojson.Marshal(req)
	require.NoError(t, err)
	var payload structpb.Struct
	require.NoError(t, protojson.Unmarshal(b, &payload))
	return &payload
}
