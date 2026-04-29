package httpserver

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/avf/avf-vending-api/internal/app/adminops"
	"github.com/avf/avf-vending-api/internal/app/api"
	appdevice "github.com/avf/avf-vending-api/internal/app/device"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func mountAdminOperationsRoutes(r chi.Router, app *api.HTTPApplication, writeRL func(http.Handler) http.Handler) {
	if app == nil || app.AdminOps == nil {
		return
	}
	if writeRL == nil {
		writeRL = func(h http.Handler) http.Handler { return h }
	}

	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAnyPermission(auth.PermFleetRead, auth.PermTelemetryRead))
		r.Get("/operations/machines/health", serveAdminOperationsMachineHealthList(app))
		r.Get("/machines/{machineId}/health", serveAdminOperationsMachineHealthGet(app))
		r.Get("/machines/{machineId}/timeline", serveAdminOperationsMachineTimeline(app))
	})

	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAnyPermission(auth.PermFleetRead))
		r.Get("/commands", serveAdminOperationsCommandsList(app))
		r.Get("/commands/{commandId}", serveAdminOperationsCommandGet(app))
	})

	r.Group(func(r chi.Router) {
		r.Use(auth.RequirePermission(auth.PermDeviceCommandsWrite))
		r.With(writeRL).Post("/commands/{commandId}/retry", serveAdminOperationsCommandRetry(app))
		r.With(writeRL).Post("/commands/{commandId}/cancel", serveAdminOperationsCommandCancel(app))
		r.With(writeRL).Post("/machines/{machineId}/commands", serveAdminOperationsMachineCommandDispatch(app))
	})

	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAnyPermission(auth.PermInventoryRead, auth.PermFleetRead))
		r.Get("/inventory/anomalies", serveAdminOperationsInventoryAnomaliesList(app))
		r.Get("/machines/{machineId}/inventory/anomalies", serveAdminOperationsInventoryAnomaliesForMachine(app))
	})

	r.Group(func(r chi.Router) {
		r.Use(auth.RequirePermission(auth.PermInventoryAdjust))
		r.With(writeRL).Post("/inventory/anomalies/{anomalyId}/resolve", serveAdminOperationsInventoryAnomalyResolve(app))
		r.With(writeRL).Post("/machines/{machineId}/inventory/reconcile", serveAdminOperationsInventoryReconcile(app))
	})
}

func serveAdminOperationsMachineHealthList(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, err := parseAdminFleetOrganizationScope(r)
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		limit, offset, err := parseAdminLimitOffset(r)
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		rows, err := app.AdminOps.ListMachineHealth(r.Context(), orgID, int32(limit), int32(offset))
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		items := make([]map[string]any, 0, len(rows))
		for _, row := range rows {
			items = append(items, machineHealthJSON(row))
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items})
	}
}

func serveAdminOperationsMachineHealthGet(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, err := parseAdminFleetOrganizationScope(r)
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		machineID, ok := parseChiUUID(w, r, "machineId")
		if !ok {
			return
		}
		row, err := app.AdminOps.GetMachineHealth(r.Context(), orgID, machineID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				writeAPIError(w, r.Context(), http.StatusNotFound, "not_found", "machine health not found")
				return
			}
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, machineHealthJSON(row))
	}
}

func serveAdminOperationsMachineTimeline(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, err := parseAdminFleetOrganizationScope(r)
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		machineID, ok := parseChiUUID(w, r, "machineId")
		if !ok {
			return
		}
		limit, offset, err := parseAdminLimitOffset(r)
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		events, err := app.AdminOps.MachineTimeline(r.Context(), orgID, machineID, int32(limit), int32(offset))
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		items := make([]map[string]any, 0, len(events))
		for _, ev := range events {
			items = append(items, map[string]any{
				"occurredAt": ev.OccurredAt.UTC().Format(time.RFC3339Nano),
				"eventKind":  ev.EventKind,
				"title":      ev.Title,
				"payload":    json.RawMessage(ev.Payload),
				"refId":      ev.RefID,
			})
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items})
	}
}

func serveAdminOperationsCommandsList(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		scope, err := parseAdminFleetListScope(r)
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		out, err := app.AdminCommands.ListCommands(r.Context(), scope)
		writeV1Collection(w, r.Context(), out, err)
	}
}

func serveAdminOperationsCommandGet(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, err := parseAdminFleetOrganizationScope(r)
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		commandID, ok := parseChiUUID(w, r, "commandId")
		if !ok {
			return
		}
		detail, err := app.AdminOps.GetCommandDetail(r.Context(), orgID, commandID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				writeAPIError(w, r.Context(), http.StatusNotFound, "not_found", "command not found")
				return
			}
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, commandDetailJSON(detail))
	}
}

