package httpserver

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/avf/avf-vending-api/internal/app/api"
	"github.com/avf/avf-vending-api/internal/app/productmastercatalog"
	"github.com/avf/avf-vending-api/internal/app/salecatalog"
	"github.com/avf/avf-vending-api/internal/app/setupapp"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/avf/avf-vending-api/internal/modules/postgres"
	"github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/avf/avf-vending-api/internal/platform/pgxutil"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// mountMachineMerchCatalogRoutes registers split catalog HTTP used by deployed vending APKs.
// These routes stay available when legacy machine REST is disabled (same policy as merge-pairs).
func mountMachineMerchCatalogRoutes(r chi.Router, app *api.HTTPApplication) {
	if app == nil {
		return
	}
	r.With(
		RequireMachineCompanyAccess(app, "machineId"),
		auth.RequireInteractivePermissionOrMachinePrincipal(auth.PermCatalogRead),
	).Get("/machines/{machineId}/catalog/bundle-meta", getMachineCatalogBundleMeta(app))
	r.With(
		RequireMachineCompanyAccess(app, "machineId"),
		auth.RequireInteractivePermissionOrMachinePrincipal(auth.PermCatalogRead),
	).Get("/machines/{machineId}/catalog/planogram", getMachineCatalogPlanogram(app))
	r.With(
		RequireMachineCompanyAccess(app, "machineId"),
		auth.RequireInteractivePermissionOrMachinePrincipal(auth.PermCatalogRead),
	).Get("/machines/{machineId}/catalog/merch/products", getMachineCatalogMerchProducts(app))
	r.With(
		RequireMachineCompanyAccess(app, "machineId"),
		auth.RequireInteractivePermissionOrMachinePrincipal(auth.PermCatalogRead),
	).Get("/machines/{machineId}/product-lists", getMachineProductLists(app))
	r.With(
		RequireMachineCompanyAccess(app, "machineId"),
		auth.RequireInteractivePermissionOrMachinePrincipal(auth.PermCatalogRead),
	).Get("/machines/{machineId}/product-lists/{listId}/snapshot", getMachineProductListSnapshot(app))
}

func getMachineCatalogBundleMeta(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		machineID, ok := parseMachineIDParam(w, r)
		if !ok {
			return
		}
		meta, err := loadMachineCatalogBundleMeta(ctx, app, machineID)
		if err != nil {
			writeMachineCatalogError(w, ctx, err)
			return
		}
		writeJSON(w, http.StatusOK, meta)
	}
}

func getMachineCatalogPlanogram(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		machineID, ok := parseMachineIDParam(w, r)
		if !ok {
			return
		}
		rows, err := loadMachineCatalogPlanogram(ctx, app, machineID)
		if err != nil {
			writeMachineCatalogError(w, ctx, err)
			return
		}
		writeJSON(w, http.StatusOK, rows)
	}
}

func getMachineCatalogMerchProducts(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		machineID, ok := parseMachineIDParam(w, r)
		if !ok {
			return
		}
		includeImages := true
		if v := strings.TrimSpace(r.URL.Query().Get("include_images")); v == "false" || v == "0" {
			includeImages = false
		}
		products, err := loadMachineCatalogMerchProducts(ctx, app, machineID, includeImages)
		if err != nil {
			writeMachineCatalogError(w, ctx, err)
			return
		}
		writeJSON(w, http.StatusOK, products)
	}
}

func machineCatalogPool(app *api.HTTPApplication) (*pgxpool.Pool, error) {
	if app == nil || app.TelemetryStore == nil || app.TelemetryStore.Pool() == nil {
		return nil, fmt.Errorf("telemetry store not configured")
	}
	return app.TelemetryStore.Pool(), nil
}

func loadMachineCatalogBundleMeta(ctx context.Context, app *api.HTTPApplication, machineID uuid.UUID) (map[string]any, error) {
	pool, err := machineCatalogPool(app)
	if err != nil {
		return nil, err
	}
	repo := postgres.NewSetupRepository(pool)
	bootstrap, err := repo.GetMachineBootstrap(ctx, machineID)
	if err != nil {
		return nil, err
	}
	q := pgxutil.NewQueries(pool)
	invSlots, _ := q.InventoryAdminListMachineSlots(ctx, machineID)
	snap, err := buildMachineSaleCatalogSnapshot(ctx, app, machineID)
	if err != nil {
		return nil, err
	}
	catalogRev, planogramRev := resolveMachineCatalogRevisions(bootstrap, snap, invSlots)
	return map[string]any{
		"machineId":            machineID.String(),
		"catalogMerchRevision": catalogRev,
		"planogramRevision":    planogramRev,
	}, nil
}

