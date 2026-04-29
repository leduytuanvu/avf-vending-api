package salecatalog

import (
	"strings"
	"time"

	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/google/uuid"
)

// MediaVariantKind mirrors avf.machine.v1.MediaVariantKind numeric values (local to avoid importing gRPC types here).
type MediaVariantKind int32

const (
	MediaVariantKindUnspecified MediaVariantKind = 0
	MediaVariantKindOriginal    MediaVariantKind = 1
	MediaVariantKindThumb       MediaVariantKind = 2
	MediaVariantKindDisplay     MediaVariantKind = 3
)

// ImageVariantMeta is one rendition row for durable kiosk cache keys (URLs may be presigned).
type ImageVariantMeta struct {
	Kind           MediaVariantKind
	MediaAssetID   uuid.UUID
	URL            string
	StorageKey     string
	ContentType    string
	ChecksumSHA256 string
	Etag           string
	SizeBytes      int64
	Width          int32
	Height         int32
	MediaVersion   int32
	UpdatedAt      time.Time
}

func variantObjectHash(row db.RuntimeListProductImagesForProductsRow, role, objectKey, catalogHash string) string {
	objectKey = strings.TrimSpace(objectKey)
	catalogHash = strings.TrimSpace(catalogHash)
	if objectKey == "" {
		return catalogHash
	}
	base := productImageContentHash(row)
	dk := strings.TrimSpace(row.DisplayObjectKey)
	if role == "display" && objectKey == dk {
		return base
	}
	if row.AssetSha256.Valid {
		s := strings.TrimSpace(row.AssetSha256.String)
		if s != "" {
			return "sha256:" + sha256HexString(role+":"+objectKey+":"+s)
		}
	}
	return "sha256:" + sha256HexString(role+":"+objectKey+":"+strings.TrimPrefix(base, "sha256:"))
}

func buildMediaVariants(
	row db.RuntimeListProductImagesForProductsRow,
	mediaAssetID uuid.UUID,
	thumbURL, displayURL, contentType string,
	width, height int32,
	sizeBytes int64,
	mediaVersion int32,
	updatedAt time.Time,
	catalogHash string,
) []ImageVariantMeta {
	origURL := strings.TrimSpace(row.OriginalCdnUrl)
	tk := strings.TrimSpace(row.ThumbObjectKey)
	dk := strings.TrimSpace(row.DisplayObjectKey)
	ok := strings.TrimSpace(row.OriginalObjectKey)
	tu := strings.TrimSpace(thumbURL)
	du := strings.TrimSpace(displayURL)

	var out []ImageVariantMeta
	add := func(kind MediaVariantKind, url, storageKey string) {
		if strings.TrimSpace(url) == "" && strings.TrimSpace(storageKey) == "" {
			return
		}
		role := ""
		switch kind {
		case MediaVariantKindOriginal:
			role = "original"
		case MediaVariantKindThumb:
			role = "thumb"
		case MediaVariantKindDisplay:
			role = "display"
		default:
			role = "unknown"
		}
		vhash := variantObjectHash(row, role, storageKey, catalogHash)
		vetag := productImageEtag(row, vhash)
		out = append(out, ImageVariantMeta{
			Kind:           kind,
			MediaAssetID:   mediaAssetID,
			URL:            strings.TrimSpace(url),
			StorageKey:     strings.TrimSpace(storageKey),
			ContentType:    strings.TrimSpace(contentType),
			ChecksumSHA256: vhash,
			Etag:           vetag,
			SizeBytes:      sizeBytes,
			Width:          width,
			Height:         height,
			MediaVersion:   mediaVersion,
			UpdatedAt:      updatedAt,
		})
	}

	// Original: skip when it duplicates display URL and has no dedicated object key.
	if ok != "" || (origURL != "" && origURL != du) {
		add(MediaVariantKindOriginal, origURL, ok)
	}
	if tu != du || tk != dk {
		add(MediaVariantKindThumb, tu, tk)
	}
	add(MediaVariantKindDisplay, du, dk)
	return out
}
