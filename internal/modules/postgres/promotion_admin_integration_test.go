package postgres_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	appcatalogadmin "github.com/avf/avf-vending-api/internal/app/catalogadmin"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/avf/avf-vending-api/internal/testfixtures"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestPromotion_adminCRUD_preview_and_tenantIsolation(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	q := db.New(pool)
	svc, err := appcatalogadmin.NewService(q, pool, nil)
	require.NoError(t, err)

	org := testfixtures.DevOrganizationID
	at := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
	starts := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	ends := time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)

	promoLow, err := svc.CreatePromotion(ctx, appcatalogadmin.CreatePromotionInput{
		OrganizationID: org,
		Name:           "Low priority pct",
		StartsAt:       starts,
		EndsAt:         ends,
		Priority:       1,
		Stackable:      false,
		Rules: []appcatalogadmin.PromotionRuleInput{
			{
				RuleType: appcatalogadmin.RulePercentageDiscount,
				Priority: 0,
				Payload:  json.RawMessage(`{"percent":5}`),
			},
		},
	})
	require.NoError(t, err)

	promoHigh, err := svc.CreatePromotion(ctx, appcatalogadmin.CreatePromotionInput{
		OrganizationID: org,
		Name:           "High priority pct",
		StartsAt:       starts,
		EndsAt:         ends,
		Priority:       10,
		Stackable:      false,
		Rules: []appcatalogadmin.PromotionRuleInput{
			{
				RuleType: appcatalogadmin.RulePercentageDiscount,
				Priority: 0,
				Payload:  json.RawMessage(`{"percent":25}`),
			},
		},
	})
	require.NoError(t, err)

	_, err = svc.PatchPromotion(ctx, org, promoHigh.ID, appcatalogadmin.PatchPromotionInput{
		Name: ptrStr("High priority pct renamed"),
	})
	require.NoError(t, err)

	for _, p := range []uuid.UUID{promoLow.ID, promoHigh.ID} {
		_, err := svc.ActivatePromotion(ctx, org, p)
		require.NoError(t, err)
	}

	fixAmt, err := svc.CreatePromotion(ctx, appcatalogadmin.CreatePromotionInput{
		OrganizationID: org,
		Name:           "Fixed 500 minor",
		StartsAt:       starts,
		EndsAt:         ends,
		Priority:       20,
		Stackable:      false,
		Rules: []appcatalogadmin.PromotionRuleInput{
			{
				RuleType: appcatalogadmin.RuleFixedAmountDiscount,
				Priority: 0,
				Payload:  json.RawMessage(`{"amount_minor":500}`),
			},
		},
	})
	require.NoError(t, err)
	_, err = svc.ActivatePromotion(ctx, org, fixAmt.ID)
	require.NoError(t, err)

	tgt, err := svc.AssignPromotionTarget(ctx, appcatalogadmin.AssignPromotionTargetInput{
		OrganizationID: org,
		PromotionID:    fixAmt.ID,
		TargetType:     "organization",
		OrgTargetID:    &org,
	})
	require.NoError(t, err)
	require.NoError(t, svc.DeletePromotionTarget(ctx, org, fixAmt.ID, tgt.ID))

	_, err = svc.DeactivatePromotion(ctx, org, fixAmt.ID)
	require.NoError(t, err)

	prev, err := svc.PreviewPromotions(ctx, appcatalogadmin.PromotionPreviewParams{
		OrganizationID: org,
		ProductIDs:     []uuid.UUID{testfixtures.DevProductWater},
		At:             at,
	})
	require.NoError(t, err)
	require.Len(t, prev.Lines, 1)
	require.Contains(t, prev.Lines[0].AppliedPromotionIDs, promoHigh.ID)
	require.NotContains(t, prev.Lines[0].AppliedPromotionIDs, promoLow.ID)

	_, err = svc.PausePromotion(ctx, org, promoHigh.ID)
	require.NoError(t, err)
	prevPaused, err := svc.PreviewPromotions(ctx, appcatalogadmin.PromotionPreviewParams{
		OrganizationID: org,
		ProductIDs:     []uuid.UUID{testfixtures.DevProductWater},
		At:             at,
	})
	require.NoError(t, err)
	require.Len(t, prevPaused.Lines, 1)
	require.NotContains(t, prevPaused.Lines[0].AppliedPromotionIDs, promoHigh.ID)

	_, err = svc.ActivatePromotion(ctx, org, promoHigh.ID)
	require.NoError(t, err)
	_, err = svc.DeactivatePromotion(ctx, org, promoHigh.ID)
	require.NoError(t, err)
	prevDeact, err := svc.PreviewPromotions(ctx, appcatalogadmin.PromotionPreviewParams{
		OrganizationID: org,
		ProductIDs:     []uuid.UUID{testfixtures.DevProductWater},
		At:             at,
	})
	require.NoError(t, err)
	require.Len(t, prevDeact.Lines, 1)
	require.NotContains(t, prevDeact.Lines[0].AppliedPromotionIDs, promoHigh.ID)

	wrongOrg := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	_, err = svc.GetPromotion(ctx, wrongOrg, promoLow.ID)
	require.ErrorIs(t, err, appcatalogadmin.ErrNotFound)
}

func ptrStr(s string) *string { return &s }
