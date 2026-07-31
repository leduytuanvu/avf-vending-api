package httpserver

import (
	"context"
	"sort"
	"strings"

	"github.com/avf/avf-vending-api/internal/app/api"
	appmediaadmin "github.com/avf/avf-vending-api/internal/app/mediaadmin"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/google/uuid"
)

func adminProductStatus(active bool) string {
	if active {
		return "active"
	}
	return "inactive"
}

func isHTTPURL(s string) bool {
	u := strings.TrimSpace(s)
	return strings.HasPrefix(strings.ToLower(u), "http://") ||
		strings.HasPrefix(strings.ToLower(u), "https://")
}

func hostedMediaURL(asset db.MediaAsset) string {
	if asset.OriginalUrl.Valid {
		if u := strings.TrimSpace(asset.OriginalUrl.String); isHTTPURL(u) {
			return u
		}
	}
	if isHTTPURL(asset.DisplayObjectKey) {
		return strings.TrimSpace(asset.DisplayObjectKey)
	}
	if asset.ObjectKey.Valid {
		if u := strings.TrimSpace(asset.ObjectKey.String); isHTTPURL(u) {
			return u
		}
	}
	return ""
}

func hostedMediaThumbURL(asset db.MediaAsset, displayURL string) string {
	if isHTTPURL(asset.ThumbObjectKey) {
		return strings.TrimSpace(asset.ThumbObjectKey)
	}
	return displayURL
}

func mapMediaVariantDownloadURL(ctx context.Context, media *appmediaadmin.Service, objectKey string) string {
	key := strings.TrimSpace(objectKey)
	if key == "" {
		return ""
	}
	if isHTTPURL(key) {
		return key
	}
	if media == nil {
		return ""
	}
	u, err := media.PresignedDownloadURL(ctx, key)
	if err != nil {
		return ""
	}
	return u
}

func mapMediaVariantsForDoc(ctx context.Context, media *appmediaadmin.Service, asset db.MediaAsset, variants []db.MediaVariant) []V1AdminProductMediaVariantDoc {
	if len(variants) == 0 {
		switch strings.ToLower(strings.TrimSpace(asset.SourceType)) {
		case "cloudinary", "external":
			display := hostedMediaURL(asset)
			if display == "" {
				return nil
			}
			thumb := hostedMediaThumbURL(asset, display)
			out := make([]V1AdminProductMediaVariantDoc, 0, 2)
			out = append(out, V1AdminProductMediaVariantDoc{
				Variant:     "display",
				DownloadURL: display,
				Version:     asset.ObjectVersion,
			})
			if thumb != "" && thumb != display {
				out = append(out, V1AdminProductMediaVariantDoc{
					Variant:     "thumb",
					DownloadURL: thumb,
					Version:     asset.ObjectVersion,
				})
			}
			if asset.MimeType.Valid && strings.TrimSpace(asset.MimeType.String) != "" {
				for i := range out {
					out[i].MimeType = asset.MimeType.String
				}
			}
			if asset.Width.Valid {
				for i := range out {
					out[i].Width = asset.Width.Int32
				}
			}
			if asset.Height.Valid {
				for i := range out {
					out[i].Height = asset.Height.Int32
				}
			}
			if asset.SizeBytes.Valid {
				for i := range out {
					out[i].SizeBytes = asset.SizeBytes.Int64
				}
			}
			return out
		default:
			return nil
		}
	}
	if media == nil {
		return nil
	}
	order := map[string]int{"thumb": 0, "display": 1, "original": 2, "fallback": 3}
	cp := append([]db.MediaVariant(nil), variants...)
	sort.SliceStable(cp, func(i, j int) bool {
		oi, oj := order[cp[i].Variant], order[cp[j].Variant]
		if oi != oj {
			return oi < oj
		}
		return cp[i].Variant < cp[j].Variant
	})
	out := make([]V1AdminProductMediaVariantDoc, 0, len(cp))
	for _, v := range cp {
		u := mapMediaVariantDownloadURL(ctx, media, v.ObjectKey)
		item := V1AdminProductMediaVariantDoc{
			Variant:     v.Variant,
			DownloadURL: u,
			Version:     v.Version,
		}
		if v.MimeType.Valid && strings.TrimSpace(v.MimeType.String) != "" {
			item.MimeType = v.MimeType.String
		}
		if v.Width.Valid {
			item.Width = v.Width.Int32
		}
		if v.Height.Valid {
			item.Height = v.Height.Int32
		}
		if v.SizeBytes.Valid {
			item.SizeBytes = v.SizeBytes.Int64
		}
		if v.Sha256.Valid && strings.TrimSpace(v.Sha256.String) != "" {
			item.SHA256 = v.Sha256.String
		}
		out = append(out, item)
	}
	return out
}

