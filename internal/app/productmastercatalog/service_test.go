package productmastercatalog

import (
	"testing"

	"github.com/google/uuid"
)

func TestCatalogVersion_deterministicForSameProducts(t *testing.T) {
	id := uuid.MustParse("00000000-0000-4000-8000-000000000001")
	records := []Record{
		{
			ProductID:      id,
			SKU:            "SKU-1",
			Name:           "Product 1",
			Active:         true,
			BasePriceMinor: 10000,
			Currency:       "VND",
		},
	}
	v1 := catalogVersion(records)
	v2 := catalogVersion(records)
	if v1 == "" || v1 != v2 {
		t.Fatalf("expected stable version, got %q and %q", v1, v2)
	}
}

func TestCatalogVersion_changesWhenMembershipChanges(t *testing.T) {
	id1 := uuid.MustParse("00000000-0000-4000-8000-000000000001")
	id2 := uuid.MustParse("00000000-0000-4000-8000-000000000002")
	base := []Record{{ProductID: id1, SKU: "A", Name: "A", Active: true, Currency: "VND"}}
	extended := []Record{
		{ProductID: id1, SKU: "A", Name: "A", Active: true, Currency: "VND"},
		{ProductID: id2, SKU: "B", Name: "B", Active: true, Currency: "VND"},
	}
	if catalogVersion(base) == catalogVersion(extended) {
		t.Fatal("expected version to change when product membership changes")
	}
}