func serveAdminOperationsCommandRetry(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, err := parseAdminFleetOrganizationScope(r)
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		commandID, ok := parseChiUUID(w, r, "commandId")
		if !ok {
			return
		}
		res, err := app.AdminOps.RetryCommand(r.Context(), orgID, commandID)
		switch {
		case err == nil:
			fleetAudit(r.Context(), app, orgID, "admin.command.retry", "command_ledger", stringPtrOrNil(commandID.String()), map[string]any{
				"dispatchState": res.DispatchState,
				"replay":        res.Replay,
			})
			writeJSON(w, http.StatusOK, map[string]any{
				"commandId":        res.CommandID.String(),
				"sequence":         res.Sequence,
				"attemptId":        res.AttemptID.String(),
				"dispatchState":    res.DispatchState,
				"replay":           res.Replay,
				"skippedRepublish": res.SkippedRepublish,
			})
			return
		case errors.Is(err, appdevice.ErrCommandNotRetryable):
			writeAPIError(w, r.Context(), http.StatusConflict, "command_not_retryable", err.Error())
			return
		case errors.Is(err, appdevice.ErrCommandRetryRequiresIdempotency):
			writeAPIError(w, r.Context(), http.StatusConflict, "command_retry_requires_idempotency", err.Error())
			return
		case errors.Is(err, pgx.ErrNoRows), errors.Is(err, appdevice.ErrNotFound):
			writeAPIError(w, r.Context(), http.StatusNotFound, "not_found", "command not found")
			return
		case errors.Is(err, appdevice.ErrMQTTCommandPublisherMissing):
			writeCapabilityNotConfigured(w, r.Context(), "mqtt_dispatch", "MQTT command publisher is not configured")
			return
		default:
			writeAPIError(w, r.Context(), http.StatusBadGateway, "command_retry_failed", err.Error())
			return
		}
	}
}

func serveAdminOperationsCommandCancel(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, err := parseAdminFleetOrganizationScope(r)
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		commandID, ok := parseChiUUID(w, r, "commandId")
		if !ok {
			return
		}
		n, err := app.AdminOps.CancelCommand(r.Context(), orgID, commandID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				writeAPIError(w, r.Context(), http.StatusNotFound, "not_found", "command not found")
				return
			}
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		if n == 0 {
			writeAPIError(w, r.Context(), http.StatusConflict, "nothing_to_cancel", "no pending or sent attempts to cancel")
			return
		}
		fleetAudit(r.Context(), app, orgID, "admin.command.cancel", "command_ledger", stringPtrOrNil(commandID.String()), map[string]any{"attemptsCancelled": n})
		writeJSON(w, http.StatusOK, map[string]any{"attemptsCancelled": n})
	}
}

type adminMachineCommandBody struct {
	CommandType string          `json:"commandType"`
	Payload     json.RawMessage `json:"payload"`
}

func serveAdminOperationsMachineCommandDispatch(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, err := parseAdminFleetOrganizationScope(r)
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		machineID, ok := parseChiUUID(w, r, "machineId")
		if !ok {
			return
		}
		idem, err := requireWriteIdempotencyKey(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "missing_idempotency_key", err.Error())
			return
		}
		var body adminMachineCommandBody
		if r.Body != nil {
			_ = json.NewDecoder(r.Body).Decode(&body)
		}
		res, err := app.AdminOps.DispatchMachineCommand(r.Context(), orgID, machineID, strings.TrimSpace(body.CommandType), body.Payload, idem)
		switch {
		case err == nil:
			fleetAudit(r.Context(), app, orgID, "admin.command.dispatch", "command_ledger", stringPtrOrNil(res.CommandID.String()), map[string]any{
				"machineId":     machineID.String(),
				"commandType":   body.CommandType,
				"dispatchState": res.DispatchState,
			})
			writeJSON(w, http.StatusAccepted, map[string]any{
				"commandId":     res.CommandID.String(),
				"sequence":      res.Sequence,
				"attemptId":     res.AttemptID.String(),
				"dispatchState": res.DispatchState,
				"replay":        res.Replay,
			})
			return
		case errors.Is(err, appdevice.ErrMQTTCommandPublisherMissing):
			writeCapabilityNotConfigured(w, r.Context(), "mqtt_dispatch", "MQTT command publisher is not configured")
			return
		default:
			writeAPIError(w, r.Context(), http.StatusBadGateway, "command_dispatch_failed", err.Error())
			return
		}
	}
}

