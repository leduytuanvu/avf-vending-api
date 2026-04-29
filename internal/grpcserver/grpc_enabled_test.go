package grpcserver

import (
	"testing"

	"github.com/avf/avf-vending-api/internal/config"
	"go.uber.org/zap"
)

func TestNewServer_DisabledReturnsNil(t *testing.T) {
	t.Parallel()

	s, err := NewServer(&config.Config{
		GRPC: config.GRPCConfig{Enabled: false},
	}, zap.NewNop(), nil, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if s != nil {
		t.Fatal("expected nil Server when GRPC.Enabled=false")
	}
}
