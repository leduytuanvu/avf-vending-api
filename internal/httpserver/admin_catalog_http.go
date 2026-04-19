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

func mountAdminCatalogRoutes(r chi.Router, app *api.HTTPApplication) {
	if app == nil || app.CatalogAdmin == nil {
		return
	}
	svc := app.CatalogAdmin
	r.Get("/products", listAdminProducts(svc))
	r.Get("/products/{productId}", getAdminProduct(svc))
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
				Name:           row.Name,
				Description:    row.Description,
				Active:         row.Active,
				CategoryID:     uuidPtrFromPgUUID(row.CategoryID),
				BrandID:        uuidPtrFromPgUUID(row.BrandID),
				CreatedAt:      row.CreatedAt.UTC().Format(timeRFC3339Nano),
				UpdatedAt:      row.UpdatedAt.UTC().Format(timeRFC3339Nano),
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
				CreatedAt:   s.CreatedAt.UTC().Format(timeRFC3339Nano),
			})
		}
		writeJSON(w, http.StatusOK, V1AdminPlanogramDetail{
			Planogram: mapPlanogram(pg),
			Slots:     slotItems,
		})
	}
}

const timeRFC3339Nano = "2006-01-02T15:04:05.999999999Z07:00"

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
		CreatedAt:       p.CreatedAt.UTC().Format(timeRFC3339Nano),
		UpdatedAt:       p.UpdatedAt.UTC().Format(timeRFC3339Nano),
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
		EffectiveFrom:  pb.EffectiveFrom.UTC().Format(timeRFC3339Nano),
		EffectiveTo:    timePtrFromTimestamptz(pb.EffectiveTo),
		IsDefault:      pb.IsDefault,
		ScopeType:      pb.ScopeType,
		SiteID:         uuidPtrFromPgUUID(pb.SiteID),
		MachineID:      uuidPtrFromPgUUID(pb.MachineID),
		Priority:       pb.Priority,
		CreatedAt:      pb.CreatedAt.UTC().Format(timeRFC3339Nano),
	}
}

func mapPlanogram(pg db.Planogram) V1AdminPlanogram {
	out := V1AdminPlanogram{
		ID:             pg.ID.String(),
		OrganizationID: pg.OrganizationID.String(),
		Name:           pg.Name,
		Revision:       pg.Revision,
		Status:         pg.Status,
		CreatedAt:      pg.CreatedAt.UTC().Format(timeRFC3339Nano),
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
	s := ts.Time.UTC().Format(timeRFC3339Nano)
	return &s
}
