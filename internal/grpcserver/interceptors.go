package grpcserver

import (
	"context"
	"fmt"
	"runtime/debug"
	"strings"
	"time"

	"github.com/avf/avf-vending-api/internal/config"
	"github.com/avf/avf-vending-api/internal/observability/grpcprom"
	"github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/avf/avf-vending-api/internal/platform/auth/revocation"
	"github.com/avf/avf-vending-api/internal/platform/observability/productionmetrics"
	"github.com/avf/avf-vending-api/internal/platform/ratelimit"
	machinev1 "github.com/avf/avf-vending-api/proto/avf/machine/v1"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type grpcMetaContextKey struct{}

// GRPCRequestMeta carries request identifiers attached by unaryRequestMetaInterceptor.
type GRPCRequestMeta struct {
	RequestID     string
	CorrelationID string
}

func withGRPCRequestMeta(ctx context.Context, m GRPCRequestMeta) context.Context {
	return context.WithValue(ctx, grpcMetaContextKey{}, m)
}

// GRPCRequestMetaFromContext returns metadata attached for logging and auth diagnostics.
func GRPCRequestMetaFromContext(ctx context.Context) (GRPCRequestMeta, bool) {
	v, ok := ctx.Value(grpcMetaContextKey{}).(GRPCRequestMeta)
	return v, ok
}

func newAccessTokenValidator(cfg *config.Config, accessRevocation revocation.Store) (auth.AccessTokenValidator, error) {
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
	if cfg.RedisRuntime.AuthAccessJTIRevocationEnabled && accessRevocation != nil {
		validator = auth.WrapWithRevocation(validator, accessRevocation, cfg.RedisRuntime.AuthRevocationRedisFailOpen)
	}
	return validator, nil
}

func chainUnaryInterceptors(cfg *config.Config, log *zap.Logger, validator auth.AccessTokenValidator, accessRevocation revocation.Store, rlBackend ratelimit.Backend, replayLedger *MachineReplayLedger, machineTokenChecker auth.MachineTokenCredentialChecker, machineCertChecker auth.MachineGRPCClientCertChecker) grpc.ServerOption {
	if cfg == nil || log == nil {
		log = zap.NewNop()
	}
	return grpc.ChainUnaryInterceptor(
		unaryRecoveryInterceptor(log),
		grpcprom.UnaryServerInterceptor(),
		unaryRequestMetaInterceptor(),
		unaryAccessLogInterceptor(log),
		unaryDeadlineInterceptor(cfg),
		unaryAuthChainInterceptor(cfg, log, validator, accessRevocation, machineTokenChecker, machineCertChecker),
		newUnaryMachineReplayInterceptor(cfg, replayLedger),
		unaryMachineHotRPCInterceptor(cfg, rlBackend),
	)
}

func unaryRecoveryInterceptor(log *zap.Logger) grpc.UnaryServerInterceptor {
	if log == nil {
		log = zap.NewNop()
	}
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		defer func() {
			if r := recover(); r != nil {
				meta, _ := GRPCRequestMetaFromContext(ctx)
				log.Error("grpc panic recovered",
					zap.Any("panic", r),
					zap.String("grpc_method", info.FullMethod),
					zap.String("request_id", meta.RequestID),
					zap.String("correlation_id", meta.CorrelationID),
					zap.ByteString("stack", debug.Stack()),
				)
				err = status.Errorf(codes.Internal, "internal error")
			}
		}()
		return handler(ctx, req)
	}
}

func unaryRequestMetaInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
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
		ctx = withGRPCRequestMeta(ctx, GRPCRequestMeta{
			RequestID:     requestID,
			CorrelationID: correlationID,
		})
		return handler(ctx, req)
	}
}

func unaryAccessLogInterceptor(log *zap.Logger) grpc.UnaryServerInterceptor {
	if log == nil {
		log = zap.NewNop()
	}
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()
		meta, _ := GRPCRequestMetaFromContext(ctx)
		resp, err := handler(ctx, req)
		code := status.Code(err)
		md, _ := metadata.FromIncomingContext(ctx)
		fields := []zap.Field{
			zap.String("grpc_method", info.FullMethod),
			zap.String("grpc_code", code.String()),
			zap.String("request_id", meta.RequestID),
			zap.String("trace_id", grpcAccessTraceID(md, meta)),
			zap.String("correlation_id", meta.CorrelationID),
			zap.Duration("duration", time.Since(start)),
			zap.String("operation", info.FullMethod),
			zap.String("result", code.String()),
		}
		if err != nil {
			fields = append(fields, zap.String("error_code", code.String()))
		} else {
			fields = append(fields, zap.String("error_code", ""))
		}
		fields = append(fields, grpcActorIdentityFields(ctx)...)
		log.Info("grpc unary completed", fields...)
		return resp, err
	}
}

