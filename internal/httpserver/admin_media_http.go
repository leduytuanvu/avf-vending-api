package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/avf/avf-vending-api/internal/app/api"
	appcatalogadmin "github.com/avf/avf-vending-api/internal/app/catalogadmin"
	appmediaadmin "github.com/avf/avf-vending-api/internal/app/mediaadmin"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const adminMediaCapabilityMsg = "enterprise media pipeline requires API_ARTIFACTS_ENABLED object storage"

func withMediaAdmin(app *api.HTTPApplication, fn func(*appmediaadmin.Service) http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if app == nil || app.MediaAdmin == nil {
			writeCapabilityNotConfigured(w, r.Context(), "admin.media", adminMediaCapabilityMsg)
			return
		}
		fn(app.MediaAdmin)(w, r)
	}
}

func mountAdminMediaRoutes(r chi.Router, app *api.HTTPApplication, writeRL func(http.Handler) http.Handler) {
	if app == nil {
		return
	}
	if writeRL == nil {
		writeRL = func(h http.Handler) http.Handler { return h }
	}
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAnyPermission(auth.PermMediaRead, auth.PermCatalogRead))
		r.Get("/media", withMediaAdmin(app, listAdminMedia))
		r.Get("/media/assets", withMediaAdmin(app, listAdminMedia))
		r.Get("/media/{mediaId}", withMediaAdmin(app, getAdminMedia))
		r.Get("/media/assets/{mediaId}", withMediaAdmin(app, getAdminMedia))
	})
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAnyPermission(auth.PermMediaWrite, auth.PermCatalogWrite))
		r.With(writeRL).Post("/media/assets", withMediaAdmin(app, postAdminMediaUploadInitLegacy))
		r.With(writeRL).Post("/media/uploads/init", withMediaAdmin(app, postAdminMediaUploadInitV2))
		r.With(writeRL).Post("/media/uploads/{mediaId}/complete", withMediaAdmin(app, postAdminMediaUploadComplete))
		r.With(writeRL).Post("/media/uploads", withMediaAdmin(app, postAdminMediaUploadInitLegacy))
		r.With(writeRL).Post("/media/{mediaId}/complete", withMediaAdmin(app, postAdminMediaUploadComplete))
		r.With(writeRL).Delete("/media/{mediaId}", withMediaAdmin(app, deleteAdminMedia))
		r.With(writeRL).Delete("/media/assets/{mediaId}", withMediaAdmin(app, deleteAdminMedia))
	})
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAnyPermission(auth.PermMediaRead, auth.PermCatalogRead))
	})
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAnyPermission(auth.PermMediaWrite, auth.PermCatalogWrite))
	})
}

func parseAdminOptionalMediaRouteID(r *http.Request) (uuid.UUID, error) {
	for _, k := range []string{"mediaId", "assetId"} {
		if raw := strings.TrimSpace(chi.URLParam(r, k)); raw != "" {
			id, err := uuid.Parse(raw)
			if err != nil || id == uuid.Nil {
				return uuid.Nil, fmt.Errorf("invalid %s", k)
			}
			return id, nil
		}
	}
	return uuid.Nil, fmt.Errorf("missing media id")
}

func adminMediaOrgAllowed(p auth.Principal, scopeID uuid.UUID) bool {
	if p.HasRole(auth.RolePlatformAdmin) {
		return true
	}
	if uuid.Nil != scopeID {
		return false
	}
	return auth.HasPermission(p, auth.PermMediaRead) || auth.HasPermission(p, auth.PermCatalogRead) ||
		auth.HasPermission(p, auth.PermCatalogWrite) || auth.HasPermission(p, auth.PermMediaWrite)
}

func writeMediaAdminError(w http.ResponseWriter, ctx context.Context, err error) {
	switch {
	case err == nil:
		return
	case errors.Is(err, appmediaadmin.ErrNotConfigured):
		writeCapabilityNotConfigured(w, ctx, "admin.media", adminMediaCapabilityMsg)
	case errors.Is(err, appmediaadmin.ErrNotFound):
		writeAPIError(w, ctx, http.StatusNotFound, "not_found", err.Error())
	case errors.Is(err, appmediaadmin.ErrInvalidArgument):
		writeAPIError(w, ctx, http.StatusBadRequest, "invalid_argument", err.Error())
	case errors.Is(err, appmediaadmin.ErrConflict):
		writeAPIError(w, ctx, http.StatusConflict, "conflict", err.Error())
	default:
		writeAPIError(w, ctx, http.StatusInternalServerError, "internal", err.Error())
	}
}

