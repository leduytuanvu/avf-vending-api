package api

import (
	"context"
	"testing"

	appauth "github.com/avf/avf-vending-api/internal/app/auth"
	"github.com/avf/avf-vending-api/internal/domain/compliance"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

type auditSpy struct {
	last compliance.EnterpriseAuditRecord
}

func (a *auditSpy) Record(_ context.Context, in compliance.EnterpriseAuditRecord) error {
	a.last = in
	return nil
}

func (a *auditSpy) RecordCritical(_ context.Context, in compliance.EnterpriseAuditRecord) error {
	a.last = in
	return nil
}

func (a *auditSpy) RecordCriticalTx(_ context.Context, _ pgx.Tx, in compliance.EnterpriseAuditRecord) error {
	a.last = in
	return nil
}

func TestWireAuthAdminMutationAudit_roleChangedCanonicalAction(t *testing.T) {
	t.Parallel()
	rec := &auditSpy{}
	hook := WireAuthAdminMutationAudit(rec)
	org := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	actor := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	target := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	require.NoError(t, hook(context.Background(), nil, appauth.AuthAdminMutationEvent{
		Action:          authAuditPatchUser,
		OrganizationID:  org,
		ActorAccountID:  actor,
		TargetAccountID: target,
		Details: map[string]any{
			"roles_before": []string{"viewer"},
			"roles_after":  []string{"catalog_manager"},
		},
	}))
	require.Equal(t, compliance.ActionRoleChanged, rec.last.Action)
	require.Equal(t, "auth.account", rec.last.ResourceType)
}

func TestWireAuthAdminMutationAudit_setRolesCanonicalAction(t *testing.T) {
	t.Parallel()
	rec := &auditSpy{}
	hook := WireAuthAdminMutationAudit(rec)
	org := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	actor := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	target := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	require.NoError(t, hook(context.Background(), nil, appauth.AuthAdminMutationEvent{
		Action:          authAuditSetRoles,
		OrganizationID:  org,
		ActorAccountID:  actor,
		TargetAccountID: target,
		Details: map[string]any{
			"roles_before": []string{"viewer"},
			"roles_after":  []string{"finance_admin"},
		},
	}))
	require.Equal(t, compliance.ActionRoleChanged, rec.last.Action)
}

func TestWireAuthAdminMutationAudit_nilRecorderSafe(t *testing.T) {
	t.Parallel()
	hook := WireAuthAdminMutationAudit(nil)
	org := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	require.NoError(t, hook(context.Background(), nil, appauth.AuthAdminMutationEvent{
		Action:          authAuditPatchUser,
		OrganizationID:  org,
		ActorAccountID:  uuid.New(),
		TargetAccountID: uuid.New(),
	}))
}
