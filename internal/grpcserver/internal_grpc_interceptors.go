package grpcserver

import (
	"bytes"
	"context"
	"time"

	"github.com/avf/avf-vending-api/internal/config"
	"github.com/avf/avf-vending-api/internal/observability/grpcprom"
	"github.com/avf/avf-vending-api/internal/platform/auth"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func chainInternalUnaryInterceptors(cfg *config.Config, log *zap.Logger, secrets [][]byte) grpc.ServerOption {
	if cfg == nil || log == nil {
		log = zap.NewNop()
	}
	return grpc.ChainUnaryInterceptor(
		unaryRecoveryInterceptor(log),
		grpcprom.UnaryServerInterceptor(),
		unaryRequestMetaInterceptor(),
		unaryAccessLogInterceptor(log),
		unaryInternalDeadlineInterceptor(cfg),
		unaryInternalServiceTokenAuthInterceptor(cfg, log, secrets),
	)
}

func unaryInternalDeadlineInterceptor(cfg *config.Config) grpc.UnaryServerInterceptor {
	timeout := time.Minute
	if cfg != nil && cfg.InternalGRPC.Enabled && cfg.InternalGRPC.UnaryHandlerTimeout > 0 {
		timeout = cfg.InternalGRPC.UnaryHandlerTimeout
	}
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if _, ok := ctx.Deadline(); ok || timeout <= 0 {
			return handler(ctx, req)
		}
		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		return handler(ctx, req)
	}
}

func unaryInternalServiceTokenAuthInterceptor(cfg *config.Config, log *zap.Logger, secrets [][]byte) grpc.UnaryServerInterceptor {
	if log == nil {
		log = zap.NewNop()
	}
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if isUnauthenticatedInternalGRPCMethod(info.FullMethod) {
			return handler(ctx, req)
		}
		meta, _ := GRPCRequestMetaFromContext(ctx)
		md, _ := metadata.FromIncomingContext(ctx)
		raw := bearerFromMetadata(md)
		if raw == "" {
			log.Warn("internal grpc auth rejected",
				zap.String("grpc_method", info.FullMethod),
				zap.String("request_id", meta.RequestID),
				zap.String("reason", "missing_bearer_token"),
			)
			return nil, status.Error(codes.Unauthenticated, "missing bearer token")
		}
		leeway := time.Minute
		if cfg != nil {
			leeway = cfg.HTTPAuth.JWTLeeway
		}
		principal, err := auth.ValidateInternalServiceAccessJWT(raw, secrets, leeway)
		if err != nil {
			log.Warn("internal grpc auth rejected",
				zap.String("grpc_method", info.FullMethod),
				zap.String("request_id", meta.RequestID),
				zap.Error(err),
			)
			if err == auth.ErrMisconfigured {
				return nil, status.Error(codes.Unavailable, "internal grpc auth misconfigured")
			}
			return nil, status.Error(codes.Unauthenticated, "invalid bearer token")
		}
		return handler(auth.WithPrincipal(ctx, principal), req)
	}
}

func isUnauthenticatedInternalGRPCMethod(fullMethod string) bool {
	switch fullMethod {
	case "/grpc.health.v1.Health/Check", "/grpc.health.v1.Health/Watch":
		return true
	default:
		return false
	}
}

func internalGRPCVerifierSecrets(cfg *config.Config) [][]byte {
	if cfg == nil {
		return nil
	}
	var out [][]byte
	if s := bytes.TrimSpace(cfg.InternalGRPC.ServiceTokenSecret); len(s) > 0 {
		out = append(out, s)
	}
	if len(out) == 0 && (cfg.AppEnv == config.AppEnvDevelopment || cfg.AppEnv == config.AppEnvTest) {
		if s := bytes.TrimSpace(cfg.HTTPAuth.JWTSecret); len(s) > 0 {
			out = append(out, s)
		} else if s := bytes.TrimSpace(cfg.HTTPAuth.LoginJWTSecret); len(s) > 0 {
			out = append(out, s)
		}
	}
	return dedupeSecretBytes(out)
}

func dedupeSecretBytes(in [][]byte) [][]byte {
	seen := make(map[string]struct{})
	var out [][]byte
	for _, b := range in {
		if len(b) == 0 {
			continue
		}
		k := string(b)
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, b)
	}
	return out
}
