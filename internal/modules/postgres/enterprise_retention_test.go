package postgres

import (
	"strings"
	"testing"
)

func TestEnterpriseRetentionSQLGuardsActiveRecords(t *testing.T) {
	t.Parallel()

	assertContains := func(name, sql string, parts ...string) {
		t.Helper()
		lower := strings.ToLower(sql)
		for _, part := range parts {
			if !strings.Contains(lower, strings.ToLower(part)) {
				t.Fatalf("%s missing guard %q in:\n%s", name, part, sql)
			}
		}
	}
	assertContains("outbox", deletePublishedOutboxSQL, "status = 'published'", "published_at is not null", "limit $2")
	assertContains("payment events", deleteResolvedPaymentProviderEventsSQL, "join payments", "payment_id", "legal_hold = false", "reconciliation_status in ('matched', 'not_required')", "state in")
	assertContains("commands", deleteTerminalCommandLedgerSQL, "not exists", "status in ('pending', 'sent')", "limit $2")
	assertContains("command receipts", deleteDeviceCommandReceiptsSQL, "received_at < $1", "not exists", "status in ('pending', 'sent')", "limit $2")
	assertContains("dedupe", deleteMessagingConsumerDedupeSQL, "processed_at < $1", "limit $2")
	assertContains("refresh tokens", deleteExpiredAuthRefreshTokensSQL, "expires_at < $1", "revoked_at is not null", "limit $2")
	assertContains("reset tokens", deleteExpiredPasswordResetTokensSQL, "expires_at < $1", "used_at is not null", "limit $2")
	assertContains("audit events", deleteAuditEventsSQL, "legal_hold = false", "limit $2")
	assertContains("audit logs", deleteAuditLogsSQL, "legal_hold = false", "limit $2")
	assertContains("inventory events", deleteInventoryEventsSQL, "inventory_events", "occurred_at < $1", "limit $2")
	assertContains("offline terminal", deleteMachineOfflineTerminalSQL, "processing_status in", "received_at < $1", "limit $2")
}

func TestEnterpriseRetentionSQLDoesNotMassDeleteOrdersOrPayments(t *testing.T) {
	t.Parallel()

	protectedDeletes := []string{
		"delete from orders",
		"delete from payments",
		"delete from order_lines",
		"delete from vend_sessions",
	}
	stages := []struct {
		name string
		sql  string
	}{
		{"outbox", deletePublishedOutboxSQL},
		{"payment events", deleteResolvedPaymentProviderEventsSQL},
		{"command receipts", deleteDeviceCommandReceiptsSQL},
		{"commands", deleteTerminalCommandLedgerSQL},
		{"dedupe", deleteMessagingConsumerDedupeSQL},
		{"inventory", deleteInventoryEventsSQL},
		{"offline", deleteMachineOfflineTerminalSQL},
		{"refresh", deleteExpiredAuthRefreshTokensSQL},
		{"reset", deleteExpiredPasswordResetTokensSQL},
		{"audit events", deleteAuditEventsSQL},
		{"audit logs", deleteAuditLogsSQL},
	}
	for _, stage := range stages {
		lower := strings.ToLower(stage.sql)
		for _, banned := range protectedDeletes {
			if strings.Contains(lower, banned) {
				t.Fatalf("%s must not contain %q:\n%s", stage.name, banned, stage.sql)
			}
		}
	}
}
