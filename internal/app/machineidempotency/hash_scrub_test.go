package machineidempotency

import (
	"testing"
	"time"

	machinev1 "github.com/avf/avf-vending-api/proto/avf/machine/v1"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestHashMutationRequest_StripsPricingSnapshotCapturedAt(t *testing.T) {
	t.Parallel()
	base := &machinev1.CreateQuoteRequest{
		Context: &machinev1.IdempotencyContext{
			IdempotencyKey:  "quote-1",
			ClientEventId:   "evt-1",
			ClientCreatedAt: timestamppb.Now(),
		},
		MachineId: ptrString("00000000-0000-0000-0000-000000000099"),
		Currency:  "VND",
		PricingSnapshot: &machinev1.MachinePricingSnapshot{
			TotalMinor: 15000,
			CapturedAt: timestamppb.New(time.Unix(1, 0)),
		},
	}
	other := proto.Clone(base).(*machinev1.CreateQuoteRequest)
	other.PricingSnapshot.CapturedAt = timestamppb.New(time.Unix(99, 0))
	other.Context.ClientCreatedAt = timestamppb.New(time.Unix(99, 0))
	other.Context.ClientEventId = "evt-2"

	a, err := HashMutationRequest(base)
	require.NoError(t, err)
	b, err := HashMutationRequest(other)
	require.NoError(t, err)
	require.Equal(t, a, b)
}
