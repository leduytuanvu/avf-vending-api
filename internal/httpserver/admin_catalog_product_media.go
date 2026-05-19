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

func mapMediaVariantsForDoc(ctx context.Context, media *appmediaadmin.Service, variants []db.MediaVariant) []V1AdminProductMediaVariantDoc {
	if media == nil || len(variants) == 0 {
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
		u, err := media.PresignedDownloadURL(ctx, v.ObjectKey)
		if err != nil {
			u = ""
		}
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
				Variants: mapMediaVariantsForDoc(ctx, app.MediaAdmin, varByAsset[aid]),
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
