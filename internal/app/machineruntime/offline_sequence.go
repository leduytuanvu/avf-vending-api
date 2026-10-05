package machineruntime

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Offline sync uses a monotonic offline_sequence cursor per machine+stream stored in Postgres
// (see machine_sync_cursors). Incoming batches sort by sequence; per-event processing updates the cursor only
// on success.
//
// When GapTolerant is enabled (default), offline_sequence may skip ahead of last_sequence+1; missing slots
// are not required to be filled. When disabled, a gap still fails the batch with Aborted OfflineSequenceOutOfOrder.
//
// Duplicate or already-synced (seq ≤ last_sequence): replayed idempotently with REPLAYED without re-dispatch.

// OfflineSequenceOutOfOrder signals the client's offline_sequence stream skipped ahead or duplicated.
// Servers must use a retryable gRPC code so kiosks can rewind/resend batches deterministically.
func OfflineSequenceOutOfOrder(expected, got int64) error {
	return status.Errorf(codes.Aborted, "offline sequence out of order: expected %d got %d", expected, got)
}