func serveAdminOperationsInventoryAnomaliesList(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, err := parseAdminFleetOrganizationScope(r)
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		limit, offset, err := parseAdminLimitOffset(r)
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		refresh := strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("refresh")), "true")
		rows, err := app.AdminOps.ListInventoryAnomalies(r.Context(), orgID, nil, int32(limit), int32(offset), refresh)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		items := make([]map[string]any, 0, len(rows))
		for _, row := range rows {
			items = append(items, inventoryAnomalyJSON(row))
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items})
	}
}

func serveAdminOperationsInventoryAnomaliesForMachine(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, err := parseAdminFleetOrganizationScope(r)
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		machineID, ok := parseChiUUID(w, r, "machineId")
		if !ok {
			return
		}
		limit, offset, err := parseAdminLimitOffset(r)
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		refresh := strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("refresh")), "true")
		rows, err := app.AdminOps.ListInventoryAnomalies(r.Context(), orgID, &machineID, int32(limit), int32(offset), refresh)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		items := make([]map[string]any, 0, len(rows))
		for _, row := range rows {
			items = append(items, inventoryAnomalyJSON(row))
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items})
	}
}

type resolveAnomalyBody struct {
	Note string `json:"note"`
}

func serveAdminOperationsInventoryAnomalyResolve(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, err := parseAdminFleetOrganizationScope(r)
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		anomalyID, ok := parseChiUUID(w, r, "anomalyId")
		if !ok {
			return
		}
		var body resolveAnomalyBody
		if r.Body != nil {
			_ = json.NewDecoder(r.Body).Decode(&body)
		}
		actor := uuid.Nil
		if p, ok := auth.PrincipalFromContext(r.Context()); ok {
			if aid, err := uuid.Parse(strings.TrimSpace(p.Subject)); err == nil {
				actor = aid
			}
		}
		err = app.AdminOps.ResolveInventoryAnomaly(r.Context(), orgID, anomalyID, actor, body.Note)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				writeAPIError(w, r.Context(), http.StatusNotFound, "not_found", "anomaly not found or already resolved")
				return
			}
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		fleetAudit(r.Context(), app, orgID, "admin.inventory.anomaly.resolve", "inventory_anomalies", stringPtrOrNil(anomalyID.String()), map[string]any{"note": body.Note})
		writeJSON(w, http.StatusOK, map[string]any{"anomalyId": anomalyID.String(), "status": "resolved"})
	}
}

type inventoryReconcileBody struct {
	Reason string `json:"reason"`
}

func serveAdminOperationsInventoryReconcile(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, err := parseAdminFleetOrganizationScope(r)
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		machineID, ok := parseChiUUID(w, r, "machineId")
		if !ok {
			return
		}
		if _, err := app.AdminOps.GetMachineHealth(r.Context(), orgID, machineID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				writeAPIError(w, r.Context(), http.StatusNotFound, "machine_not_found", "machine not found")
				return
			}
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		var body inventoryReconcileBody
		if r.Body != nil {
			_ = json.NewDecoder(r.Body).Decode(&body)
		}
		meta, _ := json.Marshal(map[string]any{
			"reason": strings.TrimSpace(body.Reason),
			"source": "admin_api",
		})
		id, err := app.AdminOps.InsertInventoryReconcileMarker(r.Context(), orgID, machineID, strings.TrimSpace(body.Reason), meta)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		fleetAudit(r.Context(), app, orgID, "admin.inventory.reconcile_marker", "inventory_events", nil, map[string]any{
			"machineId":           machineID.String(),
			"inventoryEventSeqId": strconv.FormatInt(id, 10),
		})
		writeJSON(w, http.StatusAccepted, map[string]any{"inventoryEventId": id})
	}
}

func parseChiUUID(w http.ResponseWriter, r *http.Request, key string) (uuid.UUID, bool) {
	raw := strings.TrimSpace(chi.URLParam(r, key))
	id, err := uuid.Parse(raw)
	if err != nil || id == uuid.Nil {
		writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_uuid", key+" must be a UUID")
		return uuid.Nil, false
	}
	return id, true
}

