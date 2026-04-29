package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
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

func mountAdminMediaRoutes(r chi.Router, app *api.HTTPApplication, writeRL func(http.Handler) http.Handler) {
	if app == nil || app.MediaAdmin == nil {
		return
	}
	if writeRL == nil {
		writeRL = func(h http.Handler) http.Handler { return h }
	}
	svc := app.MediaAdmin
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAnyPermission(auth.PermMediaRead, auth.PermCatalogRead))
		r.Get("/media", listAdminMedia(svc))
		r.Get("/media/assets", listAdminMedia(svc))
		r.Get("/organizations/{organizationId}/media/assets", listAdminMedia(svc))
		r.Get("/media/{mediaId}", getAdminMedia(svc))
		r.Get("/media/assets/{mediaId}", getAdminMedia(svc))
		r.Get("/organizations/{organizationId}/media/assets/{assetId}", getAdminMedia(svc))
	})
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAnyPermission(auth.PermMediaWrite, auth.PermCatalogWrite))
		r.With(writeRL).Post("/media/assets", postAdminMediaUploadInit(svc))
		r.With(writeRL).Post("/media/uploads", postAdminMediaUploadInit(svc))
		r.With(writeRL).Post("/organizations/{organizationId}/media/product-images", postAdminMediaUploadInit(svc))
		r.With(writeRL).Post("/media/{mediaId}/complete", postAdminMediaUploadComplete(svc))
		r.With(writeRL).Delete("/media/{mediaId}", deleteAdminMedia(svc))
		r.With(writeRL).Delete("/media/assets/{mediaId}", deleteAdminMedia(svc))
		r.With(writeRL).Delete("/organizations/{organizationId}/media/assets/{assetId}", deleteAdminMedia(svc))
	})
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAnyPermission(auth.PermMediaRead, auth.PermCatalogRead))
		r.Get("/organizations/{organizationId}/products/{productId}/images", listAdminProductImages(app))
	})
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAnyPermission(auth.PermMediaWrite, auth.PermCatalogWrite))
		r.With(writeRL).Post("/organizations/{organizationId}/media/uploads/init", postAdminMediaUploadInit(svc))
		r.With(writeRL).Post("/organizations/{organizationId}/media/uploads/complete", postAdminMediaUploadCompleteByBody(svc))
		r.With(writeRL).Post("/organizations/{organizationId}/products/{productId}/media", bindAdminProductMedia(app))
		r.With(writeRL).Delete("/organizations/{organizationId}/products/{productId}/media/{mediaId}", deleteAdminProductMedia(app))
		r.With(writeRL).Post("/organizations/{organizationId}/products/{productId}/images", bindAdminProductMedia(app))
		r.With(writeRL).Patch("/organizations/{organizationId}/products/{productId}/images/{imageId}", patchAdminProductImage(app))
		r.With(writeRL).Delete("/organizations/{organizationId}/products/{productId}/images/{imageId}", deleteAdminProductImageByID(app))
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

func adminMediaOrgAllowed(p auth.Principal, orgID uuid.UUID) bool {
	if p.HasRole(auth.RolePlatformAdmin) {
		return true
	}
	if !p.HasOrganization() || p.OrganizationID != orgID {
		return false
	}
	return auth.HasPermission(p, auth.PermMediaRead) || auth.HasPermission(p, auth.PermCatalogRead) ||
		auth.HasPermission(p, auth.PermCatalogWrite) || auth.HasPermission(p, auth.PermMediaWrite)
}

