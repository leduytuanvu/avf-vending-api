package grpcserver

import (
	"context"

	"github.com/avf/avf-vending-api/internal/app/machineruntime"
	"github.com/avf/avf-vending-api/internal/gen/db"
	machinev1 "github.com/avf/avf-vending-api/proto/avf/machine/v1"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func telemetryStatusForKeyOfflineFallback(
	ctx context.Context,
	q *db.Queries,
	machineID uuid.UUID,
	key string,
) (*machinev1.TelemetryEventStatus, error) {
	offline, err := machineruntime.LookupOfflineEventReconcileStatus(ctx, q, machineID, key)
	if err != nil {
		return nil, status.Error(codes.Internal, "offline event status lookup failed")
	}
	if !offline.Found {
		return &machinev1.TelemetryEventStatus{IdempotencyKey: key, Status: "not_found", Retryable: true}, nil
	}
	return &machinev1.TelemetryEventStatus{
		IdempotencyKey: key,
		Status:         offline.Status,
		Retryable:      offline.Retryable,
		EventType:      offline.EventType,
	}, nil
}