func postAdminMediaUploadInitLegacy(svc *appmediaadmin.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			writeAPIError(w, r.Context(), http.StatusUnauthorized, "unauthenticated", "unauthenticated")
			return
		}
		scopeID, err := requireCatalogPrincipalUUID(r)
		_ = scopeID
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "company_scope_required", err.Error())
			return
		}
		if !adminMediaOrgAllowed(p, scopeID) {
			writeAPIError(w, r.Context(), http.StatusForbidden, "forbidden", auth.ErrForbidden.Error())
			return
		}
		var body V1AdminMediaUploadInitRequest
		if !decodeStrictJSON(w, r, &body) {
			return
		}
		out, err := svc.InitUpload(r.Context(), scopeID, "", body.ContentType, "product_image")
		if err != nil {
			writeMediaAdminError(w, r.Context(), err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"media_id":       out.MediaID.String(),
			"upload_url":     out.UploadURL,
			"upload_method":  out.UploadMethod,
			"upload_headers": out.UploadHeaders,
			"expires_at":     formatAPITimeRFC3339Nano(out.ExpiresAt),
			"complete_path":  out.CompletePath,
		})
	}
}

func postAdminMediaUploadInitV2(svc *appmediaadmin.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			writeAPIError(w, r.Context(), http.StatusUnauthorized, "unauthenticated", "unauthenticated")
			return
		}
		scopeID, err := requireCatalogPrincipalUUID(r)
		_ = scopeID
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "company_scope_required", err.Error())
			return
		}
		if !adminMediaOrgAllowed(p, scopeID) {
			writeAPIError(w, r.Context(), http.StatusForbidden, "forbidden", auth.ErrForbidden.Error())
			return
		}
		var body V1AdminMediaUploadInitRequestV2
		if !decodeStrictJSON(w, r, &body) {
			return
		}
		fn := strings.TrimSpace(body.Filename)
		ct := strings.TrimSpace(body.ContentType)
		if fn == "" || ct == "" {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_argument", "filename and contentType are required")
			return
		}
		out, err := svc.InitUpload(r.Context(), scopeID, fn, ct, body.Purpose)
		if err != nil {
			writeMediaAdminError(w, r.Context(), err)
			return
		}
		writeJSON(w, http.StatusOK, V1AdminMediaUploadInitResponseV2{
			MediaID:      out.MediaID.String(),
			UploadURL:    out.UploadURL,
			ObjectKey:    out.OriginalKey,
			Status:       out.Status,
			CompletePath: out.CompletePath,
		})
	}
}

func postAdminMediaUploadComplete(svc *appmediaadmin.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			writeAPIError(w, r.Context(), http.StatusUnauthorized, "unauthenticated", "unauthenticated")
			return
		}
		scopeID, err := requireCatalogPrincipalUUID(r)
		_ = scopeID
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "company_scope_required", err.Error())
			return
		}
		if !adminMediaOrgAllowed(p, scopeID) {
			writeAPIError(w, r.Context(), http.StatusForbidden, "forbidden", auth.ErrForbidden.Error())
			return
		}
		mediaID, err := parseAdminOptionalMediaRouteID(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_media_id", err.Error())
			return
		}
		var opts appmediaadmin.CompleteUploadOptions
		if r.Body != nil {
			dec := json.NewDecoder(r.Body)
			dec.DisallowUnknownFields()
			var body V1AdminMediaUploadCompleteRequestV2
			if err := dec.Decode(&body); err != nil && !errors.Is(err, io.EOF) {
				writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_json", "invalid request body")
				return
			}
			opts.SizeBytes = body.SizeBytes
			opts.SHA256Hex = body.SHA256
			opts.ContentType = body.ContentType
		}
		row, err := svc.CompleteUploadWithOptions(r.Context(), scopeID, mediaID, opts)
		if err != nil {
			writeMediaAdminError(w, r.Context(), err)
			return
		}
		writeMediaUploadCompleteV2(w, r.Context(), svc, row)
	}
}

