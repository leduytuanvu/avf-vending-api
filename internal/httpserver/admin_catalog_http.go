package httpserver

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/avf/avf-vending-api/internal/app/api"
	appcatalogadmin "github.com/avf/avf-vending-api/internal/app/catalogadmin"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func mountAdminCatalogRoutes(r chi.Router, app *api.HTTPApplication, writeRL func(http.Handler) http.Handler) {
	if app == nil || app.CatalogAdmin == nil {
		return
	}
	if writeRL == nil {
		writeRL = func(h http.Handler) http.Handler { return h }
	}
	svc := app.CatalogAdmin
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAnyPermission(auth.PermCatalogRead))
		r.Get("/products", listAdminProducts(app))
		r.Get("/products/{productId}", getAdminProduct(app))
		r.Get("/brands", listAdminBrands(svc))
		r.Get("/categories", listAdminCategories(svc))
		r.Get("/tags", listAdminTags(svc))
		r.Get("/price-books", listAdminPriceBooks(svc))
		r.Get("/price-books/{priceBookId}", getAdminPriceBookDetail(svc))
		r.Get("/price-books/{priceBookId}/items", getAdminPriceBookItems(svc))
		r.Post("/pricing/preview", postAdminPricingPreview(svc))
		registerAdminPromotionReadRoutes(r, svc)
		r.Get("/planograms", listAdminPlanograms(svc))
		r.Get("/planograms/{planogramId}", getAdminPlanogram(svc))
	})
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAnyPermission(auth.PermCatalogWrite))
		r.With(writeRL).Post("/products", postAdminProductCreate(app))
		r.With(writeRL).Put("/products/{productId}", putAdminProductUpdate(app))
		r.With(writeRL).Patch("/products/{productId}", putAdminProductUpdate(app))
		r.With(writeRL).With(auth.RequireAnyPermission(auth.PermCatalogDelete)).Delete("/products/{productId}", deleteAdminProduct(app))
		r.With(writeRL).With(auth.RequireAnyPermission(auth.PermMediaWrite, auth.PermCatalogWrite)).Post("/products/{productId}/image", bindAdminProductImage(app))
		r.With(writeRL).With(auth.RequireAnyPermission(auth.PermMediaWrite, auth.PermCatalogWrite)).Put("/products/{productId}/image", bindAdminProductImage(app))
		r.With(writeRL).With(auth.RequireAnyPermission(auth.PermMediaWrite, auth.PermCatalogWrite)).Delete("/products/{productId}/image", deleteAdminProductImage(app))
		r.With(writeRL).With(auth.RequireAnyPermission(auth.PermMediaWrite, auth.PermCatalogWrite)).Post("/products/{productId}/media", bindAdminProductMedia(app))
		r.With(writeRL).With(auth.RequireAnyPermission(auth.PermMediaWrite, auth.PermCatalogWrite)).Put("/products/{productId}/media", bindAdminProductMedia(app))
		r.With(writeRL).With(auth.RequireAnyPermission(auth.PermMediaWrite, auth.PermCatalogWrite)).Delete("/products/{productId}/media/{mediaId}", deleteAdminProductMedia(app))
		r.With(writeRL).Post("/brands", postAdminBrandCreate(svc))
		r.With(writeRL).Put("/brands/{brandId}", putAdminBrandUpdate(svc))
		r.With(writeRL).Patch("/brands/{brandId}", putAdminBrandUpdate(svc))
		r.With(writeRL).Delete("/brands/{brandId}", deleteAdminBrand(svc))
		r.With(writeRL).Post("/categories", postAdminCategoryCreate(svc))
		r.With(writeRL).Put("/categories/{categoryId}", putAdminCategoryUpdate(svc))
		r.With(writeRL).Patch("/categories/{categoryId}", putAdminCategoryUpdate(svc))
		r.With(writeRL).Delete("/categories/{categoryId}", deleteAdminCategory(svc))
		r.With(writeRL).Post("/tags", postAdminTagCreate(svc))
		r.With(writeRL).Put("/tags/{tagId}", putAdminTagUpdate(svc))
		r.With(writeRL).Patch("/tags/{tagId}", putAdminTagUpdate(svc))
		r.With(writeRL).Delete("/tags/{tagId}", deleteAdminTag(svc))
		r.With(writeRL).Post("/price-books", postAdminPriceBookCreate(svc))
		r.With(writeRL).Patch("/price-books/{priceBookId}", patchAdminPriceBook(svc))
		r.With(writeRL).Post("/price-books/{priceBookId}/activate", postAdminPriceBookActivate(svc, app))
		r.With(writeRL).Post("/price-books/{priceBookId}/archive", postAdminPriceBookArchive(svc, app))
		r.With(writeRL).Post("/price-books/{priceBookId}/deactivate", postAdminPriceBookDeactivate(svc, app))
		r.With(writeRL).Put("/price-books/{priceBookId}/items", putAdminPriceBookItems(svc))
		r.With(writeRL).Patch("/price-books/{priceBookId}/items/{productId}", patchAdminPriceBookItem(svc))
		r.With(writeRL).Delete("/price-books/{priceBookId}/items/{productId}", deleteAdminPriceBookItem(svc))
		r.With(writeRL).Post("/price-books/{priceBookId}/assign-target", postAdminPriceBookAssignTarget(svc))
		r.With(writeRL).Delete("/price-books/{priceBookId}/targets/{targetId}", deleteAdminPriceBookTarget(svc))
		registerAdminPromotionWriteRoutes(r, svc, writeRL)
		r.With(writeRL).Post("/planograms", postAdminPlanogramCreate(svc))
		r.With(writeRL).Patch("/planograms/{planogramId}", patchAdminPlanogramUpdate(svc))
		r.With(writeRL).Put("/planograms/{planogramId}", patchAdminPlanogramUpdate(svc))
		r.With(writeRL).With(auth.RequireAnyPermission(auth.PermCatalogDelete)).Delete("/planograms/{planogramId}", deleteAdminPlanogram(svc))
		r.With(writeRL).Put("/planograms/{planogramId}/slots", putAdminPlanogramSlotsReplace(svc))
	})
}

