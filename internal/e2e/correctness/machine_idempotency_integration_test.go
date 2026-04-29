package correctness

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/avf/avf-vending-api/internal/app/machineidempotency"
	"github.com/avf/avf-vending-api/internal/gen/db"
	plauth "github.com/avf/avf-vending-api/internal/platform/auth"
	machinev1 "github.com/avf/avf-vending-api/proto/avf/machine/v1"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// TestP06_E2E_MachineIdempotencyLedger_* exercises PostgreSQL-backed mutation idempotency (same payload replay,
// conflicting payload hash, concurrent in-flight callers). Mirrors production unary interceptor ledger semantics.

func TestP06_E2E_MachineIdempotencyLedger_sameKeySamePayloadReturnsReplay(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	orgID := uuid.New()
	siteID := uuid.New()
	machineID := uuid.New()
	insertOrganizationSiteMachine(t, ctx, pool, orgID, siteID, machineID, "online", 1)

	ledger := machineidempotency.NewLedger(pool, nil)
	claims := plauth.MachineAccessClaims{OrganizationID: orgID, MachineID: machineID, CredentialVersion: 1}
	machineIDStr := machineID.String()
	req := &machinev1.CreateOrderRequest{
		Context: &machinev1.IdempotencyContext{
			IdempotencyKey:  "p06-idem-replay",
			ClientEventId:   "evt-replay",
			ClientCreatedAt: timestamppb.New(time.Now().UTC()),
		},
		MachineId: &machineIDStr,
		ProductId: uuid.NewString(),
		Currency:  "USD",
	}
	hash, err := machineidempotency.HashMutationRequest(req)
	require.NoError(t, err)

	op := machinev1.MachineCommerceService_CreateOrder_FullMethodName
	row, replay, err := ledger.BeginMutation(ctx, claims, op, "p06-idem-replay", hash, "trace-1", func(string) proto.Message {
		return &machinev1.CreateOrderResponse{}
	})
	require.NoError(t, err)
	require.Nil(t, replay)
	require.True(t, row.Inserted)

	resp := &machinev1.CreateOrderResponse{OrderId: "order-p06-replay"}
	require.NoError(t, ledger.MarkSucceeded(ctx, claims, op, "p06-idem-replay", resp, "trace-1"))

	row2, replay2, err := ledger.BeginMutation(ctx, claims, op, "p06-idem-replay", hash, "trace-2", func(string) proto.Message {
		return &machinev1.CreateOrderResponse{}
	})
	require.NoError(t, err)
	require.False(t, row2.Inserted)
	out, ok := replay2.(*machinev1.CreateOrderResponse)
	require.True(t, ok)
	require.Equal(t, "order-p06-replay", out.GetOrderId())
}

func TestP06_E2E_MachineIdempotencyLedger_sameKeyDifferentPayloadConflict(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	orgID := uuid.New()
	siteID := uuid.New()
	machineID := uuid.New()
	insertOrganizationSiteMachine(t, ctx, pool, orgID, siteID, machineID, "online", 1)

	ledger := machineidempotency.NewLedger(pool, nil)
	claims := plauth.MachineAccessClaims{OrganizationID: orgID, MachineID: machineID, CredentialVersion: 1}
	machineIDStr := machineID.String()
	req := &machinev1.CreateOrderRequest{
		Context: &machinev1.IdempotencyContext{
			IdempotencyKey:  "p06-idem-conflict",
			ClientEventId:   "evt-c",
			ClientCreatedAt: timestamppb.New(time.Now().UTC()),
		},
		MachineId: &machineIDStr,
		ProductId: uuid.NewString(),
		Currency:  "USD",
	}
	hash, err := machineidempotency.HashMutationRequest(req)
	require.NoError(t, err)
	op := machinev1.MachineCommerceService_CreateOrder_FullMethodName
	row, replay, err := ledger.BeginMutation(ctx, claims, op, "p06-idem-conflict", hash, "trace-a", func(string) proto.Message {
		return &machinev1.CreateOrderResponse{}
	})
	require.NoError(t, err)
	require.Nil(t, replay)
	require.True(t, row.Inserted)
	require.NoError(t, ledger.MarkSucceeded(ctx, claims, op, "p06-idem-conflict", &machinev1.CreateOrderResponse{OrderId: "o1"}, "trace-a"))

	req.Currency = "EUR"
	otherHash, err := machineidempotency.HashMutationRequest(req)
	require.NoError(t, err)
	require.False(t, bytes.Equal(hash, otherHash))
	_, _, err = ledger.BeginMutation(ctx, claims, op, "p06-idem-conflict", otherHash, "trace-b", func(string) proto.Message {
		return &machinev1.CreateOrderResponse{}
	})
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
	require.Contains(t, status.Convert(err).Message(), machineidempotency.ErrMsgIdempotencyPayloadMismatch)
}

