//go:build !windows

package telemetryapp

import (
	"context"
	"testing"

	"github.com/avf/avf-vending-api/internal/config"
)

func TestJetStreamWorkers_Ready_noThresholds(t *testing.T) {
	t.Parallel()
	w := &JetStreamWorkers{cfg: config.TelemetryJetStreamConfig{}}
	if err := w.Ready(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
}
