package httpserver

// rbac:handlers-only: product-list routes are registered from mountAdminCatalogRoutes (catalog read/write permission layers).

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	appcatalogadmin "github.com/avf/avf-vending-api/internal/app/catalogadmin"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/google/uuid"
)

func listAdminProductLists(svc *appcatalogadmin.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := requireCatalogPrincipalUUID(r, nil); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_scope", err.Error())
			return
		}
		limit, offset, err := parseAdminLimitOffset(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_pagination", err.Error())
			return
		}
		res, err := svc.ListProductLists(r.Context(), appcatalogadmin.ListProductListsParams{
			Limit:  limit,
			Offset: offset,
			Search: strings.TrimSpace(r.URL.Query().Get("q")),
			Status: strings.TrimSpace(r.URL.Query().Get("status")),
			Kind:   strings.TrimSpace(r.URL.Query().Get("kind")),
		})
		if err != nil {
			writeAdminCatalogError(w, r, err)
			return
		}
		items := make([]V1AdminProductList, 0, len(res.Items))
		for _, row := range res.Items {
			items = append(items, mapAdminProductList(row.List, row.ItemCount, nil))
		}
		writeJSON(w, http.StatusOK, V1AdminNamedProductListEnvelope{
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

func getAdminProductList(svc *appcatalogadmin.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := requireCatalogPrincipalUUID(r, nil); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_scope", err.Error())
			return
		}
		listID, ok := parseChiUUID(w, r, "listId")
		if !ok {
			return
		}
		detail, err := svc.GetProductList(r.Context(), listID)
		if err != nil {
			writeAdminCatalogError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, mapAdminProductListDetail(*detail))
	}
}

func postAdminProductListCreate(svc *appcatalogadmin.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := requireCatalogPrincipalUUID(r, nil); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_scope", err.Error())
			return
		}
		var body V1AdminProductListCreateRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_json", "invalid json")
			return
		}
		row, err := svc.CreateProductList(r.Context(), appcatalogadmin.CreateProductListInput{
			Name:        body.Name,
			Status:      body.Status,
			Kind:        body.Kind,
			Description: body.Description,
		})
		if err != nil {
			writeAdminCatalogError(w, r, err)
			return
		}
		writeJSON(w, http.StatusCreated, mapAdminProductList(row, 0, nil))
	}
}

func patchAdminProductList(svc *appcatalogadmin.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := requireCatalogPrincipalUUID(r, nil); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_scope", err.Error())
			return
		}
		listID, ok := parseChiUUID(w, r, "listId")
		if !ok {
			return
		}
		var body V1AdminProductListPatchRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_json", "invalid json")
			return
		}
		row, err := svc.UpdateProductList(r.Context(), appcatalogadmin.UpdateProductListInput{
			ListID:      listID,
			Name:        body.Name,
			Status:      body.Status,
			Description: body.Description,
		})
		if err != nil {
			writeAdminCatalogError(w, r, err)
			return
		}
		detail, err := svc.GetProductList(r.Context(), row.ID)
		if err != nil {
			writeAdminCatalogError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, mapAdminProductListDetail(*detail))
	}
}

func putAdminProductListItems(svc *appcatalogadmin.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := requireCatalogPrincipalUUID(r, nil); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_scope", err.Error())
			return
		}
		listID, ok := parseChiUUID(w, r, "listId")
		if !ok {
			return
		}
		var body V1AdminProductListItemsReplaceRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_json", "invalid json")
			return
		}
		ids, err := parseUUIDStrings(body.ProductIDs)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_product_id", err.Error())
			return
		}
		detail, err := svc.ReplaceProductListItems(r.Context(), listID, ids)
		if err != nil {
			writeAdminCatalogError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, mapAdminProductListDetail(*detail))
	}
}

func postAdminProductListImport(svc *appcatalogadmin.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		scopeID, err := requireCatalogPrincipalUUID(r, nil)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_scope", err.Error())
			return
		}
		listID, ok := parseChiUUID(w, r, "listId")
		if !ok {
			return
		}
		ct := strings.ToLower(strings.TrimSpace(r.Header.Get("Content-Type")))
		var result *appcatalogadmin.ImportProductListResult
		switch {
		case strings.Contains(ct, "text/csv") || strings.Contains(ct, "application/csv"):
			result, err = svc.ImportProductListCSV(r.Context(), scopeID, listID, r.Body)
		default:
			raw, rerr := io.ReadAll(io.LimitReader(r.Body, 8<<20))
			if rerr != nil {
				writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_body", "unable to read body")
				return
			}
			result, err = svc.ImportProductListJSON(r.Context(), scopeID, listID, raw)
		}
		if err != nil {
			writeAdminCatalogError(w, r, err)
			return
		}
		out := mapAdminProductListDetail(result.Detail)
		out.CreatedCount = result.CreatedCount
		out.AddedCount = result.AddedCount
		out.ExistingCount = result.ExistingCount
		out.SkippedCount = result.SkippedCount
		writeJSON(w, http.StatusOK, out)
	}
}

