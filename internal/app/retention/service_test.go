package retention_test

import (
	"context"
	"testing"

	appretention "github.com/avf/avf-vending-api/internal/app/retention"
	"github.com/avf/avf-vending-api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestService_Stats_InvalidDeps(t *testing.T) {
	t.Parallel()
	var s appretention.Service
	_, err := s.Stats(context.Background(), &config.Config{})
	require.Error(t, err)
}

func TestService_Run_InvalidDeps(t *testing.T) {
	t.Parallel()
	var s appretention.Service
	_, err := s.Run(context.Background(), &config.Config{})
	require.Error(t, err)
}
