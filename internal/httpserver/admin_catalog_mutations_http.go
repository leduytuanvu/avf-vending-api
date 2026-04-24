package httpserver

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	appcatalogadmin "github.com/avf/avf-vending-api/internal/app/catalogadmin"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func writeAdminCatalogError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, appcatalogadmin.ErrNotFound):
		writeAPIError(w, r.Context(), http.StatusNotFound, "not_found", "resource not found")
	case errors.Is(err, appcatalogadmin.ErrDuplicateSKU):
		writeAPIError(w, r.Context(), http.StatusConflict, "duplicate_sku", "sku already exists in this organization")
	case errors.Is(err, appcatalogadmin.ErrDuplicateBarcode):
		writeAPIError(w, r.Context(), http.StatusConflict, "duplicate_barcode", "barcode already exists in this organization")
	case errors.Is(err, appcatalogadmin.ErrDuplicateSlug):
		writeAPIError(w, r.Context(), http.StatusConflict, "duplicate_slug", "slug already exists in this organization")
	case errors.Is(err, appcatalogadmin.ErrInvalidArgument):
		writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_argument", err.Error())
	default:
		writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
	}
}

func uuidFromOptionalString(s *string) (*uuid.UUID, error) {
	if s == nil {
		return nil, nil
	}
	raw := strings.TrimSpace(*s)
	if raw == "" {
		return nil, nil
	}
	u, err := uuid.Parse(raw)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func mergeOptionalUUID(body *string, cur pgtype.UUID) (*uuid.UUID, error) {
	if body != nil {
		raw := strings.TrimSpace(*body)
		if raw == "" {
			return nil, nil
		}
		u, err := uuid.Parse(raw)
		if err != nil {
			return nil, err
		}
		return &u, nil
	}
	if cur.Valid {
		u := uuid.UUID(cur.Bytes)
		return &u, nil
	}
	return nil, nil
}

func mergeOptionalText(body *string, cur pgtype.Text) *string {
	if body != nil {
		s := strings.TrimSpace(*body)
		return &s
	}
	if cur.Valid {
		s := strings.TrimSpace(cur.String)
		return &s
	}
	empty := ""
	return &empty
}

func mergeProductMutation(cur db.Product, body V1AdminProductMutationRequest) (appcatalogadmin.UpdateProductInput, error) {
	sku := strings.TrimSpace(body.Sku)
	if sku == "" {
		sku = cur.Sku
	}
	name := strings.TrimSpace(body.Name)
	if name == "" {
		name = cur.Name
	}
	desc := body.Description
	if strings.TrimSpace(desc) == "" {
		desc = cur.Description
	}
	attrs := body.Attrs
	if len(attrs) == 0 && len(cur.Attrs) > 0 {
		attrs = json.RawMessage(append([]byte(nil), cur.Attrs...))
	}
	var bc *string
	if body.Barcode != nil {
		v := strings.TrimSpace(*body.Barcode)
		bc = &v
	} else if cur.Barcode.Valid {
		v := strings.TrimSpace(cur.Barcode.String)
		bc = &v
	} else {
		empty := ""
		bc = &empty
	}
	cat, err := mergeOptionalUUID(body.CategoryID, cur.CategoryID)
	if err != nil {
		return appcatalogadmin.UpdateProductInput{}, err
	}
	brand, err := mergeOptionalUUID(body.BrandID, cur.BrandID)
	if err != nil {
		return appcatalogadmin.UpdateProductInput{}, err
	}
	allergen := append([]string(nil), cur.AllergenCodes...)
	if body.AllergenCodes != nil {
		allergen = append([]string(nil), body.AllergenCodes...)
	}
	return appcatalogadmin.UpdateProductInput{
		OrganizationID:  cur.OrganizationID,
		ProductID:       cur.ID,
		Sku:             sku,
		Barcode:         bc,
		Name:            name,
		Description:     desc,
		Attrs:           attrs,
		Active:          body.Active,
		CategoryID:      cat,
		BrandID:         brand,
		CountryOfOrigin: mergeOptionalText(body.CountryOfOrigin, cur.CountryOfOrigin),
		AgeRestricted:   body.AgeRestricted,
		AllergenCodes:   allergen,
		NutritionalNote: mergeOptionalText(body.NutritionalNote, cur.NutritionalNote),
	}, nil
}

func postAdminProductCreate(svc *appcatalogadmin.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := requireWriteIdempotencyKey(r); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "missing_idempotency_key", err.Error())
			return
		}
		orgID, err := adminCatalogOrganizationID(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_scope", err.Error())
			return
		}
		var body V1AdminProductMutationRequest
		if !decodeStrictJSON(w, r, &body) {
			return
		}
		cat, err := uuidFromOptionalString(body.CategoryID)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_category_id", "invalid categoryId")
			return
		}
		brand, err := uuidFromOptionalString(body.BrandID)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_brand_id", "invalid brandId")
			return
		}
		row, err := svc.CreateProduct(r.Context(), appcatalogadmin.CreateProductInput{
			OrganizationID:  orgID,
			Sku:             body.Sku,
			Barcode:         body.Barcode,
			Name:            body.Name,
			Description:     body.Description,
			Attrs:           body.Attrs,
			Active:          body.Active,
			CategoryID:      cat,
			BrandID:         brand,
			CountryOfOrigin: body.CountryOfOrigin,
			AgeRestricted:   body.AgeRestricted,
			AllergenCodes:   body.AllergenCodes,
			NutritionalNote: body.NutritionalNote,
		})
		if err != nil {
			writeAdminCatalogError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, mapAdminProduct(row))
	}
}

