package assignmentcatalog

import (
	"testing"
	"time"

	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/google/uuid"
)

func TestImageThumbURLPrefersThumbCdn(t *testing.T) {
	row := db.RuntimeListProductImagesForProductsRow{
		ThumbCdnUrl: "https://cdn.example/thumb.webp",
		CdnUrl:      "https://cdn.example/display.webp",
	}
	if got := imageThumbURL(row); got != "https://cdn.example/thumb.webp" {
		t.Fatalf("thumb url: got %q", got)
	}
}

func TestImageDisplayURLFallsBackToOriginal(t *testing.T) {
	row := db.RuntimeListProductImagesForProductsRow{
		CdnUrl:         "",
		OriginalCdnUrl: "https://adm.avf.vn/product.jpg",
	}
	if got := imageDisplayURL(row); got != "https://adm.avf.vn/product.jpg" {
		t.Fatalf("display url: got %q", got)
	}
}

func TestCatalogVersionChangesWhenThumbURLAdded(t *testing.T) {
	id := uuid.MustParse("0194a1b2-c3d4-7890-abcd-ef1234567890")
	ts := timeNowUTC()
	withoutThumb := []Product{{ID: id, UpdatedAt: ts, ImageHash: "asset-1"}}
	withThumb := []Product{{ID: id, UpdatedAt: ts, ImageHash: "asset-1", ThumbURL: "https://adm.avf.vn/t.jpg"}}
	v1 := CatalogVersionInt(withoutThumb)
	v2 := CatalogVersionInt(withThumb)
	if v1 == v2 {
		t.Fatalf("expected catalog version to change when thumb URL is added")
	}
}

func timeNowUTC() time.Time {
	return time.Date(2026, 3, 14, 10, 0, 0, 0, time.UTC)
}