func mediaDocsByAssetIDs(ctx context.Context, app *api.HTTPApplication, assetIDs []uuid.UUID) (map[uuid.UUID]*V1AdminProductMediaDoc, error) {
	if app == nil || app.MediaAdmin == nil || len(assetIDs) == 0 {
		return map[uuid.UUID]*V1AdminProductMediaDoc{}, nil
	}
	uniq := make([]uuid.UUID, 0, len(assetIDs))
	seen := map[uuid.UUID]struct{}{}
	for _, aid := range assetIDs {
		if aid == uuid.Nil {
			continue
		}
		if _, ok := seen[aid]; ok {
			continue
		}
		seen[aid] = struct{}{}
		uniq = append(uniq, aid)
	}
	if len(uniq) == 0 {
		return map[uuid.UUID]*V1AdminProductMediaDoc{}, nil
	}
	assets, err := app.MediaAdmin.ListAssetsByIDs(ctx, uniq)
	if err != nil {
		return nil, err
	}
	assetBy := make(map[uuid.UUID]db.MediaAsset, len(assets))
	for _, a := range assets {
		assetBy[a.ID] = a
	}
	variants, err := app.MediaAdmin.ListVariantsForAssets(ctx, uniq)
	if err != nil {
		return nil, err
	}
	varByAsset := make(map[uuid.UUID][]db.MediaVariant)
	for _, v := range variants {
		varByAsset[v.MediaAssetID] = append(varByAsset[v.MediaAssetID], v)
	}
	out := make(map[uuid.UUID]*V1AdminProductMediaDoc, len(uniq))
	for _, aid := range uniq {
		a, ok := assetBy[aid]
		if !ok {
			continue
		}
		out[aid] = &V1AdminProductMediaDoc{
			Primary: &V1AdminProductMediaPrimaryDoc{
				ID:       a.ID.String(),
				Status:   a.Status,
				Version:  a.ObjectVersion,
				Variants: mapMediaVariantsForDoc(ctx, app.MediaAdmin, a, varByAsset[aid]),
			},
		}
	}
	return out, nil
}

// batchAdminProductMediaDocs maps product id -> media envelope using primary media_asset_id bindings.
func batchAdminProductMediaDocs(ctx context.Context, app *api.HTTPApplication, productToAsset map[uuid.UUID]uuid.UUID) (map[uuid.UUID]*V1AdminProductMediaDoc, error) {
	if len(productToAsset) == 0 {
		return map[uuid.UUID]*V1AdminProductMediaDoc{}, nil
	}
	uniq := make([]uuid.UUID, 0, len(productToAsset))
	seen := map[uuid.UUID]struct{}{}
	for _, aid := range productToAsset {
		if aid == uuid.Nil {
			continue
		}
		if _, ok := seen[aid]; ok {
			continue
		}
		seen[aid] = struct{}{}
		uniq = append(uniq, aid)
	}
	byAsset, err := mediaDocsByAssetIDs(ctx, app, uniq)
	if err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID]*V1AdminProductMediaDoc, len(productToAsset))
	for pid, aid := range productToAsset {
		if doc, ok := byAsset[aid]; ok {
			out[pid] = doc
		}
	}
	return out, nil
}

func buildAdminProductMediaDoc(ctx context.Context, app *api.HTTPApplication, img *db.ProductImage) *V1AdminProductMediaDoc {
	if app == nil || app.MediaAdmin == nil || img == nil || !img.MediaAssetID.Valid {
		return nil
	}
	aid := uuid.UUID(img.MediaAssetID.Bytes)
	by, err := mediaDocsByAssetIDs(ctx, app, []uuid.UUID{aid})
	if err != nil || by == nil {
		return nil
	}
	return by[aid]
}

func primaryMediaIDPtr(img *db.ProductImage) *string {
	if img == nil || !img.MediaAssetID.Valid {
		return nil
	}
	s := uuid.UUID(img.MediaAssetID.Bytes).String()
	return &s
}
