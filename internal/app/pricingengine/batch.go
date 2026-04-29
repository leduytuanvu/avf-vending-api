package pricingengine

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/avf/avf-vending-api/internal/app/setupapp"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/google/uuid"
)

type Batch struct {
	q            *db.Queries
	orgID        uuid.UUID
	machineID    uuid.UUID
	siteID       uuid.UUID
	at           time.Time
	currency     string
	sortedPromos []db.Promotion
	rulesByPromo map[uuid.UUID][]db.PromotionRule
	targets      []db.PromotionTarget
	promoNames   map[uuid.UUID]string

	overridePrice map[uuid.UUID]int64
	overrideCur   map[uuid.UUID]string
}

// NewBatch loads promotion state and machine price overrides once for many catalog lines.
func (e *Engine) NewBatch(ctx context.Context, orgID, machineID uuid.UUID, at time.Time) (*Batch, error) {
	return e.newBatch(ctx, orgID, machineID, at)
}

func (e *Engine) newBatch(ctx context.Context, orgID, machineID uuid.UUID, at time.Time) (*Batch, error) {
	if e == nil || e.pool == nil {
		return nil, fmt.Errorf("pricingengine: nil engine")
	}
	at = at.UTC()
	q := db.New(e.pool)
	cur, err := q.InventoryAdminGetOrgDefaultCurrency(ctx, orgID)
	if err != nil {
		return nil, err
	}
	currency := strings.ToUpper(strings.TrimSpace(cur))
	if currency == "" {
		currency = "USD"
	}
	siteID, err := q.CatalogAdminGetMachineSiteForOrg(ctx, db.CatalogAdminGetMachineSiteForOrgParams{
		OrganizationID: orgID,
		ID:             machineID,
	})
	if err != nil {
		return nil, err
	}
	ovRows, err := q.PricingRuntimeListMachineOverridesAt(ctx, db.PricingRuntimeListMachineOverridesAtParams{
		OrganizationID: orgID,
		MachineID:      machineID,
		EvalAt:         at,
	})
	if err != nil {
		return nil, err
	}
	overridePrice := make(map[uuid.UUID]int64, len(ovRows))
	overrideCur := make(map[uuid.UUID]string, len(ovRows))
	for _, r := range ovRows {
		overridePrice[r.ProductID] = r.UnitPriceMinor
		overrideCur[r.ProductID] = strings.ToUpper(strings.TrimSpace(r.Currency))
	}
	promos, err := q.PromotionAdminListPromotionsForPreview(ctx, db.PromotionAdminListPromotionsForPreviewParams{
		OrganizationID: orgID,
		Column2:        at,
	})
	if err != nil {
		return nil, err
	}
	sortedPromos := SortPromotions(promos)
	ids := make([]uuid.UUID, 0, len(sortedPromos))
	for _, p := range sortedPromos {
		ids = append(ids, p.ID)
	}
	var ruleRows []db.PromotionRule
	var tgtRows []db.PromotionTarget
	if len(ids) > 0 {
		ruleRows, err = q.PromotionAdminListRulesForPromotions(ctx, ids)
		if err != nil {
			return nil, err
		}
		tgtRows, err = q.PromotionAdminListTargetsForOrgPromotions(ctx, db.PromotionAdminListTargetsForOrgPromotionsParams{
			OrganizationID: orgID,
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
	return &Batch{
		q:             q,
		orgID:         orgID,
		machineID:     machineID,
		siteID:        siteID,
		at:            at,
		currency:      currency,
		sortedPromos:  sortedPromos,
		rulesByPromo:  rulesByPromo,
		targets:       tgtRows,
		promoNames:    PromotionNameByID(sortedPromos),
		overridePrice: overridePrice,
		overrideCur:   overrideCur,
	}, nil
}

// PriceLine applies machine overrides and active promotions to one slot line.
func (b *Batch) PriceLine(ctx context.Context, in PriceLineInput) (LinePriceResult, error) {
	out := LinePriceResult{}
	if b == nil {
		return out, fmt.Errorf("pricingengine: nil batch")
	}
	if in.OrganizationID != b.orgID || in.MachineID != b.machineID {
		return out, fmt.Errorf("pricingengine: batch org/machine mismatch")
	}
	qty := in.Quantity
	if qty <= 0 {
		qty = 1
	}
	slotList := in.SlotListUnitMinor
	register := slotList
	trace := make([]string, 0, 6)
	trace = append(trace, "slot_list:"+strconv.FormatInt(slotList, 10))
	if ov, ok := b.overridePrice[in.ProductID]; ok {
		oc := strings.ToUpper(strings.TrimSpace(b.overrideCur[in.ProductID]))
		if oc != "" && oc != b.currency {
			return out, fmt.Errorf("pricingengine: machine price override currency %s does not match org currency %s", oc, b.currency)
		}
		register = ov
		trace = append(trace, "machine_override:"+strconv.FormatInt(ov, 10))
	}
	machPtr := &b.machineID
	sitePtr := &b.siteID
	cat, err := b.q.PromotionAdminGetProductCategory(ctx, db.PromotionAdminGetProductCategoryParams{
		OrganizationID: in.OrganizationID,
		ID:             in.ProductID,
	})
	if err != nil {
		return out, err
	}
	tagRows, err := b.q.PromotionAdminListProductTagIDs(ctx, db.PromotionAdminListProductTagIDsParams{
		OrganizationID: in.OrganizationID,
		ProductID:      in.ProductID,
	})
	if err != nil {
		return out, err
	}
	tags := append([]uuid.UUID(nil), tagRows...)
	eCtx := NewPromoEvalCtx(in.OrganizationID, machPtr, sitePtr, in.ProductID, cat, tags)
	var disc int64
	var applied []uuid.UUID
	var appliedRules []string
	if len(b.sortedPromos) > 0 {
		disc, applied, appliedRules, _ = EvaluatePromotionDiscountForProduct(register, b.sortedPromos, b.rulesByPromo, b.targets, eCtx)
	}
	_ = appliedRules // reserved for extended audit payloads
	trace = append(trace, "promo_discount:"+strconv.FormatInt(disc, 10))
	eff := register - disc
	if eff < 0 {
		eff = 0
	}
	sub := eff * int64(qty)
	label := ""
	if len(applied) > 0 {
		if n, ok := b.promoNames[applied[0]]; ok {
			label = n
		}
	}
	fp := LinePricingFingerprint(in.OrganizationID, in.MachineID, in.SlotConfigID, in.SlotIndex, in.ProductID, slotList, register, eff, applied)
	out = LinePriceResult{
		SlotListUnitMinor:   slotList,
		RegisterUnitMinor:   register,
		DiscountUnitMinor:   disc,
		EffectiveUnitMinor:  eff,
		SubtotalMinor:       sub,
		TaxMinor:            0,
		TotalMinor:          sub,
		Currency:            b.currency,
		AppliedPromotionIDs: applied,
		PromotionLabel:      label,
		PricingFingerprint:  fp,
		CalcTrace:           trace,
	}
	return out, nil
}

// LinePricingFingerprint hashes inputs that affect the charged unit for this line.
func LinePricingFingerprint(orgID, machineID, slotCfg uuid.UUID, slotIdx int32, productID uuid.UUID, slotList, register, eff int64, promoIDs []uuid.UUID) string {
	parts := []string{
		"org:" + orgID.String(),
		"mach:" + machineID.String(),
		"slot_cfg:" + slotCfg.String(),
		"idx:" + strconv.FormatInt(int64(slotIdx), 10),
		"prod:" + productID.String(),
		"slot_list:" + strconv.FormatInt(slotList, 10),
		"reg:" + strconv.FormatInt(register, 10),
		"eff:" + strconv.FormatInt(eff, 10),
	}
	for _, id := range promoIDs {
		parts = append(parts, "promo:"+id.String())
	}
	return setupapp.SortedKeyFingerprint("price_line_v1", parts)
}

// NowUTC is the default evaluation clock inside the API.
func NowUTC() time.Time { return time.Now().UTC() }
