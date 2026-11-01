package grpcserver

import (
	"context"
	"log/slog"
	"strings"
	"time"

	appcommerce "github.com/avf/avf-vending-api/internal/app/commerce"
	plauth "github.com/avf/avf-vending-api/internal/platform/auth"
	machinev1 "github.com/avf/avf-vending-api/proto/avf/machine/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (s *machineOfflineSyncServer) replayOfflineSale(ctx context.Context, payload []byte, meta *machinev1.MachineRequestMeta) error {
	if meta == nil {
		return status.Error(codes.InvalidArgument, "offline event meta required")
	}
	claims, ok := plauth.MachineAccessClaimsFromContext(ctx)
	if !ok {
		return status.Error(codes.Unauthenticated, "missing machine credentials")
	}
	if s.deps.Commerce == nil {
		return status.Error(codes.Unavailable, "commerce not configured")
	}
	if s.deps.Config == nil {
		return status.Error(codes.Unavailable, "config not configured")
	}
	topic := strings.TrimSpace(s.deps.Config.Commerce.PaymentOutboxTopic)
	evType := strings.TrimSpace(s.deps.Config.Commerce.PaymentOutboxEventType)
	aggType := strings.TrimSpace(s.deps.Config.Commerce.PaymentOutboxAggregateType)
	if topic == "" || evType == "" || aggType == "" {
		return status.Error(codes.Unavailable, "commerce outbox not configured")
	}
	svc, ok := s.deps.Commerce.(*appcommerce.Service)
	if !ok {
		return status.Error(codes.Unavailable, "offline_sale requires commerce service")
	}
	key := strings.TrimSpace(meta.GetIdempotencyKey())
	clientEventID := strings.TrimSpace(meta.GetClientEventId())
	var occurredAt time.Time
	if ts := meta.GetOccurredAt(); ts != nil && ts.IsValid() {
		occurredAt = ts.AsTime().UTC()
	}
	err := svc.ProcessOfflineSale(
		ctx,
		claims.MachineID,
		key,
		clientEventID,
		payload,
		appcommerce.OfflineSaleOutboxConfig{
			Topic:         topic,
			EventType:     evType,
			AggregateType: aggType,
		},
		occurredAt,
	)
	if err != nil {
		slog.Warn("offline sale dispatch failed",
			"idempotencyKey", key,
			"clientEventId", clientEventID,
			"err", err.Error(),
		)
		return mapCommerceGRPCErr(err)
	}
	slog.Info("offline sale dispatch ok",
		"idempotencyKey", key,
		"clientEventId", clientEventID,
		"processingStatus", "processed",
	)
	return nil
}
