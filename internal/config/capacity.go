package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// CapacityLimitsConfig bounds hot-path throughput for OLTP safety at scale (100–1000 machines).
type CapacityLimitsConfig struct {
	MaxTelemetryGRPCBatchEvents int
	MaxTelemetryGRPCBatchBytes  int
	MaxOfflineEventsPerRequest  int
	MaxMediaManifestEntries     int
	ReportingSyncMaxSpanDays    int
	// ReportingExportMaxSpanDays caps CSV/async-style report exports (wider than sync JSON by default).
	ReportingExportMaxSpanDays int
	WorkerRecoveryScanMaxItems int32
	// WorkerOutboxDispatchMaxItems optional per-tick cap for outbox lease/list batch (zero = use WorkerRecoveryScanMaxItems).
	WorkerOutboxDispatchMaxItems   int32
	WorkerTickOutbox               time.Duration
	WorkerTickPaymentTimeout       time.Duration
	WorkerTickStuckCommand         time.Duration
	WorkerCycleBackoffAfterFailure time.Duration
}

const (
	defaultMaxTelemetryGRPCBatchEvents = 500
	defaultMaxTelemetryGRPCBatchBytes  = 2 << 20 // 2MiB protobuf envelope bound
	defaultMaxOfflineEventsPerRequest  = 200
	defaultMaxMediaManifestEntries     = 5000
	defaultReportingSyncMaxSpanDays    = 366
	defaultReportingExportMaxSpanDays  = 730
	defaultWorkerRecoveryScanMaxItems  = 200
)

func loadCapacityLimitsConfig() CapacityLimitsConfig {
	syncDays := getenvInt("REPORTING_SYNC_MAX_SPAN_DAYS", defaultReportingSyncMaxSpanDays)
	exportDays := defaultReportingExportMaxSpanDays
	if raw, ok := os.LookupEnv("REPORTING_EXPORT_MAX_SPAN_DAYS"); ok && strings.TrimSpace(raw) != "" {
		if v, err := strconv.Atoi(strings.TrimSpace(raw)); err == nil {
			exportDays = v
		}
	} else if exportDays < syncDays {
		// When export horizon is unset, widen to at least the sync horizon so wide sync windows stay valid.
		exportDays = syncDays
	}
	return CapacityLimitsConfig{
		MaxTelemetryGRPCBatchEvents:    getenvInt("CAPACITY_MAX_TELEMETRY_GRPC_BATCH_EVENTS", defaultMaxTelemetryGRPCBatchEvents),
		MaxTelemetryGRPCBatchBytes:     getenvInt("CAPACITY_MAX_TELEMETRY_GRPC_BATCH_BYTES", defaultMaxTelemetryGRPCBatchBytes),
		MaxOfflineEventsPerRequest:     getenvInt("CAPACITY_MAX_OFFLINE_EVENTS_PER_REQUEST", defaultMaxOfflineEventsPerRequest),
		MaxMediaManifestEntries:        getenvInt("CAPACITY_MAX_MEDIA_MANIFEST_ENTRIES", defaultMaxMediaManifestEntries),
		ReportingSyncMaxSpanDays:       syncDays,
		ReportingExportMaxSpanDays:     exportDays,
		WorkerRecoveryScanMaxItems:     int32(getenvInt("WORKER_RECOVERY_SCAN_MAX_ITEMS", defaultWorkerRecoveryScanMaxItems)),
		WorkerOutboxDispatchMaxItems:   int32(getenvInt("WORKER_OUTBOX_DISPATCH_MAX_ITEMS", 0)),
		WorkerTickOutbox:               mustParseDuration("WORKER_TICK_OUTBOX_DISPATCH", getenv("WORKER_TICK_OUTBOX_DISPATCH", "3s")),
		WorkerTickPaymentTimeout:       mustParseDuration("WORKER_TICK_PAYMENT_TIMEOUT_SCAN", getenv("WORKER_TICK_PAYMENT_TIMEOUT_SCAN", "10s")),
		WorkerTickStuckCommand:         mustParseDuration("WORKER_TICK_STUCK_COMMAND_SCAN", getenv("WORKER_TICK_STUCK_COMMAND_SCAN", "15s")),
		WorkerCycleBackoffAfterFailure: loadOptionalDurationPanic("WORKER_CYCLE_BACKOFF_AFTER_FAILURE", os.Getenv("WORKER_CYCLE_BACKOFF_AFTER_FAILURE")),
	}
}

func loadOptionalDurationPanic(name, raw string) time.Duration {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}
	d, err := parseDurationEnv(name, raw)
	if err != nil {
		panic(err.Error())
	}
	return d
}