func listAdminProducts(app *api.HTTPApplication) http.HandlerFunc {
	svc := app.CatalogAdmin
	return func(w http.ResponseWriter, r *http.Request) {
		scopeID, err := requireCatalogPrincipalUUID(r, app.MediaAdmin)
		_ = scopeID
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_scope", err.Error())
			return
		}
		limit, offset, err := parseAdminLimitOffset(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_pagination", err.Error())
			return
		}
		search := strings.TrimSpace(r.URL.Query().Get("q"))
		activeOnly := strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("active_only")), "true") ||
			strings.TrimSpace(r.URL.Query().Get("active_only")) == "1"
		var brandID *uuid.UUID
		if raw := strings.TrimSpace(r.URL.Query().Get("brand_id")); raw != "" {
			bid, perr := uuid.Parse(raw)
			if perr != nil || bid == uuid.Nil {
				writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_brand_id", "invalid brand_id")
				return
			}
			brandID = &bid
		}
		var categoryID *uuid.UUID
		if raw := strings.TrimSpace(r.URL.Query().Get("category_id")); raw != "" {
			cid, perr := uuid.Parse(raw)
			if perr != nil || cid == uuid.Nil {
				writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_category_id", "invalid category_id")
				return
			}
			categoryID = &cid
		}
		res, err := svc.ListProducts(r.Context(), appcatalogadmin.ListProductsParams{
			Limit:      limit,
			Offset:     offset,
			Search:     search,
			ActiveOnly: activeOnly,
			BrandID:    brandID,
			CategoryID: categoryID,
		})
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		pids := make([]uuid.UUID, len(res.Items))
		for i := range res.Items {
			pids[i] = res.Items[i].ID
		}
		tagsBy, err := svc.ProductTagsByProductIDs(r.Context(), pids)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		assetByProd, err := svc.PrimaryMediaAssetByProductIDs(r.Context(), pids)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		var mediaByProd map[uuid.UUID]*V1AdminProductMediaDoc
		if app != nil && app.MediaAdmin != nil && len(assetByProd) > 0 {
			mediaByProd, err = batchAdminProductMediaDocs(r.Context(), app, assetByProd)
			if err != nil {
				log.Printf("admin products: media enrichment degraded: %v", err)
				mediaByProd = nil
			}
		}
		items := make([]V1AdminProductListItem, 0, len(res.Items))
		for _, row := range res.Items {
			tagRows := tagsBy[row.ID]
			tags := make([]V1AdminTag, 0, len(tagRows))
			for _, tr := range tagRows {
				tags = append(tags, mapAdminTag(tr))
			}
			var pmID *string
			if aid, ok := assetByProd[row.ID]; ok {
				s := aid.String()
				pmID = &s
			}
			var md *V1AdminProductMediaDoc
			if mediaByProd != nil {
				md = mediaByProd[row.ID]
			}
			items = append(items, V1AdminProductListItem{
				ID:             row.ID.String(),
				Sku:            row.Sku,
				Barcode:        textFromPgText(row.Barcode),
				Name:           row.Name,
				Description:    row.Description,
				Active:         row.Active,
				Status:         adminProductStatus(row.Active),
				CategoryID:     uuidPtrFromPgUUID(row.CategoryID),
				BrandID:        uuidPtrFromPgUUID(row.BrandID),
				PrimaryMediaID: pmID,
				Media:          md,
				Tags:           tags,
				CreatedAt:      formatAPITimeRFC3339Nano(row.CreatedAt),
				UpdatedAt:      formatAPITimeRFC3339Nano(row.UpdatedAt),
			})
		}
		writeJSON(w, http.StatusOK, V1AdminProductListEnvelope{
			Items: items,
			Meta: V1AdminPageMeta{
				Limit:      limit,
				Offset:     offset,
				Returned:   len(items),
				TotalCount: res.TotalCount,
			},
		})
	}
}

