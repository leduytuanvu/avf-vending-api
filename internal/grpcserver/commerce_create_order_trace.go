package grpcserver

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"strings"
	"time"

	"go.uber.org/zap"
)

const createOrderMethodDeadline = 15 * time.Second

func createOrderTraceEnabled() bool {
	return strings.TrimSpace(os.Getenv("COMMERCE_CREATE_ORDER_TRACE")) == "1"
}

func idempotencyKeyHash(key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:4])
}

func slotIdxValue(slotIdx *int32) int32 {
	if slotIdx == nil {
		return 0
	}
	return *slotIdx
}

type createOrderTrace struct {
	log *zap.Logger
}

func newCreateOrderTrace(log *zap.Logger) *createOrderTrace {
	if log == nil {
		log = zap.NewNop()
	}
	return &createOrderTrace{log: log}
}

func (t *createOrderTrace) checkpoint(ctx context.Context, step string, fields ...zap.Field) {
	if t == nil || !createOrderTraceEnabled() {
		return
	}
	meta, _ := GRPCRequestMetaFromContext(ctx)
	base := []zap.Field{
		zap.String("checkpoint", step),
		zap.String("rpc", "CreateOrder"),
	}
	if meta.RequestID != "" {
		base = append(base, zap.String("request_id", meta.RequestID))
	}
	if meta.CorrelationID != "" {
		base = append(base, zap.String("correlation_id", meta.CorrelationID))
	}
	base = append(base, fields...)
	t.log.Info("commerce_create_order_trace", base...)
}

func withCreateOrderDeadline(ctx context.Context) (context.Context, context.CancelFunc) {
	if _, ok := ctx.Deadline(); !ok {
		return context.WithTimeout(ctx, createOrderMethodDeadline)
	}
	deadline, ok := ctx.Deadline()
	if !ok {
		return context.WithTimeout(ctx, createOrderMethodDeadline)
	}
	remaining := time.Until(deadline)
	if remaining > createOrderMethodDeadline {
		return context.WithTimeout(ctx, createOrderMethodDeadline)
	}
	return ctx, func() {}
}