func putAdminProductUpdate(svc *appcatalogadmin.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := requireWriteIdempotencyKey(r); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "missing_idempotency_key", err.Error())
			return
		}
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
		cur, err := svc.GetProduct(r.Context(), orgID, pid)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				writeAPIError(w, r.Context(), http.StatusNotFound, "product_not_found", "product not found")
				return
			}
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		var body V1AdminProductMutationRequest
		if !decodeStrictJSON(w, r, &body) {
			return
		}
		in, err := mergeProductMutation(cur, body)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_reference_id", err.Error())
			return
		}
		row, err := svc.UpdateProduct(r.Context(), in)
		if err != nil {
			writeAdminCatalogError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, mapAdminProduct(row))
	}
}

func deleteAdminProduct(svc *appcatalogadmin.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := requireWriteIdempotencyKey(r); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "missing_idempotency_key", err.Error())
			return
		}
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
		row, err := svc.DeactivateProduct(r.Context(), orgID, pid)
		if err != nil {
			writeAdminCatalogError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, mapAdminProduct(row))
	}
}

func putAdminProductImage(svc *appcatalogadmin.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := requireWriteIdempotencyKey(r); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "missing_idempotency_key", err.Error())
			return
		}
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
		var body V1AdminProductImageBindRequest
		if !decodeStrictJSON(w, r, &body) {
			return
		}
		if err := validateProductImageBindInput(body.DisplayURL, body.ThumbURL, body.ContentHash, body.MimeType); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_image_bind", err.Error())
			return
		}
		aid, err := uuid.Parse(strings.TrimSpace(body.ArtifactID))
		if err != nil || aid == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_artifact_id", "invalid artifactId")
			return
		}
		row, err := svc.BindProductPrimaryImage(r.Context(), appcatalogadmin.BindProductImageInput{
			OrganizationID: orgID,
			ProductID:      pid,
			ArtifactID:     aid,
			ThumbURL:       body.ThumbURL,
			DisplayURL:     body.DisplayURL,
			ContentHash:    body.ContentHash,
			Width:          body.Width,
			Height:         body.Height,
			MimeType:       body.MimeType,
		})
		if err != nil {
			writeAdminCatalogError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, mapAdminProduct(row))
	}
}

func deleteAdminProductImage(svc *appcatalogadmin.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := requireWriteIdempotencyKey(r); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "missing_idempotency_key", err.Error())
			return
		}
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
		row, err := svc.ClearProductPrimaryImage(r.Context(), orgID, pid)
		if err != nil {
			writeAdminCatalogError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, mapAdminProduct(row))
	}
}

func postAdminBrandCreate(svc *appcatalogadmin.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := requireWriteIdempotencyKey(r); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "missing_idempotency_key", err.Error())
			return
		}
		orgID, err := adminCatalogOrganizationID(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_scope", err.Error())
			return
		}
		var body V1AdminBrandMutationRequest
		if !decodeStrictJSON(w, r, &body) {
			return
		}
		row, err := svc.CreateBrand(r.Context(), appcatalogadmin.CreateBrandInput{
			OrganizationID: orgID,
			Slug:           body.Slug,
			Name:           body.Name,
			Active:         body.Active,
		})
		if err != nil {
			writeAdminCatalogError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, mapAdminBrand(row))
	}
}

func putAdminBrandUpdate(svc *appcatalogadmin.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := requireWriteIdempotencyKey(r); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "missing_idempotency_key", err.Error())
			return
		}
		orgID, err := adminCatalogOrganizationID(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_scope", err.Error())
			return
		}
		bid, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "brandId")))
		if err != nil || bid == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_brand_id", "invalid brandId")
			return
		}
		var body V1AdminBrandMutationRequest
		if !decodeStrictJSON(w, r, &body) {
			return
		}
		row, err := svc.UpdateBrand(r.Context(), appcatalogadmin.UpdateBrandInput{
			OrganizationID: orgID,
			BrandID:        bid,
			Slug:           body.Slug,
			Name:           body.Name,
			Active:         body.Active,
		})
		if err != nil {
			writeAdminCatalogError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, mapAdminBrand(row))
	}
}