func getAdminProduct(app *api.HTTPApplication) http.HandlerFunc {
	svc := app.CatalogAdmin
	return func(w http.ResponseWriter, r *http.Request) {
		scopeID, err := requireCatalogPrincipalUUID(r, app.MediaAdmin)
		_ = scopeID
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_scope", err.Error())
			return
		}
		pid, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "productId")))
		if err != nil || pid == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_product_id", "invalid productId")
			return
		}
		row, err := svc.GetProduct(r.Context(), scopeID, pid)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				writeAPIError(w, r.Context(), http.StatusNotFound, "product_not_found", "product not found")
				return
			}
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		writeAdminProductResponse(w, r, app, svc, scopeID, row)
	}
}

func listAdminPriceBooks(svc *appcatalogadmin.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		scopeID, err := requireCatalogPrincipalUUID(r, nil)
		_ = scopeID
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_scope", err.Error())
			return
		}
		limit, offset, err := parseAdminLimitOffset(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_pagination", err.Error())
			return
		}
		includeInactive := strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("include_inactive")), "true") ||
			strings.TrimSpace(r.URL.Query().Get("include_inactive")) == "1"
		rows, total, err := svc.ListPriceBooks(r.Context(), appcatalogadmin.ListPriceBooksParams{
			Limit:           limit,
			Offset:          offset,
			IncludeInactive: includeInactive,
		})
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		items := make([]V1AdminPriceBook, 0, len(rows))
		for _, pb := range rows {
			items = append(items, mapPriceBook(pb))
		}
		writeJSON(w, http.StatusOK, V1AdminPriceBookListEnvelope{
			Items: items,
			Meta: V1AdminPageMeta{
				Limit:      limit,
				Offset:     offset,
				Returned:   len(items),
				TotalCount: total,
			},
		})
	}
}