func loadMachineCatalogPlanogram(ctx context.Context, app *api.HTTPApplication, machineID uuid.UUID) ([]map[string]any, error) {
	pool, err := machineCatalogPool(app)
	if err != nil {
		return nil, err
	}
	q := pgxutil.NewQueries(pool)
	configs, err := q.InventoryAdminListCurrentMachineSlotConfigsByMachine(ctx, machineID)
	if err != nil {
		return nil, err
	}
	invByIndex := map[int32]db.InventoryAdminListMachineSlotsRow{}
	invSlots, err := q.InventoryAdminListMachineSlots(ctx, machineID)
	if err != nil {
		return nil, err
	}
	for _, row := range invSlots {
		invByIndex[row.SlotIndex] = row
	}
	if len(configs) == 0 {
		return mapInventorySlotsToPlanogram(invSlots), nil
	}
	return mapSlotConfigsToPlanogram(configs, invByIndex), nil
}

func loadMachineCatalogMerchProducts(ctx context.Context, app *api.HTTPApplication, machineID uuid.UUID, includeImages bool) ([]map[string]any, error) {
	snap, err := buildMachineSaleCatalogSnapshot(ctx, app, machineID, includeImages)
	if err != nil {
		return nil, err
	}
	return mapSaleCatalogItemsToMerchProducts(snap, includeImages), nil
}

func buildMachineSaleCatalogSnapshot(ctx context.Context, app *api.HTTPApplication, machineID uuid.UUID, includeImages ...bool) (salecatalog.Snapshot, error) {
	withImages := true
	if len(includeImages) > 0 {
		withImages = includeImages[0]
	}
	svc := app.SaleCatalog
	pool, err := machineCatalogPool(app)
	if err != nil {
		return salecatalog.Snapshot{}, err
	}
	if svc == nil {
		svc = salecatalog.NewService(pool)
	}
	return svc.BuildSnapshot(ctx, machineID, salecatalog.Options{
		IncludeUnavailable: true,
		IncludeImages:      withImages,
	})
}

func writeMachineCatalogError(w http.ResponseWriter, ctx context.Context, err error) {
	if err == nil {
		return
	}
	switch {
	case errors.Is(err, setupapp.ErrNotFound):
		writeAPIError(w, ctx, http.StatusNotFound, "machine_not_found", "machine not found")
	case errors.Is(err, setupapp.ErrMachineNotEligibleForBootstrap):
		writeAPIError(w, ctx, http.StatusForbidden, "machine_not_eligible", "machine not eligible")
	default:
		if strings.Contains(err.Error(), "not configured") {
			writeAPIError(w, ctx, http.StatusServiceUnavailable, "unavailable", err.Error())
			return
		}
		writeAPIError(w, ctx, http.StatusInternalServerError, "internal", err.Error())
	}
}

// resolveMachineCatalogRevisions mirrors bootstrap gRPC published planogram version for APK revision gates.
func resolveMachineCatalogRevisions(
	bootstrap setupapp.MachineBootstrap,
	snap salecatalog.Snapshot,
	invSlots []db.InventoryAdminListMachineSlotsRow,
) (catalogMerchRevision int32, planogramRevision int32) {
	planogramRevision = bootstrap.PublishedPlanogramVersionNo
	if planogramRevision <= 0 {
		for _, row := range invSlots {
			if row.PlanogramRevisionApplied > planogramRevision {
				planogramRevision = row.PlanogramRevisionApplied
			}
		}
	}
	if planogramRevision <= 0 && snap.ConfigVersion > 0 {
		planogramRevision = int32(snap.ConfigVersion)
	}
	catalogMerchRevision = planogramRevision
	if catalogMerchRevision <= 0 && snap.ConfigVersion > 0 {
		catalogMerchRevision = int32(snap.ConfigVersion)
	}
	return catalogMerchRevision, planogramRevision
}

