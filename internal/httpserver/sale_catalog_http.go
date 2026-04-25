package httpserver

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/avf/avf-vending-api/internal/app/api"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/avf/avf-vending-api/internal/modules/postgres"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func mountSaleCatalogRoute(r chi.Router, app *api.HTTPApplication) {
	if app == nil || app.TelemetryStore == nil {
		return
	}
	r.With(RequireMachineTenantAccess(app, "machineId")).Get("/machines/{machineId}/sale-catalog", getSaleCatalog(app))
}

func getSaleCatalog(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		machineID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "machineId")))
		if err != nil || machineID == uuid.Nil {
			writeAPIError(w, ctx, http.StatusBadRequest, "invalid_machine_id", "invalid machineId")
			return
		}
		q := r.URL.Query()
		includeUnavailable := strings.EqualFold(q.Get("include_unavailable"), "true")
		includeImages := true
		if v := strings.TrimSpace(q.Get("include_images")); v == "false" || v == "0" {
			includeImages = false
		}
		var ifNone *int64
		if raw := strings.TrimSpace(q.Get("if_none_match_config_version")); raw != "" {
			n, perr := strconv.ParseInt(raw, 10, 64)
			if perr == nil {
				ifNone = &n
			}
		}

		pool := app.TelemetryStore.Pool()
		queries := db.New(pool)
		repo := postgres.NewSetupRepository(pool)
		bootstrap, err := repo.GetMachineBootstrap(ctx, machineID)
		if err != nil {
			writeAPIError(w, ctx, http.StatusNotFound, "machine_not_found", "machine not found")
			return
		}
		slotView, err := repo.GetMachineSlotView(ctx, machineID)
		if err != nil {
			writeAPIError(w, ctx, http.StatusInternalServerError, "internal", err.Error())
			return
		}
		legacyByIndex := make(map[int32]struct {
			qty       int32
			maxQty    int32
			price     int64
			planogram string
		})
		for _, l := range slotView.LegacySlots {
			legacyByIndex[l.SlotIndex] = struct {
				qty       int32
				maxQty    int32
				price     int64
				planogram string
			}{l.CurrentQuantity, l.MaxQuantity, l.PriceMinor, l.PlanogramName}
		}

		sortByProduct := make(map[uuid.UUID]int32)
		for _, ap := range bootstrap.AssortmentProducts {
			sortByProduct[ap.ProductID] = ap.SortOrder
		}

		var cfgVersion int64
		ver, err := queries.GetMachineShadowVersion(ctx, machineID)
		if err != nil {
			if err != pgx.ErrNoRows {
				writeAPIError(w, ctx, http.StatusInternalServerError, "internal", err.Error())
				return
			}
			cfgVersion = 0
		} else {
			cfgVersion = ver
		}
		if ifNone != nil && *ifNone == cfgVersion {
			w.WriteHeader(http.StatusNotModified)
			return
		}

		cur, err := queries.InventoryAdminGetOrgDefaultCurrency(ctx, bootstrap.Machine.OrganizationID)
		if err != nil {
			writeAPIError(w, ctx, http.StatusInternalServerError, "internal", err.Error())
			return
		}

		productIDs := make([]uuid.UUID, 0)
		for _, sl := range bootstrap.CurrentCabinetSlots {
			if !sl.IsCurrent || sl.ProductID == nil {
				continue
			}
			productIDs = append(productIDs, *sl.ProductID)
		}
		prodByID := make(map[uuid.UUID]db.RuntimeGetProductsByIDsRow)
		if len(productIDs) > 0 {
			prodRows, err := queries.RuntimeGetProductsByIDs(ctx, db.RuntimeGetProductsByIDsParams{
				OrganizationID: bootstrap.Machine.OrganizationID,
				Column2:        productIDs,
			})
			if err != nil {
				writeAPIError(w, ctx, http.StatusInternalServerError, "internal", err.Error())
				return
			}
			for _, p := range prodRows {
				prodByID[p.ID] = p
			}
		}

		imgByProduct := make(map[uuid.UUID][]db.RuntimeListProductImagesForProductsRow)
		if includeImages && len(productIDs) > 0 {
			imgs, ierr := queries.RuntimeListProductImagesForProducts(ctx, productIDs)
			if ierr != nil {
				writeAPIError(w, ctx, http.StatusInternalServerError, "internal", ierr.Error())
				return
			}
			for _, im := range imgs {
				imgByProduct[im.ProductID] = append(imgByProduct[im.ProductID], im)
			}
		}

		items := make([]map[string]any, 0)
		for _, sl := range bootstrap.CurrentCabinetSlots {
			if !sl.IsCurrent || sl.ProductID == nil {
				continue
			}
			pid := *sl.ProductID
			pmeta, ok := prodByID[pid]
			if !ok {
				continue
			}
			var slotIdx int32 = -1
			if sl.SlotIndex != nil {
				slotIdx = *sl.SlotIndex
			}
			leg, hasLeg := legacyByIndex[slotIdx]
			qty := int32(0)
			maxQ := sl.MaxQuantity
			price := sl.PriceMinor
			if hasLeg {
				qty = leg.qty
				if maxQ <= 0 {
					maxQ = leg.maxQty
				}
				if price == 0 {
					price = leg.price
				}
			}
			priceOK := price > 0
			stockOK := qty > 0
			activeOK := pmeta.Active
			reasons := make([]string, 0)
			if !activeOK {
				reasons = append(reasons, "product_inactive")
			}
			if !priceOK {
				reasons = append(reasons, "no_price")
			}
			if !stockOK {
				reasons = append(reasons, "out_of_stock")
			}
			available := activeOK && priceOK && stockOK
			if !available && !includeUnavailable {
				continue
			}
			var unavail *string
			if !available {
				s := strings.Join(reasons, ",")
				unavail = &s
			}
			shortName := shortNameFromAttrs(pmeta.Name, pmeta.Attrs)
			entry := map[string]any{
				"slotIndex":         slotIdx,
				"slotCode":          sl.SlotCode,
				"cabinetCode":       sl.CabinetCode,
				"productId":         pid.String(),
				"sku":               pmeta.Sku,
				"name":              pmeta.Name,
				"shortName":         shortName,
				"priceMinor":        price,
				"availableQuantity": qty,
				"maxQuantity":       maxQ,
				"isAvailable":       available,
				"unavailableReason": nil,
				"sortOrder":         sortByProduct[pid],
			}
			if unavail != nil {
				entry["unavailableReason"] = *unavail
			}
			if includeImages {
				if imgs := imgByProduct[pid]; len(imgs) > 0 {
					im := pickDisplayImage(imgs)
					thumb := productImageThumbURL(im)
					disp := productImageDisplayURL(im)
					if thumb == "" {
						thumb = "https://cdn.example.invalid/missing-thumb.webp"
					}
					if disp == "" {
						disp = thumb
					}
					entry["image"] = map[string]any{
						"thumbUrl":    thumb,
						"displayUrl":  disp,
						"contentHash": productImageContentHash(im),
						"updatedAt":   im.CreatedAt.UTC().Format(time.RFC3339),
					}
				} else {
					entry["image"] = nil
				}
			}
			items = append(items, entry)
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"machineId":      machineID.String(),
			"organizationId": bootstrap.Machine.OrganizationID.String(),
			"siteId":         bootstrap.Machine.SiteID.String(),
			"configVersion":  cfgVersion,
			"currency":       strings.ToUpper(strings.TrimSpace(cur)),
			"generatedAt":    time.Now().UTC().Format(time.RFC3339),
			"items":          items,
		})
	}
}