func listAdminPlanograms(svc *appcatalogadmin.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		scopeID, err := requireCatalogPrincipalUUID(r, nil)
		_ = scopeID
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_scope", err.Error())
			return
		}
		limit, offset, err := parseAdminLimitOffset(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_pagination", err.Error())
			return
		}
		rows, total, err := svc.ListPlanograms(r.Context(), scopeID, limit, offset)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		items := make([]V1AdminPlanogram, 0, len(rows))
		for _, pg := range rows {
			items = append(items, mapPlanogram(pg))
		}
		writeJSON(w, http.StatusOK, V1AdminPlanogramListEnvelope{
			Items: items,
			Meta: V1AdminPageMeta{
				Limit:      limit,
				Offset:     offset,
				Returned:   len(items),
				TotalCount: total,
			},
		})
	}
}

func getAdminPlanogram(svc *appcatalogadmin.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		scopeID, err := requireCatalogPrincipalUUID(r, nil)
		_ = scopeID
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_scope", err.Error())
			return
		}
		pgid, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "planogramId")))
		if err != nil || pgid == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_planogram_id", "invalid planogramId")
			return
		}
		pg, err := svc.GetPlanogram(r.Context(), scopeID, pgid)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				writeAPIError(w, r.Context(), http.StatusNotFound, "planogram_not_found", "planogram not found")
				return
			}
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		slots, err := svc.ListPlanogramSlots(r.Context(), scopeID, pgid)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		slotItems := make([]V1AdminPlanogramSlot, 0, len(slots))
		for _, s := range slots {
			slotItems = append(slotItems, V1AdminPlanogramSlot{
				ID:          s.ID.String(),
				PlanogramID: s.PlanogramID.String(),
				SlotIndex:   s.SlotIndex,
				ProductID:   uuidPtrFromPgUUID(s.ProductID),
				MaxQuantity: s.MaxQuantity,
				ProductSku:  textFromPgText(s.ProductSku),
				ProductName: textFromPgText(s.ProductName),
				CreatedAt:   formatAPITimeRFC3339Nano(s.CreatedAt),
			})
		}
		writeJSON(w, http.StatusOK, V1AdminPlanogramDetail{
			Planogram: mapPlanogram(pg),
			Slots:     slotItems,
		})
	}
}

func postAdminPlanogramCreate(svc *appcatalogadmin.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := requireWriteIdempotencyKey(r); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "missing_idempotency_key", err.Error())
			return
		}
		scopeID, err := requireCatalogPrincipalUUID(r, nil)
		_ = scopeID
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_scope", err.Error())
			return
		}
		var body V1AdminPlanogramCreateRequest
		if !decodeStrictJSON(w, r, &body) {
			return
		}
		in := appcatalogadmin.CreatePlanogramInput{
			Name:   body.Name,
			Status: body.Status,
			Meta:   body.Meta,
		}
		if body.Revision != nil {
			in.Revision = *body.Revision
		}
		row, err := svc.CreatePlanogram(r.Context(), in)
		if err != nil {
			writeAdminCatalogError(w, r, err)
			return
		}
		writeJSON(w, http.StatusCreated, mapPlanogram(row))
	}
}

func patchAdminPlanogramUpdate(svc *appcatalogadmin.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := requireWriteIdempotencyKey(r); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "missing_idempotency_key", err.Error())
			return
		}
		if _, err := requireCatalogPrincipalUUID(r, nil); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_scope", err.Error())
			return
		}
		pgid, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "planogramId")))
		if err != nil || pgid == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_planogram_id", "invalid planogramId")
			return
		}
		var body V1AdminPlanogramPatchRequest
		if !decodeStrictJSON(w, r, &body) {
			return
		}
		row, err := svc.UpdatePlanogram(r.Context(), appcatalogadmin.UpdatePlanogramInput{
			PlanogramID: pgid,
			Name:        body.Name,
			Status:      body.Status,
			Revision:    body.Revision,
		})
		if err != nil {
			writeAdminCatalogError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, mapPlanogram(row))
	}
}

