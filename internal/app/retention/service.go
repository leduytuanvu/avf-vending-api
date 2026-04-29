package retention

import (
	"context"
	"errors"
	"time"

	"github.com/avf/avf-vending-api/internal/config"
	"github.com/avf/avf-vending-api/internal/modules/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrDestructiveRetentionForbidden indicates Postgres DELETE retention would run but APP_ENV forbids it without RETENTION_ALLOW_DESTRUCTIVE_LOCAL.
var ErrDestructiveRetentionForbidden = errors.New("retention: destructive Postgres retention is disabled for this deployment (development/test requires RETENTION_ALLOW_DESTRUCTIVE_LOCAL=true)")

// Service performs operational retention previews and bounded deletes via postgres adapters.
type Service struct {
	Pool *pgxpool.Pool
}

// New constructs a retention helper.
func New(pool *pgxpool.Pool) *Service {
	return &Service{Pool: pool}
}

// EnterpriseTableRow mirrors postgres retention status rows for HTTP envelopes.
type EnterpriseTableRow struct {
	TableName      string     `json:"tableName"`
	TotalRows      int64      `json:"totalRows"`
	OldestRecordAt *time.Time `json:"oldestRecordAt,omitempty"`
}

// Stats aggregates enterprise table footprints plus configured horizons.
type Stats struct {
	Tables []EnterpriseTableRow `json:"tables"`
	Policy PolicySnapshot       `json:"policy"`
	Flags  RuntimeFlags         `json:"runtime"`
}

// PolicySnapshot exposes configured retention horizons (days).
type PolicySnapshot struct {
	TelemetryRetentionDays           int `json:"telemetryRetentionDays"`
	TelemetryCriticalRetentionDays   int `json:"telemetryCriticalRetentionDays"`
	AuditRetentionDays               int `json:"auditRetentionDays"`
	CommandRetentionDays             int `json:"commandRetentionDays"`
	CommandReceiptRetentionDays      int `json:"commandReceiptRetentionDays"`
	PaymentWebhookEventRetentionDays int `json:"paymentWebhookEventRetentionDays"`
	// PaymentEventRetentionDays mirrors PAYMENT_EVENT_RETENTION_DAYS (same horizon as paymentWebhookEventRetentionDays).
	PaymentEventRetentionDays    int `json:"paymentEventRetentionDays"`
	OutboxPublishedRetentionDays int `json:"outboxPublishedRetentionDays"`
	// OutboxRetentionDays mirrors OUTBOX_RETENTION_DAYS (same horizon as outboxPublishedRetentionDays).
	OutboxRetentionDays           int `json:"outboxRetentionDays"`
	ProcessedMessageRetentionDays int `json:"processedMessageRetentionDays"`
	OfflineEventRetentionDays     int `json:"offlineEventRetentionDays"`
	InventoryEventRetentionDays   int `json:"inventoryEventRetentionDays"`
}

// RuntimeFlags exposes worker switches and deployment guardrails.
type RuntimeFlags struct {
	EnableRetentionWorker       bool `json:"enableRetentionWorker"`
	TelemetryCleanupEnabled     bool `json:"telemetryCleanupEnabled"`
	EnterpriseCleanupEnabled    bool `json:"enterpriseCleanupEnabled"`
	GlobalDryRun                bool `json:"globalDryRun"`
	DestructiveRetentionAllowed bool `json:"destructiveRetentionAllowed"`
}

func policySnapshot(cfg *config.Config) PolicySnapshot {
	return PolicySnapshot{
		TelemetryRetentionDays:           cfg.TelemetryDataRetention.RetentionDays,
		TelemetryCriticalRetentionDays:   cfg.TelemetryDataRetention.CriticalRetentionDays,
		AuditRetentionDays:               cfg.EnterpriseRetention.AuditRetentionDays,
		CommandRetentionDays:             cfg.EnterpriseRetention.CommandRetentionDays,
		CommandReceiptRetentionDays:      cfg.EnterpriseRetention.CommandReceiptRetentionDays,
		PaymentWebhookEventRetentionDays: cfg.EnterpriseRetention.PaymentWebhookEventRetentionDays,
		PaymentEventRetentionDays:        cfg.EnterpriseRetention.PaymentWebhookEventRetentionDays,
		OutboxPublishedRetentionDays:     cfg.EnterpriseRetention.OutboxPublishedRetentionDays,
		OutboxRetentionDays:              cfg.EnterpriseRetention.OutboxPublishedRetentionDays,
		ProcessedMessageRetentionDays:    cfg.EnterpriseRetention.ProcessedMessageRetentionDays,
		OfflineEventRetentionDays:        cfg.EnterpriseRetention.OfflineEventRetentionDays,
		InventoryEventRetentionDays:      cfg.EnterpriseRetention.InventoryEventRetentionDays,
	}
}

func runtimeSnapshot(cfg *config.Config) RuntimeFlags {
	return RuntimeFlags{
		EnableRetentionWorker:       cfg.RetentionWorker.Enabled,
		TelemetryCleanupEnabled:     cfg.TelemetryDataRetention.CleanupEnabled,
		EnterpriseCleanupEnabled:    cfg.EnterpriseRetention.CleanupEnabled,
		GlobalDryRun:                cfg.RetentionWorker.GlobalDryRun,
		DestructiveRetentionAllowed: config.DestructiveRetentionAllowed(cfg),
	}
}

// Stats returns footprint rows plus configured horizons (no deletes).
func (s *Service) Stats(ctx context.Context, cfg *config.Config) (Stats, error) {
	var out Stats
	if s == nil || s.Pool == nil || cfg == nil {
		return out, errors.New("retention: invalid dependencies")
	}
	rows, err := postgres.GetEnterpriseRetentionStatus(ctx, s.Pool)
	if err != nil {
		return out, err
	}
	out.Tables = make([]EnterpriseTableRow, 0, len(rows))
	for _, row := range rows {
		item := EnterpriseTableRow{
			TableName: row.TableName,
			TotalRows: row.TotalRows,
		}
		if row.OldestRecordAt != nil {
			t := row.OldestRecordAt.UTC()
			item.OldestRecordAt = &t
		}
		out.Tables = append(out.Tables, item)
	}
	out.Policy = policySnapshot(cfg)
	out.Flags = runtimeSnapshot(cfg)
	return out, nil
}

// RunOutcome records telemetry + enterprise bounded-delete outcomes for admin APIs.
type RunOutcome struct {
	Telemetry           TelemetryOutcome           `json:"telemetry"`
	Enterprise          EnterpriseRetentionOutcome `json:"enterprise"`
	OverallDryRun       bool                       `json:"overallDryRun"`
	WouldModifyDatabase bool                       `json:"wouldModifyDatabase"`
}

// TelemetryOutcome captures telemetry retention projections pruning.
type TelemetryOutcome struct {
	Enabled bool             `json:"enabled"`
	DryRun  bool             `json:"dryRun"`
	Stages  map[string]int64 `json:"stages,omitempty"`
}

// EnterpriseRetentionOutcome captures enterprise bounded deletes or candidate totals.
type EnterpriseRetentionOutcome struct {
	Enabled    bool             `json:"enabled"`
	DryRun     bool             `json:"dryRun"`
	Candidates map[string]int64 `json:"candidates,omitempty"`
	Deleted    map[string]int64 `json:"deleted,omitempty"`
}

func effectiveTelemetry(cfg *config.Config) config.TelemetryDataRetentionConfig {
	c := cfg.TelemetryDataRetention
	c.CleanupDryRun = config.EffectiveRetentionDryRun(cfg.RetentionWorker.GlobalDryRun, cfg.TelemetryDataRetention.CleanupDryRun)
	return c
}

func effectiveEnterprise(cfg *config.Config) config.EnterpriseRetentionConfig {
	c := cfg.EnterpriseRetention
	c.CleanupDryRun = config.EffectiveRetentionDryRun(cfg.RetentionWorker.GlobalDryRun, cfg.EnterpriseRetention.CleanupDryRun)
	return c
}

// DryRun evaluates candidate totals without issuing deletes (forces subsystem dry-run flags).
func (s *Service) DryRun(ctx context.Context, cfg *config.Config) (RunOutcome, error) {
	return s.run(ctx, cfg, true)
}

// Run executes telemetry + enterprise retention using configured horizons.
// ErrDestructiveRetentionForbidden when APP_ENV forbids deletes unless RETENTION_ALLOW_DESTRUCTIVE_LOCAL=true.
func (s *Service) Run(ctx context.Context, cfg *config.Config) (RunOutcome, error) {
	return s.run(ctx, cfg, false)
}

func (s *Service) run(ctx context.Context, cfg *config.Config, forceDryRun bool) (RunOutcome, error) {
	var out RunOutcome
	if s == nil || s.Pool == nil || cfg == nil {
		return out, errors.New("retention: invalid dependencies")
	}

	tel := effectiveTelemetry(cfg)
	ent := effectiveEnterprise(cfg)
	if forceDryRun {
		tel.CleanupDryRun = true
		ent.CleanupDryRun = true
	}

	wouldModifyTelemetry := cfg.TelemetryDataRetention.CleanupEnabled && !tel.CleanupDryRun
	wouldModifyEnterprise := cfg.EnterpriseRetention.CleanupEnabled && !ent.CleanupDryRun
	out.WouldModifyDatabase = wouldModifyTelemetry || wouldModifyEnterprise

	if !forceDryRun && out.WouldModifyDatabase && !config.DestructiveRetentionAllowed(cfg) {
		return out, ErrDestructiveRetentionForbidden
	}

	now := time.Now().UTC()

	if cfg.TelemetryDataRetention.CleanupEnabled {
		out.Telemetry.Enabled = true
		res, err := postgres.RunTelemetryRetention(ctx, s.Pool, tel, now)
		if err != nil {
			return out, err
		}
		out.Telemetry.DryRun = res.DryRun
		out.Telemetry.Stages = res.Stages
	}

	if cfg.EnterpriseRetention.CleanupEnabled {
		out.Enterprise.Enabled = true
		res, err := postgres.RunEnterpriseRetention(ctx, s.Pool, ent, now)
		if err != nil {
			return out, err
		}
		out.Enterprise.DryRun = res.DryRun
		if res.DryRun {
			out.Enterprise.Candidates = res.Candidates
		} else {
			out.Enterprise.Deleted = res.Deleted
		}
	}

	out.OverallDryRun = (!out.Telemetry.Enabled || out.Telemetry.DryRun) && (!out.Enterprise.Enabled || out.Enterprise.DryRun)
	return out, nil
}
