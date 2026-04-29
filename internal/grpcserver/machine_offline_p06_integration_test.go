package grpcserver

import (
	"context"
	"testing"

	"github.com/avf/avf-vending-api/internal/app/activation"
	"github.com/avf/avf-vending-api/internal/app/api"
	appaudit "github.com/avf/avf-vending-api/internal/app/audit"
	appcommerce "github.com/avf/avf-vending-api/internal/app/commerce"
	"github.com/avf/avf-vending-api/internal/app/salecatalog"
	"github.com/avf/avf-vending-api/internal/app/workfloworch"
	"github.com/avf/avf-vending-api/internal/modules/postgres"
	plauth "github.com/avf/avf-vending-api/internal/platform/auth"
	platformpayments "github.com/avf/avf-vending-api/internal/platform/payments"
	"github.com/avf/avf-vending-api/internal/testfixtures"
	machinev1 "github.com/avf/avf-vending-api/proto/avf/machine/v1"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func offlineSyncIntegrationDeps(t *testing.T, pool *pgxpool.Pool) MachineGRPCServicesDeps {
	t.Helper()
	cfg := testMachineGRPCConfig()
	issuer, err := plauth.NewSessionIssuerFromHTTPAuth(cfg.HTTPAuth)
	require.NoError(t, err)
	pepper := plauth.TrimSecret(cfg.HTTPAuth.JWTSecret)
	act := activation.NewService(pool, issuer, pepper, nil)
	store := postgres.NewStore(pool)
	auditSvc := appaudit.NewService(pool)
	commerceSvc := appcommerce.NewService(appcommerce.Deps{
		OrderVend:              store,
		PaymentOutbox:          store,
		Lifecycle:              store,
		WebhookPersist:         store,
		SaleLines:              store,
		WorkflowOrchestration:  workfloworch.NewDisabled(),
		EnterpriseAudit:        auditSvc,
		PaymentSessionRegistry: platformpayments.NewRegistry(cfg),
	})
	machineQueries := api.NewInternalMachineQueryService(store, api.NewSQLMachineShadow(pool))
	return MachineGRPCServicesDeps{
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
	}
}

func offlineProtoPayload(t *testing.T, msg proto.Message) *structpb.Struct {
	t.Helper()
	b, err := protojson.Marshal(msg)
	require.NoError(t, err)
	var s structpb.Struct
	require.NoError(t, protojson.Unmarshal(b, &s))
	return &s
}

func TestP06_OfflineSync_sortedDescendingMetaStillProcessesAscendingSequences(t *testing.T) {
	t.Parallel()

	pool := machineGRPCTestPool(t)
	ctx := context.Background()
	orgID := uuid.New()
	siteID := uuid.New()
	machineID := uuid.New()
	require.NoError(t, insertMachineReplayLedgerFixture(ctx, pool, orgID, siteID, machineID))

	deps := offlineSyncIntegrationDeps(t, pool)
	srv := &machineOfflineSyncServer{deps: deps}

	tbReq := &machinev1.SubmitTelemetryBatchRequest{
		Context: &machinev1.IdempotencyContext{
			IdempotencyKey:  "offline-seq-sort",
			ClientEventId:   "client-seq-sort",
			ClientCreatedAt: timestamppb.Now(),
		},
		Events: []*machinev1.TelemetryEvent{
			{
				EventType:  "heartbeat",
				OccurredAt: timestamppb.Now(),
				EventId:    "evt-hb",
			},
		},
	}
	payloadJSON, err := protojson.Marshal(tbReq)
	require.NoError(t, err)
	var payloadStruct structpb.Struct
	require.NoError(t, protojson.Unmarshal(payloadJSON, &payloadStruct))

	claims := plauth.MachineAccessClaims{OrganizationID: orgID, MachineID: machineID, CredentialVersion: 1}
	ctxClaims := plauth.WithMachineAccessClaims(ctx, claims)

	out, err := srv.PushOfflineEvents(ctxClaims, &machinev1.SyncOfflineEventsRequest{
		Meta: &machinev1.MachineRequestMeta{IdempotencyKey: "sync-sort", RequestId: "sync-sort"},
		Events: []*machinev1.OfflineEvent{
			{
				Meta: &machinev1.MachineRequestMeta{
					OfflineSequence: 2,
					IdempotencyKey:  "oe-2",
					RequestId:       "req-2",
					ClientEventId:   "cli-2",
					OccurredAt:      timestamppb.Now(),
				},
				EventType: "telemetry.batch",
				Payload:   &payloadStruct,
			},
			{
				Meta: &machinev1.MachineRequestMeta{
					OfflineSequence: 1,
					IdempotencyKey:  "oe-1",
					RequestId:       "req-1",
					ClientEventId:   "cli-1",
					OccurredAt:      timestamppb.Now(),
				},
				EventType: "telemetry.batch",
				Payload:   &payloadStruct,
			},
		},
	})
	require.NoError(t, err)
	require.Len(t, out.GetResults(), 2)
	require.Equal(t, int64(1), out.GetResults()[0].GetOfflineSequence())
	require.Equal(t, machinev1.MachineResponseStatus_MACHINE_RESPONSE_STATUS_ACCEPTED, out.GetResults()[0].GetStatus())
	require.Equal(t, int64(2), out.GetResults()[1].GetOfflineSequence())
	require.Equal(t, machinev1.MachineResponseStatus_MACHINE_RESPONSE_STATUS_ACCEPTED, out.GetResults()[1].GetStatus())
}

func TestP06_OfflineSync_duplicateOfflineSequenceReplayed(t *testing.T) {
	t.Parallel()

	pool := machineGRPCTestPool(t)
	ctx := context.Background()
	orgID := uuid.New()
	siteID := uuid.New()
	machineID := uuid.New()
	require.NoError(t, insertMachineReplayLedgerFixture(ctx, pool, orgID, siteID, machineID))

	deps := offlineSyncIntegrationDeps(t, pool)
	srv := &machineOfflineSyncServer{deps: deps}

	tbReq := &machinev1.SubmitTelemetryBatchRequest{
		Context: &machinev1.IdempotencyContext{
			IdempotencyKey:  "offline-dup-seq",
			ClientEventId:   "client-dup-seq",
			ClientCreatedAt: timestamppb.Now(),
		},
		Events: []*machinev1.TelemetryEvent{
			{
				EventType:  "heartbeat",
				OccurredAt: timestamppb.Now(),
				EventId:    "evt-hb-dup",
			},
		},
	}
	payloadJSON, err := protojson.Marshal(tbReq)
	require.NoError(t, err)
	var payloadStruct structpb.Struct
	require.NoError(t, protojson.Unmarshal(payloadJSON, &payloadStruct))

	claims := plauth.MachineAccessClaims{OrganizationID: orgID, MachineID: machineID, CredentialVersion: 1}
	ctxClaims := plauth.WithMachineAccessClaims(ctx, claims)

	ev := &machinev1.OfflineEvent{
		Meta: &machinev1.MachineRequestMeta{
			OfflineSequence: 1,
			IdempotencyKey:  "oe-dup",
			RequestId:       "req-dup",
			ClientEventId:   "cli-dup-replay",
			OccurredAt:      timestamppb.Now(),
		},
		EventType: "telemetry.batch",
		Payload:   &payloadStruct,
	}

	req := &machinev1.SyncOfflineEventsRequest{
		Meta:   &machinev1.MachineRequestMeta{IdempotencyKey: "sync-dup", RequestId: "sync-dup"},
		Events: []*machinev1.OfflineEvent{ev},
	}

	out1, err := srv.PushOfflineEvents(ctxClaims, req)
	require.NoError(t, err)
	require.Len(t, out1.GetResults(), 1)
	require.Equal(t, machinev1.MachineResponseStatus_MACHINE_RESPONSE_STATUS_ACCEPTED, out1.GetResults()[0].GetStatus())

	out2, err := srv.PushOfflineEvents(ctxClaims, req)
	require.NoError(t, err)
	require.Len(t, out2.GetResults(), 1)
	require.Equal(t, machinev1.MachineResponseStatus_MACHINE_RESPONSE_STATUS_REPLAYED, out2.GetResults()[0].GetStatus())
	require.Contains(t, out2.GetResults()[0].GetReason(), "already synced")
}

func TestP06_OfflineSync_gapInSequenceRejectedAfterSuccessfulCursorBump(t *testing.T) {
	t.Parallel()

	pool := machineGRPCTestPool(t)
	ctx := context.Background()
	orgID := uuid.New()
	siteID := uuid.New()
	machineID := uuid.New()
	require.NoError(t, insertMachineReplayLedgerFixture(ctx, pool, orgID, siteID, machineID))

	deps := offlineSyncIntegrationDeps(t, pool)
	srv := &machineOfflineSyncServer{deps: deps}

	tbReq := &machinev1.SubmitTelemetryBatchRequest{
		Context: &machinev1.IdempotencyContext{
			IdempotencyKey:  "offline-gap",
			ClientEventId:   "client-gap",
			ClientCreatedAt: timestamppb.Now(),
		},
		Events: []*machinev1.TelemetryEvent{
			{
				EventType:  "heartbeat",
				OccurredAt: timestamppb.Now(),
				EventId:    "evt-gap",
			},
		},
	}
	payloadJSON, err := protojson.Marshal(tbReq)
	require.NoError(t, err)
	var payloadStruct structpb.Struct
	require.NoError(t, protojson.Unmarshal(payloadJSON, &payloadStruct))

	claims := plauth.MachineAccessClaims{OrganizationID: orgID, MachineID: machineID, CredentialVersion: 1}
	ctxClaims := plauth.WithMachineAccessClaims(ctx, claims)

	_, err = srv.PushOfflineEvents(ctxClaims, &machinev1.SyncOfflineEventsRequest{
		Meta: &machinev1.MachineRequestMeta{IdempotencyKey: "sync-gap-a", RequestId: "sync-gap-a"},
		Events: []*machinev1.OfflineEvent{{
			Meta: &machinev1.MachineRequestMeta{
				OfflineSequence: 1,
				IdempotencyKey:  "oe-gap-1",
				RequestId:       "req-gap-1",
				ClientEventId:   "cli-gap-1",
				OccurredAt:      timestamppb.Now(),
			},
			EventType: "telemetry.batch",
			Payload:   &payloadStruct,
		}},
	})
	require.NoError(t, err)

	_, err = srv.PushOfflineEvents(ctxClaims, &machinev1.SyncOfflineEventsRequest{
		Meta: &machinev1.MachineRequestMeta{IdempotencyKey: "sync-gap-b", RequestId: "sync-gap-b"},
		Events: []*machinev1.OfflineEvent{{
			Meta: &machinev1.MachineRequestMeta{
				OfflineSequence: 3,
				IdempotencyKey:  "oe-gap-3",
				RequestId:       "req-gap-3",
				ClientEventId:   "cli-gap-3",
				OccurredAt:      timestamppb.Now(),
			},
			EventType: "telemetry.batch",
			Payload:   &payloadStruct,
		}},
	})
	require.Equal(t, codes.Aborted, status.Code(err))
}

func TestP06_OfflineSync_outOfOrderErrorIncludesExpectedSequence(t *testing.T) {
	t.Parallel()

	pool := machineGRPCTestPool(t)
	ctx := context.Background()
	orgID := uuid.New()
	siteID := uuid.New()
	machineID := uuid.New()
	require.NoError(t, insertMachineReplayLedgerFixture(ctx, pool, orgID, siteID, machineID))

	deps := offlineSyncIntegrationDeps(t, pool)
	srv := &machineOfflineSyncServer{deps: deps}

	tbReq := &machinev1.SubmitTelemetryBatchRequest{
		Context: &machinev1.IdempotencyContext{
			IdempotencyKey:  "offline-oo-msg",
			ClientEventId:   "client-oo-msg",
			ClientCreatedAt: timestamppb.Now(),
		},
		Events: []*machinev1.TelemetryEvent{{
			EventType:  "heartbeat",
			OccurredAt: timestamppb.Now(),
			EventId:    "evt-oo",
		}},
	}
	payloadJSON, err := protojson.Marshal(tbReq)
	require.NoError(t, err)
	var payloadStruct structpb.Struct
	require.NoError(t, protojson.Unmarshal(payloadJSON, &payloadStruct))

	claims := plauth.MachineAccessClaims{OrganizationID: orgID, MachineID: machineID, CredentialVersion: 1}
	ctxClaims := plauth.WithMachineAccessClaims(ctx, claims)

	_, err = srv.PushOfflineEvents(ctxClaims, &machinev1.SyncOfflineEventsRequest{
		Meta: &machinev1.MachineRequestMeta{IdempotencyKey: "sync-oo", RequestId: "sync-oo"},
		Events: []*machinev1.OfflineEvent{{
			Meta: &machinev1.MachineRequestMeta{
				OfflineSequence: 5,
				IdempotencyKey:  "oe-oo",
				RequestId:       "req-oo",
				ClientEventId:   "cli-oo",
				OccurredAt:      timestamppb.Now(),
			},
			EventType: "telemetry.batch",
			Payload:   &payloadStruct,
		}},
	})
	require.Error(t, err)
	require.Equal(t, codes.Aborted, status.Code(err))
	require.Contains(t, err.Error(), "expected 1 got 5")
}

func TestP06_OfflineSync_duplicateClientEventIdAtLaterSequenceRejected(t *testing.T) {
	t.Parallel()

	pool := machineGRPCTestPool(t)
	ctx := context.Background()
	orgID := uuid.New()
	siteID := uuid.New()
	machineID := uuid.New()
	require.NoError(t, insertMachineReplayLedgerFixture(ctx, pool, orgID, siteID, machineID))

	deps := offlineSyncIntegrationDeps(t, pool)
	srv := &machineOfflineSyncServer{deps: deps}

	mkPayload := func() *structpb.Struct {
		tbReq := &machinev1.SubmitTelemetryBatchRequest{
			Context: &machinev1.IdempotencyContext{
				IdempotencyKey:  "offline-dup-ce",
				ClientEventId:   "client-dup-ce",
				ClientCreatedAt: timestamppb.Now(),
			},
			Events: []*machinev1.TelemetryEvent{{
				EventType:  "heartbeat",
				OccurredAt: timestamppb.Now(),
				EventId:    "evt-dup-ce",
			}},
		}
		payloadJSON, err := protojson.Marshal(tbReq)
		require.NoError(t, err)
		var payloadStruct structpb.Struct
		require.NoError(t, protojson.Unmarshal(payloadJSON, &payloadStruct))
		return &payloadStruct
	}

	claims := plauth.MachineAccessClaims{OrganizationID: orgID, MachineID: machineID, CredentialVersion: 1}
	ctxClaims := plauth.WithMachineAccessClaims(ctx, claims)

	out1, err := srv.PushOfflineEvents(ctxClaims, &machinev1.SyncOfflineEventsRequest{
		Meta: &machinev1.MachineRequestMeta{IdempotencyKey: "sync-dup-ce-1", RequestId: "r1"},
		Events: []*machinev1.OfflineEvent{{
			Meta: &machinev1.MachineRequestMeta{
				OfflineSequence: 1,
				IdempotencyKey:  "k1",
				RequestId:       "r1",
				ClientEventId:   "shared-ce-id",
				OccurredAt:      timestamppb.Now(),
			},
			EventType: "telemetry.batch",
			Payload:   mkPayload(),
		}},
	})
	require.NoError(t, err)
	require.Equal(t, machinev1.MachineResponseStatus_MACHINE_RESPONSE_STATUS_ACCEPTED, out1.GetResults()[0].GetStatus())

	out2, err := srv.PushOfflineEvents(ctxClaims, &machinev1.SyncOfflineEventsRequest{
		Meta: &machinev1.MachineRequestMeta{IdempotencyKey: "sync-dup-ce-2", RequestId: "r2"},
		Events: []*machinev1.OfflineEvent{{
			Meta: &machinev1.MachineRequestMeta{
				OfflineSequence: 2,
				IdempotencyKey:  "k2",
				RequestId:       "r2",
				ClientEventId:   "shared-ce-id",
				OccurredAt:      timestamppb.Now(),
			},
			EventType: "telemetry.batch",
			Payload:   mkPayload(),
		}},
	})
	require.NoError(t, err)
	require.Len(t, out2.GetResults(), 1)
	require.Equal(t, machinev1.MachineResponseStatus_MACHINE_RESPONSE_STATUS_REJECTED, out2.GetResults()[0].GetStatus())
	require.Contains(t, out2.GetResults()[0].GetReason(), "duplicate client_event_id")
}

func TestP06_OfflineSync_devMachineCashAndVendReplayDoesNotDoubleDecrement(t *testing.T) {
	pool := machineGRPCTestPool(t)
	ctx := context.Background()

	var qtyRestore int32
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT current_quantity FROM machine_slot_state WHERE machine_id = $1 AND slot_index = 0`,
		testfixtures.DevMachineID).Scan(&qtyRestore))
	t.Cleanup(func() {
		ctx2 := context.Background()
		_, _ = pool.Exec(ctx2, `UPDATE machine_slot_state SET current_quantity = $1 WHERE machine_id = $2 AND slot_index = 0`,
			qtyRestore, testfixtures.DevMachineID)
		_, _ = pool.Exec(ctx2, `DELETE FROM machine_offline_events WHERE machine_id = $1`, testfixtures.DevMachineID)
		_, _ = pool.Exec(ctx2, `DELETE FROM machine_sync_cursor WHERE machine_id = $1 AND stream_name = 'offline'`, testfixtures.DevMachineID)
	})

	_, err := pool.Exec(ctx, `DELETE FROM machine_offline_events WHERE machine_id = $1`, testfixtures.DevMachineID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `DELETE FROM machine_sync_cursor WHERE machine_id = $1 AND stream_name = 'offline'`, testfixtures.DevMachineID)
	require.NoError(t, err)

	deps := offlineSyncIntegrationDeps(t, pool)
	srv := &machineOfflineSyncServer{deps: deps}
	var credVer int64
	require.NoError(t, pool.QueryRow(ctx, `SELECT credential_version FROM machines WHERE id = $1`, testfixtures.DevMachineID).Scan(&credVer))
	claims := plauth.MachineAccessClaims{OrganizationID: testfixtures.DevOrganizationID, MachineID: testfixtures.DevMachineID, CredentialVersion: credVer}
	ctxClaims := plauth.WithMachineAccessClaims(ctx, claims)

	base := "p06-offcash-" + uuid.NewString()
	toStruct := func(msg proto.Message) *structpb.Struct {
		var payloadStruct structpb.Struct
		b, mErr := protojson.Marshal(msg)
		require.NoError(t, mErr)
		require.NoError(t, protojson.Unmarshal(b, &payloadStruct))
		return &payloadStruct
	}

	co := &machinev1.CreateOrderRequest{
		Context: &machinev1.IdempotencyContext{
			IdempotencyKey:  base + ":co",
			ClientEventId:   base + ":ce:co",
			ClientCreatedAt: timestamppb.Now(),
		},
		ProductId: testfixtures.DevProductCola.String(),
		Currency:  "USD",
		Slot:      &machinev1.SlotSelection{SlotIndex: ptrInt32(0)},
	}
	coPayload := toStruct(co)

	push := func(idem, rid string, seq int64, typ string, payload *structpb.Struct) *machinev1.SyncOfflineEventsResponse {
		out, pErr := srv.PushOfflineEvents(ctxClaims, &machinev1.SyncOfflineEventsRequest{
			Meta: &machinev1.MachineRequestMeta{IdempotencyKey: idem, RequestId: rid},
			Events: []*machinev1.OfflineEvent{{
				Meta: &machinev1.MachineRequestMeta{
					OfflineSequence: seq,
					IdempotencyKey:  idem + ":oe",
					RequestId:       rid,
					ClientEventId:   rid + ":cli",
					OccurredAt:      timestamppb.Now(),
				},
				EventType: typ,
				Payload:   payload,
			}},
		})
		require.NoError(t, pErr)
		return out
	}

	r1 := push(base+"sync1", "s1", 1, "commerce.create_order", coPayload)
	require.Equal(t, machinev1.MachineResponseStatus_MACHINE_RESPONSE_STATUS_ACCEPTED, r1.GetResults()[0].GetStatus())

	var orderID string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT id::text FROM orders WHERE machine_id = $1 ORDER BY created_at DESC LIMIT 1`,
		testfixtures.DevMachineID).Scan(&orderID))

	cash := &machinev1.ConfirmCashPaymentRequest{
		Context: &machinev1.IdempotencyContext{
			IdempotencyKey:  base + ":cash",
			ClientEventId:   base + ":ce:cash",
			ClientCreatedAt: timestamppb.Now(),
		},
		OrderId: orderID,
	}
	r2 := push(base+"sync2", "s2", 2, "commerce.confirm_cash_payment", toStruct(cash))
	require.Equal(t, machinev1.MachineResponseStatus_MACHINE_RESPONSE_STATUS_ACCEPTED, r2.GetResults()[0].GetStatus())

	r2b := push(base+"sync2b", "s2b", 2, "commerce.confirm_cash_payment", toStruct(cash))
	require.Len(t, r2b.GetResults(), 1)
	require.Equal(t, machinev1.MachineResponseStatus_MACHINE_RESPONSE_STATUS_REPLAYED, r2b.GetResults()[0].GetStatus())

	vstart := &machinev1.StartVendRequest{
		Context: &machinev1.IdempotencyContext{
			IdempotencyKey:  base + ":vs",
			ClientEventId:   base + ":ce:vs",
			ClientCreatedAt: timestamppb.Now(),
		},
		OrderId:   orderID,
		SlotIndex: 0,
	}
	r3 := push(base+"sync3", "s3", 3, "commerce.start_vend", toStruct(vstart))
	require.Equal(t, machinev1.MachineResponseStatus_MACHINE_RESPONSE_STATUS_ACCEPTED, r3.GetResults()[0].GetStatus())

	var qtyBefore int32
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT current_quantity FROM machine_slot_state WHERE machine_id = $1 AND slot_index = 0`,
		testfixtures.DevMachineID).Scan(&qtyBefore))

	vsucc := &machinev1.ConfirmVendSuccessRequest{
		Context: &machinev1.IdempotencyContext{
			IdempotencyKey:  base + ":vsucc",
			ClientEventId:   base + ":ce:vsucc",
			ClientCreatedAt: timestamppb.Now(),
		},
		OrderId:   orderID,
		SlotIndex: 0,
	}
	r4 := push(base+"sync4", "s4", 4, "commerce.confirm_vend_success", toStruct(vsucc))
	require.Equal(t, machinev1.MachineResponseStatus_MACHINE_RESPONSE_STATUS_ACCEPTED, r4.GetResults()[0].GetStatus())

	var qtyMid int32
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT current_quantity FROM machine_slot_state WHERE machine_id = $1 AND slot_index = 0`,
		testfixtures.DevMachineID).Scan(&qtyMid))
	require.Equal(t, qtyBefore-1, qtyMid)

	r4b := push(base+"sync4b", "s4b", 4, "commerce.confirm_vend_success", toStruct(vsucc))
	require.Equal(t, machinev1.MachineResponseStatus_MACHINE_RESPONSE_STATUS_REPLAYED, r4b.GetResults()[0].GetStatus())

	var qtyAfter int32
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT current_quantity FROM machine_slot_state WHERE machine_id = $1 AND slot_index = 0`,
		testfixtures.DevMachineID).Scan(&qtyAfter))
	require.Equal(t, qtyMid, qtyAfter)
}

func TestP06_OfflineSync_duplicateInventoryAdjustmentDoesNotDoubleApply(t *testing.T) {
	pool := machineGRPCTestPool(t)
	ctx := context.Background()

	var qtyRestore int32
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT current_quantity FROM machine_slot_state WHERE machine_id = $1 AND slot_index = 1`,
		testfixtures.DevMachineID).Scan(&qtyRestore))
	t.Cleanup(func() {
		ctx2 := context.Background()
		_, _ = pool.Exec(ctx2, `UPDATE machine_slot_state SET current_quantity = $1 WHERE machine_id = $2 AND slot_index = 1`,
			qtyRestore, testfixtures.DevMachineID)
		_, _ = pool.Exec(ctx2, `DELETE FROM machine_offline_events WHERE machine_id = $1`, testfixtures.DevMachineID)
		_, _ = pool.Exec(ctx2, `DELETE FROM machine_sync_cursor WHERE machine_id = $1 AND stream_name = 'offline'`, testfixtures.DevMachineID)
	})

	_, err := pool.Exec(ctx, `DELETE FROM machine_offline_events WHERE machine_id = $1`, testfixtures.DevMachineID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `DELETE FROM machine_sync_cursor WHERE machine_id = $1 AND stream_name = 'offline'`, testfixtures.DevMachineID)
	require.NoError(t, err)

	deps := offlineSyncIntegrationDeps(t, pool)
	srv := &machineOfflineSyncServer{deps: deps}
	var credVer int64
	require.NoError(t, pool.QueryRow(ctx, `SELECT credential_version FROM machines WHERE id = $1`, testfixtures.DevMachineID).Scan(&credVer))
	claims := plauth.MachineAccessClaims{OrganizationID: testfixtures.DevOrganizationID, MachineID: testfixtures.DevMachineID, CredentialVersion: credVer}
	ctxClaims := plauth.WithMachineAccessClaims(ctx, claims)

	base := "p06-adj-off-" + uuid.NewString()
	idemKey := base + ":inv"
	qb := qtyRestore
	adj := &machinev1.SubmitInventoryAdjustmentRequest{
		Context: &machinev1.IdempotencyContext{
			IdempotencyKey:  idemKey,
			ClientEventId:   base + ":ce:adj",
			ClientCreatedAt: timestamppb.Now(),
		},
		Reason: "manual_adjustment",
		Lines: []*machinev1.AdjustmentLine{{
			PlanogramId:    testfixtures.DevPlanogramID.String(),
			SlotIndex:      1,
			ProductId:      ptrString(testfixtures.DevProductWater.String()),
			QuantityBefore: qb,
			QuantityAfter:  qb - 1,
		}},
	}
	adjPayload := offlineProtoPayload(t, adj)

	push := func(syncIDem, rid string, seq int64, typ string, payload *structpb.Struct) *machinev1.SyncOfflineEventsResponse {
		out, pErr := srv.PushOfflineEvents(ctxClaims, &machinev1.SyncOfflineEventsRequest{
			Meta: &machinev1.MachineRequestMeta{IdempotencyKey: syncIDem, RequestId: rid},
			Events: []*machinev1.OfflineEvent{{
				Meta: &machinev1.MachineRequestMeta{
					OfflineSequence: seq,
					IdempotencyKey:  syncIDem + ":oe",
					RequestId:       rid,
					ClientEventId:   rid + ":cli",
					OccurredAt:      timestamppb.Now(),
				},
				EventType: typ,
				Payload:   payload,
			}},
		})
		require.NoError(t, pErr)
		return out
	}

	r1 := push(base+"s1", "a1", 1, "inventory.submit_stock_adjustment", adjPayload)
	require.Equal(t, machinev1.MachineResponseStatus_MACHINE_RESPONSE_STATUS_ACCEPTED, r1.GetResults()[0].GetStatus())

	var qty1 int32
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT current_quantity FROM machine_slot_state WHERE machine_id = $1 AND slot_index = 1`,
		testfixtures.DevMachineID).Scan(&qty1))
	require.Equal(t, qb-1, qty1)

	adj2Payload := offlineProtoPayload(t, adj)
	r2 := push(base+"s2", "a2", 2, "inventory.submit_stock_adjustment", adj2Payload)
	require.Equal(t, machinev1.MachineResponseStatus_MACHINE_RESPONSE_STATUS_ACCEPTED, r2.GetResults()[0].GetStatus())

	var qty2 int32
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT current_quantity FROM machine_slot_state WHERE machine_id = $1 AND slot_index = 1`,
		testfixtures.DevMachineID).Scan(&qty2))
	require.Equal(t, qty1, qty2)
}
