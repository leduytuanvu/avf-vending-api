package salecatalog

import (
	"context"
	"strings"
	"time"

	"github.com/avf/avf-vending-api/internal/platform/objectstore"
	"golang.org/x/sync/errgroup"
)

const presignRefreshSkew = 60 * time.Second
const presignMaxConcurrent = 8

// RefreshPresignedProductMediaURLs replaces thumb/display/original URLs with fresh presigned GET URLs when a store is wired.
// It sets URLExpiresAt on each ImageMeta for gRPC clients that need HTTPS cache invalidation hints.
func RefreshPresignedProductMediaURLs(ctx context.Context, store objectstore.Store, ttl time.Duration, snap *Snapshot) {
	if store == nil || ttl <= 0 || snap == nil {
		return
	}
	if snapshotMediaURLsStillValid(snap, time.Now().UTC()) {
		return
	}
	exp := time.Now().UTC().Add(ttl)
	var g errgroup.Group
	g.SetLimit(presignMaxConcurrent)
	for i := range snap.Items {
		im := snap.Items[i].Image
		if im == nil || im.Deleted {
			continue
		}
		idx := i
		g.Go(func() error {
			refreshOneProductImage(ctx, store, ttl, snap.Items[idx].Image)
			return nil
		})
	}
	_ = g.Wait()
	for i := range snap.Items {
		im := snap.Items[i].Image
		if im != nil && !im.Deleted {
			im.URLExpiresAt = exp
		}
	}
}

func snapshotMediaURLsStillValid(snap *Snapshot, now time.Time) bool {
	deadline := now.Add(presignRefreshSkew)
	for _, it := range snap.Items {
		im := it.Image
		if im == nil || im.Deleted {
			continue
		}
		if im.URLExpiresAt.IsZero() || !im.URLExpiresAt.After(deadline) {
			return false
		}
	}
	return true
}

func refreshOneProductImage(ctx context.Context, store objectstore.Store, ttl time.Duration, im *ImageMeta) {
	if im == nil {
		return
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
		return
	}
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
