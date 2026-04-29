package api

import (
	"context"
	"encoding/json"
	"fmt"

	appauth "github.com/avf/avf-vending-api/internal/app/auth"
	"github.com/avf/avf-vending-api/internal/domain/compliance"
	"github.com/jackc/pgx/v5"
)

// Internal auth audit action strings (must match internal/app/auth/admin_users.go).
const (
	authAuditCreateUser         = "auth_admin.create_user"
	authAuditPatchUser          = "auth_admin.patch_user"
	authAuditActivateUser       = "auth_admin.activate_user"
	authAuditDeactivateUser     = "auth_admin.deactivate_user"
	authAuditResetPassword      = "auth_admin.reset_password"
	authAuditRequestReset       = "auth.request_password_reset"
	authAuditConfirmReset       = "auth.confirm_password_reset"
	authAuditRevokeSessions     = "auth_admin.revoke_sessions"
	authAuditSetRoles           = "auth_admin.set_roles"
	authAuditRemoveRole         = "auth_admin.remove_role"
	authAuditSelfChangePassword = "auth.change_password"
)

// WireAuthAdminMutationAudit maps auth admin mutations into audit_events (mandatory via RecordCritical / RecordCriticalTx).
func WireAuthAdminMutationAudit(rec compliance.EnterpriseRecorder) func(context.Context, pgx.Tx, appauth.AuthAdminMutationEvent) error {
	return func(ctx context.Context, tx pgx.Tx, e appauth.AuthAdminMutationEvent) error {
		if rec == nil {
			return nil
		}
		actor := e.ActorAccountID.String()
		target := e.TargetAccountID.String()
		md, err := json.Marshal(e.Details)
		if err != nil {
			md = []byte("{}")
		}
		md = compliance.SanitizeJSONBytes(md)

		action := resolveAuthAuditCanonicalAction(e)
		before, after := patchRoleSnapshots(e)

		recRecord := compliance.EnterpriseAuditRecord{
			OrganizationID: e.OrganizationID,
			ActorType:      compliance.ActorUser,
			ActorID:        &actor,
			Action:         action,
			ResourceType:   "auth.account",
			ResourceID:     &target,
			BeforeJSON:     before,
			AfterJSON:      after,
			Metadata:       md,
		}
		if tx != nil {
			return rec.RecordCriticalTx(ctx, tx, recRecord)
		}
		return rec.RecordCritical(ctx, recRecord)
	}
}

func resolveAuthAuditCanonicalAction(e appauth.AuthAdminMutationEvent) string {
	switch e.Action {
	case authAuditCreateUser:
		return compliance.ActionAuthUserCreated
	case authAuditPatchUser:
		if rolesChanged(e.Details) {
			return compliance.ActionRoleChanged
		}
		return compliance.ActionAuthUserUpdated
	case authAuditSetRoles:
		return compliance.ActionRoleChanged
	case authAuditRemoveRole:
		return compliance.ActionRoleChanged
	case authAuditActivateUser:
		return compliance.ActionAuthUserActivated
	case authAuditDeactivateUser:
		return compliance.ActionAuthUserDeactivated
	case authAuditResetPassword:
		return compliance.ActionAuthPasswordReset
	case authAuditSelfChangePassword:
		return compliance.ActionAuthPasswordChange
	case authAuditRequestReset:
		return compliance.ActionAuthPasswordResetRequest
	case authAuditConfirmReset:
		return compliance.ActionAuthPasswordResetConfirm
	case authAuditRevokeSessions:
		return compliance.ActionAuthUserSessionsRevoked
	default:
		return compliance.ActionAuthUserUpdated
	}
}

func rolesChanged(d map[string]any) bool {
	if len(d) == 0 {
		return false
	}
	rb, okb := d["roles_before"]
	ra, oka := d["roles_after"]
	if !okb || !oka {
		return false
	}
	return fmt.Sprintf("%v", rb) != fmt.Sprintf("%v", ra)
}

func patchRoleSnapshots(e appauth.AuthAdminMutationEvent) (before, after []byte) {
	if (e.Action != authAuditPatchUser && e.Action != authAuditSetRoles && e.Action != authAuditRemoveRole) || len(e.Details) == 0 {
		return nil, nil
	}
	rb, okb := e.Details["roles_before"]
	ra, oka := e.Details["roles_after"]
	if !okb || !oka {
		return nil, nil
	}
	before, _ = json.Marshal(rb)
	after, _ = json.Marshal(ra)
	return compliance.SanitizeJSONBytes(before), compliance.SanitizeJSONBytes(after)
}
