package pricingengine

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

const (
	RulePercentageDiscount  = "percentage_discount"
	RuleFixedAmountDiscount = "fixed_amount_discount"
	RuleBuyXGetY            = "buy_x_get_y"
)

type PromoEvalCtx struct {
	Org        uuid.UUID
	MachineID  *uuid.UUID
	SiteID     *uuid.UUID
	ProductID  uuid.UUID
	CategoryID pgtype.UUID
	TagIDs     []uuid.UUID
}

// NewPromoEvalCtx builds promotion targeting state for EvaluatePromotionDiscountForProduct.
func NewPromoEvalCtx(org uuid.UUID, machineID, siteID *uuid.UUID, productID uuid.UUID, categoryID pgtype.UUID, tagIDs []uuid.UUID) PromoEvalCtx {
	return PromoEvalCtx{
		Org:        org,
		MachineID:  machineID,
		SiteID:     siteID,
		ProductID:  productID,
		CategoryID: categoryID,
		TagIDs:     tagIDs,
	}
}

func promoLess(a, b db.Promotion) bool {
	if a.Priority != b.Priority {
		return a.Priority > b.Priority
	}
	if !a.StartsAt.Equal(b.StartsAt) {
		return a.StartsAt.After(b.StartsAt)
	}
	return a.ID.String() < b.ID.String()
}

// SortPromotions returns a copy sorted for deterministic evaluation (higher priority first).
func SortPromotions(ps []db.Promotion) []db.Promotion {
	out := append([]db.Promotion(nil), ps...)
	sort.SliceStable(out, func(i, j int) bool { return promoLess(out[i], out[j]) })
	return out
}

func targetMatches(ctx PromoEvalCtx, t db.PromotionTarget) bool {
	switch t.TargetType {
	case "organization":
		return t.OrganizationTargetID.Valid && uuid.UUID(t.OrganizationTargetID.Bytes) == ctx.Org
	case "site":
		return ctx.SiteID != nil && t.SiteID.Valid && uuid.UUID(t.SiteID.Bytes) == *ctx.SiteID
	case "machine":
		return ctx.MachineID != nil && t.MachineID.Valid && uuid.UUID(t.MachineID.Bytes) == *ctx.MachineID
	case "product":
		return t.ProductID.Valid && uuid.UUID(t.ProductID.Bytes) == ctx.ProductID
	case "category":
		return ctx.CategoryID.Valid && t.CategoryID.Valid && uuid.UUID(t.CategoryID.Bytes) == uuid.UUID(ctx.CategoryID.Bytes)
	case "tag":
		if !t.TagID.Valid {
			return false
		}
		tid := uuid.UUID(t.TagID.Bytes)
		for _, x := range ctx.TagIDs {
			if x == tid {
				return true
			}
		}
		return false
	default:
		return false
	}
}

func promotionApplicable(ctx PromoEvalCtx, promoID uuid.UUID, targets []db.PromotionTarget) bool {
	var promoTargets []db.PromotionTarget
	for _, t := range targets {
		if t.PromotionID == promoID {
			promoTargets = append(promoTargets, t)
		}
	}
	if len(promoTargets) == 0 {
		return true
	}
	for _, t := range promoTargets {
		if targetMatches(ctx, t) {
			return true
		}
	}
	return false
}

type rulePayloadPct struct {
	Percent          float64 `json:"percent"`
	MaxDiscountMinor *int64  `json:"max_discount_minor"`
}

type rulePayloadFixed struct {
	AmountMinor      int64  `json:"amount_minor"`
	MaxDiscountMinor *int64 `json:"max_discount_minor"`
}

func discountFromRule(basePriceMinor int64, rule db.PromotionRule) (discount int64, skipped []SkippedPromotionRule) {
	switch rule.RuleType {
	case RuleBuyXGetY:
		skipped = append(skipped, SkippedPromotionRule{
			PromotionID: rule.PromotionID,
			RuleID:      rule.ID,
			RuleType:    rule.RuleType,
			Reason:      "rule_type_not_implemented_buy_x_get_y_deferred",
		})
		return 0, skipped
	case RulePercentageDiscount:
		var p rulePayloadPct
		if err := json.Unmarshal(rule.Payload, &p); err != nil || p.Percent <= 0 {
			skipped = append(skipped, SkippedPromotionRule{
				PromotionID: rule.PromotionID,
				RuleID:      rule.ID,
				RuleType:    rule.RuleType,
				Reason:      "invalid_payload_percentage_discount",
			})
			return 0, skipped
		}
		raw := int64(float64(basePriceMinor) * (p.Percent / 100.0))
		d := raw
		if p.MaxDiscountMinor != nil && *p.MaxDiscountMinor >= 0 && d > *p.MaxDiscountMinor {
			d = *p.MaxDiscountMinor
		}
		if d > basePriceMinor {
			d = basePriceMinor
		}
		if d < 0 {
			d = 0
		}
		return d, skipped
	case RuleFixedAmountDiscount:
		var p rulePayloadFixed
		if err := json.Unmarshal(rule.Payload, &p); err != nil || p.AmountMinor <= 0 {
			skipped = append(skipped, SkippedPromotionRule{
				PromotionID: rule.PromotionID,
				RuleID:      rule.ID,
				RuleType:    rule.RuleType,
				Reason:      "invalid_payload_fixed_amount_discount",
			})
			return 0, skipped
		}
		d := p.AmountMinor
		if p.MaxDiscountMinor != nil && *p.MaxDiscountMinor >= 0 && d > *p.MaxDiscountMinor {
			d = *p.MaxDiscountMinor
		}
		if d > basePriceMinor {
			d = basePriceMinor
		}
		return d, skipped
	default:
		skipped = append(skipped, SkippedPromotionRule{
			PromotionID: rule.PromotionID,
			RuleID:      rule.ID,
			RuleType:    rule.RuleType,
			Reason:      "unknown_rule_type",
		})
		return 0, skipped
	}
}