func mapSlotConfigsToPlanogram(
	configs []db.InventoryAdminListCurrentMachineSlotConfigsByMachineRow,
	invByIndex map[int32]db.InventoryAdminListMachineSlotsRow,
) []map[string]any {
	out := make([]map[string]any, 0, len(configs))
	for _, cfg := range configs {
		slotIndex := int32(0)
		if cfg.SlotIndex.Valid {
			slotIndex = cfg.SlotIndex.Int32
		}
		inv := invByIndex[slotIndex]
		stock := inv.CurrentQuantity
		lane := strings.TrimSpace(cfg.SlotCode)
		if lane == "" {
			lane = fmt.Sprintf("S%d", slotIndex)
		}
		slotID := cfg.ID.String()
		if slotID == "" || slotID == uuid.Nil.String() {
			slotID = lane
		}
		entry := map[string]any{
			"slotId":      slotID,
			"laneCode":    lane,
			"cabinetNo":   int(cfg.CabinetIndex) + 1,
			"slotNo":      slotIndex,
			"isEnabled":   true,
			"isLocked":    false,
			"stockOnHand": stock,
		}
		if cfg.MaxQuantity > 0 {
			entry["capacity"] = cfg.MaxQuantity
			entry["parLevel"] = cfg.MaxQuantity
		}
		if cfg.ProductID.Valid {
			pid := uuid.UUID(cfg.ProductID.Bytes).String()
			assign := map[string]any{
				"productId": pid,
			}
			if cfg.PriceMinor > 0 {
				assign["priceMinor"] = strconv.FormatInt(cfg.PriceMinor, 10)
			}
			if cfg.ProductSku.Valid && cfg.ProductSku.String != "" {
				assign["sku"] = cfg.ProductSku.String
			}
			if cfg.ProductName.Valid && cfg.ProductName.String != "" {
				assign["displayNameOverride"] = cfg.ProductName.String
			}
			entry["assignment"] = assign
		}
		if inv.IsEmpty && stock <= 0 {
			entry["stockStatus"] = "empty"
		}
		out = append(out, entry)
	}
	return out
}

func mapInventorySlotsToPlanogram(invSlots []db.InventoryAdminListMachineSlotsRow) []map[string]any {
	out := make([]map[string]any, 0, len(invSlots))
	for _, row := range invSlots {
		lane := fmt.Sprintf("%s%d", strings.TrimSpace(row.CabinetCode), row.SlotIndex+1)
		if strings.TrimSpace(row.CabinetCode) == "" {
			lane = fmt.Sprintf("S%d", row.SlotIndex)
		}
		entry := map[string]any{
			"slotId":      row.PlanogramID.String() + ":" + strconv.Itoa(int(row.SlotIndex)),
			"laneCode":    lane,
			"cabinetNo":   int(row.CabinetIndex) + 1,
			"slotNo":      row.SlotIndex,
			"isEnabled":   true,
			"isLocked":    false,
			"stockOnHand": row.CurrentQuantity,
		}
		if row.MaxQuantity > 0 {
			entry["capacity"] = row.MaxQuantity
		}
		if row.ProductID.Valid {
			pid := uuid.UUID(row.ProductID.Bytes).String()
			assign := map[string]any{"productId": pid}
			if row.PriceMinor > 0 {
				assign["priceMinor"] = strconv.FormatInt(row.PriceMinor, 10)
			}
			if row.ProductSku.Valid {
				assign["sku"] = row.ProductSku.String
			}
			if row.ProductName.Valid {
				assign["displayNameOverride"] = row.ProductName.String
			}
			entry["assignment"] = assign
		}
		out = append(out, entry)
	}
	return out
}

func getMachineProductLists(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		machineID, ok := parseMachineIDParam(w, r)
		if !ok {
			return
		}
		pool, err := machineCatalogPool(app)
		if err != nil {
			writeMachineCatalogError(w, ctx, err)
			return
		}
		lists, err := productmastercatalog.NewService(pool).ListVisibleLists(ctx, machineID)
		if err != nil {
			writeMachineCatalogError(w, ctx, err)
			return
		}
		items := make([]map[string]any, 0, len(lists))
		for _, row := range lists {
			items = append(items, map[string]any{
				"listId":    row.ListID.String(),
				"name":      row.Name,
				"kind":      row.Kind,
				"status":    row.Status,
				"isPrimary": row.IsPrimary,
				"assigned":  row.Assigned,
				"updatedAt": row.UpdatedAt.UTC().Format("2006-01-02T15:04:05.000000000Z07:00"),
			})
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items})
	}
}

