package config

import (
	"errors"
	"fmt"
	"time"
)

// EnterpriseRetentionConfig controls bounded cleanup for non-telemetry operational tables.
// It is disabled by default so local/API startup is unaffected and compliance data is never pruned accidentally.
type EnterpriseRetentionConfig struct {
	CleanupEnabled   bool
	CleanupDryRun    bool
	CleanupBatchSize int32

	CommandRetentionDays             int
	CommandReceiptRetentionDays      int
	PaymentWebhookEventRetentionDays int
	OutboxPublishedRetentionDays     int
	AuditRetentionDays               int
	ProcessedMessageRetentionDays    int
	RefreshTokenRetentionDays        int
	PasswordResetTokenRetentionDays  int
	// InventoryEventRetentionDays prunes inventory_events occurred_at older than horizon (append-only ledger trim).
	InventoryEventRetentionDays int
	// OfflineEventRetentionDays prunes processed terminal machine_offline_events rows older than horizon.
	OfflineEventRetentionDays int
}

func loadEnterpriseRetentionConfig() EnterpriseRetentionConfig {
	paymentDays := getenvInt("PAYMENT_WEBHOOK_EVENT_RETENTION_DAYS", 0)
	if paymentDays <= 0 {
		paymentDays = getenvInt("PAYMENT_EVENT_RETENTION_DAYS", 365)
	}
	outboxDays := getenvInt("OUTBOX_PUBLISHED_RETENTION_DAYS", 0)
	if outboxDays <= 0 {
		outboxDays = getenvInt("OUTBOX_RETENTION_DAYS", 30)
	}
	return EnterpriseRetentionConfig{
		CleanupEnabled:                   getenvBool("ENTERPRISE_RETENTION_CLEANUP_ENABLED", false),
		CleanupDryRun:                    getenvBool("ENTERPRISE_RETENTION_CLEANUP_DRY_RUN", false),
		CleanupBatchSize:                 int32(getenvInt("ENTERPRISE_RETENTION_CLEANUP_BATCH_SIZE", 500)),
		CommandRetentionDays:             getenvInt("COMMAND_RETENTION_DAYS", 180),
		CommandReceiptRetentionDays:      getenvInt("COMMAND_RECEIPT_RETENTION_DAYS", 180),
		PaymentWebhookEventRetentionDays: paymentDays,
		OutboxPublishedRetentionDays:     outboxDays,
		AuditRetentionDays:               getenvInt("AUDIT_RETENTION_DAYS", 2555),
		ProcessedMessageRetentionDays:    getenvInt("PROCESSED_MESSAGE_RETENTION_DAYS", 30),
		RefreshTokenRetentionDays:        getenvInt("REFRESH_TOKEN_RETENTION_DAYS", 90),
		PasswordResetTokenRetentionDays:  getenvInt("PASSWORD_RESET_TOKEN_RETENTION_DAYS", 30),
		InventoryEventRetentionDays:      getenvInt("INVENTORY_EVENT_RETENTION_DAYS", 730),
		OfflineEventRetentionDays:        getenvInt("OFFLINE_EVENT_RETENTION_DAYS", 180),
	}
}

func (c EnterpriseRetentionConfig) validate() error {
	if c.CleanupBatchSize <= 0 {
		return errors.New("config: ENTERPRISE_RETENTION_CLEANUP_BATCH_SIZE must be > 0")
	}
	if c.CleanupBatchSize > 50000 {
		return errors.New("config: ENTERPRISE_RETENTION_CLEANUP_BATCH_SIZE must be <= 50000")
	}
	checkDays := []struct {
		name string
		days int
	}{
		{"COMMAND_RETENTION_DAYS", c.CommandRetentionDays},
		{"COMMAND_RECEIPT_RETENTION_DAYS", c.CommandReceiptRetentionDays},
		{"PAYMENT_WEBHOOK_EVENT_RETENTION_DAYS / PAYMENT_EVENT_RETENTION_DAYS", c.PaymentWebhookEventRetentionDays},
		{"OUTBOX_PUBLISHED_RETENTION_DAYS / OUTBOX_RETENTION_DAYS", c.OutboxPublishedRetentionDays},
		{"AUDIT_RETENTION_DAYS", c.AuditRetentionDays},
		{"PROCESSED_MESSAGE_RETENTION_DAYS", c.ProcessedMessageRetentionDays},
		{"REFRESH_TOKEN_RETENTION_DAYS", c.RefreshTokenRetentionDays},
		{"PASSWORD_RESET_TOKEN_RETENTION_DAYS", c.PasswordResetTokenRetentionDays},
		{"INVENTORY_EVENT_RETENTION_DAYS", c.InventoryEventRetentionDays},
		{"OFFLINE_EVENT_RETENTION_DAYS", c.OfflineEventRetentionDays},
	}
	for _, item := range checkDays {
		if item.days <= 0 {
			return fmt.Errorf("config: %s must be > 0", item.name)
		}
	}
	return nil
}

func (c EnterpriseRetentionConfig) Cutoffs(now time.Time) EnterpriseRetentionCutoffs {
	utc := now.UTC()
	daysAgo := func(days int) time.Time {
		return utc.Add(-time.Duration(days) * 24 * time.Hour)
	}
	return EnterpriseRetentionCutoffs{
		Command:             daysAgo(c.CommandRetentionDays),
		CommandReceipt:      daysAgo(c.CommandReceiptRetentionDays),
		PaymentWebhookEvent: daysAgo(c.PaymentWebhookEventRetentionDays),
		OutboxPublished:     daysAgo(c.OutboxPublishedRetentionDays),
		Audit:               daysAgo(c.AuditRetentionDays),
		ProcessedMessage:    daysAgo(c.ProcessedMessageRetentionDays),
		RefreshToken:        daysAgo(c.RefreshTokenRetentionDays),
		PasswordResetToken:  daysAgo(c.PasswordResetTokenRetentionDays),
		InventoryEvent:      daysAgo(c.InventoryEventRetentionDays),
		OfflineEvent:        daysAgo(c.OfflineEventRetentionDays),
	}
}

type EnterpriseRetentionCutoffs struct {
	Command             time.Time
	CommandReceipt      time.Time
	PaymentWebhookEvent time.Time
	OutboxPublished     time.Time
	Audit               time.Time
	ProcessedMessage    time.Time
	RefreshToken        time.Time
	PasswordResetToken  time.Time
	InventoryEvent      time.Time
	OfflineEvent        time.Time
}
