package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/avf/avf-vending-api/internal/config"
	"github.com/avf/avf-vending-api/internal/modules/postgres"
	"github.com/avf/avf-vending-api/internal/testfixtures"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestRunEnterpriseRetention_dryRunDoesNotDeletePublishedOutbox(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	org := testfixtures.DevOrganizationID
	topic := "retention-dry-" + uuid.New().String()

	_, err := pool.Exec(ctx, `
INSERT INTO outbox_events (
  organization_id, topic, event_type, payload, aggregate_type, aggregate_id,
  status, published_at, created_at, updated_at
)
VALUES (
  $1, $2, 'test.event', '{}'::jsonb, 'test', gen_random_uuid(),
  'published', now() - interval '400 days', now() - interval '400 days', now()
)
`, org, topic)
	require.NoError(t, err)

	var rowID int64
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT id FROM outbox_events WHERE topic = $1`, topic,
	).Scan(&rowID))
	defer func() {
		_, _ = pool.Exec(ctx, `DELETE FROM outbox_events WHERE id = $1`, rowID)
	}()

	cfg := config.EnterpriseRetentionConfig{
		CleanupDryRun:                    true,
		CleanupBatchSize:                 50,
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
	res, err := postgres.RunEnterpriseRetention(ctx, pool, cfg, time.Now().UTC())
	require.NoError(t, err)
	require.True(t, res.DryRun)
	require.GreaterOrEqual(t, res.Candidates["outbox_events_published"], int64(1))
	require.Empty(t, res.Deleted)

	var cnt int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM outbox_events WHERE id = $1`, rowID,
	).Scan(&cnt))
	require.Equal(t, 1, cnt)
}
