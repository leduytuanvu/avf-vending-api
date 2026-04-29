package httpserver

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/avf/avf-vending-api/internal/app/api"
	"github.com/avf/avf-vending-api/internal/app/salecatalog"
	"github.com/avf/avf-vending-api/internal/app/setupapp"
	"github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func mountSaleCatalogRoute(r chi.Router, app *api.HTTPApplication) {
	// Legacy HTTP sale-catalog snapshot. Prefer MachineCatalogService gRPC (GetSaleCatalog / SyncSaleCatalog).
	if app == nil || app.TelemetryStore == nil {
		return
	}
	r.With(RequireMachineTenantAccess(app, "machineId"), auth.RequireInteractivePermissionOrMachinePrincipal(auth.PermCatalogRead)).Get("/machines/{machineId}/sale-catalog", getSaleCatalog(app))
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

		svc := app.SaleCatalog
		if svc == nil {
			svc = salecatalog.NewService(app.TelemetryStore.Pool())
		}
		snap, err := svc.BuildSnapshot(ctx, machineID, salecatalog.Options{
			IncludeUnavailable:       includeUnavailable,
			IncludeImages:            includeImages,
			IfNoneMatchConfigVersion: ifNone,
		})
		if err != nil {
			switch {
			case errors.Is(err, setupapp.ErrNotFound):
				writeAPIError(w, ctx, http.StatusNotFound, "machine_not_found", "machine not found")
			case errors.Is(err, setupapp.ErrMachineNotEligibleForBootstrap):
				writeAPIError(w, ctx, http.StatusForbidden, "machine_not_eligible", "machine not eligible")
			default:
				writeAPIError(w, ctx, http.StatusInternalServerError, "internal", err.Error())
			}
			return
		}
		if snap.NotModified {
			w.WriteHeader(http.StatusNotModified)
			return
		}

		items := make([]map[string]any, 0, len(snap.Items))
		for _, it := range snap.Items {
			entry := map[string]any{
				"slotIndex":         it.SlotIndex,
				"slotCode":          it.SlotCode,
				"cabinetCode":       it.CabinetCode,
				"productId":         it.ProductID.String(),
				"sku":               it.SKU,
				"name":              it.Name,
				"shortName":         it.ShortName,
				"priceMinor":        it.PriceMinor,
				"availableQuantity": it.AvailableQuantity,
				"maxQuantity":       it.MaxQuantity,
				"isAvailable":       it.IsAvailable,
				"unavailableReason": nil,
				"sortOrder":         it.SortOrder,
			}
			if it.UnavailableReason != "" {
				entry["unavailableReason"] = it.UnavailableReason
			}
			if includeImages {
				if it.Image != nil {
					if it.Image.Deleted {
						entry["image"] = map[string]any{"deleted": true}
					} else {
						img := map[string]any{
							"thumbUrl":    it.Image.ThumbURL,
							"displayUrl":  it.Image.DisplayURL,
							"contentHash": it.Image.ContentHash,
							"etag":        it.Image.Etag,
							"updatedAt":   it.Image.UpdatedAt.UTC().Format(time.RFC3339),
						}
						if it.Image.MediaID != uuid.Nil {
							img["mediaId"] = it.Image.MediaID.String()
						}
						if it.Image.SizeBytes > 0 {
							img["sizeBytes"] = it.Image.SizeBytes
						}
						if it.Image.ObjectVersion != 0 {
							img["objectVersion"] = it.Image.ObjectVersion
						}
						if it.Image.MediaVersion != 0 {
							img["mediaVersion"] = it.Image.MediaVersion
						}
						if it.Image.Width > 0 {
							img["width"] = it.Image.Width
						}
						if it.Image.Height > 0 {
							img["height"] = it.Image.Height
						}
						entry["image"] = img
					}
				} else {
					entry["image"] = nil
				}
			}
			items = append(items, entry)
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"machineId":      snap.MachineID.String(),
			"organizationId": snap.OrganizationID.String(),
			"siteId":         snap.SiteID.String(),
			"configVersion":  snap.ConfigVersion,
			"catalogVersion": snap.CatalogVersion,
			"currency":       snap.Currency,
			"generatedAt":    snap.GeneratedAt.UTC().Format(time.RFC3339),
			"items":          items,
		})
	}
}
