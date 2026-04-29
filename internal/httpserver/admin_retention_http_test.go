package httpserver

import (
	"context"
	"encoding/json"
	"testing"

	appretention "github.com/avf/avf-vending-api/internal/app/retention"
	"github.com/avf/avf-vending-api/internal/domain/compliance"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

type captureEnterpriseRecorder struct {
	last compliance.EnterpriseAuditRecord
}

func (c *captureEnterpriseRecorder) Record(ctx context.Context, in compliance.EnterpriseAuditRecord) error {
	_ = ctx
	c.last = in
	return nil
}

func (c *captureEnterpriseRecorder) RecordCritical(ctx context.Context, in compliance.EnterpriseAuditRecord) error {
	_, _ = ctx, in
	return nil
}

func (c *captureEnterpriseRecorder) RecordCriticalTx(ctx context.Context, tx pgx.Tx, in compliance.EnterpriseAuditRecord) error {
	_, _, _ = ctx, tx, in
	return nil
}

func TestRecordRetentionAuditEvent_retentionDryRunMetadata(t *testing.T) {
	t.Parallel()
	var rec captureEnterpriseRecorder
	org := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	ctx := context.Background()
	outcome := appretention.RunOutcome{
		OverallDryRun:       true,
		WouldModifyDatabase: false,
		Telemetry:           appretention.TelemetryOutcome{Enabled: true, DryRun: true},
		Enterprise:          appretention.EnterpriseRetentionOutcome{Enabled: true, DryRun: true, Candidates: map[string]int64{"outbox_events_published": 3}},
	}
	recordRetentionAuditEvent(ctx, &rec, org, compliance.ActionRetentionDryRun, outcome)

	require.Equal(t, org, rec.last.OrganizationID)
	require.Equal(t, compliance.ActionRetentionDryRun, rec.last.Action)
	require.Equal(t, "system_retention", rec.last.ResourceType)
	require.NotNil(t, rec.last.ResourceID)
	require.Equal(t, "system_retention", *rec.last.ResourceID)

	var meta map[string]any
	require.NoError(t, json.Unmarshal(rec.last.Metadata, &meta))
	require.Equal(t, true, meta["overallDryRun"])
}

func TestRecordRetentionAuditEvent_nilRecorderNoPanic(t *testing.T) {
	t.Parallel()
	org := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	recordRetentionAuditEvent(context.Background(), nil, org, compliance.ActionRetentionRun, appretention.RunOutcome{})
}
