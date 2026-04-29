package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/avf/avf-vending-api/internal/config"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

const maxEnterpriseRetentionBatchesPerStage = 500

// EnterpriseRetentionResult records bounded-delete outcomes or candidate totals when CleanupDryRun=true.
type EnterpriseRetentionResult struct {
	Deleted    map[string]int64 // executed deletes (DryRun=false)
	Candidates map[string]int64 // rows matching retention predicates (DryRun=true)
	DryRun     bool
}

type EnterpriseRetentionTableStatus struct {
	TableName      string
	TotalRows      int64
	OldestRecordAt *time.Time
}

// RunEnterpriseRetention deletes only aged terminal/evidence rows in bounded batches.
// It intentionally keeps pending outbox rows, unresolved payments, and active command attempts.
func RunEnterpriseRetention(ctx context.Context, pool *pgxpool.Pool, cfg config.EnterpriseRetentionConfig, now time.Time) (EnterpriseRetentionResult, error) {
	if pool == nil {
		return EnterpriseRetentionResult{}, errors.New("postgres: nil pool")
	}
	result := EnterpriseRetentionResult{
		Deleted:    map[string]int64{},
		Candidates: map[string]int64{},
		DryRun:     cfg.CleanupDryRun,
	}
	if cfg.CleanupBatchSize <= 0 {
		return result, errors.New("postgres: enterprise retention batch size must be > 0")
	}

	cutoffs := cfg.Cutoffs(now)
	q := db.New(pool)

	if cfg.CleanupDryRun {
		m, err := enterpriseRetentionCandidateCounts(ctx, q, cutoffs)
		if err != nil {
			return result, err
		}
		result.Candidates = m
		return result, nil
	}

	stages := []struct {
		name   string
		sql    string
		cutoff time.Time
	}{
		{"outbox_events_published", deletePublishedOutboxSQL, cutoffs.OutboxPublished},
		{"payment_provider_events_resolved", deleteResolvedPaymentProviderEventsSQL, cutoffs.PaymentWebhookEvent},
		{"device_command_receipts", deleteDeviceCommandReceiptsSQL, cutoffs.CommandReceipt},
		{"command_ledger_terminal", deleteTerminalCommandLedgerSQL, cutoffs.Command},
		{"messaging_consumer_dedupe", deleteMessagingConsumerDedupeSQL, cutoffs.ProcessedMessage},
		{"inventory_events", deleteInventoryEventsSQL, cutoffs.InventoryEvent},
		{"machine_offline_events_terminal", deleteMachineOfflineTerminalSQL, cutoffs.OfflineEvent},
		{"auth_refresh_tokens_expired", deleteExpiredAuthRefreshTokensSQL, cutoffs.RefreshToken},
		{"password_reset_tokens_expired", deleteExpiredPasswordResetTokensSQL, cutoffs.PasswordResetToken},
		{"audit_events", deleteAuditEventsSQL, cutoffs.Audit},
		{"audit_logs", deleteAuditLogsSQL, cutoffs.Audit},
	}
	for _, stage := range stages {
		total, err := runRetentionDeleteStage(ctx, pool, stage.sql, stage.cutoff, cfg.CleanupBatchSize)
		if err != nil {
			return result, fmt.Errorf("postgres: enterprise retention %s: %w", stage.name, err)
		}
		result.Deleted[stage.name] = total
	}
	return result, nil
}