func deleteAdminBrand(svc *appcatalogadmin.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := requireWriteIdempotencyKey(r); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "missing_idempotency_key", err.Error())
			return
		}
		orgID, err := adminCatalogOrganizationID(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_scope", err.Error())
			return
		}
		bid, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "brandId")))
		if err != nil || bid == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_brand_id", "invalid brandId")
			return
		}
		row, err := svc.DeactivateBrand(r.Context(), orgID, bid)
		if err != nil {
			writeAdminCatalogError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, mapAdminBrand(row))
	}
}

func postAdminCategoryCreate(svc *appcatalogadmin.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := requireWriteIdempotencyKey(r); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "missing_idempotency_key", err.Error())
			return
		}
		orgID, err := adminCatalogOrganizationID(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_scope", err.Error())
			return
		}
		var body V1AdminCategoryMutationRequest
		if !decodeStrictJSON(w, r, &body) {
			return
		}
		parent, err := uuidFromOptionalString(body.ParentID)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_parent_id", "invalid parentId")
			return
		}
		row, err := svc.CreateCategory(r.Context(), appcatalogadmin.CreateCategoryInput{
			OrganizationID: orgID,
			Slug:           body.Slug,
			Name:           body.Name,
			ParentID:       parent,
			Active:         body.Active,
		})
		if err != nil {
			writeAdminCatalogError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, mapAdminCategory(row))
	}
}

func putAdminCategoryUpdate(svc *appcatalogadmin.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := requireWriteIdempotencyKey(r); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "missing_idempotency_key", err.Error())
			return
		}
		orgID, err := adminCatalogOrganizationID(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_scope", err.Error())
			return
		}
		cid, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "categoryId")))
		if err != nil || cid == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_category_id", "invalid categoryId")
			return
		}
		var body V1AdminCategoryMutationRequest
		if !decodeStrictJSON(w, r, &body) {
			return
		}
		parent, err := uuidFromOptionalString(body.ParentID)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_parent_id", "invalid parentId")
			return
		}
		row, err := svc.UpdateCategory(r.Context(), appcatalogadmin.UpdateCategoryInput{
			OrganizationID: orgID,
			CategoryID:     cid,
			Slug:           body.Slug,
			Name:           body.Name,
			ParentID:       parent,
			Active:         body.Active,
		})
		if err != nil {
			writeAdminCatalogError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, mapAdminCategory(row))
	}
}

func deleteAdminCategory(svc *appcatalogadmin.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := requireWriteIdempotencyKey(r); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "missing_idempotency_key", err.Error())
			return
		}
		orgID, err := adminCatalogOrganizationID(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_scope", err.Error())
			return
		}
		cid, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "categoryId")))
		if err != nil || cid == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_category_id", "invalid categoryId")
			return
		}
		row, err := svc.DeactivateCategory(r.Context(), orgID, cid)
		if err != nil {
			writeAdminCatalogError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, mapAdminCategory(row))
	}
}

func postAdminTagCreate(svc *appcatalogadmin.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := requireWriteIdempotencyKey(r); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "missing_idempotency_key", err.Error())
			return
		}
		orgID, err := adminCatalogOrganizationID(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_scope", err.Error())
			return
		}
		var body V1AdminTagMutationRequest
		if !decodeStrictJSON(w, r, &body) {
			return
		}
		row, err := svc.CreateTag(r.Context(), appcatalogadmin.CreateTagInput{
			OrganizationID: orgID,
			Slug:           body.Slug,
			Name:           body.Name,
			Active:         body.Active,
		})
		if err != nil {
			writeAdminCatalogError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, mapAdminTag(row))
	}
}

func putAdminTagUpdate(svc *appcatalogadmin.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := requireWriteIdempotencyKey(r); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "missing_idempotency_key", err.Error())
			return
		}
		orgID, err := adminCatalogOrganizationID(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_scope", err.Error())
			return
		}
		tid, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "tagId")))
		if err != nil || tid == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_tag_id", "invalid tagId")
			return
		}
		var body V1AdminTagMutationRequest
		if !decodeStrictJSON(w, r, &body) {
			return
		}
		row, err := svc.UpdateTag(r.Context(), appcatalogadmin.UpdateTagInput{
			OrganizationID: orgID,
			TagID:          tid,
			Slug:           body.Slug,
			Name:           body.Name,
			Active:         body.Active,
		})
		if err != nil {
			writeAdminCatalogError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, mapAdminTag(row))
	}
}

func deleteAdminTag(svc *appcatalogadmin.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := requireWriteIdempotencyKey(r); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "missing_idempotency_key", err.Error())
			return
		}
		orgID, err := adminCatalogOrganizationID(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_scope", err.Error())
			return
		}
		tid, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "tagId")))
		if err != nil || tid == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_tag_id", "invalid tagId")
			return
		}
		row, err := svc.DeactivateTag(r.Context(), orgID, tid)
		if err != nil {
			writeAdminCatalogError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, mapAdminTag(row))
	}
}
