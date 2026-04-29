package auth

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/avf/avf-vending-api/internal/domain/compliance"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/google/uuid"
)

const (
	auditActionMFATOTPEnrollBegin    = "auth.mfa.totp.enroll_begin"
	auditActionMFATOTPActivated      = "auth.mfa.totp.activated"
	auditActionMFALoginMFACompleted  = "auth.mfa.login.completed"
	auditActionMFATOTPDisabled       = "auth.mfa.totp.disabled"
	auditActionSessionRevokeSelf     = "auth.session.revoke.self"
	auditActionSessionsRevokedOthers = "auth.session.revoke.others"
)

func strPtrNonEmpty(s string) *string {
	v := strings.TrimSpace(s)
	if v == "" {
		return nil
	}
	return &v
}

// maskEmailForAudit keeps domain visible but folds local-part for failed-login correlation safety.
func maskEmailForAudit(email string) string {
	e := strings.TrimSpace(strings.ToLower(email))
	at := strings.LastIndex(e, "@")
	if at <= 0 || at >= len(e)-1 {
		return "***"
	}
	local, domain := e[:at], e[at+1:]
	if len(local) <= 1 {
		return "*@" + domain
	}
	return string(local[0]) + "***@" + domain
}

func (s *Service) auditLoginFailure(ctx context.Context, organizationID uuid.UUID, email string, reason string) {
	if s == nil || s.enterpriseAudit == nil {
		return
	}
	meta := compliance.TransportMetaFromContext(ctx)
	payload := map[string]any{"email": maskEmailForAudit(email)}
	if strings.TrimSpace(reason) != "" {
		payload["reason"] = reason
	}
	md, _ := json.Marshal(payload)
	md = compliance.SanitizeJSONBytes(md)
	_ = s.enterpriseAudit.Record(ctx, compliance.EnterpriseAuditRecord{
		OrganizationID: organizationID,
		ActorType:      compliance.ActorUser,
		Action:         compliance.ActionAuthLoginFailed,
		ResourceType:   "auth.login",
		RequestID:      strPtrNonEmpty(meta.RequestID),
		TraceID:        strPtrNonEmpty(meta.TraceID),
		IPAddress:      strPtrNonEmpty(meta.IP),
		UserAgent:      strPtrNonEmpty(meta.UserAgent),
		Metadata:       md,
		Outcome:        compliance.OutcomeFailure,
	})
}

func (s *Service) auditLoginSuccess(ctx context.Context, acct db.PlatformAuthAccount) error {
	if s == nil || s.enterpriseAudit == nil {
		return nil
	}
	meta := compliance.TransportMetaFromContext(ctx)
	sub := acct.ID.String()
	md, err := json.Marshal(map[string]any{"email": acct.Email})
	if err != nil {
		return err
	}
	return s.enterpriseAudit.RecordCritical(ctx, compliance.EnterpriseAuditRecord{
		OrganizationID: acct.OrganizationID,
		ActorType:      compliance.ActorUser,
		ActorID:        &sub,
		Action:         compliance.ActionAuthLoginSuccess,
		ResourceType:   "auth.account",
		ResourceID:     &sub,
		RequestID:      strPtrNonEmpty(meta.RequestID),
		TraceID:        strPtrNonEmpty(meta.TraceID),
		IPAddress:      strPtrNonEmpty(meta.IP),
		UserAgent:      strPtrNonEmpty(meta.UserAgent),
		Metadata:       md,
		Outcome:        compliance.OutcomeSuccess,
	})
}