func unaryDeadlineInterceptor(cfg *config.Config) grpc.UnaryServerInterceptor {
	timeout := time.Minute
	if cfg != nil && cfg.GRPC.Enabled && cfg.GRPC.UnaryHandlerTimeout > 0 {
		timeout = cfg.GRPC.UnaryHandlerTimeout
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

func unaryAuthChainInterceptor(cfg *config.Config, log *zap.Logger, validator auth.AccessTokenValidator, accessRevocation revocation.Store, machineTokenChecker auth.MachineTokenCredentialChecker, machineCertChecker auth.MachineGRPCClientCertChecker) grpc.UnaryServerInterceptor {
	if log == nil {
		log = zap.NewNop()
	}
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if isUnauthenticatedGRPCMethod(info.FullMethod) {
			return handler(ctx, req)
		}
		meta, _ := GRPCRequestMetaFromContext(ctx)
		md, _ := metadata.FromIncomingContext(ctx)

		if requiresMachineAccessJWT(info.FullMethod) || isMachineGRPCMethod(info.FullMethod) {
			return unaryMachineAuth(ctx, req, info, handler, cfg, log, meta, md, accessRevocation, machineTokenChecker, machineCertChecker)
		}
		return unaryInternalUserAuth(ctx, req, info, handler, log, validator, meta, md)
	}
}

func recordGRPCAuthFailureFromStatus(err error) {
	if err == nil {
		return
	}
	switch status.Code(err) {
	case codes.Unauthenticated, codes.PermissionDenied:
		productionmetrics.RecordGRPCAuthFailure(classifyGRPCAuthFailureReason(err))
	default:
	}
}

func classifyGRPCAuthFailureReason(err error) string {
	msg := strings.ToLower(strings.TrimSpace(status.Convert(err).Message()))
	switch {
	case strings.Contains(msg, "missing bearer"):
		return "missing_bearer_token"
	case strings.Contains(msg, "machine jwt required"):
		return "machine_jwt_required"
	case strings.Contains(msg, "invalid bearer"):
		return "invalid_bearer_token"
	case strings.Contains(msg, "invalid client certificate"):
		return "invalid_client_certificate"
	case strings.Contains(msg, "certificate"):
		return "certificate_mismatch"
	case strings.Contains(msg, "token revoked"):
		return "token_revoked"
	case strings.Contains(msg, "misconfigured"):
		return "auth_misconfigured"
	case strings.Contains(msg, "machine_id does not match"):
		return "request_scope_mismatch"
	case strings.Contains(msg, "organization_id does not match"):
		return "request_scope_mismatch"
	default:
		return "other"
	}
}

func grpcAccessTraceID(md metadata.MD, meta GRPCRequestMeta) string {
	if md != nil {
		if v := firstMetadata(md, "traceparent"); v != "" {
			parts := strings.Split(v, "-")
			if len(parts) >= 2 && strings.TrimSpace(parts[1]) != "" {
				return strings.TrimSpace(parts[1])
			}
		}
		if v := firstMetadata(md, "x-cloud-trace-context"); v != "" {
			parts := strings.Split(v, "/")
			if len(parts) >= 1 && strings.TrimSpace(parts[0]) != "" {
				return strings.TrimSpace(parts[0])
			}
		}
	}
	if meta.CorrelationID != "" {
		return meta.CorrelationID
	}
	return meta.RequestID
}

func grpcActorIdentityFields(ctx context.Context) []zap.Field {
	if c, ok := auth.MachineAccessClaimsFromContext(ctx); ok {
		return []zap.Field{
			zap.String("actor_type", "machine"),
			zap.String("actor_id", c.MachineID.String()),
			zap.String("machine_id", c.MachineID.String()),
			zap.String("organization_id", c.OrganizationID.String()),
		}
	}
	if p, ok := auth.PrincipalFromContext(ctx); ok {
		at, aid := p.Actor()
		fields := []zap.Field{
			zap.String("actor_type", at),
			zap.String("actor_id", aid),
		}
		if p.HasOrganization() {
			fields = append(fields, zap.String("organization_id", p.OrganizationID.String()))
		}
		return fields
	}
	return []zap.Field{zap.String("actor_type", "anonymous")}
}

func unaryMachineAuth(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler, cfg *config.Config, log *zap.Logger, meta GRPCRequestMeta, md metadata.MD, accessRevocation revocation.Store, machineTokenChecker auth.MachineTokenCredentialChecker, machineCertChecker auth.MachineGRPCClientCertChecker) (any, error) {
	prefix := ""
	if cfg != nil {
		prefix = cfg.GRPC.TLS.MachineIDFromCertURIPrefix
	}
	peerLeaf, hadCert := PeerClientCertificate(ctx)
	allowCertOnly := cfg != nil && cfg.GRPC.TLS.AllowMachineAuthCertOnly && machineCertChecker != nil

	raw := bearerFromMetadata(md)
	if raw == "" && allowCertOnly && hadCert && peerLeaf != nil {
		claims, err := machineCertChecker.ResolveMachineAccessFromClientCert(ctx, peerLeaf)
		if err != nil {
			log.Warn("grpc machine cert auth rejected",
				zap.String("grpc_method", info.FullMethod),
				zap.String("request_id", meta.RequestID),
				zap.String("correlation_id", meta.CorrelationID),
				zap.Error(err),
			)
			productionmetrics.RecordGRPCAuthFailure("invalid_client_certificate")
			return nil, status.Error(codes.Unauthenticated, "invalid client certificate")
		}
		if err := checkMachineBearerRevocation(ctx, cfg, accessRevocation, claims); err != nil {
			recordGRPCAuthFailureFromStatus(err)
			return nil, err
		}
		if err := validateMachineRequestScope(req, claims); err != nil {
			recordGRPCAuthFailureFromStatus(err)
			return nil, err
		}
		return handler(auth.WithMachineAccessClaims(ctx, claims), req)
	}

	if raw == "" {
		log.Warn("grpc machine auth rejected",
			zap.String("grpc_method", info.FullMethod),
			zap.String("request_id", meta.RequestID),
			zap.String("correlation_id", meta.CorrelationID),
			zap.String("reason", "missing_bearer_token"),
		)
		productionmetrics.RecordGRPCAuthFailure("missing_bearer_token")
		return nil, status.Error(codes.Unauthenticated, "missing bearer token")
	}
	machineJWT := effectiveMachineJWTConfig(cfg)
	if len(auth.MachineJWTSecretsFromConfig(machineJWT)) == 0 && !strings.Contains(strings.ToLower(strings.TrimSpace(machineJWT.Mode)), "jwks") &&
		!strings.Contains(strings.ToLower(strings.TrimSpace(machineJWT.Mode)), "pem") {
		log.Error("grpc machine auth misconfigured",
			zap.String("grpc_method", info.FullMethod),
			zap.String("request_id", meta.RequestID),
			zap.String("correlation_id", meta.CorrelationID),
			zap.String("reason", "no_machine_jwt_secrets"),
		)
		return nil, status.Error(codes.Unavailable, "machine jwt not configured")
	}
	claims, err := auth.ValidateMachineAccessJWTWithConfig(ctx, raw, machineJWT)
	if err != nil {
		log.Warn("grpc machine auth rejected",
			zap.String("grpc_method", info.FullMethod),
			zap.String("request_id", meta.RequestID),
			zap.String("correlation_id", meta.CorrelationID),
			zap.Error(err),
		)
		productionmetrics.RecordGRPCAuthFailure("invalid_bearer_token")
		return nil, status.Error(codes.Unauthenticated, "invalid bearer token")
	}
	if err := checkMachineBearerRevocation(ctx, cfg, accessRevocation, claims); err != nil {
		recordGRPCAuthFailureFromStatus(err)
		return nil, err
	}
	if machineTokenChecker != nil {
		if err := machineTokenChecker.ValidateMachineAccessClaims(ctx, claims); err != nil {
			recordGRPCAuthFailureFromStatus(err)
			return nil, err
		}
	}
	if err := validateMachineRequestScope(req, claims); err != nil {
		recordGRPCAuthFailureFromStatus(err)
		return nil, err
	}
	if machineCertChecker != nil && hadCert && peerLeaf != nil {
		certClaims, err := machineCertChecker.ResolveMachineAccessFromClientCert(ctx, peerLeaf)
		if err != nil {
			log.Warn("grpc machine cert check rejected",
				zap.String("grpc_method", info.FullMethod),
				zap.String("request_id", meta.RequestID),
				zap.String("correlation_id", meta.CorrelationID),
				zap.Error(err),
			)
			productionmetrics.RecordGRPCAuthFailure("invalid_client_certificate")
			return nil, status.Error(codes.Unauthenticated, "invalid client certificate")
		}
		if certClaims.MachineID != claims.MachineID || certClaims.OrganizationID != claims.OrganizationID {
			productionmetrics.RecordGRPCAuthFailure("certificate_token_mismatch")
			return nil, status.Error(codes.Unauthenticated, "client certificate does not match token")
		}
	} else if hadCert && peerLeaf != nil {
		if mid, ok := auth.ParseMachineIDFromClientCertURIs(peerLeaf, prefix); ok && mid != claims.MachineID {
			productionmetrics.RecordGRPCAuthFailure("certificate_token_mismatch")
			return nil, status.Error(codes.Unauthenticated, "client certificate does not match token")
		}
	}
	return handler(auth.WithMachineAccessClaims(ctx, claims), req)
}

func effectiveMachineJWTConfig(cfg *config.Config) config.MachineJWTConfig {
	if cfg == nil {
		return config.MachineJWTConfig{}
	}
	m := cfg.MachineJWT
	if strings.TrimSpace(m.Mode) != "" || len(m.JWTSecret) > 0 || len(m.JWTSecretPrevious) > 0 ||
		len(m.AdditionalHS256Secrets) > 0 || len(m.RSAPublicKeyPEM) > 0 || len(m.Ed25519PublicKeyPEM) > 0 ||
		strings.TrimSpace(m.JWKSURL) != "" {
		return m
	}
	return config.MachineJWTConfig{
		Mode:                   auth.HTTPAuthModeHS256,
		JWTLeeway:              cfg.HTTPAuth.JWTLeeway,
		JWTSecret:              cfg.HTTPAuth.LoginJWTSecret,
		JWTSecretPrevious:      cfg.HTTPAuth.JWTSecretPrevious,
		AdditionalHS256Secrets: [][]byte{cfg.HTTPAuth.JWTSecret},
		ExpectedIssuer:         cfg.HTTPAuth.ExpectedIssuer,
		ExpectedAudience:       auth.AudienceMachineGRPC,
		AccessTokenTTL:         cfg.HTTPAuth.AccessTokenTTL,
		RefreshTokenTTL:        cfg.HTTPAuth.RefreshTokenTTL,
		RequireAudience:        true,
	}
}

func checkMachineBearerRevocation(ctx context.Context, cfg *config.Config, store revocation.Store, claims auth.MachineAccessClaims) error {
	if cfg == nil || !cfg.RedisRuntime.AuthAccessJTIRevocationEnabled || store == nil {
		return nil
	}
	failOpen := cfg.RedisRuntime.AuthRevocationRedisFailOpen
	if jti := strings.TrimSpace(claims.JTI); jti != "" {
		revoked, err := store.IsJTIRevoked(ctx, jti)
		if err != nil {
			if failOpen {
				return nil
			}
			return status.Error(codes.Unauthenticated, "auth check failed")
		}
		if revoked {
			return status.Error(codes.Unauthenticated, "token revoked")
		}
	}
	if sub := strings.TrimSpace(claims.Subject); sub != "" {
		revoked, err := store.IsSubjectRevoked(ctx, sub)
		if err != nil {
			if failOpen {
				return nil
			}
			return status.Error(codes.Unauthenticated, "auth check failed")
		}
		if revoked {
			return status.Error(codes.Unauthenticated, "token revoked")
		}
	}
	return nil
}

func unaryMachineHotRPCInterceptor(cfg *config.Config, backend ratelimit.Backend) grpc.UnaryServerInterceptor {
	if cfg == nil || backend == nil {
		return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
			return handler(ctx, req)
		}
	}
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if !isMachineHotGRPCMethod(info.FullMethod) {
			return handler(ctx, req)
		}
		claims, ok := auth.MachineAccessClaimsFromContext(ctx)
		if !ok {
			return handler(ctx, req)
		}
		lim := cfg.RedisRuntime.GRPCMachineHotPerMinute
		if lim <= 0 {
			lim = 900
		}
		key := ratelimit.StableKey("machine_grpc", claims.MachineID.String(), info.FullMethod)
		okAllow, _ := backend.Allow(ctx, key, int64(lim), time.Minute)
		if !okAllow {
			return nil, status.Error(codes.ResourceExhausted, "rate limited")
		}
		return handler(ctx, req)
	}
}

