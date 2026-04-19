package httpserver

import (
	"errors"
	"net/http"
	"strings"

	"github.com/avf/avf-vending-api/internal/app/api"
	appinventoryadmin "github.com/avf/avf-vending-api/internal/app/inventoryadmin"
	"github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func mountAdminInventoryRoutes(r chi.Router, app *api.HTTPApplication) {
	if app == nil || app.InventoryAdmin == nil {
		return
	}
	svc := app.InventoryAdmin
	r.Get("/machines/{machineId}/slots", listAdminMachineSlots(svc))
	r.Get("/machines/{machineId}/inventory", getAdminMachineInventory(svc))
}

func listAdminMachineSlots(svc *appinventoryadmin.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		mid, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "machineId")))
		if err != nil || mid == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_machine_id", "invalid machineId")
			return
		}
		head, err := resolveInventoryMachine(r, svc, mid)
		if err != nil {
			writeInventoryAccessOrResolveError(w, r, err)
			return
		}
		rows, err := svc.ListSlots(r.Context(), mid)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		items := make([]V1AdminMachineSlot, 0, len(rows))
		for _, row := range rows {
			items = append(items, V1AdminMachineSlot{
				MachineID:                row.MachineID.String(),
				MachineName:              head.Name,
				MachineStatus:            head.Status,
				PlanogramID:              row.PlanogramID.String(),
				PlanogramName:            row.PlanogramName,
				SlotIndex:                row.SlotIndex,
				CurrentQuantity:          row.CurrentQuantity,
				MaxQuantity:              row.MaxQuantity,
				PriceMinor:               row.PriceMinor,
				PlanogramRevisionApplied: row.PlanogramRevisionApplied,
				UpdatedAt:                row.UpdatedAt.UTC().Format(timeRFC3339Nano),
				ProductID:                uuidPtrFromPgUUID(row.ProductID),
				ProductSku:               textFromPgText(row.ProductSku),
				ProductName:              textFromPgText(row.ProductName),
				IsEmpty:                  row.IsEmpty,
				LowStock:                 boolFromPgBool(row.LowStock),
			})
		}
		writeJSON(w, http.StatusOK, V1AdminMachineSlotListEnvelope{Items: items})
	}
}

func getAdminMachineInventory(svc *appinventoryadmin.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		mid, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "machineId")))
		if err != nil || mid == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_machine_id", "invalid machineId")
			return
		}
		head, err := resolveInventoryMachine(r, svc, mid)
		if err != nil {
			writeInventoryAccessOrResolveError(w, r, err)
			return
		}
		rows, err := svc.AggregateInventory(r.Context(), mid)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		items := make([]V1AdminMachineInventoryLine, 0, len(rows))
		for _, row := range rows {
			items = append(items, V1AdminMachineInventoryLine{
				MachineID:          head.ID.String(),
				MachineName:        head.Name,
				MachineStatus:      head.Status,
				ProductID:          row.ProductID.String(),
				ProductName:        row.ProductName,
				ProductSku:         row.ProductSku,
				TotalQuantity:      row.TotalQuantity,
				SlotCount:          row.SlotCount,
				MaxCapacityAnySlot: row.MaxCapacityAnySlot,
				LowStock:           row.LowStock,
			})
		}
		writeJSON(w, http.StatusOK, V1AdminMachineInventoryEnvelope{Items: items})
	}
}

func resolveInventoryMachine(r *http.Request, svc *appinventoryadmin.Service, machineID uuid.UUID) (appinventoryadmin.MachineHead, error) {
	p, ok := auth.PrincipalFromContext(r.Context())
	if !ok {
		return appinventoryadmin.MachineHead{}, errInventoryUnauthenticated
	}
	head, err := svc.ResolveMachine(r.Context(), machineID)
	if err != nil {
		return appinventoryadmin.MachineHead{}, err
	}
	if p.HasRole(auth.RolePlatformAdmin) {
		return head, nil
	}
	if p.HasRole(auth.RoleOrgAdmin) && p.HasOrganization() && head.OrganizationID == p.OrganizationID {
		return head, nil
	}
	return appinventoryadmin.MachineHead{}, appinventoryadmin.ErrForbidden
}

func writeInventoryAccessOrResolveError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, errInventoryUnauthenticated):
		writeAPIError(w, r.Context(), http.StatusUnauthorized, "unauthenticated", auth.ErrUnauthenticated.Error())
	case errors.Is(err, appinventoryadmin.ErrForbidden):
		writeAPIError(w, r.Context(), http.StatusForbidden, "forbidden", "forbidden")
	case errors.Is(err, appinventoryadmin.ErrMachineNotFound):
		writeAPIError(w, r.Context(), http.StatusNotFound, "machine_not_found", "machine not found")
	default:
		writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
	}
}

var errInventoryUnauthenticated = errors.New("unauthenticated")

func boolFromPgBool(b pgtype.Bool) bool {
	return b.Valid && b.Bool
}
