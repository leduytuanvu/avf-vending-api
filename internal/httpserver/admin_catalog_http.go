package httpserver

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/avf/avf-vending-api/internal/app/api"
	appcatalogadmin "github.com/avf/avf-vending-api/internal/app/catalogadmin"
	"github.com/avf/avf-vending-api/internal/gen/db"
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
	r.Get("/products", listAdminProducts(svc))
	r.Get("/products/{productId}", getAdminProduct(svc))
	r.With(writeRL).Post("/products", postAdminProductCreate(svc))
	r.With(writeRL).Put("/products/{productId}", putAdminProductUpdate(svc))
	r.With(writeRL).Patch("/products/{productId}", putAdminProductUpdate(svc))
	r.With(writeRL).Delete("/products/{productId}", deleteAdminProduct(svc))
	r.With(writeRL).Put("/products/{productId}/image", putAdminProductImage(svc))
	r.With(writeRL).Delete("/products/{productId}/image", deleteAdminProductImage(svc))

	r.Get("/brands", listAdminBrands(svc))
	r.Get("/categories", listAdminCategories(svc))
	r.Get("/tags", listAdminTags(svc))
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

	r.Get("/price-books", listAdminPriceBooks(svc))
	r.Get("/planograms", listAdminPlanograms(svc))
	r.Get("/planograms/{planogramId}", getAdminPlanogram(svc))
}

func listAdminProducts(svc *appcatalogadmin.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, err := adminCatalogOrganizationID(r)
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
		res, err := svc.ListProducts(r.Context(), appcatalogadmin.ListProductsParams{
			OrganizationID: orgID,
			Limit:          limit,
			Offset:         offset,
			Search:         search,
			ActiveOnly:     activeOnly,
		})
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		items := make([]V1AdminProductListItem, 0, len(res.Items))
		for _, row := range res.Items {
			items = append(items, V1AdminProductListItem{
				ID:             row.ID.String(),
				OrganizationID: row.OrganizationID.String(),
				Sku:            row.Sku,
				Barcode:        textFromPgText(row.Barcode),
				Name:           row.Name,
				Description:    row.Description,
				Active:         row.Active,
				CategoryID:     uuidPtrFromPgUUID(row.CategoryID),
				BrandID:        uuidPtrFromPgUUID(row.BrandID),
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

func getAdminProduct(svc *appcatalogadmin.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, err := adminCatalogOrganizationID(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_scope", err.Error())
			return
		}
		pid, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "productId")))
		if err != nil || pid == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_product_id", "invalid productId")
			return
		}
		row, err := svc.GetProduct(r.Context(), orgID, pid)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				writeAPIError(w, r.Context(), http.StatusNotFound, "product_not_found", "product not found")
				return
			}
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, mapAdminProduct(row))
	}
}

func listAdminPriceBooks(svc *appcatalogadmin.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, err := adminCatalogOrganizationID(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_scope", err.Error())
			return
		}
		limit, offset, err := parseAdminLimitOffset(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_pagination", err.Error())
			return
		}
		rows, total, err := svc.ListPriceBooks(r.Context(), orgID, limit, offset)
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
		orgID, err := adminCatalogOrganizationID(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_scope", err.Error())
			return
		}
		limit, offset, err := parseAdminLimitOffset(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_pagination", err.Error())
			return
		}
		rows, total, err := svc.ListPlanograms(r.Context(), orgID, limit, offset)
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
		orgID, err := adminCatalogOrganizationID(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_scope", err.Error())
			return
		}
		pgid, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "planogramId")))
		if err != nil || pgid == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_planogram_id", "invalid planogramId")
			return
		}
		pg, err := svc.GetPlanogram(r.Context(), orgID, pgid)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				writeAPIError(w, r.Context(), http.StatusNotFound, "planogram_not_found", "planogram not found")
				return
			}
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		slots, err := svc.ListPlanogramSlots(r.Context(), orgID, pgid)
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

