package assignmentcatalog

import (
	"context"
	"strings"
	"time"

	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/avf/avf-vending-api/internal/platform/objectstore"
)

func imageDisplayURL(r db.RuntimeListProductImagesForProductsRow) string {
	if s := strings.TrimSpace(r.CdnUrl); s != "" {
		return s
	}
	return strings.TrimSpace(r.OriginalCdnUrl)
}

func imageThumbURL(r db.RuntimeListProductImagesForProductsRow) string {
	if s := strings.TrimSpace(r.ThumbCdnUrl); s != "" {
		return s
	}
	return imageDisplayURL(r)
}

func imageContentHash(r db.RuntimeListProductImagesForProductsRow) string {
	if r.AssetSha256.Valid {
		s := strings.TrimSpace(r.AssetSha256.String)
		if s != "" {
			return s
		}
	}
	if r.ContentHash.Valid {
		s := strings.TrimSpace(r.ContentHash.String)
		if s != "" {
			return s
		}
	}
	return ""
}

func resolveProductImageURLs(
	ctx context.Context,
	store objectstore.Store,
	ttl time.Duration,
	row db.RuntimeListProductImagesForProductsRow,
) (thumb string, display string) {
	thumb = imageThumbURL(row)
	display = imageDisplayURL(row)
	if display == "" {
		display = thumb
	}
	if store == nil || ttl <= 0 {
		return thumb, display
	}
	tk := strings.TrimSpace(row.ThumbObjectKey)
	dk := strings.TrimSpace(row.DisplayObjectKey)
	if tk != "" {
		if signed, err := store.PresignGet(ctx, tk, ttl); err == nil && strings.TrimSpace(signed.URL) != "" {
			thumb = signed.URL
		}
	}
	if dk != "" {
		if signed, err := store.PresignGet(ctx, dk, ttl); err == nil && strings.TrimSpace(signed.URL) != "" {
			display = signed.URL
		}
	}
	if display == "" {
		display = thumb
	}
	return thumb, display
}
