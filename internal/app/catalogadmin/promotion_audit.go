package catalogadmin

import (
	"context"

	"github.com/google/uuid"
)

// PromotionAuditEvent is emitted on promotion mutations when a hook is configured (future audit pipeline).
type PromotionAuditEvent struct {
	OrganizationID uuid.UUID
	PromotionID    uuid.UUID
	Action         string // create|update|lifecycle|assign_target|delete_target
	Detail         string // optional short machine-readable note
}

// SetPromotionAuditHook registers a callback invoked after successful promotion mutations.
func (s *Service) SetPromotionAuditHook(h func(context.Context, PromotionAuditEvent)) {
	if s == nil {
		return
	}
	s.promotionAudit = h
}

func (s *Service) emitPromotionAudit(ctx context.Context, ev PromotionAuditEvent) {
	if s == nil || s.promotionAudit == nil {
		return
	}
	s.promotionAudit(ctx, ev)
}
