package catalogadmin

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/avf/avf-vending-api/internal/domain/compliance"
	"github.com/avf/avf-vending-api/internal/gen/db"
	plauth "github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/google/uuid"
)

func strPtrAudit(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return &s
}

// recordCatalogWriteAudit appends an enterprise audit row after a successful catalog mutation (fail-open).
func (s *Service) recordCatalogWriteAudit(ctx context.Context, org uuid.UUID, action, resourceType string, resourceID uuid.UUID, after map[string]any) {
	if s == nil || s.audit == nil || org == uuid.Nil || resourceID == uuid.Nil {
		return
	}
	rid := resourceID.String()
	meta := compliance.TransportMetaFromContext(ctx)
	actorType, actorID := compliance.ActorSystem, ""
	if p, ok := plauth.PrincipalFromContext(ctx); ok {
		actorType, actorID = p.Actor()
	}
	var afterB []byte
	if len(after) > 0 {
		b, err := json.Marshal(after)
		if err == nil {
			afterB = compliance.SanitizeJSONBytes(b)
		}
	}
	var aidPtr *string
	if actorID != "" {
		aidPtr = &actorID
	}
	_ = s.audit.Record(ctx, compliance.EnterpriseAuditRecord{
		OrganizationID: org,
		ActorType:      actorType,
		ActorID:        aidPtr,
		Action:         action,
		ResourceType:   resourceType,
		ResourceID:     &rid,
		RequestID:      strPtrAudit(meta.RequestID),
		TraceID:        strPtrAudit(meta.TraceID),
		IPAddress:      strPtrAudit(meta.IP),
		UserAgent:      strPtrAudit(meta.UserAgent),
		AfterJSON:      afterB,
		Outcome:        compliance.OutcomeSuccess,
	})
}

func productAuditSnapshot(p db.Product) map[string]any {
	return map[string]any{
		"productId": p.ID.String(),
		"sku":       p.Sku,
		"name":      p.Name,
		"active":    p.Active,
	}
}

func brandAuditSnapshot(b db.Brand) map[string]any {
	return map[string]any{
		"brandId": b.ID.String(),
		"slug":    b.Slug,
		"name":    b.Name,
		"active":  b.Active,
	}
}

func categoryAuditSnapshot(c db.Category) map[string]any {
	out := map[string]any{
		"categoryId": c.ID.String(),
		"slug":       c.Slug,
		"name":       c.Name,
		"active":     c.Active,
	}
	if c.ParentID.Valid {
		out["parentId"] = uuid.UUID(c.ParentID.Bytes).String()
	}
	return out
}

func tagAuditSnapshot(t db.Tag) map[string]any {
	return map[string]any{
		"tagId":  t.ID.String(),
		"slug":   t.Slug,
		"name":   t.Name,
		"active": t.Active,
	}
}

func priceBookAuditSnapshot(p db.PriceBook) map[string]any {
	m := map[string]any{
		"priceBookId": p.ID.String(),
		"name":        p.Name,
		"currency":    p.Currency,
		"active":      p.Active,
		"scopeType":   p.ScopeType,
	}
	if p.SiteID.Valid {
		m["siteId"] = uuid.UUID(p.SiteID.Bytes).String()
	}
	if p.MachineID.Valid {
		m["machineId"] = uuid.UUID(p.MachineID.Bytes).String()
	}
	return m
}
