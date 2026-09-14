package assignmentcatalog

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestCatalogVersionIntStable(t *testing.T) {
	id := uuid.MustParse("0194a1b2-c3d4-7890-abcd-ef1234567890")
	ts := time.Date(2026, 3, 14, 10, 0, 0, 0, time.UTC)
	products := []Product{
		{ID: id, UpdatedAt: ts, ImageHash: "abc"},
	}
	v1 := CatalogVersionInt(products)
	v2 := CatalogVersionInt(products)
	if v1 <= 0 {
		t.Fatalf("expected positive version, got %d", v1)
	}
	if v1 != v2 {
		t.Fatalf("expected stable version, got %d vs %d", v1, v2)
	}
}

func TestBuildBootstrapBundleZipContainsManifest(t *testing.T) {
	snap := &Snapshot{
		MachineID:      uuid.Nil,
		CatalogVersion: 42,
		GeneratedAt:    time.Now().UTC(),
		Products: []Product{
			{ID: uuid.New(), SKU: "SKU1", Name: "Product 1", Active: true, Currency: "VND"},
		},
	}
	data, err := BuildBootstrapBundleZip(snap, "thumb")
	if err != nil {
		t.Fatalf("BuildBootstrapBundleZip: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("expected non-empty zip")
	}
}