func listAdminBrands(svc *appcatalogadmin.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, err := adminCatalogOrganizationID(r)
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
			OrganizationID: orgID,
			Limit:          limit,
			Offset:         offset,
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
		orgID, err := adminCatalogOrganizationID(r)
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
			OrganizationID: orgID,
			Limit:          limit,
			Offset:         offset,
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
		orgID, err := adminCatalogOrganizationID(r)
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
			OrganizationID: orgID,
			Limit:          limit,
			Offset:         offset,
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
		ID:             b.ID.String(),
		OrganizationID: b.OrganizationID.String(),
		Slug:           b.Slug,
		Name:           b.Name,
		Active:         b.Active,
		CreatedAt:      formatAPITimeRFC3339Nano(b.CreatedAt),
		UpdatedAt:      formatAPITimeRFC3339Nano(b.UpdatedAt),
	}
}

func mapAdminCategory(c db.Category) V1AdminCategory {
	out := V1AdminCategory{
		ID:             c.ID.String(),
		OrganizationID: c.OrganizationID.String(),
		Slug:           c.Slug,
		Name:           c.Name,
		Active:         c.Active,
		CreatedAt:      formatAPITimeRFC3339Nano(c.CreatedAt),
		UpdatedAt:      formatAPITimeRFC3339Nano(c.UpdatedAt),
	}
	if c.ParentID.Valid {
		s := uuid.UUID(c.ParentID.Bytes).String()
		out.ParentID = &s
	}
	return out
}

func mapAdminTag(t db.Tag) V1AdminTag {
	return V1AdminTag{
		ID:             t.ID.String(),
		OrganizationID: t.OrganizationID.String(),
		Slug:           t.Slug,
		Name:           t.Name,
		Active:         t.Active,
		CreatedAt:      formatAPITimeRFC3339Nano(t.CreatedAt),
		UpdatedAt:      formatAPITimeRFC3339Nano(t.UpdatedAt),
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

func mapAdminProduct(p db.Product) V1AdminProduct {
	out := V1AdminProduct{
		ID:              p.ID.String(),
		OrganizationID:  p.OrganizationID.String(),
		Sku:             p.Sku,
		Barcode:         textFromPgText(p.Barcode),
		Name:            p.Name,
		Description:     p.Description,
		Active:          p.Active,
		CategoryID:      uuidPtrFromPgUUID(p.CategoryID),
		BrandID:         uuidPtrFromPgUUID(p.BrandID),
		PrimaryImageID:  uuidPtrFromPgUUID(p.PrimaryImageID),
		CountryOfOrigin: textFromPgText(p.CountryOfOrigin),
		AgeRestricted:   p.AgeRestricted,
		AllergenCodes:   append([]string(nil), p.AllergenCodes...),
		NutritionalNote: textFromPgText(p.NutritionalNote),
		CreatedAt:       formatAPITimeRFC3339Nano(p.CreatedAt),
		UpdatedAt:       formatAPITimeRFC3339Nano(p.UpdatedAt),
	}
	if len(p.Attrs) > 0 && json.Valid(p.Attrs) {
		out.Attrs = json.RawMessage(p.Attrs)
	} else if len(p.Attrs) > 0 {
		out.Attrs = json.RawMessage([]byte(`{}`))
	}
	return out
}

func mapPriceBook(pb db.PriceBook) V1AdminPriceBook {
	return V1AdminPriceBook{
		ID:             pb.ID.String(),
		OrganizationID: pb.OrganizationID.String(),
		Name:           pb.Name,
		Currency:       pb.Currency,
		EffectiveFrom:  formatAPITimeRFC3339Nano(pb.EffectiveFrom),
		EffectiveTo:    timePtrFromTimestamptz(pb.EffectiveTo),
		IsDefault:      pb.IsDefault,
		ScopeType:      pb.ScopeType,
		SiteID:         uuidPtrFromPgUUID(pb.SiteID),
		MachineID:      uuidPtrFromPgUUID(pb.MachineID),
		Priority:       pb.Priority,
		CreatedAt:      formatAPITimeRFC3339Nano(pb.CreatedAt),
	}
}

func mapPlanogram(pg db.Planogram) V1AdminPlanogram {
	out := V1AdminPlanogram{
		ID:             pg.ID.String(),
		OrganizationID: pg.OrganizationID.String(),
		Name:           pg.Name,
		Revision:       pg.Revision,
		Status:         pg.Status,
		CreatedAt:      formatAPITimeRFC3339Nano(pg.CreatedAt),
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