func machineHealthJSON(h adminops.MachineHealth) map[string]any {
	out := map[string]any{
		"machineId":             h.MachineID.String(),
		"status":                h.Status,
		"pendingCommandCount":   h.PendingCommandCount,
		"failedCommandCount":    h.FailedCommandCount,
		"inventoryAnomalyCount": h.InventoryAnomalyCount,
	}
	if h.LastSeenAt != nil {
		out["lastSeenAt"] = h.LastSeenAt.UTC().Format(time.RFC3339Nano)
	}
	if h.LastCheckInAt != nil {
		out["lastCheckInAt"] = h.LastCheckInAt.UTC().Format(time.RFC3339Nano)
	}
	if strings.TrimSpace(h.AppVersion) != "" {
		out["appVersion"] = h.AppVersion
	}
	if strings.TrimSpace(h.ConfigVersion) != "" {
		out["configVersion"] = h.ConfigVersion
	}
	if strings.TrimSpace(h.CatalogVersion) != "" {
		out["catalogVersion"] = h.CatalogVersion
	}
	if strings.TrimSpace(h.MediaVersion) != "" {
		out["mediaVersion"] = h.MediaVersion
	}
	if h.MqttConnected != nil {
		out["mqttConnected"] = *h.MqttConnected
	}
	if strings.TrimSpace(h.LastErrorCode) != "" {
		out["lastErrorCode"] = h.LastErrorCode
	}
	if h.TelemetryFreshnessSeconds != nil {
		out["telemetryFreshnessSeconds"] = *h.TelemetryFreshnessSeconds
	}
	return out
}

func commandDetailJSON(d adminops.CommandDetail) map[string]any {
	ld := d.Ledger
	out := map[string]any{
		"commandId":      ld.ID.String(),
		"machineId":      ld.MachineID.String(),
		"organizationId": ld.OrganizationID.String(),
		"sequence":       ld.Sequence,
		"commandType":    ld.CommandType,
		"payload":        json.RawMessage(ld.Payload),
		"createdAt":      ld.CreatedAt.UTC().Format(time.RFC3339Nano),
	}
	if ld.CorrelationID.Valid {
		x := uuid.UUID(ld.CorrelationID.Bytes)
		out["correlationId"] = x.String()
	}
	if ld.IdempotencyKey.Valid {
		out["idempotencyKey"] = ld.IdempotencyKey.String
	}
	atts := make([]map[string]any, 0, len(d.Attempts))
	for _, a := range d.Attempts {
		row := map[string]any{
			"id":            a.ID.String(),
			"attemptNo":     a.AttemptNo,
			"status":        a.Status,
			"sentAt":        a.SentAt.UTC().Format(time.RFC3339Nano),
			"dispatchState": appdevice.MapAttemptTransportState(a.Status),
		}
		if a.AckDeadlineAt.Valid {
			row["ackDeadlineAt"] = a.AckDeadlineAt.Time.UTC().Format(time.RFC3339Nano)
		}
		if a.ResultReceivedAt.Valid {
			row["resultReceivedAt"] = a.ResultReceivedAt.Time.UTC().Format(time.RFC3339Nano)
		}
		if a.TimeoutReason.Valid {
			row["timeoutReason"] = a.TimeoutReason.String
		}
		atts = append(atts, row)
	}
	out["attempts"] = atts
	return out
}

func inventoryAnomalyJSON(row db.AdminOpsListInventoryAnomaliesByOrgRow) map[string]any {
	out := map[string]any{
		"id":                  row.ID.String(),
		"organizationId":      row.OrganizationID.String(),
		"machineId":           row.MachineID.String(),
		"machineName":         row.MachineName,
		"machineSerialNumber": row.MachineSerialNumber,
		"anomalyType":         row.AnomalyType,
		"status":              row.Status,
		"fingerprint":         row.Fingerprint,
		"detectedAt":          row.DetectedAt.UTC().Format(time.RFC3339Nano),
		"createdAt":           row.CreatedAt.UTC().Format(time.RFC3339Nano),
		"updatedAt":           row.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
	if row.SlotCode.Valid {
		out["slotCode"] = row.SlotCode.String
	}
	if row.ProductID.Valid {
		pid := uuid.UUID(row.ProductID.Bytes)
		out["productId"] = pid.String()
	}
	if len(row.Payload) > 0 {
		out["payload"] = json.RawMessage(row.Payload)
	}
	if row.ResolvedAt.Valid {
		out["resolvedAt"] = row.ResolvedAt.Time.UTC().Format(time.RFC3339Nano)
	}
	if row.ResolvedBy.Valid {
		out["resolvedBy"] = uuid.UUID(row.ResolvedBy.Bytes).String()
	}
	if row.ResolutionNote.Valid {
		out["resolutionNote"] = row.ResolutionNote.String
	}
	return out
}
