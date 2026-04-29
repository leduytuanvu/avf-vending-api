package grpcprom

import (
	"context"
	"strings"
	"time"

	"github.com/avf/avf-vending-api/internal/platform/observability/productionmetrics"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

// UnaryServerInterceptor records Prometheus metrics for unary RPCs (no request bodies logged).
func UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		svc, method := splitFullMethod(info.FullMethod)
		start := time.Now()
		resp, err := handler(ctx, req)
		code := status.Code(err)
		codeStr := code.String()
		d := time.Since(start)
		productionmetrics.RecordGRPCUnary(svc, method, codeStr, d)
		return resp, err
	}
}

func splitFullMethod(full string) (service, method string) {
	f := strings.TrimSpace(full)
	if f == "" {
		return "unknown", "unknown"
	}
	f = strings.TrimPrefix(f, "/")
	idx := strings.LastIndex(f, "/")
	if idx <= 0 || idx == len(f)-1 {
		return "unknown", "unknown"
	}
	return f[:idx], f[idx+1:]
}
