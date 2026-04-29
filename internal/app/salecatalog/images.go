package salecatalog

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/avf/avf-vending-api/internal/gen/db"
)

func productImageDisplayURL(r db.RuntimeListProductImagesForProductsRow) string {
	return strings.TrimSpace(r.CdnUrl)
}

func productImageThumbURL(r db.RuntimeListProductImagesForProductsRow) string {
	s := strings.TrimSpace(r.ThumbCdnUrl)
	if s != "" {
		return s
	}
	return productImageDisplayURL(r)
}

func productImageContentHash(r db.RuntimeListProductImagesForProductsRow) string {
	if r.AssetSha256.Valid {
		s := strings.TrimSpace(r.AssetSha256.String)
		if s != "" {
			if strings.HasPrefix(strings.ToLower(s), "sha256:") {
				return s
			}
			return "sha256:" + s
		}
	}
	if r.ContentHash.Valid {
		s := strings.TrimSpace(r.ContentHash.String)
		if s != "" {
			return s
		}
	}
	return "sha256:" + sha256HexString(r.StorageKey)
}

func productImageEtag(r db.RuntimeListProductImagesForProductsRow, contentHash string) string {
	if r.AssetEtag.Valid {
		s := strings.TrimSpace(r.AssetEtag.String)
		if s != "" {
			return s
		}
	}
	return mediaEtag(contentHash, r.CreatedAt.UTC())
}

func pickDisplayImage(rows []db.RuntimeListProductImagesForProductsRow) db.RuntimeListProductImagesForProductsRow {
	for _, r := range rows {
		if r.IsPrimary {
			return r
		}
	}
	return rows[0]
}

func sha256HexString(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}