func isMachineHotGRPCMethod(fullMethod string) bool {
	switch fullMethod {
	case machinev1.MachineCatalogService_GetCatalogSnapshot_FullMethodName,
		machinev1.MachineCatalogService_GetSaleCatalog_FullMethodName,
		machinev1.MachineCatalogService_SyncSaleCatalog_FullMethodName,
		machinev1.MachineMediaService_GetMediaManifest_FullMethodName,
		machinev1.MachineInventoryService_GetInventorySnapshot_FullMethodName,
		machinev1.MachineInventoryService_GetPlanogram_FullMethodName,
		machinev1.MachineOperatorService_HeartbeatOperatorSession_FullMethodName,
		machinev1.MachineCommerceService_GetOrderStatus_FullMethodName:
		return true
	default:
		return false
	}
}

func unaryInternalUserAuth(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler, log *zap.Logger, validator auth.AccessTokenValidator, meta GRPCRequestMeta, md metadata.MD) (any, error) {
	if isMachineGRPCMethod(info.FullMethod) {
		log.Warn("grpc auth rejected",
			zap.String("grpc_method", info.FullMethod),
			zap.String("request_id", meta.RequestID),
			zap.String("correlation_id", meta.CorrelationID),
			zap.String("reason", "machine_api_requires_machine_jwt"),
		)
		productionmetrics.RecordGRPCAuthFailure("machine_jwt_required")
		return nil, status.Error(codes.Unauthenticated, "machine jwt required")
	}
	raw := bearerFromMetadata(md)
	if raw == "" {
		log.Warn("grpc auth rejected",
			zap.String("grpc_method", info.FullMethod),
			zap.String("request_id", meta.RequestID),
			zap.String("correlation_id", meta.CorrelationID),
			zap.String("reason", "missing_bearer_token"),
		)
		productionmetrics.RecordGRPCAuthFailure("missing_bearer_token")
		return nil, status.Error(codes.Unauthenticated, "missing bearer token")
	}
	principal, err := validator.ValidateAccessToken(ctx, raw)
	if err != nil {
		log.Warn("grpc auth rejected",
			zap.String("grpc_method", info.FullMethod),
			zap.String("request_id", meta.RequestID),
			zap.String("correlation_id", meta.CorrelationID),
			zap.Error(err),
		)
		if err == auth.ErrMisconfigured {
			return nil, status.Error(codes.Unavailable, "gRPC auth misconfigured")
		}
		productionmetrics.RecordGRPCAuthFailure("invalid_bearer_token")
		return nil, status.Error(codes.Unauthenticated, "invalid bearer token")
	}
	return handler(auth.WithPrincipal(ctx, principal), req)
}