func deleteAdminPlanogram(svc *appcatalogadmin.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := requireWriteIdempotencyKey(r); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "missing_idempotency_key", err.Error())
			return
		}
		if _, err := requireCatalogPrincipalUUID(r, nil); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_scope", err.Error())
			return
		}
		pgid, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "planogramId")))
		if err != nil || pgid == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_planogram_id", "invalid planogramId")
			return
		}
		if err := svc.DeletePlanogram(r.Context(), pgid); err != nil {
			writeAdminCatalogError(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func putAdminPlanogramSlotsReplace(svc *appcatalogadmin.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := requireWriteIdempotencyKey(r); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "missing_idempotency_key", err.Error())
			return
		}
		scopeID, err := requireCatalogPrincipalUUID(r, nil)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_scope", err.Error())
			return
		}
		pgid, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "planogramId")))
		if err != nil || pgid == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_planogram_id", "invalid planogramId")
			return
		}
		var body V1AdminPlanogramSlotsReplaceRequest
		if !decodeStrictJSON(w, r, &body) {
			return
		}
		slots := make([]appcatalogadmin.PlanogramSlotReplaceInput, 0, len(body.Slots))
		for _, item := range body.Slots {
			var productID *uuid.UUID
			if item.ProductID != nil {
				pid, perr := uuidFromOptionalString(item.ProductID)
				if perr != nil {
					writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_product_id", "invalid productId")
					return
				}
				productID = pid
			}
			slots = append(slots, appcatalogadmin.PlanogramSlotReplaceInput{
				SlotIndex:   item.SlotIndex,
				ProductID:   productID,
				MaxQuantity: item.MaxQuantity,
			})
		}
		pg, err := svc.ReplacePlanogramSlots(r.Context(), appcatalogadmin.ReplacePlanogramSlotsInput{
			PlanogramID: pgid,
			Slots:       slots,
		})
		if err != nil {
			writeAdminCatalogError(w, r, err)
			return
		}
		slotRows, err := svc.ListPlanogramSlots(r.Context(), scopeID, pgid)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, buildAdminPlanogramDetail(pg, slotRows))
	}
}

func buildAdminPlanogramDetail(pg db.Planogram, slots []db.CatalogAdminListSlotsByPlanogramRow) V1AdminPlanogramDetail {
	slotItems := make([]V1AdminPlanogramSlot, 0, len(slots))
	for _, s := range slots {
		slotItems = append(slotItems, V1AdminPlanogramSlot{
			ID:          s.ID.String(),
			PlanogramID: s.PlanogramID.String(),
			SlotIndex:   s.SlotIndex,
			ProductID:   uuidPtrFromPgUUID(s.ProductID),
			MaxQuantity: s.MaxQuantity,
			ProductSku:  textFromPgText(s.ProductSku),
			ProductName: textFromPgText(s.ProductName),
			CreatedAt:   formatAPITimeRFC3339Nano(s.CreatedAt),
		})
	}
	return V1AdminPlanogramDetail{
		Planogram: mapPlanogram(pg),
		Slots:     slotItems,
	}
}

func listAdminBrands(svc *appcatalogadmin.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		scopeID, err := requireCatalogPrincipalUUID(r, nil)
		_ = scopeID
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_scope", err.Error())
			return
		}
		limit, offset, err := parseAdminLimitOffset(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_pagination", err.Error())
			return
		}
		rows, total, err := svc.ListBrands(r.Context(), appcatalogadmin.ListBrandsParams{
			Limit:  limit,
			Offset: offset,
		})
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		items := make([]V1AdminBrand, 0, len(rows))
		for _, b := range rows {
			items = append(items, mapAdminBrand(b))
		}
		writeJSON(w, http.StatusOK, V1AdminBrandListEnvelope{
			Items: items,
			Meta: V1AdminPageMeta{
				Limit:      limit,
				Offset:     offset,
				Returned:   len(items),
				TotalCount: total,
			},
		})
	}
}