func postAdminMediaUploadCompleteByBody(svc *appmediaadmin.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			MediaID      string `json:"media_id"`
			MediaIDCamel string `json:"mediaId"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_json", "invalid json body")
			return
		}
		rctx := chi.NewRouteContext()
		if existing := chi.RouteContext(r.Context()); existing != nil {
			*rctx = *existing
		}
		rawMediaID := strings.TrimSpace(body.MediaID)
		if rawMediaID == "" {
			rawMediaID = strings.TrimSpace(body.MediaIDCamel)
		}
		rctx.URLParams.Add("mediaId", rawMediaID)
		postAdminMediaUploadComplete(svc).ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx)))
	}
}

func listAdminMedia(svc *appmediaadmin.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			writeAPIError(w, r.Context(), http.StatusUnauthorized, "unauthenticated", "unauthenticated")
			return
		}
		scopeID, err := requireCatalogPrincipalUUID(r)
		_ = scopeID
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "company_scope_required", err.Error())
			return
		}
		if !adminMediaOrgAllowed(p, scopeID) {
			writeAPIError(w, r.Context(), http.StatusForbidden, "forbidden", auth.ErrForbidden.Error())
			return
		}
		limit := int32(50)
		if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
			n, perr := strconv.ParseInt(raw, 10, 32)
			if perr != nil || n <= 0 {
				writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_query", "invalid limit")
				return
			}
			limit = int32(n)
		}
		offset := int32(0)
		if raw := strings.TrimSpace(r.URL.Query().Get("offset")); raw != "" {
			n, perr := strconv.ParseInt(raw, 10, 32)
			if perr != nil || n < 0 {
				writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_query", "invalid offset")
				return
			}
			offset = int32(n)
		}
		rows, total, err := svc.ListAssetsPage(r.Context(), scopeID, limit, offset)
		if err != nil {
			writeMediaAdminError(w, r.Context(), err)
			return
		}
		items := make([]map[string]any, 0, len(rows))
		for i := range rows {
			items = append(items, mapAdminMediaAssetJSON(rows[i]))
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"items": items,
			"meta": map[string]any{
				"limit":       limit,
				"offset":      offset,
				"returned":    len(items),
				"total_count": total,
			},
		})
	}
}

func getAdminMedia(svc *appmediaadmin.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			writeAPIError(w, r.Context(), http.StatusUnauthorized, "unauthenticated", "unauthenticated")
			return
		}
		scopeID, err := requireCatalogPrincipalUUID(r)
		_ = scopeID
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "company_scope_required", err.Error())
			return
		}
		if !adminMediaOrgAllowed(p, scopeID) {
			writeAPIError(w, r.Context(), http.StatusForbidden, "forbidden", auth.ErrForbidden.Error())
			return
		}
		mediaID, err := parseAdminOptionalMediaRouteID(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_media_id", err.Error())
			return
		}
		row, err := svc.GetAsset(r.Context(), scopeID, mediaID)
		if err != nil {
			writeMediaAdminError(w, r.Context(), err)
			return
		}
		writeJSON(w, http.StatusOK, mapAdminMediaAssetJSON(row))
	}
}

func deleteAdminMedia(svc *appmediaadmin.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			writeAPIError(w, r.Context(), http.StatusUnauthorized, "unauthenticated", "unauthenticated")
			return
		}
		scopeID, err := requireCatalogPrincipalUUID(r)
		_ = scopeID
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "company_scope_required", err.Error())
			return
		}
		if !adminMediaOrgAllowed(p, scopeID) {
			writeAPIError(w, r.Context(), http.StatusForbidden, "forbidden", auth.ErrForbidden.Error())
			return
		}
		mediaID, err := parseAdminOptionalMediaRouteID(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_media_id", err.Error())
			return
		}
		if err := svc.DeleteAsset(r.Context(), scopeID, mediaID); err != nil {
			writeMediaAdminError(w, r.Context(), err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func writeMediaUploadCompleteV2(w http.ResponseWriter, ctx context.Context, svc *appmediaadmin.Service, asset *db.MediaAsset) {
	if asset == nil {
		writeAPIError(w, ctx, http.StatusInternalServerError, "internal", "nil asset")
		return
	}
	variants, err := svc.ListVariantsForAssets(ctx, []uuid.UUID{asset.ID})
	if err != nil {
		writeMediaAdminError(w, ctx, err)
		return
	}
	order := map[string]int{"thumb": 0, "display": 1, "original": 2, "fallback": 3}
	sort.SliceStable(variants, func(i, j int) bool {
		oi, oj := order[variants[i].Variant], order[variants[j].Variant]
		if oi != oj {
			return oi < oj
		}
		return variants[i].Variant < variants[j].Variant
	})
	items := make([]V1AdminMediaVariantResponse, 0, len(variants))
	for _, v := range variants {
		u, err := svc.PresignedDownloadURL(ctx, v.ObjectKey)
		if err != nil {
			u = ""
		}
		item := V1AdminMediaVariantResponse{
			Variant:     v.Variant,
			Version:     v.Version,
			DownloadURL: u,
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
		items = append(items, item)
	}
	writeJSON(w, http.StatusOK, V1AdminMediaUploadCompleteResponseV2{
		ID:       asset.ID.String(),
		Status:   asset.Status,
		Variants: items,
	})
}

func mapAdminMediaAssetJSON(a db.MediaAsset) map[string]any {
	out := map[string]any{
		"id":             a.ID.String(),
		"kind":           a.Kind,
		"status":         a.Status,
		"object_version": a.ObjectVersion,
		"created_at":     formatAPITimeRFC3339Nano(a.CreatedAt),
		"updated_at":     formatAPITimeRFC3339Nano(a.UpdatedAt),
	}
	if a.MimeType.Valid {
		out["mime_type"] = a.MimeType.String
	}
	if a.SizeBytes.Valid {
		out["size_bytes"] = a.SizeBytes.Int64
	}
	if a.Sha256.Valid {
		out["sha256"] = a.Sha256.String
	}
	if a.Width.Valid {
		out["width"] = a.Width.Int32
	}
	if a.Height.Valid {
		out["height"] = a.Height.Int32
	}
	if a.Etag.Valid {
		out["etag"] = a.Etag.String
	}
	return out
}

// Product media bind (object-storage pipeline) — mounted from admin catalog routes.

func bindAdminProductMedia(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if app == nil || app.MediaAdmin == nil {
			writeCapabilityNotConfigured(w, r.Context(), "admin.media", "enterprise media pipeline requires API_ARTIFACTS_ENABLED object storage")
			return
		}
		p, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			writeAPIError(w, r.Context(), http.StatusUnauthorized, "unauthenticated", "unauthenticated")
			return
		}
		scopeID, err := requireCatalogPrincipalUUID(r)
		_ = scopeID
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "company_scope_required", err.Error())
			return
		}
		if !adminMediaOrgAllowed(p, scopeID) {
			writeAPIError(w, r.Context(), http.StatusForbidden, "forbidden", auth.ErrForbidden.Error())
			return
		}
		productID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "productId")))
		if err != nil || productID == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_product_id", "invalid productId")
			return
		}
		var body struct {
			MediaID      string `json:"media_id"`
			MediaIDCamel string `json:"mediaId"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_json", "invalid json body")
			return
		}
		rawMediaID := strings.TrimSpace(body.MediaID)
		if rawMediaID == "" {
			rawMediaID = strings.TrimSpace(body.MediaIDCamel)
		}
		mediaID, err := uuid.Parse(rawMediaID)
		if err != nil || mediaID == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_media_id", "media_id required")
			return
		}
		prod, err := app.MediaAdmin.BindProductPrimaryMedia(r.Context(), scopeID, productID, mediaID)
		if err != nil {
			writeMediaAdminError(w, r.Context(), err)
			return
		}
		writeAdminProductResponse(w, r, app, app.CatalogAdmin, scopeID, *prod)
	}
}