func shortNameFromAttrs(full string, attrs []byte) string {
	var m map[string]any
	if len(attrs) > 0 {
		_ = json.Unmarshal(attrs, &m)
	}
	if m != nil {
		if s, ok := m["short_name"].(string); ok && strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
		if s, ok := m["shortName"].(string); ok && strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
	}
	if len(full) > 24 {
		return full[:24]
	}
	return full
}

func productImageDisplayURL(r db.RuntimeListProductImagesForProductsRow) string {
	if r.CdnUrl.Valid {
		return strings.TrimSpace(r.CdnUrl.String)
	}
	return ""
}

func productImageThumbURL(r db.RuntimeListProductImagesForProductsRow) string {
	if r.ThumbCdnUrl.Valid {
		s := strings.TrimSpace(r.ThumbCdnUrl.String)
		if s != "" {
			return s
		}
	}
	return productImageDisplayURL(r)
}

func productImageContentHash(r db.RuntimeListProductImagesForProductsRow) string {
	if r.ContentHash.Valid {
		s := strings.TrimSpace(r.ContentHash.String)
		if s != "" {
			return s
		}
	}
	return "sha256:" + sha256Hex(r.StorageKey)
}

func pickDisplayImage(rows []db.RuntimeListProductImagesForProductsRow) db.RuntimeListProductImagesForProductsRow {
	for _, r := range rows {
		if r.IsPrimary {
			return r
		}
	}
	return rows[0]
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}
