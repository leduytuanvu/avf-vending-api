package httpserver

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/avf/avf-vending-api/internal/app/api"
	"github.com/avf/avf-vending-api/internal/modules/postgres"
	"github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func mountMachineTelemetryRoutes(r chi.Router, app *api.HTTPApplication) {
	if app == nil || app.TelemetryStore == nil {
		return
	}
	st := app.TelemetryStore
	r.With(auth.RequireMachineURLAccess("machineId")).Get("/machines/{machineId}/telemetry/snapshot", telemetrySnapshotHandler(st))
	r.With(auth.RequireMachineURLAccess("machineId")).Get("/machines/{machineId}/telemetry/incidents", telemetryIncidentsHandler(st))
	r.With(auth.RequireMachineURLAccess("machineId")).Get("/machines/{machineId}/telemetry/rollups", telemetryRollupsHandler(st))
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
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"machine_id":        row.MachineID,
			"organization_id":   row.OrganizationID,
			"site_id":           row.SiteID,
			"reported_state":    json.RawMessage(row.ReportedState),
			"metrics_state":     json.RawMessage(row.MetricsState),
			"last_heartbeat_at": row.LastHeartbeatAt,
			"app_version":       row.AppVersion,
			"firmware_version":  row.FirmwareVersion,
			"updated_at":        row.UpdatedAt,
		})
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
		items := make([]map[string]any, 0, len(rows))
		for _, x := range rows {
			items = append(items, map[string]any{
				"id":         x.ID,
				"severity":   x.Severity,
				"code":       x.Code,
				"title":      x.Title,
				"detail":     json.RawMessage(x.Detail),
				"dedupe_key": x.DedupeKey,
				"opened_at":  x.OpenedAt,
				"updated_at": x.UpdatedAt,
			})
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"items": items, "meta": map[string]any{"limit": limit, "returned": len(items)}})
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
			if t, err := time.Parse(time.RFC3339, v); err == nil {
				from = t.UTC()
			}
		}
		if v := r.URL.Query().Get("to"); v != "" {
			if t, err := time.Parse(time.RFC3339, v); err == nil {
				to = t.UTC()
			}
		}
		limit := int32(500)
		rows, err := st.ListTelemetryRollupsInRange(r.Context(), id, from, to, granularity, limit)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		items := make([]map[string]any, 0, len(rows))
		for _, x := range rows {
			items = append(items, map[string]any{
				"bucket_start": x.BucketStart,
				"granularity":  x.Granularity,
				"metric_key":   x.MetricKey,
				"sample_count": x.SampleCount,
				"sum":          x.SumVal,
				"min":          x.MinVal,
				"max":          x.MaxVal,
				"last":         x.LastVal,
				"extra":        json.RawMessage(x.Extra),
			})
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items": items,
			"meta": map[string]any{
				"granularity": granularity,
				"from":        from,
				"to":          to,
				"returned":    len(items),
				"note":        "Rollup buckets only — not raw MQTT telemetry history.",
			},
		})
	}
}