func deleteAdminProductMedia(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if app == nil || app.MediaAdmin == nil {
			writeCapabilityNotConfigured(w, r.Context(), "admin.media", "enterprise media pipeline requires API_ARTIFACTS_ENABLED object storage")
			return
		}
		p, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			writeAPIError(w, r.Context(), http.StatusUnauthorized, "unauthenticated", "unauthenticated")
			return
		}
		scopeID, err := requireCatalogPrincipalUUID(r)
		_ = scopeID
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "company_scope_required", err.Error())
			return
		}
		if !adminMediaOrgAllowed(p, scopeID) {
			writeAPIError(w, r.Context(), http.StatusForbidden, "forbidden", auth.ErrForbidden.Error())
			return
		}
		productID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "productId")))
		if err != nil || productID == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_product_id", "invalid productId")
			return
		}
		mediaID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "mediaId")))
		if err != nil || mediaID == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_media_id", "invalid mediaId")
			return
		}
		prod, err := app.MediaAdmin.UnbindProductMedia(r.Context(), scopeID, productID, mediaID)
		if err != nil {
			writeMediaAdminError(w, r.Context(), err)
			return
		}
		writeAdminProductResponse(w, r, app, app.CatalogAdmin, scopeID, *prod)
	}
}

func listAdminProductImages(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if app == nil || app.CatalogAdmin == nil {
			writeAPIError(w, r.Context(), http.StatusServiceUnavailable, "catalog_not_configured", "catalog admin not configured")
			return
		}
		p, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			writeAPIError(w, r.Context(), http.StatusUnauthorized, "unauthenticated", "unauthenticated")
			return
		}
		scopeID, err := requireCatalogPrincipalUUID(r)
		_ = scopeID
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "company_scope_required", err.Error())
			return
		}
		if !adminMediaOrgAllowed(p, scopeID) {
			writeAPIError(w, r.Context(), http.StatusForbidden, "forbidden", auth.ErrForbidden.Error())
			return
		}
		productID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "productId")))
		if err != nil || productID == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_product_id", "invalid productId")
			return
		}
		includeArchived := strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("include_archived")), "true")
		rows, err := app.CatalogAdmin.ListProductImages(r.Context(), scopeID, productID, includeArchived)
		if err != nil {
			writeAdminCatalogError(w, r, err)
			return
		}
		pmRows, err := app.CatalogAdmin.ListProductMediumRowsForProduct(r.Context(), scopeID, productID)
		if err != nil {
			writeAdminCatalogError(w, r, err)
			return
		}
		pmByID := make(map[uuid.UUID]db.ProductMedium, len(pmRows))
		for i := range pmRows {
			pmByID[pmRows[i].ID] = pmRows[i]
		}
		items := make([]map[string]any, 0, len(rows))
		for i := range rows {
			pmRow, ok := pmByID[rows[i].ID]
			if ok {
				items = append(items, mapAdminProductImageJSON(rows[i], &pmRow))
			} else {
				items = append(items, mapAdminProductImageJSON(rows[i], nil))
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items})
	}
}

