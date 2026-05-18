package httpserver

import (
	"testing"
	"time"

	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestMergePriceBookPatch_globalLevelClearsFKColumns(t *testing.T) {
	t.Parallel()
	org := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	_ = org
	bid := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	site := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	cur := db.PriceBook{
		ID:             bid,
		Name:           "Book",
		Currency:       "USD",
		EffectiveFrom:  time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		IsDefault:      false,
		Active:         true,
		PriceBookLevel: "site",
		SiteID:         pgtype.UUID{Bytes: site, Valid: true},
		Priority:       1,
	}
	sc := "global"
	p, err := mergePriceBookPatch(cur, V1AdminPriceBookPatchRequest{PriceBookLevel: &sc})
	if err != nil {
		t.Fatal(err)
	}
	if p.PriceBookLevel != "global" || p.SiteID.Valid || p.MachineID.Valid {
		t.Fatalf("got level=%q siteValid=%v machValid=%v", p.PriceBookLevel, p.SiteID.Valid, p.MachineID.Valid)
	}
}