func (c CapacityLimitsConfig) validate() error {
	if c.MaxTelemetryGRPCBatchEvents < 1 || c.MaxTelemetryGRPCBatchEvents > 10_000 {
		return errors.New("config: CAPACITY_MAX_TELEMETRY_GRPC_BATCH_EVENTS must be in [1,10000]")
	}
	if c.MaxTelemetryGRPCBatchBytes < 4096 || c.MaxTelemetryGRPCBatchBytes > 32<<20 {
		return errors.New("config: CAPACITY_MAX_TELEMETRY_GRPC_BATCH_BYTES must be in [4096,33554432]")
	}
	if c.MaxOfflineEventsPerRequest < 1 || c.MaxOfflineEventsPerRequest > 5000 {
		return errors.New("config: CAPACITY_MAX_OFFLINE_EVENTS_PER_REQUEST must be in [1,5000]")
	}
	if c.MaxMediaManifestEntries < 64 || c.MaxMediaManifestEntries > 100_000 {
		return errors.New("config: CAPACITY_MAX_MEDIA_MANIFEST_ENTRIES must be in [64,100000]")
	}
	if c.ReportingSyncMaxSpanDays < 1 || c.ReportingSyncMaxSpanDays > 732 {
		return errors.New("config: REPORTING_SYNC_MAX_SPAN_DAYS must be in [1,732]")
	}
	if c.ReportingExportMaxSpanDays < 1 || c.ReportingExportMaxSpanDays > 2555 {
		return errors.New("config: REPORTING_EXPORT_MAX_SPAN_DAYS must be in [1,2555]")
	}
	if c.ReportingExportMaxSpanDays < c.ReportingSyncMaxSpanDays {
		return errors.New("config: REPORTING_EXPORT_MAX_SPAN_DAYS must be >= REPORTING_SYNC_MAX_SPAN_DAYS")
	}
	if c.WorkerRecoveryScanMaxItems < 10 || c.WorkerRecoveryScanMaxItems > 50_000 {
		return errors.New("config: WORKER_RECOVERY_SCAN_MAX_ITEMS must be in [10,50000]")
	}
	if c.WorkerOutboxDispatchMaxItems < 0 {
		return errors.New("config: WORKER_OUTBOX_DISPATCH_MAX_ITEMS must be >= 0")
	}
	if c.WorkerOutboxDispatchMaxItems > 0 && (c.WorkerOutboxDispatchMaxItems < 10 || c.WorkerOutboxDispatchMaxItems > 50_000) {
		return errors.New("config: WORKER_OUTBOX_DISPATCH_MAX_ITEMS must be in [10,50000] when set")
	}
	maxTick := 30 * time.Minute
	if c.WorkerTickOutbox <= 0 || c.WorkerTickOutbox > maxTick {
		return fmt.Errorf("config: WORKER_TICK_OUTBOX_DISPATCH out of range (0,%v]", maxTick)
	}
	if c.WorkerTickPaymentTimeout <= 0 || c.WorkerTickPaymentTimeout > maxTick {
		return fmt.Errorf("config: WORKER_TICK_PAYMENT_TIMEOUT_SCAN out of range (0,%v]", maxTick)
	}
	if c.WorkerTickStuckCommand <= 0 || c.WorkerTickStuckCommand > maxTick {
		return fmt.Errorf("config: WORKER_TICK_STUCK_COMMAND_SCAN out of range (0,%v]", maxTick)
	}
	if c.WorkerCycleBackoffAfterFailure < 0 || c.WorkerCycleBackoffAfterFailure > 10*time.Minute {
		return errors.New("config: WORKER_CYCLE_BACKOFF_AFTER_FAILURE must be <= 10m")
	}
	return nil
}

// EffectiveReportingSyncMaxSpan returns the maximum allowed reporting window duration.
func (c CapacityLimitsConfig) EffectiveReportingSyncMaxSpan() time.Duration {
	d := c.ReportingSyncMaxSpanDays
	if d <= 0 {
		d = defaultReportingSyncMaxSpanDays
	}
	return time.Duration(d) * 24 * time.Hour
}

// EffectiveReportingExportMaxSpan caps CSV / export report windows (async download path).
func (c CapacityLimitsConfig) EffectiveReportingExportMaxSpan() time.Duration {
	d := c.ReportingExportMaxSpanDays
	if d <= 0 {
		d = defaultReportingExportMaxSpanDays
	}
	return time.Duration(d) * 24 * time.Hour
}
