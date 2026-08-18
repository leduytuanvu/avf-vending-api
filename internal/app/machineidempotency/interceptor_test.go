package machineidempotency

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestShouldMarkLedgerRowFailed_DeadlineAndCancel(t *testing.T) {
	if !shouldMarkLedgerRowFailed(status.Error(codes.DeadlineExceeded, "deadline")) {
		t.Fatal("expected DeadlineExceeded to mark ledger failed")
	}
	if !shouldMarkLedgerRowFailed(status.Error(codes.Canceled, "canceled")) {
		t.Fatal("expected Canceled to mark ledger failed")
	}
	if shouldMarkLedgerRowFailed(status.Error(codes.Unavailable, "unavailable")) {
		t.Fatal("expected Unavailable to keep in_progress for retry")
	}
	if !shouldMarkLedgerRowFailed(status.Error(codes.InvalidArgument, "bad")) {
		t.Fatal("expected InvalidArgument to mark ledger failed")
	}
	if shouldMarkLedgerRowFailed(nil) {
		t.Fatal("nil error should not mark failed")
	}
	if shouldMarkLedgerRowFailed(errors.New("plain")) {
		t.Fatal("plain error defaults to not mark failed via unknown code")
	}
}

func TestErrMsgIdempotencyFinalizeFailed(t *testing.T) {
	require.Equal(t, "machine_idempotency_finalize_failed", ErrMsgIdempotencyFinalizeFailed)
	require.NotEqual(t, "order_created_idempotency_finalize_failed", ErrMsgIdempotencyFinalizeFailed)
	st := status.Error(codes.FailedPrecondition, ErrMsgIdempotencyFinalizeFailed)
	require.Equal(t, codes.FailedPrecondition, status.Code(st))
	require.Equal(t, ErrMsgIdempotencyFinalizeFailed, status.Convert(st).Message())
}

func TestIdempotencyKeyFingerprintStableAndTruncated(t *testing.T) {
	fp := idempotencyKeyFingerprint("inventory_ack:machine:cursor")
	require.Len(t, fp, 12)
	require.Equal(t, fp, idempotencyKeyFingerprint("inventory_ack:machine:cursor"))
	require.NotEqual(t, fp, idempotencyKeyFingerprint("other"))
}
