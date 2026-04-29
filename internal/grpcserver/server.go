// Package grpcserver hosts the optional gRPC listener for cmd/api.
//
// Production posture: HTTP remains the primary public admin API. gRPC serves:
//   - grpc.health.v1 (optional)
//   - Split-ready internal read-only query services under avf.internal.v1 on a separate loopback listener when INTERNAL_GRPC_ENABLED (Bearer internal service JWT; see docs/architecture/internal-grpc-split-ready.md)
//   - Machine lifecycle services under avf.machine.v1 (activation, refresh, bootstrap, catalog, inventory, operator heartbeat — Machine JWT on protected RPCs)
//
// When GRPC.Enabled is true (set MACHINE_GRPC_ENABLED or legacy GRPC_ENABLED), operators validate the listener on cfg.GRPC.Addr.
// When InternalGRPC.Enabled is true, internal query RPCs listen on cfg.InternalGRPC.Addr (default 127.0.0.1:9091).
// Additional services register via [ServiceRegistrar] callbacks passed to [NewServer] or [NewInternalGRPCServer].
package grpcserver

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/avf/avf-vending-api/internal/config"
	"github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/avf/avf-vending-api/internal/platform/auth/revocation"
	"github.com/avf/avf-vending-api/internal/platform/ratelimit"
	goredis "github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"
)

// ServiceRegistrar binds one or more gRPC services onto a server instance. Keep
// registrars in application or transport packages that own the generated stubs, not in
// cmd entrypoints.
type ServiceRegistrar func(*grpc.Server) error

// Server hosts gRPC services.
type Server struct {
	cfg *config.Config
	log *zap.Logger

	ln  net.Listener
	srv *grpc.Server
}

// NewServer constructs a gRPC server when enabled in configuration.
// sharedRedis is optional (distributed rate limiting for hot machine RPCs when Redis is configured).
// accessRevocation is optional; when AUTH_ACCESS_JTI_REVOCATION_ENABLED, user and machine Bearer JTIs are checked against Redis.
// ready is optional; when cfg.GRPC.HealthReflectsProcessReadiness is true, grpc.health Check delegates to ready (same probes as HTTP /health/ready, without exposing dependency errors to clients).
// machineTokenChecker is optional in tests; production wires Postgres to fail-closed on machine status/credential revocation.
// machineCertChecker is optional; when set with GRPC mTLS, peer client certificates are verified against machine_device_certificates.
func NewServer(cfg *config.Config, log *zap.Logger, sharedRedis *goredis.Client, accessRevocation revocation.Store, ready ReadinessChecker, replayLedger *MachineReplayLedger, machineTokenChecker auth.MachineTokenCredentialChecker, machineCertChecker auth.MachineGRPCClientCertChecker, register ...ServiceRegistrar) (*Server, error) {
	if cfg == nil || log == nil {
		return nil, fmt.Errorf("grpcserver: nil dependency")
	}
	if !cfg.GRPC.Enabled {
		return nil, nil
	}

	ln, err := net.Listen("tcp", cfg.GRPC.Addr)
	if err != nil {
		return nil, fmt.Errorf("grpcserver: listen %q: %w", cfg.GRPC.Addr, err)
	}

	kaParams := keepalive.ServerParameters{
		MaxConnectionIdle:     5 * time.Minute,
		MaxConnectionAgeGrace: 5 * time.Second,
		Time:                  2 * time.Minute,
		Timeout:               20 * time.Second,
	}
	kaPolicy := keepalive.EnforcementPolicy{
		MinTime:             5 * time.Second,
		PermitWithoutStream: true,
	}

	validator, err := newAccessTokenValidator(cfg, accessRevocation)
	if err != nil {
		_ = ln.Close()
		return nil, fmt.Errorf("grpcserver: access token validator: %w", err)
	}

	rlBackend, err := newMachineHotRateLimitBackend(cfg, log, sharedRedis)
	if err != nil {
		_ = ln.Close()
		return nil, err
	}

	tlsConf, terr := machineGRPCTLSConfig(cfg)
	if terr != nil {
		_ = ln.Close()
		return nil, fmt.Errorf("grpcserver: tls: %w", terr)
	}
	var opts []grpc.ServerOption
	if cfg.GRPC.MaxRecvMsgSize > 0 {
		opts = append(opts, grpc.MaxRecvMsgSize(cfg.GRPC.MaxRecvMsgSize))
	}
	if cfg.GRPC.MaxSendMsgSize > 0 {
		opts = append(opts, grpc.MaxSendMsgSize(cfg.GRPC.MaxSendMsgSize))
	}
	if tlsConf != nil {
		opts = append(opts, grpc.Creds(credentials.NewTLS(tlsConf)))
	}
	opts = append(opts,
		grpc.KeepaliveParams(kaParams),
		grpc.KeepaliveEnforcementPolicy(kaPolicy),
		chainUnaryInterceptors(cfg, log, validator, accessRevocation, rlBackend, replayLedger, machineTokenChecker, machineCertChecker),
	)
	s := grpc.NewServer(opts...)

	if cfg.GRPC.HealthEnabled {
		var chk ReadinessChecker
		if cfg.GRPC.HealthReflectsProcessReadiness {
			chk = ready
		}
		registerHealthReadinessService(s, chk)
	}

	for i, reg := range register {
		if reg == nil {
			continue
		}
		if err := reg(s); err != nil {
			_ = ln.Close()
			return nil, fmt.Errorf("grpcserver: service registrar %d: %w", i, err)
		}
	}

	if cfg.GRPC.ReflectionEnabled {
		reflection.Register(s)
	}

	return &Server{cfg: cfg, log: log, ln: ln, srv: s}, nil
}

