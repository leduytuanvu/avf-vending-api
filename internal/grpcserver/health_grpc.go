package grpcserver

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health/grpc_health_v1"
)

// ReadinessChecker matches dependency probes used for HTTP /health/ready (no sensitive details in responses).
type ReadinessChecker interface {
	Ready(ctx context.Context) error
}

type readinessHealthServer struct {
	grpc_health_v1.UnimplementedHealthServer
	ready ReadinessChecker
}

func (s *readinessHealthServer) Check(ctx context.Context, _ *grpc_health_v1.HealthCheckRequest) (*grpc_health_v1.HealthCheckResponse, error) {
	if s.ready == nil {
		return &grpc_health_v1.HealthCheckResponse{Status: grpc_health_v1.HealthCheckResponse_SERVING}, nil
	}
	if err := s.ready.Ready(ctx); err != nil {
		return &grpc_health_v1.HealthCheckResponse{Status: grpc_health_v1.HealthCheckResponse_NOT_SERVING}, nil
	}
	return &grpc_health_v1.HealthCheckResponse{Status: grpc_health_v1.HealthCheckResponse_SERVING}, nil
}

func registerHealthReadinessService(s *grpc.Server, ready ReadinessChecker) {
	if s == nil {
		return
	}
	grpc_health_v1.RegisterHealthServer(s, &readinessHealthServer{ready: ready})
}
