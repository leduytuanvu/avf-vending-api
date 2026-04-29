package grpcserver

import (
	"bytes"
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/avf/avf-vending-api/internal/app/machineidempotency"
	plauth "github.com/avf/avf-vending-api/internal/platform/auth"
	machinev1 "github.com/avf/avf-vending-api/proto/avf/machine/v1"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestMachineReplayLedger_ReplayAndConflict(t *testing.T) {
	t.Parallel()

	pool := machineGRPCTestPool(t)
	ctx := context.Background()
	orgID := uuid.New()
	siteID := uuid.New()
	machineID := uuid.New()
	require.NoError(t, insertMachineReplayLedgerFixture(ctx, pool, orgID, siteID, machineID))

	ledger := NewMachineReplayLedger(pool, nil)
	claims := plauth.MachineAccessClaims{OrganizationID: orgID, MachineID: machineID, CredentialVersion: 1}
	machineIDStr := machineID.String()
	req := &machinev1.CreateOrderRequest{
		Context: &machinev1.IdempotencyContext{
			IdempotencyKey:  "idem-1",
			ClientEventId:   "event-1",
			ClientCreatedAt: timestamppb.New(time.Now().UTC()),
		},
		MachineId: &machineIDStr,
		ProductId: uuid.NewString(),
		Currency:  "USD",
	}
	hash, err := machineidempotency.HashMutationRequest(req)
	require.NoError(t, err)

	row, replay, err := ledger.BeginMutation(ctx, claims, machinev1.MachineCommerceService_CreateOrder_FullMethodName, "idem-1", hash, "trace-1", func(string) proto.Message {
		return &machinev1.CreateOrderResponse{}
	})
	require.NoError(t, err)
	require.True(t, row.Inserted)
	require.Nil(t, replay)
	require.NoError(t, ledger.MarkSucceeded(ctx, claims, row.Operation, "idem-1", &machinev1.CreateOrderResponse{OrderId: "order-1"}, "trace-1"))

	_, replay, err = ledger.BeginMutation(ctx, claims, machinev1.MachineCommerceService_CreateOrder_FullMethodName, "idem-1", hash, "trace-2", func(string) proto.Message {
		return &machinev1.CreateOrderResponse{}
	})
	require.NoError(t, err)
	out, ok := replay.(*machinev1.CreateOrderResponse)
	require.True(t, ok)
	require.Equal(t, "order-1", out.GetOrderId())

	req.Currency = "EUR"
	otherHash, err := machineidempotency.HashMutationRequest(req)
	require.NoError(t, err)
	require.False(t, bytes.Equal(hash, otherHash))
	_, _, err = ledger.BeginMutation(ctx, claims, machinev1.MachineCommerceService_CreateOrder_FullMethodName, "idem-1", otherHash, "trace-3", func(string) proto.Message {
		return &machinev1.CreateOrderResponse{}
	})
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
	require.Contains(t, status.Convert(err).Message(), machineidempotency.ErrMsgIdempotencyPayloadMismatch)
}

func TestMachineReplayLedger_ConcurrentReplayAfterSuccess(t *testing.T) {
	t.Parallel()

	pool := machineGRPCTestPool(t)
	ctx := context.Background()
	orgID := uuid.New()
	siteID := uuid.New()
	machineID := uuid.New()
	require.NoError(t, insertMachineReplayLedgerFixture(ctx, pool, orgID, siteID, machineID))

	ledger := NewMachineReplayLedger(pool, nil)
	claims := plauth.MachineAccessClaims{OrganizationID: orgID, MachineID: machineID, CredentialVersion: 1}
	machineIDStr := machineID.String()
	req := &machinev1.CreateOrderRequest{
		Context: &machinev1.IdempotencyContext{
			IdempotencyKey:  "idem-cc",
			ClientEventId:   "event-cc",
			ClientCreatedAt: timestamppb.New(time.Now().UTC()),
		},
		MachineId: &machineIDStr,
		ProductId: uuid.NewString(),
		Currency:  "USD",
	}
	hash, err := machineidempotency.HashMutationRequest(req)
	require.NoError(t, err)

	row, replay, err := ledger.BeginMutation(ctx, claims, machinev1.MachineCommerceService_CreateOrder_FullMethodName, "idem-cc", hash, "trace-a", func(string) proto.Message {
		return &machinev1.CreateOrderResponse{}
	})
	require.NoError(t, err)
	require.Nil(t, replay)
	require.True(t, row.Inserted)
	require.NoError(t, ledger.MarkSucceeded(ctx, claims, row.Operation, "idem-cc", &machinev1.CreateOrderResponse{OrderId: "order-cc"}, "trace-a"))

	const n = 16
	errs := make(chan error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, r, err := ledger.BeginMutation(ctx, claims, machinev1.MachineCommerceService_CreateOrder_FullMethodName, "idem-cc", hash, fmt.Sprintf("trace-%d", i), func(string) proto.Message {
				return &machinev1.CreateOrderResponse{}
			})
			if err != nil {
				errs <- err
				return
			}
			if r == nil {
				errs <- fmt.Errorf("expected replay")
				return
			}
			out, ok := r.(*machinev1.CreateOrderResponse)
			if !ok || out.GetOrderId() != "order-cc" {
				errs <- fmt.Errorf("unexpected replay response")
				return
			}
			errs <- nil
		}(i)
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		require.NoError(t, e)
	}
}

func TestMachineOfflineSync_OutOfOrderRejected(t *testing.T) {
	t.Parallel()

	pool := machineGRPCTestPool(t)
	ctx := context.Background()
	orgID := uuid.New()
	siteID := uuid.New()
	machineID := uuid.New()
	require.NoError(t, insertMachineReplayLedgerFixture(ctx, pool, orgID, siteID, machineID))

	claims := plauth.MachineAccessClaims{OrganizationID: orgID, MachineID: machineID, CredentialVersion: 1}
	ctx = plauth.WithMachineAccessClaims(ctx, claims)
	_, err := (&machineOfflineSyncServer{deps: MachineGRPCServicesDeps{Pool: pool}}).PushOfflineEvents(ctx, &machinev1.SyncOfflineEventsRequest{
		Meta: &machinev1.MachineRequestMeta{IdempotencyKey: "sync-1", RequestId: "sync-req"},
		Events: []*machinev1.OfflineEvent{{
			Meta:      &machinev1.MachineRequestMeta{OfflineSequence: 2, IdempotencyKey: "event-2", RequestId: "event-2", OccurredAt: timestamppb.New(time.Now().UTC())},
			EventType: "telemetry.batch",
		}},
	})
	require.Equal(t, codes.Aborted, status.Code(err))
}

func insertMachineReplayLedgerFixture(ctx context.Context, pool *pgxpool.Pool, orgID, siteID, machineID uuid.UUID) error {
	if _, err := pool.Exec(ctx, `INSERT INTO organizations (id, name, slug, status) VALUES ($1, 'replay-ledger', $2, 'active')`, orgID, "replay-ledger-"+orgID.String()); err != nil {
		return err
	}
	if _, err := pool.Exec(ctx, `INSERT INTO sites (id, organization_id, name, code, status) VALUES ($1, $2, 's', '', 'active')`, siteID, orgID); err != nil {
		return err
	}
	_, err := pool.Exec(ctx, `INSERT INTO machines (id, organization_id, site_id, serial_number, status, credential_version) VALUES ($1, $2, $3, $4, 'online', 1)`, machineID, orgID, siteID, "sn-replay-"+machineID.String())
	return err
}
