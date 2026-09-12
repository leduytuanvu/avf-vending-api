package layoutassignment

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

const (
	FlagMultiLayoutEnabled                 = "multi_layout_enabled"
	FlagSnapshotHistoryIngestEnabled       = "snapshot_history_ingest_enabled"
	FlagPeriodicSnapshotCaptureEnabled     = "periodic_snapshot_capture_enabled"
	FlagHardwareReconcileOnActivateEnabled = "hardware_reconcile_on_activate_enabled"
)

// FeatureFlagReader resolves effective feature flags for a machine.
type FeatureFlagReader interface {
	ResolveEffectiveFlags(ctx context.Context, machineID uuid.UUID) (map[string]bool, error)
}

// SnapshotIngestEnabled returns true when snapshot history ingest is active for the machine.
func SnapshotIngestEnabled(ctx context.Context, reader FeatureFlagReader, machineID uuid.UUID) bool {
	if reader == nil {
		return true
	}
	flags, err := reader.ResolveEffectiveFlags(ctx, machineID)
	if err != nil {
		return true
	}
	if enabled, ok := flags[FlagSnapshotHistoryIngestEnabled]; ok {
		return enabled
	}
	if enabled, ok := flags[FlagMultiLayoutEnabled]; ok {
		return enabled
	}
	return true
}

// MultiLayoutEnabled returns true when multi-layout UX is enabled for the machine.
func MultiLayoutEnabled(ctx context.Context, reader FeatureFlagReader, machineID uuid.UUID) bool {
	if reader == nil {
		return false
	}
	flags, err := reader.ResolveEffectiveFlags(ctx, machineID)
	if err != nil {
		return false
	}
	return flags[FlagMultiLayoutEnabled]
}

// NormalizeIntervalKey floors captured time to UTC 30-minute boundary key.
func NormalizeIntervalKey(capturedAtMs int64) string {
	if capturedAtMs <= 0 {
		return ""
	}
	const intervalMs = 30 * 60 * 1000
	floored := (capturedAtMs / intervalMs) * intervalMs
	return fmt.Sprintf("%d", floored)
}

// ValidateLayoutName checks non-empty trimmed layout names.
func ValidateLayoutName(name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("layout name is required")
	}
	return nil
}
