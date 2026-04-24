package httpserver

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/avf/avf-vending-api/internal/app/api"
	"github.com/avf/avf-vending-api/internal/modules/postgres"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func mountMachineTelemetryRoutes(r chi.Router, app *api.HTTPApplication) {
	if app == nil || app.TelemetryStore == nil {
		return
	}
	st := app.TelemetryStore
	r.With(RequireMachineTenantAccess(app, "machineId")).Get("/machines/{machineId}/telemetry/snapshot", telemetrySnapshotHandler(st))
	r.With(RequireMachineTenantAccess(app, "machineId")).Get("/machines/{machineId}/telemetry/incidents", telemetryIncidentsHandler(st))
	r.With(RequireMachineTenantAccess(app, "machineId")).Get("/machines/{machineId}/telemetry/rollups", telemetryRollupsHandler(st))
}

func telemetrySnapshotHandler(st *postgres.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(chi.URLParam(r, "machineId"))
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_machine_id", "invalid machineId")
			return
		}
		row, err := st.GetTelemetrySnapshot(r.Context(), id)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				writeAPIError(w, r.Context(), http.StatusNotFound, "telemetry_snapshot_not_found", "no telemetry snapshot yet for this machine")
				return
			}
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		resp := V1MachineTelemetrySnapshotResponse{
			MachineID:         row.MachineID.String(),
			OrganizationID:    row.OrganizationID.String(),
			SiteID:            row.SiteID.String(),
			ReportedState:     json.RawMessage(row.ReportedState),
			MetricsState:      json.RawMessage(row.MetricsState),
			LastHeartbeatAt:   formatAPITimeRFC3339NanoPtr(row.LastHeartbeatAt),
			AppVersion:        row.AppVersion,
			FirmwareVersion:   row.FirmwareVersion,
			UpdatedAt:         formatAPITimeRFC3339Nano(row.UpdatedAt),
			AndroidID:         row.AndroidID,
			SimSerial:         row.SimSerial,
			SimIccid:          row.SimIccid,
			DeviceModel:       row.DeviceModel,
			OSVersion:         row.OSVersion,
			LastIdentityAt:    formatAPITimeRFC3339NanoPtr(row.LastIdentityAt),
			EffectiveTimezone: row.EffectiveTimezone,
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

func telemetryIncidentsHandler(st *postgres.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(chi.URLParam(r, "machineId"))
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_machine_id", "invalid machineId")
			return
		}
		limit := int32(50)
		if v := r.URL.Query().Get("limit"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 500 {
				limit = int32(n)
			}
		}
		rows, err := st.ListMachineIncidentsRecent(r.Context(), id, limit)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		items := make([]V1MachineTelemetryIncidentItem, 0, len(rows))
		for _, x := range rows {
			items = append(items, V1MachineTelemetryIncidentItem{
				ID:        x.ID.String(),
				Severity:  x.Severity,
				Code:      x.Code,
				Title:     x.Title,
				Detail:    json.RawMessage(x.Detail),
				DedupeKey: x.DedupeKey,
				OpenedAt:  formatAPITimeRFC3339Nano(x.OpenedAt),
				UpdatedAt: formatAPITimeRFC3339Nano(x.UpdatedAt),
			})
		}
		writeJSON(w, http.StatusOK, V1MachineTelemetryIncidentsResponse{
			Items: items,
			Meta: V1MachineTelemetryIncidentsMeta{
				Limit:    limit,
				Returned: len(items),
			},
		})
	}
}

func telemetryRollupsHandler(st *postgres.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(chi.URLParam(r, "machineId"))
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_machine_id", "invalid machineId")
			return
		}
		granularity := r.URL.Query().Get("granularity")
		if granularity == "" {
			granularity = "1m"
		}
		now := time.Now().UTC()
		from := now.Add(-24 * time.Hour)
		to := now
		if v := r.URL.Query().Get("from"); v != "" {
			if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
				from = t.UTC()
			} else if t, err := time.Parse(time.RFC3339, v); err == nil {
				from = t.UTC()
			}
		}
		if v := r.URL.Query().Get("to"); v != "" {
			if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
				to = t.UTC()
			} else if t, err := time.Parse(time.RFC3339, v); err == nil {
				to = t.UTC()
			}
		}
		limit := int32(500)
		rows, err := st.ListTelemetryRollupsInRange(r.Context(), id, from, to, granularity, limit)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		items := make([]V1MachineTelemetryRollupItem, 0, len(rows))
		for _, x := range rows {
			items = append(items, V1MachineTelemetryRollupItem{
				BucketStart: formatAPITimeRFC3339Nano(x.BucketStart),
				Granularity: x.Granularity,
				MetricKey:   x.MetricKey,
				SampleCount: x.SampleCount,
				Sum:         x.SumVal,
				Min:         x.MinVal,
				Max:         x.MaxVal,
				Last:        x.LastVal,
				Extra:       json.RawMessage(x.Extra),
			})
		}
		writeJSON(w, http.StatusOK, V1MachineTelemetryRollupsResponse{
			Items: items,
			Meta: V1MachineTelemetryRollupsMeta{
				Granularity: granularity,
				From:        formatAPITimeRFC3339Nano(from),
				To:          formatAPITimeRFC3339Nano(to),
				Returned:    len(items),
				Note:        "Rollup buckets only — not raw MQTT telemetry history.",
			},
		})
	}
}
