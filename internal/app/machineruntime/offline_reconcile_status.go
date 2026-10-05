package machineruntime

import (
	"context"
	"errors"
	"strings"

	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// OfflineReconcileStatus is a device-facing reconcile view of an offline ledger row.
type OfflineReconcileStatus struct {
	Status    string
	Retryable bool
	EventType string
	Found     bool
}

func LookupOfflineEventReconcileStatus(
	ctx context.Context,
	q *db.Queries,
	machineID uuid.UUID,
	idempotencyKey string,
) (OfflineReconcileStatus, error) {
	key := strings.TrimSpace(idempotencyKey)
	if key == "" {
		return OfflineReconcileStatus{}, nil
	}
	row, err := q.GetMachineOfflineEventByIdempotencyKey(ctx, db.GetMachineOfflineEventByIdempotencyKeyParams{
		MachineID:      machineID,
		IdempotencyKey: key,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return OfflineReconcileStatus{}, nil
	}
	if err != nil {
		return OfflineReconcileStatus{}, err
	}
	st, retry, ok := TelemetryStatusFromOfflineLedger(row.ProcessingStatus)
	if !ok {
		return OfflineReconcileStatus{}, nil
	}
	return OfflineReconcileStatus{
		Status:    st,
		Retryable: retry,
		EventType: row.EventType,
		Found:     true,
	}, nil
}

func TelemetryStatusFromOfflineLedger(processingStatus string) (status string, retryable bool, ok bool) {
	switch strings.TrimSpace(strings.ToLower(processingStatus)) {
	case "processed", "succeeded", "replayed", "duplicate":
		return "processed", false, true
	case "rejected", "failed_terminal":
		return "failed_terminal", false, true
	case "failed", "failed_retryable":
		return "failed_retryable", true, true
	case "pending", "processing":
		return "processing", true, true
	default:
		if strings.TrimSpace(processingStatus) == "" {
			return "", false, false
		}
		return "processing", true, true
	}
}
