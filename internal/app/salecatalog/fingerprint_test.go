package salecatalog

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestMediaFingerprint_reflectsBindingAndTombstone(t *testing.T) {
	t.Parallel()
	pid := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	mid := uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc")

	itemBase := func() Item {
		return Item{
			ProductID: pid,
			SKU:       "SKU1",
			Image: &ImageMeta{
				MediaID:      mid,
				MediaVersion: 2,
				ContentHash:  "sha256:abc",
				ContentType:  "image/webp",
				SizeBytes:    4096,
				Etag:         `W/"etag1"`,
				ThumbURL:     "https://x/t",
				DisplayURL:   "https://x/d",
				UpdatedAt:    time.Unix(1, 0).UTC(),
			},
		}
	}
	base := Snapshot{Items: []Item{itemBase()}}
	fp1 := MediaFingerprint(base)

	etag := Snapshot{Items: []Item{itemBase()}}
	etag.Items[0].Image.Etag = `W/"other"`
	if MediaFingerprint(etag) == fp1 {
		t.Fatal("expected fingerprint to change when etag changes")
	}

	hash := Snapshot{Items: []Item{itemBase()}}
	hash.Items[0].Image.ContentHash = "sha256:def"
	if MediaFingerprint(hash) == fp1 {
		t.Fatal("expected fingerprint to change when content hash changes")
	}

	tomb := Snapshot{Items: []Item{itemBase()}}
	tomb.Items[0].Image = &ImageMeta{Deleted: true, UpdatedAt: time.Unix(2, 0).UTC()}
	if MediaFingerprint(tomb) == fp1 {
		t.Fatal("expected fingerprint to change for tombstone")
	}
}

func TestMediaFingerprint_ignoresPresignedURLRotation(t *testing.T) {
	t.Parallel()
	pid := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	mid := uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc")
	metaA := &ImageMeta{
		MediaID:      mid,
		MediaVersion: 2,
		ContentHash:  "sha256:abc",
		ContentType:  "image/webp",
		SizeBytes:    4096,
		Etag:         `W/"etag1"`,
		ThumbURL:     "https://bucket/presign-a-thumb",
		DisplayURL:   "https://bucket/presign-a-display",
		UpdatedAt:    time.Unix(1, 0).UTC(),
		Variants: []ImageVariantMeta{
			{Kind: MediaVariantKindThumb, MediaAssetID: mid, StorageKey: "org/thumb", ChecksumSHA256: "sha256:abc", Etag: `W/"etag1"`},
			{Kind: MediaVariantKindDisplay, MediaAssetID: mid, StorageKey: "org/display", ChecksumSHA256: "sha256:abc", Etag: `W/"etag1"`},
		},
	}
	metaB := &ImageMeta{
		MediaID:      metaA.MediaID,
		MediaVersion: metaA.MediaVersion,
		ContentHash:  metaA.ContentHash,
		ContentType:  metaA.ContentType,
		SizeBytes:    metaA.SizeBytes,
		Etag:         metaA.Etag,
		ThumbURL:     "https://bucket/presign-b-thumb",
		DisplayURL:   "https://bucket/presign-b-display",
		UpdatedAt:    metaA.UpdatedAt,
		Variants: []ImageVariantMeta{
			{Kind: MediaVariantKindThumb, MediaAssetID: mid, StorageKey: "org/thumb", ChecksumSHA256: "sha256:abc", Etag: `W/"etag1"`},
			{Kind: MediaVariantKindDisplay, MediaAssetID: mid, StorageKey: "org/display", ChecksumSHA256: "sha256:abc", Etag: `W/"etag1"`},
		},
	}
	a := Snapshot{
		Items: []Item{{
			ProductID: pid,
			SKU:       "SKU1",
			Image:     metaA,
		}},
	}
	b := Snapshot{
		Items: []Item{{
			ProductID: pid,
			SKU:       "SKU1",
			Image:     metaB,
		}},
	}
	if MediaFingerprint(a) != MediaFingerprint(b) {
		t.Fatal("fingerprint should be stable when only presigned URLs change (hash/version/etag unchanged)")
	}
}

func TestMediaFingerprint_changesWhenVariantStorageKeyChanges(t *testing.T) {
	t.Parallel()
	pid := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	mid := uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc")
	base := Snapshot{
		Items: []Item{{
			ProductID: pid,
			SKU:       "SKU1",
			Image: &ImageMeta{
				MediaID:      mid,
				MediaVersion: 1,
				ContentHash:  "sha256:x",
				Variants: []ImageVariantMeta{
					{Kind: MediaVariantKindDisplay, MediaAssetID: mid, StorageKey: "a/display", ChecksumSHA256: "sha256:x", Etag: `W/"e"`},
				},
			},
		}},
	}
	changed := Snapshot{
		Items: []Item{{
			ProductID: pid,
			SKU:       "SKU1",
			Image: &ImageMeta{
				MediaID:      mid,
				MediaVersion: 1,
				ContentHash:  "sha256:x",
				Variants: []ImageVariantMeta{
					{Kind: MediaVariantKindDisplay, MediaAssetID: mid, StorageKey: "b/display", ChecksumSHA256: "sha256:x", Etag: `W/"e"`},
				},
			},
		}},
	}
	if MediaFingerprint(base) == MediaFingerprint(changed) {
		t.Fatal("expected fingerprint to change when a variant storage key changes")
	}
}

func TestMediaFingerprint_changesWhenVariantEtagChanges(t *testing.T) {
	t.Parallel()
	pid := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	mid := uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc")
	base := Snapshot{
		Items: []Item{{
			ProductID: pid,
			SKU:       "SKU1",
			Image: &ImageMeta{
				MediaID:      mid,
				MediaVersion: 2,
				Variants: []ImageVariantMeta{
					{Kind: MediaVariantKindDisplay, MediaAssetID: mid, StorageKey: "k/d", ChecksumSHA256: "sha256:abc", Etag: `W/"a"`},
				},
			},
		}},
	}
	changed := Snapshot{
		Items: []Item{{
			ProductID: pid,
			SKU:       "SKU1",
			Image: &ImageMeta{
				MediaID:      mid,
				MediaVersion: 2,
				Variants: []ImageVariantMeta{
					{Kind: MediaVariantKindDisplay, MediaAssetID: mid, StorageKey: "k/d", ChecksumSHA256: "sha256:abc", Etag: `W/"b"`},
				},
			},
		}},
	}
	if MediaFingerprint(base) == MediaFingerprint(changed) {
		t.Fatal("expected fingerprint to change when variant etag changes")
	}
}