func (s *Service) auditLogout(ctx context.Context, accountID uuid.UUID) error {
	if s == nil || s.enterpriseAudit == nil {
		return nil
	}
	acct, err := s.q.AuthGetAccountByID(ctx, accountID)
	if err != nil {
		return err
	}
	sub := accountID.String()
	md, err := json.Marshal(map[string]any{"email": acct.Email})
	if err != nil {
		return err
	}
	meta := compliance.TransportMetaFromContext(ctx)
	return s.enterpriseAudit.RecordCritical(ctx, compliance.EnterpriseAuditRecord{
		OrganizationID: acct.OrganizationID,
		ActorType:      compliance.ActorUser,
		ActorID:        &sub,
		Action:         compliance.ActionAuthLogout,
		ResourceType:   "auth.session",
		ResourceID:     &sub,
		RequestID:      strPtrNonEmpty(meta.RequestID),
		TraceID:        strPtrNonEmpty(meta.TraceID),
		IPAddress:      strPtrNonEmpty(meta.IP),
		UserAgent:      strPtrNonEmpty(meta.UserAgent),
		Metadata:       md,
		Outcome:        compliance.OutcomeSuccess,
	})
}

func (s *Service) auditRefreshSuccess(ctx context.Context, acct db.PlatformAuthAccount) error {
	if s == nil || s.enterpriseAudit == nil {
		return nil
	}
	sub := acct.ID.String()
	meta := compliance.TransportMetaFromContext(ctx)
	md, err := json.Marshal(map[string]any{"email": acct.Email})
	if err != nil {
		return err
	}
	return s.enterpriseAudit.RecordCritical(ctx, compliance.EnterpriseAuditRecord{
		OrganizationID: acct.OrganizationID,
		ActorType:      compliance.ActorUser,
		ActorID:        &sub,
		Action:         compliance.ActionAuthRefresh,
		ResourceType:   "auth.session",
		ResourceID:     &sub,
		RequestID:      strPtrNonEmpty(meta.RequestID),
		TraceID:        strPtrNonEmpty(meta.TraceID),
		IPAddress:      strPtrNonEmpty(meta.IP),
		UserAgent:      strPtrNonEmpty(meta.UserAgent),
		Metadata:       md,
		Outcome:        compliance.OutcomeSuccess,
	})
}

func (s *Service) auditMFASecurity(ctx context.Context, action string, orgID, actorID uuid.UUID, metadata map[string]any, outcome string) error {
	if s == nil || s.enterpriseAudit == nil {
		return nil
	}
	meta := compliance.TransportMetaFromContext(ctx)
	sub := actorID.String()
	md, err := json.Marshal(metadata)
	if err != nil {
		md = []byte("{}")
	}
	md = compliance.SanitizeJSONBytes(md)
	return s.enterpriseAudit.RecordCritical(ctx, compliance.EnterpriseAuditRecord{
		OrganizationID: orgID,
		ActorType:      compliance.ActorUser,
		ActorID:        &sub,
		Action:         action,
		ResourceType:   "auth.mfa",
		ResourceID:     &sub,
		RequestID:      strPtrNonEmpty(meta.RequestID),
		TraceID:        strPtrNonEmpty(meta.TraceID),
		IPAddress:      strPtrNonEmpty(meta.IP),
		UserAgent:      strPtrNonEmpty(meta.UserAgent),
		Metadata:       md,
		Outcome:        outcome,
	})
}

func (s *Service) auditMFATOTPFailure(ctx context.Context, acct db.PlatformAuthAccount, reason string) {
	if s == nil || s.enterpriseAudit == nil {
		return
	}
	meta := compliance.TransportMetaFromContext(ctx)
	sub := acct.ID.String()
	md, _ := json.Marshal(map[string]any{"email": maskEmailForAudit(acct.Email), "reason": reason})
	md = compliance.SanitizeJSONBytes(md)
	_ = s.enterpriseAudit.Record(ctx, compliance.EnterpriseAuditRecord{
		OrganizationID: acct.OrganizationID,
		ActorType:      compliance.ActorUser,
		ActorID:        &sub,
		Action:         compliance.ActionAuthLoginFailed,
		ResourceType:   "auth.mfa",
		ResourceID:     &sub,
		RequestID:      strPtrNonEmpty(meta.RequestID),
		TraceID:        strPtrNonEmpty(meta.TraceID),
		IPAddress:      strPtrNonEmpty(meta.IP),
		UserAgent:      strPtrNonEmpty(meta.UserAgent),
		Metadata:       md,
		Outcome:        compliance.OutcomeFailure,
	})
}