func getMachineProductListSnapshot(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		machineID, ok := parseMachineIDParam(w, r)
		if !ok {
			return
		}
		listID, ok := parseChiUUID(w, r, "listId")
		if !ok {
			return
		}
		var filterIDs []uuid.UUID
		for _, raw := range r.URL.Query()["product_id"] {
			id, err := uuid.Parse(strings.TrimSpace(raw))
			if err != nil || id == uuid.Nil {
				writeAPIError(w, ctx, http.StatusBadRequest, "invalid_product_id", "invalid product_id")
				return
			}
			filterIDs = append(filterIDs, id)
		}
		pool, err := machineCatalogPool(app)
		if err != nil {
			writeMachineCatalogError(w, ctx, err)
			return
		}
		snap, err := productmastercatalog.NewService(pool).BuildListSnapshot(ctx, machineID, listID, filterIDs)
		if err != nil {
			if strings.Contains(err.Error(), "not visible") {
				writeAPIError(w, ctx, http.StatusNotFound, "list_not_found", err.Error())
				return
			}
			writeMachineCatalogError(w, ctx, err)
			return
		}
		products := make([]map[string]any, 0, len(snap.Snapshot.Products))
		for _, p := range snap.Snapshot.Products {
			entry := map[string]any{
				"productId":      p.ProductID.String(),
				"sku":            p.SKU,
				"barcode":        p.Barcode,
				"name":           p.Name,
				"description":    p.Description,
				"basePriceMinor": p.BasePriceMinor,
				"currency":       p.Currency,
				"active":         p.Active,
			}
			if p.CategoryID != nil {
				entry["categoryId"] = p.CategoryID.String()
			}
			if p.BrandID != nil {
				entry["brandId"] = p.BrandID.String()
			}
			if p.Image != nil {
				entry["thumbUrl"] = p.Image.ThumbURL
				entry["displayUrl"] = p.Image.DisplayURL
				entry["imageHash"] = p.Image.ContentHash
				entry["imageKey"] = p.Image.CacheKey
			}
			products = append(products, entry)
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"listId":       snap.ListID.String(),
			"listVersion":  snap.ListVersion,
			"generatedAt":  snap.Snapshot.GeneratedAt.UTC().Format("2006-01-02T15:04:05.000000000Z07:00"),
			"productCount": snap.Snapshot.TotalActiveCount,
			"products":     products,
		})
	}
}

func mapSaleCatalogItemsToMerchProducts(snap salecatalog.Snapshot, includeImages bool) []map[string]any {
	seen := make(map[uuid.UUID]struct{})
	out := make([]map[string]any, 0)
	for _, it := range snap.Items {
		if it.ProductID == uuid.Nil {
			continue
		}
		if _, ok := seen[it.ProductID]; ok {
			continue
		}
		seen[it.ProductID] = struct{}{}
		entry := map[string]any{
			"id":       it.ProductID.String(),
			"sku":      it.SKU,
			"name":     it.Name,
			"isActive": it.IsAvailable || it.AvailableQuantity > 0 || it.MaxQuantity > 0,
		}
		if it.ShortName != "" {
			entry["shortName"] = it.ShortName
		}
		if it.BasePriceMinor > 0 {
			entry["basePriceMinor"] = it.BasePriceMinor
		} else if it.PriceMinor > 0 {
			entry["basePriceMinor"] = it.PriceMinor
		}
		if snap.Currency != "" {
			entry["currency"] = snap.Currency
		}
		if includeImages && it.Image != nil && !it.Image.Deleted {
			if it.Image.ThumbURL != "" {
				entry["thumbUrl"] = it.Image.ThumbURL
				entry["imageUrl"] = it.Image.ThumbURL
			}
			if it.Image.DisplayURL != "" {
				entry["displayUrl"] = it.Image.DisplayURL
			}
			if it.Image.ContentHash != "" {
				entry["imageHash"] = it.Image.ContentHash
			}
			if it.Image.CacheKey != "" {
				entry["imageKey"] = it.Image.CacheKey
			}
		}
		out = append(out, entry)
	}
	return out
}
