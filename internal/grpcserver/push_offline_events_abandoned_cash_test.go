package grpcserver

import (
	"context"
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