func enterpriseRetentionCandidateCounts(ctx context.Context, q *db.Queries, cutoffs config.EnterpriseRetentionCutoffs) (map[string]int64, error) {
	outboxTs := pgtype.Timestamptz{Time: cutoffs.OutboxPublished.UTC(), Valid: true}

	type stage struct {
		name string
		run  func() (int64, error)
	}
	stages := []stage{
		{"outbox_events_published", func() (int64, error) {
			return q.EnterpriseRetentionCountPublishedOutbox(ctx, outboxTs)
		}},
		{"payment_provider_events_resolved", func() (int64, error) {
			return q.EnterpriseRetentionCountResolvedPaymentProviderEvents(ctx, cutoffs.PaymentWebhookEvent.UTC())
		}},
		{"device_command_receipts", func() (int64, error) {
			return q.EnterpriseRetentionCountDeviceCommandReceipts(ctx, cutoffs.CommandReceipt.UTC())
		}},
		{"command_ledger_terminal", func() (int64, error) {
			return q.EnterpriseRetentionCountTerminalCommandLedger(ctx, cutoffs.Command.UTC())
		}},
		{"messaging_consumer_dedupe", func() (int64, error) {
			return q.EnterpriseRetentionCountMessagingConsumerDedupe(ctx, cutoffs.ProcessedMessage.UTC())
		}},
		{"inventory_events", func() (int64, error) {
			return q.EnterpriseRetentionCountInventoryEvents(ctx, cutoffs.InventoryEvent.UTC())
		}},
		{"machine_offline_events_terminal", func() (int64, error) {
			return q.EnterpriseRetentionCountTerminalOfflineEvents(ctx, cutoffs.OfflineEvent.UTC())
		}},
		{"auth_refresh_tokens_expired", func() (int64, error) {
			return q.EnterpriseRetentionCountExpiredAuthRefreshTokens(ctx, cutoffs.RefreshToken.UTC())
		}},
		{"password_reset_tokens_expired", func() (int64, error) {
			return q.EnterpriseRetentionCountPasswordResetTokens(ctx, cutoffs.PasswordResetToken.UTC())
		}},
		{"audit_events", func() (int64, error) {
			return q.EnterpriseRetentionCountAuditEvents(ctx, cutoffs.Audit.UTC())
		}},
		{"audit_logs", func() (int64, error) {
			return q.EnterpriseRetentionCountAuditLogs(ctx, cutoffs.Audit.UTC())
		}},
	}
	m := map[string]int64{}
	for _, s := range stages {
		n, err := s.run()
		if err != nil {
			return nil, fmt.Errorf("postgres: enterprise retention candidate count %s: %w", s.name, err)
		}
		m[s.name] = n
	}
	return m, nil
}

func runRetentionDeleteStage(ctx context.Context, pool *pgxpool.Pool, sql string, cutoff time.Time, batch int32) (int64, error) {
	var total int64
	for i := 0; i < maxEnterpriseRetentionBatchesPerStage; i++ {
		tag, err := pool.Exec(ctx, sql, cutoff, batch)
		if err != nil {
			return total, err
		}
		n := tag.RowsAffected()
		total += n
		if n == 0 || n < int64(batch) {
			break
		}
	}
	return total, nil
}

