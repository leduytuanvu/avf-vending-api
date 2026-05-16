package httpserver

import (
	"testing"
	"time"

	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestMergePriceBookPatch_companyScopeClearsFKColumns(t *testing.T) {
	t.Parallel()
	org := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	_ = org
	bid := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	site := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	cur := db.PriceBook{
		ID:            bid,
		Name:          "Book",
		Currency:      "USD",
		EffectiveFrom: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		IsDefault:     false,
		Active:        true,
		ScopeType:     "site",
		SiteID:        pgtype.UUID{Bytes: site, Valid: true},
		Priority:      1,
	}
	sc := "company"
	p, err := mergePriceBookPatch(cur, V1AdminPriceBookPatchRequest{ScopeType: &sc})
	if err != nil {
		t.Fatal(err)
	}
	if p.ScopeType != "company" || p.SiteID.Valid || p.MachineID.Valid {
		t.Fatalf("got scope=%q siteValid=%v machValid=%v", p.ScopeType, p.SiteID.Valid, p.MachineID.Valid)
	}
}