func getAdminMachineProductLists(svc *appcatalogadmin.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := requireCatalogPrincipalUUID(r, nil); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_scope", err.Error())
			return
		}
		machineID, ok := parseChiUUID(w, r, "machineId")
		if !ok {
			return
		}
		asg, err := svc.GetMachineProductLists(r.Context(), machineID)
		if err != nil {
			writeAdminCatalogError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, mapAdminMachineProductLists(*asg))
	}
}

func putAdminMachineProductLists(svc *appcatalogadmin.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := requireCatalogPrincipalUUID(r, nil); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_scope", err.Error())
			return
		}
		machineID, ok := parseChiUUID(w, r, "machineId")
		if !ok {
			return
		}
		var body V1AdminMachineProductListsPutRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_json", "invalid json")
			return
		}
		primary, err := uuid.Parse(strings.TrimSpace(body.PrimaryListID))
		if err != nil || primary == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_primary_list_id", "invalid primaryListId")
			return
		}
		supp, err := parseUUIDStrings(body.SupplementalListIDs)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_list_id", err.Error())
			return
		}
		asg, err := svc.PutMachineProductLists(r.Context(), appcatalogadmin.PutMachineProductListsInput{
			MachineID:           machineID,
			PrimaryListID:       primary,
			SupplementalListIDs: supp,
		})
		if err != nil {
			writeAdminCatalogError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, mapAdminMachineProductLists(*asg))
	}
}

func mapAdminProductList(list db.Assortment, itemCount int64, items []V1AdminProductListItemMembership) V1AdminProductList {
	out := V1AdminProductList{
		ID:          list.ID.String(),
		Name:        list.Name,
		Status:      list.Status,
		Kind:        list.Kind,
		Description: list.Description,
		ItemCount:   itemCount,
		Items:       items,
		CreatedAt:   formatAPITimeRFC3339Nano(list.CreatedAt),
		UpdatedAt:   formatAPITimeRFC3339Nano(list.UpdatedAt),
	}
	return out
}

func mapAdminProductListDetail(detail appcatalogadmin.ProductListDetail) V1AdminProductList {
	items := make([]V1AdminProductListItemMembership, 0, len(detail.Items))
	for _, it := range detail.Items {
		items = append(items, V1AdminProductListItemMembership{
			ProductID:   it.ProductID.String(),
			Sku:         it.Sku,
			Name:        it.Name,
			Active:      it.Active,
			SortOrder:   it.SortOrder,
			Barcode:     textFromPgText(it.Barcode),
			Description: it.Description,
			CategoryID:  uuidPtrFromPgUUID(it.CategoryID),
			BrandID:     uuidPtrFromPgUUID(it.BrandID),
		})
	}
	return mapAdminProductList(detail.List, int64(len(items)), items)
}

func mapAdminMachineProductLists(asg appcatalogadmin.MachineProductListsAssignment) V1AdminMachineProductLists {
	lists := make([]V1AdminMachineProductListBinding, 0, len(asg.Bindings))
	for _, b := range asg.Bindings {
		lists = append(lists, V1AdminMachineProductListBinding{
			ListID:    b.AssortmentID.String(),
			Name:      b.AssortmentName,
			Kind:      b.AssortmentKind,
			Status:    b.AssortmentStatus,
			IsPrimary: b.IsPrimary,
			ValidFrom: formatAPITimeRFC3339Nano(b.ValidFrom),
		})
	}
	out := V1AdminMachineProductLists{
		MachineID:           asg.MachineID.String(),
		SupplementalListIDs: make([]string, 0, len(asg.SupplementalListIDs)),
		Lists:               lists,
	}
	if asg.PrimaryListID != nil {
		s := asg.PrimaryListID.String()
		out.PrimaryListID = &s
	}
	for _, id := range asg.SupplementalListIDs {
		out.SupplementalListIDs = append(out.SupplementalListIDs, id.String())
	}
	return out
}

func parseUUIDStrings(raw []string) ([]uuid.UUID, error) {
	out := make([]uuid.UUID, 0, len(raw))
	for _, s := range raw {
		id, err := uuid.Parse(strings.TrimSpace(s))
		if err != nil || id == uuid.Nil {
			return nil, fmt.Errorf("invalid uuid")
		}
		out = append(out, id)
	}
	return out, nil
}