func writeMediaAdminError(w http.ResponseWriter, ctx context.Context, err error) {
	switch {
	case err == nil:
		return
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

func postAdminMediaUploadInit(svc *appmediaadmin.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			writeAPIError(w, r.Context(), http.StatusUnauthorized, "unauthenticated", "unauthenticated")
			return
		}
		orgID, err := adminCatalogOrganizationID(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "tenant_scope_required", err.Error())
			return
		}
		if !adminMediaOrgAllowed(p, orgID) {
			writeAPIError(w, r.Context(), http.StatusForbidden, "forbidden", auth.ErrForbidden.Error())
			return
		}
		var body struct {
			ContentType string `json:"content_type"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_json", "invalid json body")
			return
		}
		out, err := svc.InitUpload(r.Context(), orgID, body.ContentType)
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

func postAdminMediaUploadComplete(svc *appmediaadmin.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			writeAPIError(w, r.Context(), http.StatusUnauthorized, "unauthenticated", "unauthenticated")
			return
		}
		orgID, err := adminCatalogOrganizationID(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "tenant_scope_required", err.Error())
			return
		}
		if !adminMediaOrgAllowed(p, orgID) {
			writeAPIError(w, r.Context(), http.StatusForbidden, "forbidden", auth.ErrForbidden.Error())
			return
		}
		mediaID, err := parseAdminOptionalMediaRouteID(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_media_id", err.Error())
			return
		}
		row, err := svc.CompleteUpload(r.Context(), orgID, mediaID)
		if err != nil {
			writeMediaAdminError(w, r.Context(), err)
			return
		}
		writeJSON(w, http.StatusOK, mapAdminMediaAssetJSON(*row))
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
		orgID, err := adminCatalogOrganizationID(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "tenant_scope_required", err.Error())
			return
		}
		if !adminMediaOrgAllowed(p, orgID) {
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
		rows, total, err := svc.ListAssetsPage(r.Context(), orgID, limit, offset)
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
		orgID, err := adminCatalogOrganizationID(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "tenant_scope_required", err.Error())
			return
		}
		if !adminMediaOrgAllowed(p, orgID) {
			writeAPIError(w, r.Context(), http.StatusForbidden, "forbidden", auth.ErrForbidden.Error())
			return
		}
		mediaID, err := parseAdminOptionalMediaRouteID(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_media_id", err.Error())
			return
		}
		row, err := svc.GetAsset(r.Context(), orgID, mediaID)
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
		orgID, err := adminCatalogOrganizationID(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "tenant_scope_required", err.Error())
			return
		}
		if !adminMediaOrgAllowed(p, orgID) {
			writeAPIError(w, r.Context(), http.StatusForbidden, "forbidden", auth.ErrForbidden.Error())
			return
		}
		mediaID, err := parseAdminOptionalMediaRouteID(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_media_id", err.Error())
			return
		}
		if err := svc.DeleteAsset(r.Context(), orgID, mediaID); err != nil {
			writeMediaAdminError(w, r.Context(), err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func mapAdminMediaAssetJSON(a db.MediaAsset) map[string]any {
	out := map[string]any{
		"id":              a.ID.String(),
		"organization_id": a.OrganizationID.String(),
		"kind":            a.Kind,
		"status":          a.Status,
		"object_version":  a.ObjectVersion,
		"created_at":      formatAPITimeRFC3339Nano(a.CreatedAt),
		"updated_at":      formatAPITimeRFC3339Nano(a.UpdatedAt),
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
		orgID, err := adminCatalogOrganizationID(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "tenant_scope_required", err.Error())
			return
		}
		if !adminMediaOrgAllowed(p, orgID) {
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
		prod, err := app.MediaAdmin.BindProductPrimaryMedia(r.Context(), orgID, productID, mediaID)
		if err != nil {
			writeMediaAdminError(w, r.Context(), err)
			return
		}
		img, _ := app.CatalogAdmin.PrimaryProductImageOrNil(r.Context(), orgID, prod.ID)
		writeJSON(w, http.StatusOK, mapAdminProduct(*prod, img))
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
		orgID, err := adminCatalogOrganizationID(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "tenant_scope_required", err.Error())
			return
		}
		if !adminMediaOrgAllowed(p, orgID) {
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
		prod, err := app.MediaAdmin.UnbindProductMedia(r.Context(), orgID, productID, mediaID)
		if err != nil {
			writeMediaAdminError(w, r.Context(), err)
			return
		}
		img, _ := app.CatalogAdmin.PrimaryProductImageOrNil(r.Context(), orgID, prod.ID)
		writeJSON(w, http.StatusOK, mapAdminProduct(*prod, img))
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
		orgID, err := adminCatalogOrganizationID(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "tenant_scope_required", err.Error())
			return
		}
		if !adminMediaOrgAllowed(p, orgID) {
			writeAPIError(w, r.Context(), http.StatusForbidden, "forbidden", auth.ErrForbidden.Error())
			return
		}
		productID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "productId")))
		if err != nil || productID == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_product_id", "invalid productId")
			return
		}
		includeArchived := strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("include_archived")), "true")
		rows, err := app.CatalogAdmin.ListProductImages(r.Context(), orgID, productID, includeArchived)
		if err != nil {
			writeAdminCatalogError(w, r, err)
			return
		}
		pmRows, err := app.CatalogAdmin.ListProductMediumRowsForProduct(r.Context(), orgID, productID)
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
		orgID, err := adminCatalogOrganizationID(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "tenant_scope_required", err.Error())
			return
		}
		if !adminMediaOrgAllowed(p, orgID) {
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
			OrganizationID: orgID,
			ProductID:      productID,
			ImageID:        imageID,
			SortOrder:      body.SortOrder,
			IsPrimary:      body.IsPrimary,
			AltText:        body.AltText,
		})
		if err != nil {
			writeAdminCatalogError(w, r, err)
			return
		}
		pmRow, pmErr := app.CatalogAdmin.GetProductMediumForOrgProductImage(r.Context(), orgID, productID, imageID)
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
		orgID, err := adminCatalogOrganizationID(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "tenant_scope_required", err.Error())
			return
		}
		if !adminMediaOrgAllowed(p, orgID) {
			writeAPIError(w, r.Context(), http.StatusForbidden, "forbidden", auth.ErrForbidden.Error())
			return
		}
		productID, imageID, ok := parseProductImageRouteIDs(w, r)
		if !ok {
			return
		}
		if _, err := app.CatalogAdmin.ArchiveProductImage(r.Context(), orgID, productID, imageID); err != nil {
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
