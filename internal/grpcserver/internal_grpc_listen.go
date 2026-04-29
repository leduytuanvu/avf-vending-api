package grpcserver

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/avf/avf-vending-api/internal/config"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"
)

// InternalGRPCServer hosts avf.internal.v1 read-only services on a separate listener (default loopback).
type InternalGRPCServer struct {
	cfg *config.Config
	log *zap.Logger

	ln  net.Listener
	srv *grpc.Server
}

// NewInternalGRPCServer constructs the internal query listener when INTERNAL_GRPC_ENABLED=true.
// ready should be non-nil in production so grpc.health reflects the same probes as HTTP /health/ready.
func NewInternalGRPCServer(cfg *config.Config, log *zap.Logger, ready ReadinessChecker, register ...ServiceRegistrar) (*InternalGRPCServer, error) {
	if cfg == nil || log == nil {
		return nil, fmt.Errorf("grpcserver: nil dependency")
	}
	if !cfg.InternalGRPC.Enabled {
		return nil, nil
	}

	secrets := internalGRPCVerifierSecrets(cfg)
	if len(secrets) == 0 {
		return nil, fmt.Errorf("grpcserver: internal gRPC enabled but no verifier secrets configured")
	}

	ln, err := net.Listen("tcp", cfg.InternalGRPC.Addr)
	if err != nil {
		return nil, fmt.Errorf("grpcserver: internal listen %q: %w", cfg.InternalGRPC.Addr, err)
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

	s := grpc.NewServer(
		grpc.KeepaliveParams(kaParams),
		grpc.KeepaliveEnforcementPolicy(kaPolicy),
		chainInternalUnaryInterceptors(cfg, log, secrets),
	)

	if cfg.InternalGRPC.HealthEnabled {
		registerHealthReadinessService(s, ready)
	}

	for i, reg := range register {
		if reg == nil {
			continue
		}
		if err := reg(s); err != nil {
			_ = ln.Close()
			return nil, fmt.Errorf("grpcserver: internal service registrar %d: %w", i, err)
		}
	}

	if cfg.InternalGRPC.ReflectionEnabled {
		reflection.Register(s)
	}

	return &InternalGRPCServer{cfg: cfg, log: log, ln: ln, srv: s}, nil
}

// ListenAndServe blocks until ctx is cancelled (same semantics as Server.ListenAndServe).
func (s *InternalGRPCServer) ListenAndServe(ctx context.Context) error {
	if s == nil || s.srv == nil {
		return nil
	}

	s.log.Info("internal grpc listening",
		zap.String("addr", s.ln.Addr().String()),
		zap.Bool("reflection", s.cfg != nil && s.cfg.InternalGRPC.ReflectionEnabled),
		zap.Bool("health", s.cfg != nil && s.cfg.InternalGRPC.HealthEnabled),
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

		timer := time.NewTimer(s.cfg.InternalGRPC.ShutdownTimeout)
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