func listAdminCategories(svc *appcatalogadmin.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		scopeID, err := requireCatalogPrincipalUUID(r, nil)
		_ = scopeID
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_scope", err.Error())
			return
		}
		limit, offset, err := parseAdminLimitOffset(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_pagination", err.Error())
			return
		}
		rows, total, err := svc.ListCategories(r.Context(), appcatalogadmin.ListCategoriesParams{
			Limit:  limit,
			Offset: offset,
		})
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		items := make([]V1AdminCategory, 0, len(rows))
		for _, c := range rows {
			items = append(items, mapAdminCategory(c))
		}
		writeJSON(w, http.StatusOK, V1AdminCategoryListEnvelope{
			Items: items,
			Meta: V1AdminPageMeta{
				Limit:      limit,
				Offset:     offset,
				Returned:   len(items),
				TotalCount: total,
			},
		})
	}
}

func listAdminTags(svc *appcatalogadmin.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		scopeID, err := requireCatalogPrincipalUUID(r, nil)
		_ = scopeID
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_scope", err.Error())
			return
		}
		limit, offset, err := parseAdminLimitOffset(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_pagination", err.Error())
			return
		}
		rows, total, err := svc.ListTags(r.Context(), appcatalogadmin.ListTagsParams{
			Limit:  limit,
			Offset: offset,
		})
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		items := make([]V1AdminTag, 0, len(rows))
		for _, t := range rows {
			items = append(items, mapAdminTag(t))
		}
		writeJSON(w, http.StatusOK, V1AdminTagListEnvelope{
			Items: items,
			Meta: V1AdminPageMeta{
				Limit:      limit,
				Offset:     offset,
				Returned:   len(items),
				TotalCount: total,
			},
		})
	}
}

func mapAdminBrand(b db.Brand) V1AdminBrand {
	return V1AdminBrand{
		ID:        b.ID.String(),
		Slug:      b.Slug,
		Name:      b.Name,
		Active:    b.Active,
		CreatedAt: formatAPITimeRFC3339Nano(b.CreatedAt),
		UpdatedAt: formatAPITimeRFC3339Nano(b.UpdatedAt),
	}
}

func mapAdminCategory(c db.Category) V1AdminCategory {
	out := V1AdminCategory{
		ID:        c.ID.String(),
		Slug:      c.Slug,
		Name:      c.Name,
		Active:    c.Active,
		CreatedAt: formatAPITimeRFC3339Nano(c.CreatedAt),
		UpdatedAt: formatAPITimeRFC3339Nano(c.UpdatedAt),
	}
	if c.ParentID.Valid {
		s := uuid.UUID(c.ParentID.Bytes).String()
		out.ParentID = &s
	}
	return out
}

func mapAdminTag(t db.Tag) V1AdminTag {
	return V1AdminTag{
		ID:        t.ID.String(),
		Slug:      t.Slug,
		Name:      t.Name,
		Active:    t.Active,
		CreatedAt: formatAPITimeRFC3339Nano(t.CreatedAt),
		UpdatedAt: formatAPITimeRFC3339Nano(t.UpdatedAt),
	}
}

func uuidPtrFromPgUUID(u pgtype.UUID) *string {
	if !u.Valid {
		return nil
	}
	s := uuid.UUID(u.Bytes).String()
	return &s
}

func textFromPgText(t pgtype.Text) *string {
	if !t.Valid {
		return nil
	}
	s := t.String
	return &s
}

