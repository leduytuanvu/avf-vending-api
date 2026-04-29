package catalogadmin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// ListPromotionsParams pages promotions for an organization.
type ListPromotionsParams struct {
	OrganizationID     uuid.UUID
	Limit              int32
	Offset             int32
	IncludeDeactivated bool
}

// PromotionRuleInput binds rule payload for create/update.
type PromotionRuleInput struct {
	RuleType string          `json:"ruleType"`
	Payload  json.RawMessage `json:"payload"`
	Priority int32           `json:"priority"`
}

// CreatePromotionInput inserts promotion headers + optional rules.
type CreatePromotionInput struct {
	OrganizationID   uuid.UUID
	Name             string
	StartsAt         time.Time
	EndsAt           time.Time
	Priority         int32
	Stackable        bool
	BudgetLimitMinor *int64
	RedemptionLimit  *int32
	ChannelScope     *string
	Rules            []PromotionRuleInput
}

// PatchPromotionInput merges mutable promotion fields.
type PatchPromotionInput struct {
	Name             *string
	StartsAt         *time.Time
	EndsAt           *time.Time
	Priority         *int32
	Stackable        *bool
	BudgetLimitMinor *int64
	RedemptionLimit  *int32
	ChannelScope     *string
	ApprovalStatus   *string
	Rules            *[]PromotionRuleInput
}

// AssignPromotionTargetInput binds one promotion target row.
type AssignPromotionTargetInput struct {
	OrganizationID uuid.UUID
	PromotionID    uuid.UUID
	TargetType     string
	ProductID      *uuid.UUID
	CategoryID     *uuid.UUID
	MachineID      *uuid.UUID
	SiteID         *uuid.UUID
	OrgTargetID    *uuid.UUID
	TagID          *uuid.UUID
}

func validatePromotionWindow(startsAt, endsAt time.Time) error {
	if !endsAt.After(startsAt) {
		return fmt.Errorf("%w: ends_at must be after starts_at", ErrInvalidArgument)
	}
	return nil
}

func validateRulePayload(ruleType string, payload json.RawMessage) error {
	rt := strings.TrimSpace(strings.ToLower(ruleType))
	switch rt {
	case RulePercentageDiscount:
		var m map[string]any
		if err := json.Unmarshal(payload, &m); err != nil {
			return fmt.Errorf("%w: invalid rule payload json", ErrInvalidArgument)
		}
		pct, ok := m["percent"].(float64)
		if !ok || pct <= 0 || pct > 100 {
			return fmt.Errorf("%w: percentage_discount requires percent in (0,100]", ErrInvalidArgument)
		}
	case RuleFixedAmountDiscount:
		var m map[string]any
		if err := json.Unmarshal(payload, &m); err != nil {
			return fmt.Errorf("%w: invalid rule payload json", ErrInvalidArgument)
		}
		am, ok := m["amount_minor"].(float64)
		if !ok || am <= 0 {
			return fmt.Errorf("%w: fixed_amount_discount requires positive amount_minor", ErrInvalidArgument)
		}
	case RuleBuyXGetY:
		return fmt.Errorf("%w: buy_x_get_y is not implemented yet", ErrInvalidArgument)
	default:
		return fmt.Errorf("%w: unsupported rule_type", ErrInvalidArgument)
	}
	return nil
}

func pgInt8Ptr(v *int64) pgtype.Int8 {
	if v == nil {
		return pgtype.Int8{}
	}
	return pgtype.Int8{Int64: *v, Valid: true}
}

func pgInt4Ptr(v *int32) pgtype.Int4 {
	if v == nil {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: *v, Valid: true}
}

func pgTextPtr(v *string) pgtype.Text {
	if v == nil {
		return pgtype.Text{}
	}
	s := strings.TrimSpace(*v)
	if s == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: s, Valid: true}
}

// ListPromotions returns promotions for the tenant.
func (s *Service) ListPromotions(ctx context.Context, p ListPromotionsParams) ([]db.Promotion, int64, error) {
	if s == nil {
		return nil, 0, errors.New("catalogadmin: nil service")
	}
	if p.OrganizationID == uuid.Nil {
		return nil, 0, ErrOrganizationRequired
	}
	cnt, err := s.q.PromotionAdminCountPromotions(ctx, db.PromotionAdminCountPromotionsParams{
		OrganizationID: p.OrganizationID,
		Column2:        p.IncludeDeactivated,
	})
	if err != nil {
		return nil, 0, err
	}
	rows, err := s.q.PromotionAdminListPromotions(ctx, db.PromotionAdminListPromotionsParams{
		OrganizationID: p.OrganizationID,
		Limit:          p.Limit,
		Offset:         p.Offset,
		Column4:        p.IncludeDeactivated,
	})
	if err != nil {
		return nil, 0, err
	}
	return rows, cnt, nil
}

