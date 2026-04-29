package salecatalog

import (
	"testing"
	"time"

	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestBuildMediaVariants_includesDisplayThumbAndOriginalWhenDistinct(t *testing.T) {
	t.Parallel()
	pid := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	imgID := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	mid := uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc")
	row := db.RuntimeListProductImagesForProductsRow{
		ID:                imgID,
		ProductID:         pid,
		StorageKey:        "sk",
		CdnUrl:            "https://cdn/display.webp",
		ThumbCdnUrl:       "https://cdn/thumb.webp",
		OriginalCdnUrl:    "https://cdn/original.bin",
		ContentHash:       pgtype.Text{String: "sha256:proj", Valid: true},
		MimeType:          pgtype.Text{String: "image/webp", Valid: true},
		IsPrimary:         true,
		CreatedAt:         time.Unix(1, 0).UTC(),
		MediaVersion:      4,
		UpdatedAt:         time.Unix(2, 0).UTC(),
		MediaAssetID:      pgtype.UUID{Bytes: mid, Valid: true},
		AssetSha256:       pgtype.Text{String: "aaaabbbb", Valid: true},
		OriginalObjectKey: "o/k",
		ThumbObjectKey:    "t/k",
		DisplayObjectKey:  "d/k",
	}
	v := buildMediaVariants(row, mid, row.ThumbCdnUrl, row.CdnUrl, "image/webp", 0, 0, 100, 4, row.UpdatedAt.UTC(), productImageContentHash(row))
	if len(v) < 3 {
		t.Fatalf("want 3 variants, got %d", len(v))
	}
	if v[0].Kind != MediaVariantKindOriginal || v[1].Kind != MediaVariantKindThumb || v[2].Kind != MediaVariantKindDisplay {
		t.Fatalf("unexpected order/kinds: %+v", v)
	}
	for _, e := range v {
		if e.MediaAssetID != mid {
			t.Fatalf("media asset id: %v", e.MediaAssetID)
		}
		if e.ChecksumSHA256 == "" || e.MediaVersion != 4 {
			t.Fatalf("variant %d missing hash/version: %#v", e.Kind, e)
		}
		if e.URL == "" {
			t.Fatalf("variant %d missing url", e.Kind)
		}
	}
}

func TestBuildMediaVariants_skipsRedundantOriginalWhenSameAsDisplay(t *testing.T) {
	t.Parallel()
	pid := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	imgID := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	mid := uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc")
	row := db.RuntimeListProductImagesForProductsRow{
		ID:               imgID,
		ProductID:        pid,
		StorageKey:       "sk",
		CdnUrl:           "https://cdn/same.webp",
		ThumbCdnUrl:      "https://cdn/thumb.webp",
		OriginalCdnUrl:   "https://cdn/same.webp",
		ContentHash:      pgtype.Text{String: "sha256:x", Valid: true},
		CreatedAt:        time.Unix(1, 0).UTC(),
		MediaVersion:     1,
		UpdatedAt:        time.Unix(2, 0).UTC(),
		MediaAssetID:     pgtype.UUID{Bytes: mid, Valid: true},
		ThumbObjectKey:   "t/k",
		DisplayObjectKey: "d/k",
	}
	v := buildMediaVariants(row, mid, row.ThumbCdnUrl, row.CdnUrl, "", 0, 0, 0, 1, row.UpdatedAt.UTC(), productImageContentHash(row))
	if len(v) != 2 {
		t.Fatalf("want thumb+display only, got %d %+v", len(v), v)
	}
}
