package catalogadmin

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/avf/avf-vending-api/internal/app/pricingengine"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/google/uuid"
)

// Re-export promotion rule types for admin validation and docs.
const (
	RulePercentageDiscount  = pricingengine.RulePercentageDiscount
	RuleFixedAmountDiscount = pricingengine.RuleFixedAmountDiscount
	RuleBuyXGetY            = pricingengine.RuleBuyXGetY
)

// SkippedPromotionRule explains why a rule did not apply.
type SkippedPromotionRule = pricingengine.SkippedPromotionRule

// PromotionPreviewParams is POST /v1/admin/promotions/preview.
type PromotionPreviewParams struct {
	OrganizationID uuid.UUID
	MachineID      *uuid.UUID
	SiteID         *uuid.UUID
	ProductIDs     []uuid.UUID
	At             time.Time
}

// PromotionPreviewLine is one product row after promotions.
type PromotionPreviewLine struct {
	ProductID           uuid.UUID              `json:"productId"`
	BasePriceMinor      int64                  `json:"basePriceMinor"`
	DiscountMinor       int64                  `json:"discountMinor"`
	FinalPriceMinor     int64                  `json:"finalPriceMinor"`
	Currency            string                 `json:"currency"`
	AppliedPromotionIDs []uuid.UUID            `json:"appliedPromotionIds"`
	AppliedRuleIDs      []string               `json:"appliedRuleIds"`
	SkippedRules        []SkippedPromotionRule `json:"skippedRules"`
}

// PromotionPreviewResult is the promotion preview envelope.
type PromotionPreviewResult struct {
	At    time.Time              `json:"at"`
	Lines []PromotionPreviewLine `json:"lines"`
}

func firstSlotRowForProduct(orgID, machineID, productID uuid.UUID, rows []db.InventoryAdminListCurrentMachineSlotConfigsByMachineRow) (db.InventoryAdminListCurrentMachineSlotConfigsByMachineRow, bool) {
	for _, r := range rows {
		if r.OrganizationID != orgID {
			continue
		}
		if !r.ProductID.Valid {
			continue
		}
		if uuid.UUID(r.ProductID.Bytes) != productID {
			continue
		}
		return r, true
	}
	return db.InventoryAdminListCurrentMachineSlotConfigsByMachineRow{}, false
}

