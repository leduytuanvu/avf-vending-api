package machineidempotency

import (
	"testing"

	machinev1 "github.com/avf/avf-vending-api/proto/avf/machine/v1"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestHashMutationRequest_StripsVolatileRequestIDs(t *testing.T) {
	t.Parallel()
	mid := "00000000-0000-0000-0000-000000000099"
	req := &machinev1.ReportInventoryDeltaRequest{
		Context: &machinev1.IdempotencyContext{
			IdempotencyKey:  "idem-volatile",
			ClientEventId:   "evt-1",
			ClientCreatedAt: timestamppb.Now(),
		},
		Meta: &machinev1.MachineRequestMeta{
			RequestId: "trace-A",
			MachineId: mid,
		},
		Reason: "test_reason",
	}
	cl := proto.Clone(req).(*machinev1.ReportInventoryDeltaRequest)
	cl.Meta.RequestId = "trace-B"

	a, err := HashMutationRequest(req)
	require.NoError(t, err)
	b, err := HashMutationRequest(cl)
	require.NoError(t, err)
	require.Equal(t, a, b)

	ap, err := StableProtoHash(req)
	require.NoError(t, err)
	bp, err := StableProtoHash(cl)
	require.NoError(t, err)
	require.NotEqual(t, ap, bp)
}

func TestMutationIdempotencyKey_ReconcileEventsSynthetic(t *testing.T) {
	t.Parallel()
	req := &machinev1.ReconcileEventsRequest{IdempotencyKeys: []string{"b", "a"}}
	got, err := MutationIdempotencyKey(req)
	require.NoError(t, err)
	req2 := &machinev1.ReconcileEventsRequest{IdempotencyKeys: []string{"a", "b"}}
	got2, err := MutationIdempotencyKey(req2)
	require.NoError(t, err)
	require.Equal(t, got, got2)
}

func TestStableProtoHash_deterministic(t *testing.T) {
	t.Parallel()
	req := &machinev1.CreateOrderRequest{
		Context: &machinev1.IdempotencyContext{
			IdempotencyKey:  "k",
			ClientEventId:   "e1",
			ClientCreatedAt: timestamppb.Now(),
		},
		MachineId: ptrString("00000000-0000-0000-0000-000000000099"),
		ProductId: "550e8400-e29b-41d4-a716-446655440000",
		Currency:  "USD",
	}
	a, err := StableProtoHash(req)
	require.NoError(t, err)
	b, err := StableProtoHash(req)
	require.NoError(t, err)
	require.Equal(t, a, b)

	cp := proto.Clone(req).(*machinev1.CreateOrderRequest)
	cp.Currency = "EUR"
	c, err := StableProtoHash(cp)
	require.NoError(t, err)
	require.NotEqual(t, a, c)
}

func ptrString(s string) *string { return &s }