func bearerFromMetadata(md metadata.MD) string {
	return strings.TrimSpace(strings.TrimPrefix(firstMetadata(md, "authorization"), "Bearer "))
}

func requiresMachineAccessJWT(fullMethod string) bool {
	switch fullMethod {
	case machinev1.MachineBootstrapService_GetBootstrap_FullMethodName,
		machinev1.MachineBootstrapService_CheckIn_FullMethodName,
		machinev1.MachineBootstrapService_AckConfigVersion_FullMethodName,
		machinev1.MachineBootstrapService_CheckForUpdates_FullMethodName,
		machinev1.MachineCatalogService_SyncSaleCatalog_FullMethodName,
		machinev1.MachineCatalogService_GetSaleCatalog_FullMethodName,
		machinev1.MachineCatalogService_GetCatalogSnapshot_FullMethodName,
		machinev1.MachineCatalogService_GetCatalogDelta_FullMethodName,
		machinev1.MachineCatalogService_AckCatalogVersion_FullMethodName,
		machinev1.MachineCatalogService_GetMediaManifest_FullMethodName,
		machinev1.MachineMediaService_GetMediaManifest_FullMethodName,
		machinev1.MachineMediaService_GetMediaDelta_FullMethodName,
		machinev1.MachineMediaService_AckMediaVersion_FullMethodName,
		machinev1.MachineInventoryService_PushInventoryDelta_FullMethodName,
		machinev1.MachineInventoryService_GetInventorySnapshot_FullMethodName,
		machinev1.MachineInventoryService_AckInventorySync_FullMethodName,
		machinev1.MachineInventoryService_GetPlanogram_FullMethodName,
		machinev1.MachineInventoryService_SubmitStockSnapshot_FullMethodName,
		machinev1.MachineInventoryService_SubmitFillResult_FullMethodName,
		machinev1.MachineInventoryService_SubmitFillReport_FullMethodName,
		machinev1.MachineInventoryService_SubmitRestock_FullMethodName,
		machinev1.MachineInventoryService_SubmitInventoryAdjustment_FullMethodName,
		machinev1.MachineInventoryService_SubmitStockAdjustment_FullMethodName,
		machinev1.MachineTelemetryService_PushTelemetryBatch_FullMethodName,
		machinev1.MachineTelemetryService_PushCriticalEvent_FullMethodName,
		machinev1.MachineTelemetryService_CheckIn_FullMethodName,
		machinev1.MachineTelemetryService_SubmitTelemetryBatch_FullMethodName,
		machinev1.MachineTelemetryService_ReconcileEvents_FullMethodName,
		machinev1.MachineTelemetryService_GetEventStatus_FullMethodName,
		machinev1.MachineOperatorService_OpenOperatorSession_FullMethodName,
		machinev1.MachineOperatorService_CloseOperatorSession_FullMethodName,
		machinev1.MachineOperatorService_SubmitFillReport_FullMethodName,
		machinev1.MachineOperatorService_SubmitStockAdjustment_FullMethodName,
		machinev1.MachineOperatorService_LoginOperator_FullMethodName,
		machinev1.MachineOperatorService_LogoutOperator_FullMethodName,
		machinev1.MachineOperatorService_HeartbeatOperatorSession_FullMethodName,
		machinev1.MachineCommerceService_CreateOrder_FullMethodName,
		machinev1.MachineCommerceService_CreatePaymentSession_FullMethodName,
		machinev1.MachineCommerceService_AttachPaymentResult_FullMethodName,
		machinev1.MachineCommerceService_ConfirmCashPayment_FullMethodName,
		machinev1.MachineCommerceService_CreateCashCheckout_FullMethodName,
		machinev1.MachineCommerceService_GetOrder_FullMethodName,
		machinev1.MachineCommerceService_GetOrderStatus_FullMethodName,
		machinev1.MachineCommerceService_StartVend_FullMethodName,
		machinev1.MachineCommerceService_ConfirmVendSuccess_FullMethodName,
		machinev1.MachineCommerceService_ReportVendSuccess_FullMethodName,
		machinev1.MachineCommerceService_ReportVendFailure_FullMethodName,
		machinev1.MachineCommerceService_CancelOrder_FullMethodName,
		machinev1.MachineSaleService_CreateSale_FullMethodName,
		machinev1.MachineSaleService_AttachPayment_FullMethodName,
		machinev1.MachineSaleService_ConfirmCashReceived_FullMethodName,
		machinev1.MachineSaleService_StartVend_FullMethodName,
		machinev1.MachineSaleService_CompleteVend_FullMethodName,
		machinev1.MachineSaleService_FailVend_FullMethodName,
		machinev1.MachineSaleService_CancelSale_FullMethodName,
		machinev1.MachineOfflineSyncService_PushOfflineEvents_FullMethodName,
		machinev1.MachineOfflineSyncService_GetSyncCursor_FullMethodName,
		machinev1.MachineCommandService_GetPendingCommands_FullMethodName,
		machinev1.MachineCommandService_AckCommand_FullMethodName,
		machinev1.MachineCommandService_RejectCommand_FullMethodName,
		machinev1.MachineCommandService_GetAssignedUpdate_FullMethodName,
		machinev1.MachineCommandService_ReportUpdateStatus_FullMethodName,
		machinev1.MachineCommandService_ReportDiagnosticBundleResult_FullMethodName:
		return true
	default:
		return false
	}
}

func isMachineGRPCMethod(fullMethod string) bool {
	return strings.HasPrefix(fullMethod, "/avf.machine.v1.")
}

func isUnauthenticatedGRPCMethod(fullMethod string) bool {
	switch fullMethod {
	case "/grpc.health.v1.Health/Check", "/grpc.health.v1.Health/Watch":
		return true
	case machinev1.MachineAuthService_ActivateMachine_FullMethodName,
		machinev1.MachineAuthService_ClaimActivation_FullMethodName,
		machinev1.MachineAuthService_RefreshMachineToken_FullMethodName,
		machinev1.MachineActivationService_ClaimActivation_FullMethodName,
		machinev1.MachineTokenService_RefreshMachineToken_FullMethodName:
		// Activation / opaque refresh do not use Machine JWT (credentials are in the request body).
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
