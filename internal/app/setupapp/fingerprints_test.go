package setupapp

import (
	"testing"

	"github.com/google/uuid"
)

func TestCatalogFingerprint_OrderIndependent(t *testing.T) {
	t.Parallel()
	a := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	b := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	x := MachineBootstrap{
		AssortmentProducts: []AssortmentProductView{
			{ProductID: a, SortOrder: 2, SKU: "A"},
			{ProductID: b, SortOrder: 1, SKU: "B"},
		},
	}
	y := MachineBootstrap{
		AssortmentProducts: []AssortmentProductView{
			{ProductID: b, SortOrder: 1, SKU: "B"},
			{ProductID: a, SortOrder: 2, SKU: "A"},
		},
	}
	if CatalogFingerprint(x) != CatalogFingerprint(y) {
		t.Fatal("expected stable fingerprint regardless of slice order")
	}
}
