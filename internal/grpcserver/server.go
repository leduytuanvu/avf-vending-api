// Package grpcserver hosts the optional internal gRPC listener.
//
// Production posture: HTTP remains the primary public API. gRPC is internal-only and intended for
// service-to-service query contracts where protobuf adds value. The listener always registers
// grpc.health.v1 and may additionally mount business services via ServiceRegistrar callbacks.
// Device/runtime traffic remains on HTTP + MQTT; this package is not a device transport surface.
//
// When GRPC.Enabled is true, operators and mesh probes can validate the listener on cfg.GRPC.Addr.
// Additional services register only via non-nil [ServiceRegistrar] callbacks passed to [NewServer].
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
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/keepalive"
)

// ServiceRegistrar binds one or more gRPC services onto a server instance. Keep
// registrars in application or transport packages that own the generated stubs, not in
// cmd entrypoints.
type ServiceRegistrar func(*grpc.Server) error

// Server hosts internal gRPC services.
type Server struct {
	cfg *config.Config
	log *zap.Logger

	ln  net.Listener
	srv *grpc.Server
}

// NewServer constructs a gRPC server when enabled in configuration.
func NewServer(cfg *config.Config, log *zap.Logger, register ...ServiceRegistrar) (*Server, error) {
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

	validator, err := newAccessTokenValidator(cfg)
	if err != nil {
		_ = ln.Close()
		return nil, fmt.Errorf("grpcserver: access token validator: %w", err)
	}

	s := grpc.NewServer(
		grpc.KeepaliveParams(kaParams),
		grpc.KeepaliveEnforcementPolicy(kaPolicy),
		grpc.ChainUnaryInterceptor(unaryAuthInterceptor(log, validator)),
	)

	if err := registerHealthService(s); err != nil {
		_ = ln.Close()
		return nil, err
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

	return &Server{cfg: cfg, log: log, ln: ln, srv: s}, nil
}

func registerHealthService(s *grpc.Server) error {
	hs := health.NewServer()
	grpc_health_v1.RegisterHealthServer(s, hs)
	hs.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)
	return nil
}

// ListenAndServe starts serving and blocks until ctx is cancelled, then stops gracefully.
func (s *Server) ListenAndServe(ctx context.Context) error {
	if s == nil || s.srv == nil {
		return nil
	}

	s.log.Info("grpc listening",
		zap.String("addr", s.ln.Addr().String()),
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
