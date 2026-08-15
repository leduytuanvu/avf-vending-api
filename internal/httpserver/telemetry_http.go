package httpserver

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/avf/avf-vending-api/internal/app/alerts"
	"github.com/avf/avf-vending-api/internal/app/api"
	"github.com/avf/avf-vending-api/internal/modules/postgres"
	"github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func mountMachineTelemetryRoutes(r chi.Router, app *api.HTTPApplication, abuse *AbuseProtection) {
	// Legacy HTTP telemetry reads for machines. Prefer gRPC MachineTelemetryService for production kiosks.
	if app == nil || app.TelemetryStore == nil {
		return
	}
	if abuse == nil {
		abuse = &AbuseProtection{}
	}
	st := app.TelemetryStore
	r.With(abuse.MachineScoped(), RequireMachineCompanyAccess(app, "machineId"), auth.RequireInteractivePermissionOrMachinePrincipal(auth.PermTelemetryRead)).Get("/machines/{machineId}/telemetry/snapshot", telemetrySnapshotHandler(st))
	r.With(abuse.MachineScoped(), RequireMachineCompanyAccess(app, "machineId"), auth.RequireInteractivePermissionOrMachinePrincipal(auth.PermTelemetryRead)).Get("/machines/{machineId}/telemetry/incidents", telemetryIncidentsHandler(st))
	r.With(abuse.MachineScoped(), RequireMachineCompanyAccess(app, "machineId"), auth.RequireInteractivePermissionOrMachinePrincipal(auth.PermTelemetryRead)).Get("/machines/{machineId}/telemetry/rollups", telemetryRollupsHandler(st))
	r.With(abuse.MachineScoped(), RequireMachineCompanyAccess(app, "machineId")).Post("/machines/{machineId}/incidents", postMachineIncidentHandler(st, app.IncidentAlertPolicy))
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

// postMachineIncidentHandler accepts direct machine incident reports and is idempotent by dedupe_key/fingerprint.
func postMachineIncidentHandler(st *postgres.Store, policy alerts.Policy) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		machineID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "machineId")))
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_machine_id", "invalid machineId")
			return
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil || !json.Valid(body) {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
			return
		}
		severity, code, title, dedupe, err := postgres.ParseIncidentPayload(body)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_incident", err.Error())
			return
		}
		occurrenceID := alerts.ExtractOccurrenceIDFromDetail(body)
		_, err = st.ProjectMachineIncident(r.Context(), alerts.ProjectInput{
			MachineID:    machineID.String(),
			OccurrenceID: occurrenceID,
			Fingerprint:  dedupe,
			Severity:     severity,
			Code:         code,
			Title:        title,
			EventType:    code,
			Transport:    "http",
			Detail:       body,
			OccurredAt:   time.Now().UTC(),
		}, policy)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		writeJSON(w, http.StatusAccepted, map[string]any{
			"machineId":    machineID.String(),
			"occurrenceId": occurrenceID,
			"dedupeKey":    dedupe,
			"status":       "accepted",
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