// GetPromotion returns a promotion row.
func (s *Service) GetPromotion(ctx context.Context, organizationID, promotionID uuid.UUID) (db.Promotion, error) {
	if s == nil {
		return db.Promotion{}, errors.New("catalogadmin: nil service")
	}
	if organizationID == uuid.Nil || promotionID == uuid.Nil {
		return db.Promotion{}, ErrOrganizationRequired
	}
	row, err := s.q.PromotionAdminGetPromotion(ctx, db.PromotionAdminGetPromotionParams{
		OrganizationID: organizationID,
		ID:             promotionID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Promotion{}, ErrNotFound
		}
		return db.Promotion{}, err
	}
	return row, nil
}

// ListPromotionRules returns rules for a promotion.
func (s *Service) ListPromotionRules(ctx context.Context, promotionID uuid.UUID) ([]db.PromotionRule, error) {
	if s == nil {
		return nil, errors.New("catalogadmin: nil service")
	}
	if promotionID == uuid.Nil {
		return nil, ErrOrganizationRequired
	}
	return s.q.PromotionAdminListRulesForPromotion(ctx, promotionID)
}

// CreatePromotion inserts promotion and optional rules.
func (s *Service) CreatePromotion(ctx context.Context, in CreatePromotionInput) (db.Promotion, error) {
	if s == nil {
		return db.Promotion{}, errors.New("catalogadmin: nil service")
	}
	if in.OrganizationID == uuid.Nil {
		return db.Promotion{}, ErrOrganizationRequired
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return db.Promotion{}, fmt.Errorf("%w: name required", ErrInvalidArgument)
	}
	if err := validatePromotionWindow(in.StartsAt, in.EndsAt); err != nil {
		return db.Promotion{}, err
	}
	for _, r := range in.Rules {
		if err := validateRulePayload(r.RuleType, r.Payload); err != nil {
			return db.Promotion{}, err
		}
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return db.Promotion{}, err
	}
	defer tx.Rollback(ctx)
	qtx := s.q.WithTx(tx)

	row, err := qtx.PromotionAdminInsertPromotion(ctx, db.PromotionAdminInsertPromotionParams{
		OrganizationID:   in.OrganizationID,
		Name:             name,
		ApprovalStatus:   "approved",
		LifecycleStatus:  "draft",
		Priority:         in.Priority,
		Stackable:        in.Stackable,
		StartsAt:         in.StartsAt,
		EndsAt:           in.EndsAt,
		BudgetLimitMinor: pgInt8Ptr(in.BudgetLimitMinor),
		RedemptionLimit:  pgInt4Ptr(in.RedemptionLimit),
		ChannelScope:     pgTextPtr(in.ChannelScope),
	})
	if err != nil {
		return db.Promotion{}, err
	}

	for _, r := range in.Rules {
		payload := r.Payload
		if len(payload) == 0 {
			payload = json.RawMessage([]byte(`{}`))
		}
		if _, err := qtx.PromotionAdminUpsertPromotionRule(ctx, db.PromotionAdminUpsertPromotionRuleParams{
			PromotionID: row.ID,
			RuleType:    strings.TrimSpace(r.RuleType),
			Payload:     payload,
			Priority:    r.Priority,
		}); err != nil {
			return db.Promotion{}, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return db.Promotion{}, err
	}
	s.emitPromotionAudit(ctx, PromotionAuditEvent{OrganizationID: in.OrganizationID, PromotionID: row.ID, Action: "create"})
	return row, nil
}

// PatchPromotion updates mutable fields and optionally replaces rules.
func (s *Service) PatchPromotion(ctx context.Context, organizationID, promotionID uuid.UUID, patch PatchPromotionInput) (db.Promotion, error) {
	if s == nil {
		return db.Promotion{}, errors.New("catalogadmin: nil service")
	}
	if organizationID == uuid.Nil || promotionID == uuid.Nil {
		return db.Promotion{}, ErrOrganizationRequired
	}
	cur, err := s.q.PromotionAdminGetPromotion(ctx, db.PromotionAdminGetPromotionParams{
		OrganizationID: organizationID,
		ID:             promotionID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Promotion{}, ErrNotFound
		}
		return db.Promotion{}, err
	}

	name := cur.Name
	if patch.Name != nil {
		name = strings.TrimSpace(*patch.Name)
		if name == "" {
			return db.Promotion{}, fmt.Errorf("%w: name cannot be empty", ErrInvalidArgument)
		}
	}
	starts := cur.StartsAt
	if patch.StartsAt != nil {
		starts = *patch.StartsAt
	}
	ends := cur.EndsAt
	if patch.EndsAt != nil {
		ends = *patch.EndsAt
	}
	if err := validatePromotionWindow(starts, ends); err != nil {
		return db.Promotion{}, err
	}
	prio := cur.Priority
	if patch.Priority != nil {
		prio = *patch.Priority
	}
	stack := cur.Stackable
	if patch.Stackable != nil {
		stack = *patch.Stackable
	}
	appr := cur.ApprovalStatus
	if patch.ApprovalStatus != nil {
		appr = strings.TrimSpace(*patch.ApprovalStatus)
	}
	life := cur.LifecycleStatus

	budget := cur.BudgetLimitMinor
	if patch.BudgetLimitMinor != nil {
		budget = pgInt8Ptr(patch.BudgetLimitMinor)
	}
	red := cur.RedemptionLimit
	if patch.RedemptionLimit != nil {
		red = pgInt4Ptr(patch.RedemptionLimit)
	}
	ch := cur.ChannelScope
	if patch.ChannelScope != nil {
		ch = pgTextPtr(patch.ChannelScope)
	}

	if patch.Rules != nil {
		for _, r := range *patch.Rules {
			if err := validateRulePayload(r.RuleType, r.Payload); err != nil {
				return db.Promotion{}, err
			}
		}
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return db.Promotion{}, err
	}
	defer tx.Rollback(ctx)
	qtx := s.q.WithTx(tx)

	row, err := qtx.PromotionAdminUpdatePromotion(ctx, db.PromotionAdminUpdatePromotionParams{
		OrganizationID:   organizationID,
		ID:               promotionID,
		Name:             name,
		ApprovalStatus:   appr,
		LifecycleStatus:  life,
		Priority:         prio,
		Stackable:        stack,
		StartsAt:         starts,
		EndsAt:           ends,
		BudgetLimitMinor: budget,
		RedemptionLimit:  red,
		ChannelScope:     ch,
	})
	if err != nil {
		return db.Promotion{}, err
	}

	if patch.Rules != nil {
		if err := qtx.PromotionAdminDeleteRulesForPromotion(ctx, promotionID); err != nil {
			return db.Promotion{}, err
		}
		for _, r := range *patch.Rules {
			payload := r.Payload
			if len(payload) == 0 {
				payload = json.RawMessage([]byte(`{}`))
			}
			if _, err := qtx.PromotionAdminInsertPromotionRule(ctx, db.PromotionAdminInsertPromotionRuleParams{
				PromotionID: promotionID,
				RuleType:    strings.TrimSpace(r.RuleType),
				Payload:     payload,
				Priority:    r.Priority,
			}); err != nil {
				return db.Promotion{}, err
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return db.Promotion{}, err
	}
	s.emitPromotionAudit(ctx, PromotionAuditEvent{OrganizationID: organizationID, PromotionID: promotionID, Action: "update"})
	return row, nil
}

func (s *Service) setLifecycle(ctx context.Context, organizationID, promotionID uuid.UUID, next string, auditAction string) (db.Promotion, error) {
	row, err := s.q.PromotionAdminSetLifecycle(ctx, db.PromotionAdminSetLifecycleParams{
		OrganizationID:  organizationID,
		ID:              promotionID,
		LifecycleStatus: next,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Promotion{}, ErrNotFound
		}
		return db.Promotion{}, err
	}
	s.emitPromotionAudit(ctx, PromotionAuditEvent{OrganizationID: organizationID, PromotionID: promotionID, Action: auditAction, Detail: next})
	return row, nil
}

// ActivatePromotion sets lifecycle active.
func (s *Service) ActivatePromotion(ctx context.Context, organizationID, promotionID uuid.UUID) (db.Promotion, error) {
	if s == nil {
		return db.Promotion{}, errors.New("catalogadmin: nil service")
	}
	return s.setLifecycle(ctx, organizationID, promotionID, "active", "activate")
}

// PausePromotion sets lifecycle paused.
func (s *Service) PausePromotion(ctx context.Context, organizationID, promotionID uuid.UUID) (db.Promotion, error) {
	if s == nil {
		return db.Promotion{}, errors.New("catalogadmin: nil service")
	}
	return s.setLifecycle(ctx, organizationID, promotionID, "paused", "pause")
}

// DeactivatePromotion sets lifecycle deactivated.
func (s *Service) DeactivatePromotion(ctx context.Context, organizationID, promotionID uuid.UUID) (db.Promotion, error) {
	if s == nil {
		return db.Promotion{}, errors.New("catalogadmin: nil service")
	}
	return s.setLifecycle(ctx, organizationID, promotionID, "deactivated", "deactivate")
}

func uuidPg(u *uuid.UUID) pgtype.UUID {
	if u == nil || *u == uuid.Nil {
		return pgtype.UUID{}
	}
	return pgtype.UUID{Bytes: *u, Valid: true}
}

// AssignPromotionTarget inserts a target row for the promotion.
func (s *Service) AssignPromotionTarget(ctx context.Context, in AssignPromotionTargetInput) (db.PromotionTarget, error) {
	if s == nil {
		return db.PromotionTarget{}, errors.New("catalogadmin: nil service")
	}
	if in.OrganizationID == uuid.Nil || in.PromotionID == uuid.Nil {
		return db.PromotionTarget{}, ErrOrganizationRequired
	}
	tt := strings.TrimSpace(strings.ToLower(in.TargetType))
	if tt == "" {
		return db.PromotionTarget{}, fmt.Errorf("%w: target_type required", ErrInvalidArgument)
	}
	row, err := s.q.PromotionAdminInsertPromotionTarget(ctx, db.PromotionAdminInsertPromotionTargetParams{
		PromotionID:          in.PromotionID,
		OrganizationID:       in.OrganizationID,
		TargetType:           tt,
		ProductID:            uuidPg(in.ProductID),
		CategoryID:           uuidPg(in.CategoryID),
		MachineID:            uuidPg(in.MachineID),
		SiteID:               uuidPg(in.SiteID),
		OrganizationTargetID: uuidPg(in.OrgTargetID),
		TagID:                uuidPg(in.TagID),
	})
	if err != nil {
		return db.PromotionTarget{}, err
	}
	s.emitPromotionAudit(ctx, PromotionAuditEvent{OrganizationID: in.OrganizationID, PromotionID: in.PromotionID, Action: "assign_target", Detail: row.ID.String()})
	return row, nil
}

// DeletePromotionTarget removes a target assignment.
func (s *Service) DeletePromotionTarget(ctx context.Context, organizationID, promotionID, targetID uuid.UUID) error {
	if s == nil {
		return errors.New("catalogadmin: nil service")
	}
	if organizationID == uuid.Nil || promotionID == uuid.Nil || targetID == uuid.Nil {
		return ErrOrganizationRequired
	}
	tgt, err := s.q.PromotionAdminGetPromotionTarget(ctx, db.PromotionAdminGetPromotionTargetParams{
		OrganizationID: organizationID,
		ID:             targetID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	if tgt.PromotionID != promotionID {
		return ErrNotFound
	}
	n, err := s.q.PromotionAdminDeletePromotionTarget(ctx, db.PromotionAdminDeletePromotionTargetParams{
		OrganizationID: organizationID,
		ID:             targetID,
	})
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	s.emitPromotionAudit(ctx, PromotionAuditEvent{OrganizationID: organizationID, PromotionID: promotionID, Action: "delete_target", Detail: targetID.String()})
	return nil
}

// ListPromotionTargets lists targets for a promotion.
func (s *Service) ListPromotionTargets(ctx context.Context, organizationID, promotionID uuid.UUID) ([]db.PromotionTarget, error) {
	if s == nil {
		return nil, errors.New("catalogadmin: nil service")
	}
	if organizationID == uuid.Nil || promotionID == uuid.Nil {
		return nil, ErrOrganizationRequired
	}
	return s.q.PromotionAdminListTargetsForPromotion(ctx, db.PromotionAdminListTargetsForPromotionParams{
		OrganizationID: organizationID,
		PromotionID:    promotionID,
	})
}