func patchAdminProductImage(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if app == nil || app.CatalogAdmin == nil {
			writeAPIError(w, r.Context(), http.StatusServiceUnavailable, "catalog_not_configured", "catalog admin not configured")
			return
		}
		p, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			writeAPIError(w, r.Context(), http.StatusUnauthorized, "unauthenticated", "unauthenticated")
			return
		}
		scopeID, err := requireCatalogPrincipalUUID(r)
		_ = scopeID
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "company_scope_required", err.Error())
			return
		}
		if !adminMediaOrgAllowed(p, scopeID) {
			writeAPIError(w, r.Context(), http.StatusForbidden, "forbidden", auth.ErrForbidden.Error())
			return
		}
		productID, imageID, ok := parseProductImageRouteIDs(w, r)
		if !ok {
			return
		}
		var body struct {
			SortOrder *int32  `json:"sort_order"`
			IsPrimary *bool   `json:"is_primary"`
			AltText   *string `json:"alt_text"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_json", "invalid json body")
			return
		}
		img, err := app.CatalogAdmin.UpdateProductImage(r.Context(), appcatalogadmin.UpdateProductImageInput{
			ProductID: productID,
			ImageID:   imageID,
			SortOrder: body.SortOrder,
			IsPrimary: body.IsPrimary,
			AltText:   body.AltText,
		})
		if err != nil {
			writeAdminCatalogError(w, r, err)
			return
		}
		pmRow, pmErr := app.CatalogAdmin.GetProductMediumForOrgProductImage(r.Context(), scopeID, productID, imageID)
		var pm *db.ProductMedium
		if pmErr == nil {
			pm = &pmRow
		} else if !errors.Is(pmErr, pgx.ErrNoRows) {
			writeAdminCatalogError(w, r, pmErr)
			return
		}
		writeJSON(w, http.StatusOK, mapAdminProductImageJSON(img, pm))
	}
}

func deleteAdminProductImageByID(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if app == nil || app.CatalogAdmin == nil {
			writeAPIError(w, r.Context(), http.StatusServiceUnavailable, "catalog_not_configured", "catalog admin not configured")
			return
		}
		p, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			writeAPIError(w, r.Context(), http.StatusUnauthorized, "unauthenticated", "unauthenticated")
			return
		}
		scopeID, err := requireCatalogPrincipalUUID(r)
		_ = scopeID
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "company_scope_required", err.Error())
			return
		}
		if !adminMediaOrgAllowed(p, scopeID) {
			writeAPIError(w, r.Context(), http.StatusForbidden, "forbidden", auth.ErrForbidden.Error())
			return
		}
		productID, imageID, ok := parseProductImageRouteIDs(w, r)
		if !ok {
			return
		}
		if _, err := app.CatalogAdmin.ArchiveProductImage(r.Context(), scopeID, productID, imageID); err != nil {
			writeAdminCatalogError(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func parseProductImageRouteIDs(w http.ResponseWriter, r *http.Request) (uuid.UUID, uuid.UUID, bool) {
	productID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "productId")))
	if err != nil || productID == uuid.Nil {
		writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_product_id", "invalid productId")
		return uuid.Nil, uuid.Nil, false
	}
	imageID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "imageId")))
	if err != nil || imageID == uuid.Nil {
		writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_image_id", "invalid imageId")
		return uuid.Nil, uuid.Nil, false
	}
	return productID, imageID, true
}

func mapAdminProductImageJSON(img db.ProductImage, pm *db.ProductMedium) map[string]any {
	out := map[string]any{
		"id":          img.ID.String(),
		"product_id":  img.ProductID.String(),
		"storage_key": img.StorageKey,
		"sort_order":  img.SortOrder,
		"is_primary":  img.IsPrimary,
		"status":      img.Status,
		"created_at":  formatAPITimeRFC3339Nano(img.CreatedAt),
		"updated_at":  formatAPITimeRFC3339Nano(img.UpdatedAt),
	}
	mv := img.MediaVersion
	display := ""
	if img.CdnUrl.Valid {
		display = strings.TrimSpace(img.CdnUrl.String)
	}
	thumb := ""
	if img.ThumbCdnUrl.Valid {
		thumb = strings.TrimSpace(img.ThumbCdnUrl.String)
	}
	var hash string
	var hashOK bool
	if img.ContentHash.Valid {
		hash = strings.TrimSpace(img.ContentHash.String)
		hashOK = hash != ""
	}
	var widthVal interface{}
	var heightVal interface{}
	var mimeVal interface{}
	if img.Width.Valid {
		widthVal = img.Width.Int32
	}
	if img.Height.Valid {
		heightVal = img.Height.Int32
	}
	if img.MimeType.Valid && strings.TrimSpace(img.MimeType.String) != "" {
		mimeVal = strings.TrimSpace(img.MimeType.String)
	}

	if pm != nil {
		out["source_type"] = pm.SourceType
		out["media_status"] = pm.Status
		if pm.OriginalObjectKey.Valid && strings.TrimSpace(pm.OriginalObjectKey.String) != "" {
			out["original_object_key"] = strings.TrimSpace(pm.OriginalObjectKey.String)
		}
		if pm.ThumbObjectKey.Valid && strings.TrimSpace(pm.ThumbObjectKey.String) != "" {
			out["thumb_object_key"] = strings.TrimSpace(pm.ThumbObjectKey.String)
		}
		if pm.DisplayObjectKey.Valid && strings.TrimSpace(pm.DisplayObjectKey.String) != "" {
			out["display_object_key"] = strings.TrimSpace(pm.DisplayObjectKey.String)
		}
		if pm.OriginalUrl.Valid && strings.TrimSpace(pm.OriginalUrl.String) != "" {
			out["original_url"] = strings.TrimSpace(pm.OriginalUrl.String)
		}
		if pm.MediaType != "" {
			out["media_type"] = pm.MediaType
		}
		if pm.SizeBytes > 0 {
			out["size_bytes"] = pm.SizeBytes
		}
		mv = pm.MediaVersion
		if pm.DisplayUrl.Valid {
			if s := strings.TrimSpace(pm.DisplayUrl.String); s != "" {
				display = s
			}
		}
		if pm.ThumbUrl.Valid {
			if s := strings.TrimSpace(pm.ThumbUrl.String); s != "" {
				thumb = s
			}
		}
		if pm.ContentHash.Valid {
			if s := strings.TrimSpace(pm.ContentHash.String); s != "" {
				hash = s
				hashOK = true
			}
		}
		if pm.Width.Valid {
			widthVal = pm.Width.Int32
		}
		if pm.Height.Valid {
			heightVal = pm.Height.Int32
		}
		if pm.MimeType.Valid && strings.TrimSpace(pm.MimeType.String) != "" {
			mimeVal = strings.TrimSpace(pm.MimeType.String)
		}
	}

	out["media_version"] = mv
	if display != "" {
		out["display_url"] = display
	}
	if thumb != "" {
		out["thumb_url"] = thumb
	}
	if hashOK {
		out["content_hash"] = hash
	}
	if widthVal != nil {
		out["width"] = widthVal
	}
	if heightVal != nil {
		out["height"] = heightVal
	}
	if mimeVal != nil {
		out["mime_type"] = mimeVal
	}
	if img.MediaAssetID.Valid {
		out["media_id"] = uuid.UUID(img.MediaAssetID.Bytes).String()
	}
	return out
}