// PreviewPromotions evaluates promotions against runtime pricing (machine slot + overrides) when MachineID is set,
// otherwise against catalog price-book preview bases (legacy admin-only scope).
func (s *Service) PreviewPromotions(ctx context.Context, p PromotionPreviewParams) (*PromotionPreviewResult, error) {
	if s == nil {
		return nil, errors.New("catalogadmin: nil service")
	}
	if p.OrganizationID == uuid.Nil {
		return nil, ErrOrganizationRequired
	}
	if len(p.ProductIDs) == 0 {
		return nil, fmt.Errorf("%w: product_ids required", ErrInvalidArgument)
	}
	at := p.At
	if at.IsZero() {
		at = time.Now().UTC()
	} else {
		at = at.UTC()
	}
	eng := pricingengine.New(s.pool)

	if p.MachineID != nil && *p.MachineID != uuid.Nil {
		rows, err := s.q.InventoryAdminListCurrentMachineSlotConfigsByMachine(ctx, *p.MachineID)
		if err != nil {
			return nil, err
		}
		batch, err := eng.NewBatch(ctx, p.OrganizationID, *p.MachineID, at)
		if err != nil {
			return nil, err
		}
		out := &PromotionPreviewResult{At: at}
		for _, pid := range p.ProductIDs {
			row, ok := firstSlotRowForProduct(p.OrganizationID, *p.MachineID, pid, rows)
			if !ok {
				continue
			}
			if !row.SlotIndex.Valid {
				continue
			}
			pl, err := batch.PriceLine(ctx, pricingengine.PriceLineInput{
				OrganizationID:    p.OrganizationID,
				MachineID:         *p.MachineID,
				ProductID:         pid,
				SlotListUnitMinor: row.PriceMinor,
				SlotConfigID:      row.ID,
				CabinetCode:       row.CabinetCode,
				SlotCode:          row.SlotCode,
				SlotIndex:         row.SlotIndex.Int32,
				Quantity:          1,
			})
			if err != nil {
				return nil, err
			}
			out.Lines = append(out.Lines, PromotionPreviewLine{
				ProductID:           pid,
				BasePriceMinor:      pl.RegisterUnitMinor,
				DiscountMinor:       pl.DiscountUnitMinor,
				FinalPriceMinor:     pl.EffectiveUnitMinor,
				Currency:            pl.Currency,
				AppliedPromotionIDs: append([]uuid.UUID(nil), pl.AppliedPromotionIDs...),
			})
		}
		return out, nil
	}

	pricePrev, err := s.PreviewPricing(ctx, PricingPreviewParams{
		OrganizationID: p.OrganizationID,
		MachineID:      p.MachineID,
		SiteID:         p.SiteID,
		ProductIDs:     p.ProductIDs,
		At:             at,
	})
	if err != nil {
		return nil, err
	}

	promos, err := s.q.PromotionAdminListPromotionsForPreview(ctx, db.PromotionAdminListPromotionsForPreviewParams{
		OrganizationID: p.OrganizationID,
		Column2:        at,
	})
	if err != nil {
		return nil, err
	}

	baseByProduct := map[uuid.UUID]PricingPreviewLine{}
	for _, ln := range pricePrev.Lines {
		baseByProduct[ln.ProductID] = ln
	}

	out := &PromotionPreviewResult{At: at}
	sorted := pricingengine.SortPromotions(promos)

	var ids []uuid.UUID
	for _, x := range sorted {
		ids = append(ids, x.ID)
	}
	var ruleRows []db.PromotionRule
	var tgtRows []db.PromotionTarget
	if len(ids) > 0 {
		ruleRows, err = s.q.PromotionAdminListRulesForPromotions(ctx, ids)
		if err != nil {
			return nil, err
		}
		tgtRows, err = s.q.PromotionAdminListTargetsForOrgPromotions(ctx, db.PromotionAdminListTargetsForOrgPromotionsParams{
			OrganizationID: p.OrganizationID,
			Column2:        ids,
		})
		if err != nil {
			return nil, err
		}
	}
	rulesByPromo := map[uuid.UUID][]db.PromotionRule{}
	for _, r := range ruleRows {
		rulesByPromo[r.PromotionID] = append(rulesByPromo[r.PromotionID], r)
	}

	for _, pid := range p.ProductIDs {
		pln, ok := baseByProduct[pid]
		if !ok {
			continue
		}
		base := pln.EffectiveMinor

		cat, err := s.q.PromotionAdminGetProductCategory(ctx, db.PromotionAdminGetProductCategoryParams{
			OrganizationID: p.OrganizationID,
			ID:             pid,
		})
		if err != nil {
			return nil, err
		}
		tagRows, err := s.q.PromotionAdminListProductTagIDs(ctx, db.PromotionAdminListProductTagIDsParams{
			OrganizationID: p.OrganizationID,
			ProductID:      pid,
		})
		if err != nil {
			return nil, err
		}
		tags := append([]uuid.UUID(nil), tagRows...)

		eCtx := pricingengine.NewPromoEvalCtx(p.OrganizationID, p.MachineID, p.SiteID, pid, cat, tags)

		var disc int64
		var appIDs []uuid.UUID
		var ruleRefs []string
		var skipped []SkippedPromotionRule
		if len(sorted) > 0 {
			disc, appIDs, ruleRefs, skipped = pricingengine.EvaluatePromotionDiscountForProduct(base, sorted, rulesByPromo, tgtRows, eCtx)
		}

		final := base - disc
		if final < 0 {
			final = 0
		}
		out.Lines = append(out.Lines, PromotionPreviewLine{
			ProductID:           pid,
			BasePriceMinor:      pln.BasePriceMinor,
			DiscountMinor:       disc,
			FinalPriceMinor:     final,
			Currency:            pln.Currency,
			AppliedPromotionIDs: appIDs,
			AppliedRuleIDs:      ruleRefs,
			SkippedRules:        skipped,
		})
	}

	return out, nil
}
