package background_test

import (
	"context"
	"strings"
	"testing"
	"time"

	appbackground "github.com/avf/avf-vending-api/internal/app/background"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestRunWorker_RejectsPositiveRetentionTick(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	deps := appbackground.WorkerDeps{
		Log:                zap.NewNop(),
		OutboxTick:         24 * time.Hour,
		PaymentTimeoutTick: 24 * time.Hour,
		StuckCommandTick:   24 * time.Hour,
		RetentionTick:      time.Minute,
	}
	err := appbackground.RunWorker(ctx, deps)
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "RetentionTick"), "expected retention error, got %v", err)
}