func TestP06_E2E_MachineIdempotencyLedger_deleteStaleRowAllowsFreshInsert(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	orgID := uuid.New()
	siteID := uuid.New()
	machineID := uuid.New()
	insertOrganizationSiteMachine(t, ctx, pool, orgID, siteID, machineID, "online", 1)

	ledger := machineidempotency.NewLedger(pool, nil)
	q := db.New(pool)
	claims := plauth.MachineAccessClaims{OrganizationID: orgID, MachineID: machineID, CredentialVersion: 1}
	machineIDStr := machineID.String()
	req := &machinev1.CreateOrderRequest{
		Context: &machinev1.IdempotencyContext{
			IdempotencyKey:  "p06-stale",
			ClientEventId:   "evt-stale",
			ClientCreatedAt: timestamppb.New(time.Now().UTC()),
		},
		MachineId: &machineIDStr,
		ProductId: uuid.NewString(),
		Currency:  "USD",
	}
	hash, err := machineidempotency.HashMutationRequest(req)
	require.NoError(t, err)
	op := machinev1.MachineCommerceService_CreateOrder_FullMethodName
	row, replay, err := ledger.BeginMutation(ctx, claims, op, "p06-stale", hash, "t1", func(string) proto.Message {
		return &machinev1.CreateOrderResponse{}
	})
	require.NoError(t, err)
	require.Nil(t, replay)
	require.True(t, row.Inserted)

	_, err = pool.Exec(ctx, `
UPDATE machine_idempotency_keys
SET last_seen_at = now() - interval '3 hours'
WHERE organization_id = $1 AND machine_id = $2 AND operation = $3 AND idempotency_key = $4`,
		orgID, machineID, op, "p06-stale")
	require.NoError(t, err)

	cutoff := time.Now().UTC().Add(-time.Hour)
	require.NoError(t, q.DeleteStaleMachineIdempotencyInProgress(ctx, db.DeleteStaleMachineIdempotencyInProgressParams{
		OrganizationID: orgID,
		MachineID:      machineID,
		Operation:      op,
		IdempotencyKey: "p06-stale",
		LastSeenAt:     cutoff,
	}))

	row2, replay2, err := ledger.BeginMutation(ctx, claims, op, "p06-stale", hash, "t2", func(string) proto.Message {
		return &machinev1.CreateOrderResponse{}
	})
	require.NoError(t, err)
	require.Nil(t, replay2)
	require.True(t, row2.Inserted)
}

func TestP06_E2E_MachineIdempotencyLedger_secondCallerWhileInProgressAborts(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	orgID := uuid.New()
	siteID := uuid.New()
	machineID := uuid.New()
	insertOrganizationSiteMachine(t, ctx, pool, orgID, siteID, machineID, "online", 1)

	ledger := machineidempotency.NewLedger(pool, nil)
	claims := plauth.MachineAccessClaims{OrganizationID: orgID, MachineID: machineID, CredentialVersion: 1}
	machineIDStr := machineID.String()
	req := &machinev1.CreateOrderRequest{
		Context: &machinev1.IdempotencyContext{
			IdempotencyKey:  "p06-inflight",
			ClientEventId:   "evt-inflight",
			ClientCreatedAt: timestamppb.New(time.Now().UTC()),
		},
		MachineId: &machineIDStr,
		ProductId: uuid.NewString(),
		Currency:  "USD",
	}
	hash, err := machineidempotency.HashMutationRequest(req)
	require.NoError(t, err)
	op := machinev1.MachineCommerceService_CreateOrder_FullMethodName

	row, replay, err := ledger.BeginMutation(ctx, claims, op, "p06-inflight", hash, "trace-a", func(string) proto.Message {
		return &machinev1.CreateOrderResponse{}
	})
	require.NoError(t, err)
	require.Nil(t, replay)
	require.True(t, row.Inserted)
	// Leave row in in_progress (no MarkSucceeded).

	_, _, err = ledger.BeginMutation(ctx, claims, op, "p06-inflight", hash, "trace-b", func(string) proto.Message {
		return &machinev1.CreateOrderResponse{}
	})
	require.Equal(t, codes.Aborted, status.Code(err))
}
