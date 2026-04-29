package grpcserver

import (
	"context"
	"testing"
	"time"

	machinev1 "github.com/avf/avf-vending-api/proto/avf/machine/v1"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestParseMachineMutationContext_RequiresFields(t *testing.T) {
	t.Parallel()
	_, err := parseMachineMutationContext(context.Background(), nil)
	require.Equal(t, codes.InvalidArgument, status.Code(err))

	_, err = parseMachineMutationContext(context.Background(), &machinev1.IdempotencyContext{})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestParseMachineMutationContext_MetadataBodyIdempotencyMismatch(t *testing.T) {
	t.Parallel()
	md := metadata.Pairs("x-idempotency-key", "a")
	ctx := metadata.NewOutgoingContext(context.Background(), md)
	incoming := metadata.MD{}
	for k, v := range md {
		incoming[k] = v
	}
	ctx = metadata.NewIncomingContext(ctx, incoming)

	_, err := parseMachineMutationContext(ctx, &machinev1.IdempotencyContext{
		IdempotencyKey:  "b",
		ClientEventId:   "e1",
		ClientCreatedAt: timestamppb.New(time.Now().UTC()),
	})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestParseMachineMutationContext_MetadataSuppliesKey(t *testing.T) {
	t.Parallel()
	md := metadata.Pairs("idempotency-key", "k-meta")
	ctx := metadata.NewIncomingContext(context.Background(), md)
	out, err := parseMachineMutationContext(ctx, &machinev1.IdempotencyContext{
		ClientEventId:   "e1",
		ClientCreatedAt: timestamppb.New(time.Now().UTC()),
	})
	require.NoError(t, err)
	require.Equal(t, "k-meta", out.IdempotencyKey)
	require.Equal(t, "e1", out.ClientEventID)
}
