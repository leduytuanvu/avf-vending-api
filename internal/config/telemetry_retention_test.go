package config

import (
	"testing"
	"time"
)

func TestTelemetryDataRetentionConfig_validate(t *testing.T) {
	err := TelemetryDataRetentionConfig{
		RetentionDays:         30,
		CriticalRetentionDays: 365,
		CleanupBatchSize:      100,
	}.validate()
	if err != nil {
		t.Fatal(err)
	}

	err = TelemetryDataRetentionConfig{
		RetentionDays:         100,
		CriticalRetentionDays: 30,
		CleanupBatchSize:      100,
	}.validate()
	if err == nil {
		t.Fatal("expected error when critical < retention")
	}

	err = TelemetryDataRetentionConfig{
		RetentionDays:         0,
		CriticalRetentionDays: 30,
		CleanupBatchSize:      100,
	}.validate()
	if err == nil {
		t.Fatal("expected error for zero retention days")
	}

	err = TelemetryDataRetentionConfig{
		RetentionDays:         10,
		CriticalRetentionDays: 20,
		CleanupBatchSize:      0,
	}.validate()
	if err == nil {
		t.Fatal("expected error for zero batch size")
	}
}

func TestTelemetryDataRetentionConfig_RetentionCutoffs(t *testing.T) {
	c := TelemetryDataRetentionConfig{RetentionDays: 7, CriticalRetentionDays: 90}
	now := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
	n, crit := c.RetentionCutoffs(now)
	if !n.Equal(now.Add(-7 * 24 * time.Hour)) {
		t.Fatalf("normal cutoff: got %v want %v", n, now.Add(-7*24*time.Hour))
	}
	if !crit.Equal(now.Add(-90 * 24 * time.Hour)) {
		t.Fatalf("critical cutoff: got %v want %v", crit, now.Add(-90*24*time.Hour))
	}
}