func GetEnterpriseRetentionStatus(ctx context.Context, pool *pgxpool.Pool) ([]EnterpriseRetentionTableStatus, error) {
	if pool == nil {
		return nil, errors.New("postgres: nil pool")
	}
	rows, err := pool.Query(ctx, enterpriseRetentionStatusSQL)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []EnterpriseRetentionTableStatus
	for rows.Next() {
		var item EnterpriseRetentionTableStatus
		if err := rows.Scan(&item.TableName, &item.TotalRows, &item.OldestRecordAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

const deletePublishedOutboxSQL = `
WITH doomed AS (
	SELECT id
	FROM outbox_events
	WHERE status = 'published'
	  AND published_at IS NOT NULL
	  AND published_at < $1
	ORDER BY published_at, id
	LIMIT $2
)
DELETE FROM outbox_events o
USING doomed
WHERE o.id = doomed.id`

const deleteResolvedPaymentProviderEventsSQL = `
WITH doomed AS (
	SELECT e.id
	FROM payment_provider_events e
	JOIN payments p ON p.id = e.payment_id
	WHERE e.received_at < $1
	  AND e.legal_hold = false
	  AND p.state IN ('captured', 'failed', 'expired', 'canceled', 'refunded', 'partially_refunded')
	  AND p.reconciliation_status IN ('matched', 'not_required')
	ORDER BY e.received_at, e.id
	LIMIT $2
)
DELETE FROM payment_provider_events e
USING doomed
WHERE e.id = doomed.id`

const deleteDeviceCommandReceiptsSQL = `
WITH doomed AS (
	SELECT id
	FROM device_command_receipts
	WHERE received_at < $1
	  AND NOT EXISTS (
		SELECT 1
		FROM machine_command_attempts a
		WHERE a.id = device_command_receipts.command_attempt_id
		  AND a.status IN ('pending', 'sent')
	  )
	ORDER BY received_at, id
	LIMIT $2
)
DELETE FROM device_command_receipts r
USING doomed
WHERE r.id = doomed.id`

const deleteTerminalCommandLedgerSQL = `
WITH doomed AS (
	SELECT c.id
	FROM command_ledger c
	WHERE c.created_at < $1
	  AND NOT EXISTS (
		SELECT 1
		FROM machine_command_attempts a
		WHERE a.command_id = c.id
		  AND a.status IN ('pending', 'sent')
	  )
	ORDER BY c.created_at, c.id
	LIMIT $2
)
DELETE FROM command_ledger c
USING doomed
WHERE c.id = doomed.id`

const deleteMessagingConsumerDedupeSQL = `
WITH doomed AS (
	SELECT id
	FROM messaging_consumer_dedupe
	WHERE processed_at < $1
	ORDER BY processed_at, id
	LIMIT $2
)
DELETE FROM messaging_consumer_dedupe d
USING doomed
WHERE d.id = doomed.id`

const deleteInventoryEventsSQL = `
WITH doomed AS (
	SELECT id
	FROM inventory_events
	WHERE occurred_at < $1
	ORDER BY occurred_at, id
	LIMIT $2
)
DELETE FROM inventory_events ie
USING doomed
WHERE ie.id = doomed.id`

const deleteMachineOfflineTerminalSQL = `
WITH doomed AS (
	SELECT id
	FROM machine_offline_events
	WHERE processing_status IN ('processed', 'succeeded', 'failed', 'duplicate', 'replayed', 'rejected')
	  AND received_at < $1
	ORDER BY received_at, id
	LIMIT $2
)
DELETE FROM machine_offline_events m
USING doomed
WHERE m.id = doomed.id`

const deleteExpiredAuthRefreshTokensSQL = `
WITH doomed AS (
	SELECT id
	FROM auth_refresh_tokens
	WHERE expires_at < $1
	  AND (revoked_at IS NOT NULL OR last_used_at IS NULL OR last_used_at < $1)
	ORDER BY expires_at, id
	LIMIT $2
)
DELETE FROM auth_refresh_tokens t
USING doomed
WHERE t.id = doomed.id`

const deleteExpiredPasswordResetTokensSQL = `
WITH doomed AS (
	SELECT id
	FROM password_reset_tokens
	WHERE expires_at < $1
	   OR (used_at IS NOT NULL AND used_at < $1)
	ORDER BY expires_at, id
	LIMIT $2
)
DELETE FROM password_reset_tokens t
USING doomed
WHERE t.id = doomed.id`

const deleteAuditEventsSQL = `
WITH doomed AS (
	SELECT id
	FROM audit_events
	WHERE created_at < $1
	  AND legal_hold = false
	ORDER BY created_at, id
	LIMIT $2
)
DELETE FROM audit_events a
USING doomed
WHERE a.id = doomed.id`

const deleteAuditLogsSQL = `
WITH doomed AS (
	SELECT id
	FROM audit_logs
	WHERE created_at < $1
	  AND legal_hold = false
	ORDER BY created_at, id
	LIMIT $2
)
DELETE FROM audit_logs a
USING doomed
WHERE a.id = doomed.id`

const enterpriseRetentionStatusSQL = `
SELECT 'audit_events' AS table_name, count(*)::bigint AS total_rows, min(created_at) AS oldest_record_at FROM audit_events
UNION ALL
SELECT 'audit_logs', count(*)::bigint, min(created_at) FROM audit_logs
UNION ALL
SELECT 'auth_refresh_tokens', count(*)::bigint, min(expires_at) FROM auth_refresh_tokens
UNION ALL
SELECT 'password_reset_tokens', count(*)::bigint, min(expires_at) FROM password_reset_tokens
UNION ALL
SELECT 'command_ledger', count(*)::bigint, min(created_at) FROM command_ledger
UNION ALL
SELECT 'machine_command_attempts', count(*)::bigint, min(sent_at) FROM machine_command_attempts
UNION ALL
SELECT 'device_command_receipts', count(*)::bigint, min(received_at) FROM device_command_receipts
UNION ALL
SELECT 'payment_provider_events', count(*)::bigint, min(received_at) FROM payment_provider_events
UNION ALL
SELECT 'outbox_events', count(*)::bigint, min(created_at) FROM outbox_events
UNION ALL
SELECT 'messaging_consumer_dedupe', count(*)::bigint, min(processed_at) FROM messaging_consumer_dedupe
UNION ALL
SELECT 'inventory_events', count(*)::bigint, min(occurred_at) FROM inventory_events
UNION ALL
SELECT 'machine_offline_events', count(*)::bigint, min(received_at) FROM machine_offline_events
ORDER BY table_name`
