package machineidempotency

import (
	"context"
	"strings"

	"github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/avf/avf-vending-api/internal/platform/observability/productionmetrics"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

// InterceptorConfig selects which RPCs participate and how empty responses are allocated for replay decoding.
type InterceptorConfig struct {
	IsIdempotentMutation func(fullMethod string) bool
	NewMutationResponse  func(canonicalOperation string) proto.Message
	// CanonicalOperation maps an RPC full method name to the idempotency ledger operation key.
	// When nil or returns "", info.FullMethod is used. Alias RPCs should map to their primary method
	// so the same idempotency_key dedupes across entrypoint names.
	CanonicalOperation func(fullMethod string) string
	// TraceID optionally supplies a stable trace/request id for persistence (e.g. from grpcserver.GRPCRequestMetaFromContext).
	TraceID func(context.Context) string
	// DecorateLedgerReplayResponse, if set, runs on protobuf responses decoded from a successful ledger replay
	// before returning them to the client (e.g. force replay=true on outer envelopes).
	DecorateLedgerReplayResponse func(proto.Message)
}

// UnaryServerInterceptor records idempotency metadata before mutations and snapshots responses after success.
func UnaryServerInterceptor(ledger *Ledger, cfg InterceptorConfig) grpc.UnaryServerInterceptor {
	if ledger == nil || cfg.IsIdempotentMutation == nil || cfg.NewMutationResponse == nil {
		return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
			return handler(ctx, req)
		}
	}
	trace := cfg.TraceID
	if trace == nil {
		trace = func(context.Context) string { return "" }
	}
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if !cfg.IsIdempotentMutation(info.FullMethod) {
			return handler(ctx, req)
		}
		claims, ok := auth.MachineAccessClaimsFromContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "missing machine credentials")
		}
		pm, ok := req.(proto.Message)
		if !ok || pm == nil {
			return nil, status.Error(codes.InvalidArgument, "invalid machine mutation request")
		}
		key, err := MutationIdempotencyKey(pm)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, strings.TrimPrefix(err.Error(), "machineidempotency: "))
		}
		hash, err := HashMutationRequest(pm)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "request hash failed")
		}
		ledgerOp := info.FullMethod
		if cfg.CanonicalOperation != nil {
			if c := strings.TrimSpace(cfg.CanonicalOperation(info.FullMethod)); c != "" {
				ledgerOp = c
			}
		}
		begin, replay, err := ledger.BeginMutation(ctx, claims, ledgerOp, key, hash, trace(ctx), cfg.NewMutationResponse)
		if err != nil {
			if shouldRecordGRPCIdempotencyConflict(err) {
				productionmetrics.RecordGRPCIdempotencyConflict()
			}
			return nil, err
		}
		if replay != nil {
			productionmetrics.RecordGRPCIdempotencyReplay()
			if cfg.DecorateLedgerReplayResponse != nil {
				cfg.DecorateLedgerReplayResponse(replay)
			}
			return replay, nil
		}
		resp, err := handler(ctx, req)
		if err != nil {
			if shouldMarkLedgerRowFailed(err) {
				_ = ledger.MarkFailed(ctx, claims, ledgerOp, key, trace(ctx))
			}
			return resp, err
		}
		rpm, ok := resp.(proto.Message)
		if !ok || rpm == nil {
			_ = ledger.MarkFailed(ctx, claims, ledgerOp, key, trace(ctx))
			return nil, status.Error(codes.Internal, "machine mutation response is not protobuf")
		}
		if err := ledger.MarkSucceeded(ctx, claims, begin.Operation, key, rpm, trace(ctx)); err != nil {
			return nil, err
		}
		return resp, nil
	}
}

func shouldRecordGRPCIdempotencyConflict(err error) bool {
	if err == nil {
		return false
	}
	code := status.Code(err)
	msg := strings.ToLower(status.Convert(err).Message())
	switch code {
	case codes.FailedPrecondition:
		return strings.Contains(msg, ErrMsgIdempotencyPayloadMismatch)
	case codes.AlreadyExists:
		return true
	default:
		return false
	}
}

func shouldMarkLedgerRowFailed(handlerErr error) bool {
	code := status.Code(handlerErr)
	switch code {
	case codes.Canceled,
		codes.Unknown,
		codes.DeadlineExceeded,
		codes.Aborted,
		codes.ResourceExhausted,
		codes.Internal,
		codes.Unavailable:
		return false
	default:
		return true
	}
}
