package pricingengine

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/google/uuid"
)

func TestDiscountFromRule_percentage(t *testing.T) {
	t.Parallel()
	pid := uuid.MustParse("10101010-1010-1010-1010-101010101010")
	rid := uuid.MustParse("20202020-2020-2020-2020-202020202020")
	payload, _ := json.Marshal(map[string]any{"percent": 10.0})
	rule := db.PromotionRule{
		ID:          rid,
		PromotionID: pid,
		RuleType:    RulePercentageDiscount,
		Payload:     payload,
		Priority:    0,
	}
	d, skipped := discountFromRule(1000, rule)
	if d != 100 || len(skipped) != 0 {
		t.Fatalf("got discount=%d skipped=%v", d, skipped)
	}
}

func TestDiscountFromRule_fixedAmount(t *testing.T) {
	t.Parallel()
	pid := uuid.MustParse("10101010-1010-1010-1010-101010101010")
	rid := uuid.MustParse("20202020-2020-2020-2020-202020202020")
	payload, _ := json.Marshal(map[string]any{"amount_minor": 50.0})
	rule := db.PromotionRule{
		ID:          rid,
		PromotionID: pid,
		RuleType:    RuleFixedAmountDiscount,
		Payload:     payload,
		Priority:    0,
	}
	d, skipped := discountFromRule(1000, rule)
	if d != 50 || len(skipped) != 0 {
		t.Fatalf("got discount=%d skipped=%v", d, skipped)
	}
}

func TestEvaluatePromotionDiscount_priorityNonStackable(t *testing.T) {
	t.Parallel()
	pLow := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	pHigh := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	rLow := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	rHigh := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	pl := json.RawMessage(`{"percent":5}`)
	ph := json.RawMessage(`{"percent":20}`)
	sorted := []db.Promotion{
		{ID: pLow, Priority: 1, Stackable: false, StartsAt: t0},
		{ID: pHigh, Priority: 10, Stackable: false, StartsAt: t0},
	}
	rules := []db.PromotionRule{
		{ID: rLow, PromotionID: pLow, RuleType: RulePercentageDiscount, Payload: pl, Priority: 0},
		{ID: rHigh, PromotionID: pHigh, RuleType: RulePercentageDiscount, Payload: ph, Priority: 0},
	}
	rulesByPromo := map[uuid.UUID][]db.PromotionRule{
		pLow:  {rules[0]},
		pHigh: {rules[1]},
	}
	prod := uuid.MustParse("99999999-9999-9999-9999-999999999999")
	ctx := PromoEvalCtx{Org: uuid.Nil, ProductID: prod}
	disc, appIDs, _, _ := EvaluatePromotionDiscountForProduct(1000, sorted, rulesByPromo, nil, ctx)
	if disc != 200 {
		t.Fatalf("expected 200 minor discount, got %d", disc)
	}
	if len(appIDs) != 1 || appIDs[0] != pHigh {
		t.Fatalf("expected higher-priority promotion applied, got %#v", appIDs)
	}
}

func TestBuyXGetY_deferredSkipsWithReason(t *testing.T) {
	t.Parallel()
	pid := uuid.MustParse("10101010-1010-1010-1010-101010101010")
	rid := uuid.MustParse("20202020-2020-2020-2020-202020202020")
	rule := db.PromotionRule{
		ID:          rid,
		PromotionID: pid,
		RuleType:    RuleBuyXGetY,
		Payload:     json.RawMessage(`{}`),
		Priority:    0,
	}
	d, skipped := discountFromRule(1000, rule)
	if d != 0 {
		t.Fatalf("expected 0 discount, got %d", d)
	}
	if len(skipped) != 1 || skipped[0].Reason != "rule_type_not_implemented_buy_x_get_y_deferred" {
		t.Fatalf("unexpected skipped: %#v", skipped)
	}
}
