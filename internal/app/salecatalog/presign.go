package salecatalog

import (
	"context"
	"strings"
	"time"

	"github.com/avf/avf-vending-api/internal/platform/objectstore"
)

// RefreshPresignedProductMediaURLs replaces thumb/display/original URLs with fresh presigned GET URLs when a store is wired.
// It sets URLExpiresAt on each ImageMeta for gRPC clients that need HTTPS cache invalidation hints.
// Callers should invoke this for **GetMediaManifest** and for **sale catalog snapshots** (GetCatalogSnapshot/GetCatalogDelta)
// so kiosk clients never reuse expired presigned URLs that were persisted at product-bind time.
func RefreshPresignedProductMediaURLs(ctx context.Context, store objectstore.Store, ttl time.Duration, snap *Snapshot) {
	if store == nil || ttl <= 0 || snap == nil {
		return
	}
	exp := time.Now().UTC().Add(ttl)
	for i := range snap.Items {
		im := snap.Items[i].Image
		if im == nil || im.Deleted {
			continue
		}
		if len(im.Variants) > 0 {
			for vi := range im.Variants {
				v := &im.Variants[vi]
				sk := strings.TrimSpace(v.StorageKey)
				if sk == "" {
					continue
				}
				if signed, err := store.PresignGet(ctx, sk, ttl); err == nil {
					v.URL = signed.URL
				}
			}
			for _, v := range im.Variants {
				switch v.Kind {
				case MediaVariantKindThumb:
					if v.URL != "" {
						im.ThumbURL = v.URL
					}
				case MediaVariantKindDisplay:
					if v.URL != "" {
						im.DisplayURL = v.URL
					}
				case MediaVariantKindOriginal:
					if v.URL != "" {
						im.OriginalURL = v.URL
					}
				}
			}
		} else {
			tk := strings.TrimSpace(im.ThumbStorageKey)
			dk := strings.TrimSpace(im.DisplayStorageKey)
			if tk != "" {
				if signed, err := store.PresignGet(ctx, tk, ttl); err == nil {
					im.ThumbURL = signed.URL
				}
			}
			if dk != "" {
				if signed, err := store.PresignGet(ctx, dk, ttl); err == nil {
					im.DisplayURL = signed.URL
				}
			}
			ok := strings.TrimSpace(im.OriginalStorageKey)
			if ok != "" {
				if signed, err := store.PresignGet(ctx, ok, ttl); err == nil {
					im.OriginalURL = signed.URL
				}
			}
		}
		im.URLExpiresAt = exp
	}
}
