package catalogadmin

import (
	"context"
	"encoding/json"

	"github.com/avf/avf-vending-api/internal/domain/compliance"
	plauth "github.com/avf/avf-vending-api/internal/platform/auth"
)

// EnterprisePromotionAuditHook maps promotion mutation callbacks to audit_events (fail-open).
func EnterprisePromotionAuditHook(rec compliance.EnterpriseRecorder) func(context.Context, PromotionAuditEvent) {
	return func(ctx context.Context, ev PromotionAuditEvent) {
		if rec == nil {
			return
		}
		rid := ev.PromotionID.String()
		meta := compliance.TransportMetaFromContext(ctx)
		actorType, actorID := compliance.ActorSystem, ""
		if p, ok := plauth.PrincipalFromContext(ctx); ok {
			actorType, actorID = p.Actor()
		}
		md, err := json.Marshal(map[string]any{
			"promotionOp": ev.Action,
			"detail":      ev.Detail,
		})
		if err != nil {
			md = []byte("{}")
		}
		md = compliance.SanitizeJSONBytes(md)
		var aidPtr *string
		if actorID != "" {
			aidPtr = &actorID
		}
		action := compliance.ActionPromotionChanged
		switch ev.Action {
		case "create":
			action = compliance.ActionPromotionCreated
		case "update", "assign_target", "delete_target":
			action = compliance.ActionPromotionChanged
		case "activate":
			action = compliance.ActionPromotionActivated
		case "pause":
			action = compliance.ActionPromotionPaused
		case "deactivate":
			action = compliance.ActionPromotionArchived
		}
		_ = rec.Record(ctx, compliance.EnterpriseAuditRecord{
			OrganizationID: ev.OrganizationID,
			ActorType:      actorType,
			ActorID:        aidPtr,
			Action:         action,
			ResourceType:   "catalog.promotion",
			ResourceID:     &rid,
			RequestID:      strPtrAudit(meta.RequestID),
			TraceID:        strPtrAudit(meta.TraceID),
			IPAddress:      strPtrAudit(meta.IP),
			UserAgent:      strPtrAudit(meta.UserAgent),
			Metadata:       md,
			Outcome:        compliance.OutcomeSuccess,
		})
	}
}