func bestDiscountForPromotion(base int64, rules []db.PromotionRule) (best int64, ruleID uuid.UUID, skipped []SkippedPromotionRule) {
	var bestRule db.PromotionRule
	var have bool
	for _, r := range rules {
		d, sk := discountFromRule(base, r)
		skipped = append(skipped, sk...)
		if d <= 0 && len(sk) > 0 {
			continue
		}
		if !have || d > best {
			best = d
			bestRule = r
			have = true
		}
	}
	if have {
		ruleID = bestRule.ID
	}
	return best, ruleID, skipped
}

func applyPromotionsToBase(
	base int64,
	promos []db.Promotion,
	rulesByPromo map[uuid.UUID][]db.PromotionRule,
	targets []db.PromotionTarget,
	ctx PromoEvalCtx,
	stackChain bool,
) (discount int64, appliedIDs []uuid.UUID, appliedRules []string, skipped []SkippedPromotionRule) {
	sorted := SortPromotions(promos)
	if !stackChain {
		for _, pr := range sorted {
			if !promotionApplicable(ctx, pr.ID, targets) {
				skipped = append(skipped, SkippedPromotionRule{PromotionID: pr.ID, Reason: "target_not_matched"})
				continue
			}
			rs := rulesByPromo[pr.ID]
			d, rid, sk := bestDiscountForPromotion(base, rs)
			skipped = append(skipped, sk...)
			if d <= 0 {
				skipped = append(skipped, SkippedPromotionRule{PromotionID: pr.ID, Reason: "no_discount_from_rules"})
				continue
			}
			discount = d
			appliedIDs = append(appliedIDs, pr.ID)
			if rid != uuid.Nil {
				appliedRules = append(appliedRules, fmt.Sprintf("promotion_rule:%s", rid.String()))
			}
			return discount, appliedIDs, appliedRules, skipped
		}
		return 0, nil, nil, skipped
	}
	remaining := base
	var total int64
	for _, pr := range sorted {
		if !promotionApplicable(ctx, pr.ID, targets) {
			skipped = append(skipped, SkippedPromotionRule{PromotionID: pr.ID, Reason: "target_not_matched"})
			continue
		}
		rs := rulesByPromo[pr.ID]
		d, rid, sk := bestDiscountForPromotion(remaining, rs)
		skipped = append(skipped, sk...)
		if d <= 0 {
			skipped = append(skipped, SkippedPromotionRule{PromotionID: pr.ID, Reason: "no_discount_from_rules"})
			continue
		}
		if d > remaining {
			d = remaining
		}
		total += d
		remaining -= d
		appliedIDs = append(appliedIDs, pr.ID)
		if rid != uuid.Nil {
			appliedRules = append(appliedRules, fmt.Sprintf("promotion_rule:%s", rid.String()))
		}
		if remaining <= 0 {
			break
		}
	}
	return total, appliedIDs, appliedRules, skipped
}

// EvaluatePromotionDiscountForProduct applies stackable / non-stackable promotion rules to a unit base.
func EvaluatePromotionDiscountForProduct(
	baseBeforePromotions int64,
	sorted []db.Promotion,
	rulesByPromo map[uuid.UUID][]db.PromotionRule,
	targets []db.PromotionTarget,
	eCtx PromoEvalCtx,
) (disc int64, appliedIDs []uuid.UUID, appliedRules []string, skipped []SkippedPromotionRule) {
	var matchingNonStack []db.Promotion
	var matchingStack []db.Promotion
	for _, pr := range sorted {
		if !promotionApplicable(eCtx, pr.ID, targets) {
			skipped = append(skipped, SkippedPromotionRule{PromotionID: pr.ID, Reason: "target_not_matched"})
			continue
		}
		if !pr.Stackable {
			matchingNonStack = append(matchingNonStack, pr)
		} else {
			matchingStack = append(matchingStack, pr)
		}
	}
	remaining := baseBeforePromotions
	var totalDisc int64
	var appIDs []uuid.UUID

	if len(matchingNonStack) > 0 {
		d, ids, rr, sk := applyPromotionsToBase(remaining, matchingNonStack, rulesByPromo, targets, eCtx, false)
		skipped = append(skipped, sk...)
		totalDisc += d
		remaining -= d
		if remaining < 0 {
			remaining = 0
		}
		appIDs = append(appIDs, ids...)
		appliedRules = append(appliedRules, rr...)
	}

	if len(matchingStack) > 0 {
		d, ids, rr, sk := applyPromotionsToBase(remaining, matchingStack, rulesByPromo, targets, eCtx, true)
		skipped = append(skipped, sk...)
		totalDisc += d
		appIDs = append(appIDs, ids...)
		appliedRules = append(appliedRules, rr...)
	}

	disc = totalDisc
	appliedIDs = appIDs
	return disc, appliedIDs, appliedRules, skipped
}

// PromotionNameByID builds a lookup from promotion list.
func PromotionNameByID(promos []db.Promotion) map[uuid.UUID]string {
	m := make(map[uuid.UUID]string, len(promos))
	for _, p := range promos {
		m[p.ID] = p.Name
	}
	return m
}