func newMachineHotRateLimitBackend(cfg *config.Config, log *zap.Logger, sharedRedis *goredis.Client) (ratelimit.Backend, error) {
	if cfg == nil || !cfg.GRPC.Enabled {
		return nil, nil
	}
	if sharedRedis != nil {
		rb, err := ratelimit.NewRedisBackend(sharedRedis)
		if err != nil {
			return nil, fmt.Errorf("grpcserver: redis rate limit: %w", err)
		}
		return rb, nil
	}
	if log != nil {
		log.Warn("grpc machine hot RPC rate limiter using in-memory backend; configure REDIS_ADDR for distributed limits")
	}
	return ratelimit.NewMemoryBackend(), nil
}

// ListenAndServe starts serving and blocks until ctx is cancelled, then stops gracefully.
func (s *Server) ListenAndServe(ctx context.Context) error {
	if s == nil || s.srv == nil {
		return nil
	}

	s.log.Info("grpc listening",
		zap.String("addr", s.ln.Addr().String()),
		zap.Bool("grpc_tls_enabled", s.cfg != nil && s.cfg.GRPC.TLS.Enabled),
		zap.Bool("grpc_behind_tls_proxy", s.cfg != nil && s.cfg.GRPC.BehindTLSProxy),
		zap.String("grpc_public_base_url", grpcPublicBaseURLLog(s.cfg)),
		zap.Bool("reflection", s.cfg != nil && s.cfg.GRPC.ReflectionEnabled),
		zap.Bool("health", s.cfg != nil && s.cfg.GRPC.HealthEnabled),
		zap.Bool("health_reflects_readiness", s.cfg != nil && s.cfg.GRPC.HealthReflectsProcessReadiness),
		zap.Bool("require_machine_jwt", s.cfg != nil && s.cfg.GRPC.RequireMachineJWT),
		zap.Int("max_recv_msg_bytes", grpcMsgSizeLog(s.cfg, true)),
		zap.Int("max_send_msg_bytes", grpcMsgSizeLog(s.cfg, false)),
		zap.Int("registered_services", len(s.srv.GetServiceInfo())))

	errCh := make(chan error, 1)
	go func() {
		err := s.srv.Serve(s.ln)
		if errors.Is(err, grpc.ErrServerStopped) {
			errCh <- nil
			return
		}
		errCh <- err
	}()

	select {
	case <-ctx.Done():
		stopped := make(chan struct{})
		go func() {
			s.srv.GracefulStop()
			close(stopped)
		}()

		timer := time.NewTimer(s.cfg.GRPC.ShutdownTimeout)
		defer timer.Stop()

		select {
		case <-stopped:
		case <-timer.C:
			s.srv.Stop()
		}

		err := <-errCh
		if err != nil {
			return err
		}
		return ctx.Err()

	case err := <-errCh:
		return err
	}
}

func grpcPublicBaseURLLog(cfg *config.Config) string {
	if cfg == nil {
		return ""
	}
	return strings.TrimSpace(cfg.GRPC.PublicBaseURL)
}

func grpcMsgSizeLog(cfg *config.Config, recv bool) int {
	if cfg == nil {
		return 0
	}
	if recv {
		return cfg.GRPC.MaxRecvMsgSize
	}
	return cfg.GRPC.MaxSendMsgSize
}
