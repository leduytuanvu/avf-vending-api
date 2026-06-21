package machineidempotency

import (
	"errors"
	"testing"

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