func mapAdminProduct(p db.Product, img *db.ProductImage, tags []db.Tag, media *V1AdminProductMediaDoc) V1AdminProduct {
	if tags == nil {
		tags = []db.Tag{}
	}
	tagViews := make([]V1AdminTag, 0, len(tags))
	for _, tr := range tags {
		tagViews = append(tagViews, mapAdminTag(tr))
	}
	out := V1AdminProduct{
		ID:              p.ID.String(),
		Sku:             p.Sku,
		Barcode:         textFromPgText(p.Barcode),
		Name:            p.Name,
		Description:     p.Description,
		Active:          p.Active,
		Status:          adminProductStatus(p.Active),
		CategoryID:      uuidPtrFromPgUUID(p.CategoryID),
		BrandID:         uuidPtrFromPgUUID(p.BrandID),
		PrimaryMediaID:  primaryMediaIDPtr(img),
		Media:           media,
		PrimaryImageID:  uuidPtrFromPgUUID(p.PrimaryImageID),
		CountryOfOrigin: textFromPgText(p.CountryOfOrigin),
		AgeRestricted:   p.AgeRestricted,
		AllergenCodes:   append([]string(nil), p.AllergenCodes...),
		NutritionalNote: textFromPgText(p.NutritionalNote),
		Tags:            tagViews,
		CreatedAt:       formatAPITimeRFC3339Nano(p.CreatedAt),
		UpdatedAt:       formatAPITimeRFC3339Nano(p.UpdatedAt),
	}
	if img != nil {
		if img.CdnUrl.Valid {
			s := strings.TrimSpace(img.CdnUrl.String)
			if s != "" {
				out.DisplayURL = &s
				out.ImageURL = &s
			}
		}
		if img.ThumbCdnUrl.Valid {
			s := strings.TrimSpace(img.ThumbCdnUrl.String)
			if s != "" {
				out.ThumbURL = &s
			}
		}
	}
	if len(p.Attrs) > 0 && json.Valid(p.Attrs) {
		out.Attrs = json.RawMessage(p.Attrs)
	} else if len(p.Attrs) > 0 {
		out.Attrs = json.RawMessage([]byte(`{}`))
	}
	return out
}

func writeAdminProductResponse(w http.ResponseWriter, r *http.Request, app *api.HTTPApplication, svc *appcatalogadmin.Service, scopeID uuid.UUID, p db.Product) {
	img, err := svc.PrimaryProductImageOrNil(r.Context(), scopeID, p.ID)
	if err != nil {
		writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
		return
	}
	tagsBy, err := svc.ProductTagsByProductIDs(r.Context(), []uuid.UUID{p.ID})
	if err != nil {
		writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
		return
	}
	var media *V1AdminProductMediaDoc
	if app != nil && app.MediaAdmin != nil {
		media = buildAdminProductMediaDoc(r.Context(), app, img)
	}
	writeJSON(w, http.StatusOK, mapAdminProduct(p, img, tagsBy[p.ID], media))
}

func mapPriceBook(pb db.PriceBook) V1AdminPriceBook {
	return V1AdminPriceBook{
		ID:             pb.ID.String(),
		Name:           pb.Name,
		Currency:       pb.Currency,
		EffectiveFrom:  formatAPITimeRFC3339Nano(pb.EffectiveFrom),
		EffectiveTo:    timePtrFromTimestamptz(pb.EffectiveTo),
		IsDefault:      pb.IsDefault,
		Active:         pb.Active,
		PriceBookLevel: pb.PriceBookLevel,
		SiteID:         uuidPtrFromPgUUID(pb.SiteID),
		MachineID:      uuidPtrFromPgUUID(pb.MachineID),
		Priority:       pb.Priority,
		CreatedAt:      formatAPITimeRFC3339Nano(pb.CreatedAt),
		UpdatedAt:      formatAPITimeRFC3339Nano(pb.UpdatedAt),
	}
}

func mapPlanogram(pg db.Planogram) V1AdminPlanogram {
	out := V1AdminPlanogram{
		ID:        pg.ID.String(),
		Name:      pg.Name,
		Revision:  pg.Revision,
		Status:    pg.Status,
		CreatedAt: formatAPITimeRFC3339Nano(pg.CreatedAt),
	}
	if len(pg.Meta) > 0 && json.Valid(pg.Meta) {
		out.Meta = json.RawMessage(pg.Meta)
	}
	return out
}

func timePtrFromTimestamptz(ts pgtype.Timestamptz) *string {
	if !ts.Valid {
		return nil
	}
	s := formatAPITimeRFC3339Nano(ts.Time)
	return &s
}
