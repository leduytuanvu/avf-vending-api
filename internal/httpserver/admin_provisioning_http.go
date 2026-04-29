package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/avf/avf-vending-api/internal/app/api"
	approvisioning "github.com/avf/avf-vending-api/internal/app/provisioning"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func mountAdminProvisioningRoutes(r chi.Router, app *api.HTTPApplication, writeRL func(http.Handler) http.Handler) {
	if app == nil || app.Provisioning == nil {
		return
	}
	if writeRL == nil {
		writeRL = func(h http.Handler) http.Handler { return h }
	}

	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAnyPermission(auth.PermFleetRead))
		r.Get("/provisioning/batches/{batchId}", serveAdminProvisioningBatchGet(app))
	})

	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAnyPermission(auth.PermFleetWrite))
		r.With(writeRL).Post("/provisioning/machines/bulk", serveAdminProvisioningBulkCreate(app))
	})
}

type v1AdminProvisioningBulkCreateRequest struct {
	SiteID            uuid.UUID  `json:"siteId"`
	HardwareProfileID *uuid.UUID `json:"hardwareProfileId,omitempty"`
	CabinetType       string     `json:"cabinetType"`
	Machines          []struct {
		SerialNumber string `json:"serialNumber"`
		Name         string `json:"name,omitempty"`
		Model        string `json:"model,omitempty"`
	} `json:"machines"`
	GenerateActivationCodes bool  `json:"generateActivationCodes"`
	ExpiresInMinutes        int32 `json:"expiresInMinutes,omitempty"`
	MaxUses                 int32 `json:"maxUses,omitempty"`
}

func serveAdminProvisioningBulkCreate(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, err := parseAdminFleetOrganizationScope(r)
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		var body v1AdminProvisioningBulkCreateRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_json", "request body must be JSON")
			return
		}
		n := len(body.Machines)
		if n == 0 || body.SiteID == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_argument", "siteId and non-empty machines are required")
			return
		}
		rows := make([]approvisioning.BulkMachineRow, 0, n)
		for _, m := range body.Machines {
			rows = append(rows, approvisioning.BulkMachineRow{
				SerialNumber: m.SerialNumber,
				Name:         m.Name,
				Model:        m.Model,
			})
		}
		var cb pgtype.UUID
		if u, ok := parseInteractiveActorUUID(r); ok {
			cb = pgtype.UUID{Bytes: *u, Valid: true}
		}
		res, err := app.Provisioning.BulkCreateMachines(r.Context(), orgID, approvisioning.BulkCreateInput{
			SiteID:                  body.SiteID,
			HardwareProfileID:       body.HardwareProfileID,
			CabinetType:             body.CabinetType,
			Machines:                rows,
			GenerateActivationCodes: body.GenerateActivationCodes,
			ExpiresInMinutes:        body.ExpiresInMinutes,
			MaxUses:                 body.MaxUses,
			CreatedBy:               cb,
		})
		if err != nil {
			writeProvisioningError(w, r.Context(), err)
			return
		}
		writeJSON(w, http.StatusCreated, res)
	}
}

func serveAdminProvisioningBatchGet(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, err := parseAdminFleetOrganizationScope(r)
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		batchID, ok := parseChiUUID(w, r, "batchId")
		if !ok {
			return
		}
		batch, items, err := app.Provisioning.GetBatchDetail(r.Context(), orgID, batchID)
		if err != nil {
			writeProvisioningError(w, r.Context(), err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"batch":    encodeProvisioningBatch(batch),
			"machines": encodeProvisioningBatchMachines(items),
		})
	}
}

func encodeProvisioningBatch(b db.MachineProvisioningBatch) map[string]any {
	out := map[string]any{
		"id":                b.ID.String(),
		"organizationId":    b.OrganizationID.String(),
		"siteId":            b.SiteID.String(),
		"cabinetType":       b.CabinetType,
		"status":            b.Status,
		"machineCount":      b.MachineCount,
		"createdAt":         formatAPITimeRFC3339Nano(b.CreatedAt),
		"updatedAt":         formatAPITimeRFC3339Nano(b.UpdatedAt),
		"hardwareProfileId": nil,
	}
	if b.HardwareProfileID.Valid {
		s := uuid.UUID(b.HardwareProfileID.Bytes).String()
		out["hardwareProfileId"] = s
	}
	if b.CreatedBy.Valid {
		s := uuid.UUID(b.CreatedBy.Bytes).String()
		out["createdBy"] = s
	}
	if len(b.Metadata) > 0 {
		out["metadata"] = json.RawMessage(b.Metadata)
	} else {
		out["metadata"] = json.RawMessage([]byte("{}"))
	}
	return out
}

func encodeProvisioningBatchMachines(rows []db.ListProvisioningBatchMachinesRow) []map[string]any {
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		item := map[string]any{
			"id":           row.ID.String(),
			"batchId":      row.BatchID.String(),
			"machineId":    row.MachineID.String(),
			"serialNumber": row.SerialNumber,
			"rowNo":        row.RowNo,
			"createdAt":    formatAPITimeRFC3339Nano(row.CreatedAt),
		}
		if row.ActivationCodeID.Valid {
			item["activationCodeId"] = uuid.UUID(row.ActivationCodeID.Bytes).String()
		}
		if row.ActivationCodeStatus.Valid {
			item["activationStatus"] = row.ActivationCodeStatus.String
		}
		if row.ActivationExpiresAt.Valid {
			item["activationExpiresAt"] = formatAPITimeRFC3339Nano(row.ActivationExpiresAt.Time)
		}
		if row.ActivationUses.Valid {
			item["activationUses"] = row.ActivationUses.Int32
		}
		if row.ActivationMaxUses.Valid {
			item["activationMaxUses"] = row.ActivationMaxUses.Int32
		}
		out = append(out, item)
	}
	return out
}

func writeProvisioningError(w http.ResponseWriter, ctx context.Context, err error) {
	switch {
	case errors.Is(err, approvisioning.ErrNotFound):
		writeAPIError(w, ctx, http.StatusNotFound, "not_found", err.Error())
	case errors.Is(err, approvisioning.ErrInvalidArgument):
		writeAPIError(w, ctx, http.StatusBadRequest, "invalid_argument", err.Error())
	default:
		writeAPIError(w, ctx, http.StatusInternalServerError, "internal", err.Error())
	}
}
