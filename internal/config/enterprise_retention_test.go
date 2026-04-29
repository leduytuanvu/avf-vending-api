package config

import "testing"

func TestEnterpriseRetentionConfigValidation(t *testing.T) {
	t.Parallel()

	cfg := EnterpriseRetentionConfig{
		CleanupBatchSize:                 500,
		CommandRetentionDays:             180,
		CommandReceiptRetentionDays:      180,
		PaymentWebhookEventRetentionDays: 365,
		OutboxPublishedRetentionDays:     30,
		AuditRetentionDays:               2555,
		ProcessedMessageRetentionDays:    30,
		RefreshTokenRetentionDays:        90,
		PasswordResetTokenRetentionDays:  30,
		InventoryEventRetentionDays:      730,
		OfflineEventRetentionDays:        180,
	}
	if err := cfg.validate(); err != nil {
		t.Fatal(err)
	}
	cfg.OutboxPublishedRetentionDays = 0
	if err := cfg.validate(); err == nil {
		t.Fatal("expected invalid retention days")
	}
}

func TestLoadEnterpriseRetentionConfig(t *testing.T) {
	setMinimalValidLoadEnv(t)
	t.Setenv("ENTERPRISE_RETENTION_CLEANUP_ENABLED", "true")
	t.Setenv("RETENTION_ALLOW_DESTRUCTIVE_LOCAL", "true")
	t.Setenv("COMMAND_RETENTION_DAYS", "90")
	t.Setenv("COMMAND_RECEIPT_RETENTION_DAYS", "91")
	t.Setenv("PAYMENT_WEBHOOK_EVENT_RETENTION_DAYS", "120")
	t.Setenv("OUTBOX_PUBLISHED_RETENTION_DAYS", "14")
	t.Setenv("AUDIT_RETENTION_DAYS", "3650")
	t.Setenv("PROCESSED_MESSAGE_RETENTION_DAYS", "45")
	t.Setenv("REFRESH_TOKEN_RETENTION_DAYS", "60")
	t.Setenv("PASSWORD_RESET_TOKEN_RETENTION_DAYS", "7")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.EnterpriseRetention.CleanupEnabled {
		t.Fatal("expected enterprise retention cleanup enabled")
	}
	if cfg.EnterpriseRetention.CommandRetentionDays != 90 ||
		cfg.EnterpriseRetention.CommandReceiptRetentionDays != 91 ||
		cfg.EnterpriseRetention.PaymentWebhookEventRetentionDays != 120 ||
		cfg.EnterpriseRetention.OutboxPublishedRetentionDays != 14 ||
		cfg.EnterpriseRetention.AuditRetentionDays != 3650 ||
		cfg.EnterpriseRetention.ProcessedMessageRetentionDays != 45 ||
		cfg.EnterpriseRetention.RefreshTokenRetentionDays != 60 ||
		cfg.EnterpriseRetention.PasswordResetTokenRetentionDays != 7 ||
		cfg.EnterpriseRetention.InventoryEventRetentionDays != 730 ||
		cfg.EnterpriseRetention.OfflineEventRetentionDays != 180 {
		t.Fatalf("unexpected retention config: %+v", cfg.EnterpriseRetention)
	}
}

func TestLoadRetentionCleanupDisabledInLocalUnlessExplicitlyAllowed(t *testing.T) {
	setMinimalValidLoadEnv(t)
	t.Setenv("ENTERPRISE_RETENTION_CLEANUP_ENABLED", "true")

	if _, err := Load(); err == nil {
		t.Fatal("expected local destructive retention cleanup to require explicit opt-in")
	}

	t.Setenv("RETENTION_ALLOW_DESTRUCTIVE_LOCAL", "true")
	if _, err := Load(); err != nil {
		t.Fatal(err)
	}
}
