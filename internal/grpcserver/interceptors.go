package grpcserver

import (
	"context"
	"fmt"
	"strings"

	"github.com/avf/avf-vending-api/internal/config"
	"github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func newAccessTokenValidator(cfg *config.Config) (auth.AccessTokenValidator, error) {
	if cfg == nil {
		return nil, fmt.Errorf("grpcserver: nil config")
	}
	validator, err := auth.NewAccessTokenValidator(cfg.HTTPAuth)
	if err != nil {
		return nil, err
	}
	if sec := auth.TrimSecret(cfg.HTTPAuth.LoginJWTSecret); len(sec) > 0 {
		secondary := auth.NewHS256AccessTokenValidator(sec, cfg.HTTPAuth.JWTLeeway)
		validator = auth.ChainAccessTokenValidators(validator, secondary)
	}
	return validator, nil
}

func unaryAuthInterceptor(log *zap.Logger, validator auth.AccessTokenValidator) grpc.UnaryServerInterceptor {
	if log == nil {
		log = zap.NewNop()
	}
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if isUnauthenticatedMethod(info.FullMethod) {
			return handler(ctx, req)
		}
		md, _ := metadata.FromIncomingContext(ctx)
		requestID := firstMetadata(md, "x-request-id", "request-id")
		if requestID == "" {
			requestID = uuid.NewString()
		}
		correlationID := firstMetadata(md, "x-correlation-id", "correlation-id")
		if correlationID == "" {
			correlationID = requestID
		}
		_ = grpc.SetHeader(ctx, metadata.Pairs("x-request-id", requestID, "x-correlation-id", correlationID))
		raw := strings.TrimSpace(strings.TrimPrefix(firstMetadata(md, "authorization"), "Bearer "))
		if raw == "" {
			log.Warn("grpc auth rejected",
				zap.String("grpc_method", info.FullMethod),
				zap.String("request_id", requestID),
				zap.String("correlation_id", correlationID),
				zap.String("reason", "missing_bearer_token"),
			)
			return nil, status.Error(codes.Unauthenticated, "missing bearer token")
		}
		principal, err := validator.ValidateAccessToken(ctx, raw)
		if err != nil {
			log.Warn("grpc auth rejected",
				zap.String("grpc_method", info.FullMethod),
				zap.String("request_id", requestID),
				zap.String("correlation_id", correlationID),
				zap.Error(err),
			)
			if err == auth.ErrMisconfigured {
				return nil, status.Error(codes.Unavailable, "gRPC auth misconfigured")
			}
			return nil, status.Error(codes.Unauthenticated, "invalid bearer token")
		}
		return handler(auth.WithPrincipal(ctx, principal), req)
	}
}

func isUnauthenticatedMethod(fullMethod string) bool {
	switch fullMethod {
	case "/grpc.health.v1.Health/Check", "/grpc.health.v1.Health/Watch":
		return true
	default:
		return false
	}
}

func firstMetadata(md metadata.MD, keys ...string) string {
	for _, key := range keys {
		values := md.Get(key)
		for _, value := range values {
			if v := strings.TrimSpace(value); v != "" {
				return v
			}
		}
	}
	return ""
}
