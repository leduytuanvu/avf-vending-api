package httpserver

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/avf/avf-vending-api/internal/app/api"
	appdevice "github.com/avf/avf-vending-api/internal/app/device"
	"github.com/avf/avf-vending-api/internal/domain/compliance"
	domaindevice "github.com/avf/avf-vending-api/internal/domain/device"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

const diagnosticBundleCommandType = "DIAGNOSTIC_BUNDLE_REQUEST"

type adminDiagnosticRequestBody struct {
	Reason string `json:"reason"`
}

func mountAdminMachineDiagnosticsRoutes(r chi.Router, app *api.HTTPApplication, writeRL func(http.Handler) http.Handler) {
	if app == nil {
		return
	}
	if writeRL == nil {
		writeRL = func(h http.Handler) http.Handler { return h }
	}
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAnyPermission(auth.PermFleetRead, auth.PermTelemetryRead))
		r.Get("/machines/{machineId}/diagnostics/bundles", serveAdminMachineDiagnosticBundlesList(app))
	})
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAnyPermission(auth.PermDeviceCommandsWrite))
		r.With(writeRL).Post("/machines/{machineId}/diagnostics/requests", serveAdminMachineDiagnosticRequest(app))
	})
}

func serveAdminMachineDiagnosticRequest(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idem, err := requireWriteIdempotencyKey(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "missing_idempotency_key", err.Error())
			return
		}
		machineID, ok := parseMachineIDParam(w, r)
		if !ok {
			return
		}
		orgID, err := parseAdminFleetOrganizationScope(r)
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		if app.AdminMachines != nil {
			if _, err := app.AdminMachines.GetMachine(r.Context(), orgID, machineID); err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					writeAPIError(w, r.Context(), http.StatusNotFound, "machine_not_found", "machine not found or not in organization")
					return
				}
				writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
				return
			}
		}
		var body adminDiagnosticRequestBody
		if r.Body != nil {
			_ = json.NewDecoder(r.Body).Decode(&body)
		}
		requestID := uuid.New()
		payload, err := json.Marshal(map[string]any{
			"request_id":   requestID.String(),
			"reason":       strings.TrimSpace(body.Reason),
			"requested_at": time.Now().UTC().Format(time.RFC3339Nano),
			"constraints": map[string]any{
				"no_shell": true,
			},
		})
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		if app.RemoteCommands == nil {
			writeAPIError(w, r.Context(), http.StatusServiceUnavailable, "remote_commands_unavailable", "remote command dispatcher is not configured")
			return
		}
		res, err := app.RemoteCommands.DispatchRemoteMQTTCommand(r.Context(), appdevice.RemoteCommandDispatchInput{
			Append: domaindevice.AppendCommandInput{
				MachineID:      machineID,
				CommandType:    diagnosticBundleCommandType,
				Payload:        payload,
				IdempotencyKey: idem,
				DesiredState:   []byte(`{}`),
			},
		})
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadGateway, "diagnostic_command_dispatch_failed", err.Error())
			return
		}
		recordDiagnosticAudit(r, app, orgID, machineID, res.CommandID, requestID)
		writeJSON(w, http.StatusAccepted, map[string]any{
			"requestId":     requestID.String(),
			"machineId":     machineID.String(),
			"commandId":     res.CommandID.String(),
			"sequence":      res.Sequence,
			"dispatchState": res.DispatchState,
			"replay":        res.Replay,
		})
	}
}

func serveAdminMachineDiagnosticBundlesList(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		machineID, ok := parseMachineIDParam(w, r)
		if !ok {
			return
		}
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
		if app.TelemetryStore == nil {
			writeAPIError(w, r.Context(), http.StatusServiceUnavailable, "diagnostics_unavailable", "diagnostic bundle store is not configured")
			return
		}
		rows, err := db.New(app.TelemetryStore.Pool()).DeviceListDiagnosticBundleManifests(r.Context(), db.DeviceListDiagnosticBundleManifestsParams{
			OrganizationID: pgUUID(orgID),
			MachineID:      machineID,
			Limit:          limit,
			Offset:         offset,
		})
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		items := make([]map[string]any, 0, len(rows))
		for _, row := range rows {
			items = append(items, mapDiagnosticManifest(row))
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"items": items,
			"meta": map[string]any{
				"limit":    limit,
				"offset":   offset,
				"returned": len(items),
			},
		})
	}
}

func parseMachineIDParam(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	machineID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "machineId")))
	if err != nil || machineID == uuid.Nil {
		writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_machine_id", "invalid machineId")
		return uuid.Nil, false
	}
	return machineID, true
}

func pgUUID(id uuid.UUID) pgtype.UUID {
	if id == uuid.Nil {
		return pgtype.UUID{}
	}
	return pgtype.UUID{Bytes: id, Valid: true}
}

func mapDiagnosticManifest(row db.DiagnosticBundleManifest) map[string]any {
	out := map[string]any{
		"bundleId":        row.ID.String(),
		"machineId":       row.MachineID.String(),
		"storageKey":      row.StorageKey,
		"storageProvider": row.StorageProvider,
		"metadata":        json.RawMessage(row.Metadata),
		"status":          row.Status,
		"createdAt":       row.CreatedAt.UTC().Format(time.RFC3339Nano),
	}
	if row.OrganizationID.Valid {
		out["organizationId"] = uuid.UUID(row.OrganizationID.Bytes).String()
	}
	if row.RequestID.Valid {
		out["requestId"] = uuid.UUID(row.RequestID.Bytes).String()
	}
	if row.CommandID.Valid {
		out["commandId"] = uuid.UUID(row.CommandID.Bytes).String()
	}
	if row.ContentType.Valid {
		out["contentType"] = row.ContentType.String
	}
	if row.SizeBytes.Valid {
		out["sizeBytes"] = row.SizeBytes.Int64
	}
	if row.Sha256Hex.Valid {
		out["sha256Hex"] = row.Sha256Hex.String
	}
	if row.ExpiresAt.Valid {
		out["expiresAt"] = row.ExpiresAt.Time.UTC().Format(time.RFC3339Nano)
	}
	return out
}

func recordDiagnosticAudit(r *http.Request, app *api.HTTPApplication, orgID, machineID, commandID, requestID uuid.UUID) {
	if app == nil || app.EnterpriseAudit == nil {
		return
	}
	actorID, _ := principalAccountID(r)
	aid := actorID.String()
	rid := commandID.String()
	meta, _ := json.Marshal(map[string]any{
		"machine_id":   machineID.String(),
		"request_id":   requestID.String(),
		"command_type": diagnosticBundleCommandType,
	})
	_ = app.EnterpriseAudit.Record(r.Context(), compliance.EnterpriseAuditRecord{
		OrganizationID: orgID,
		ActorType:      compliance.ActorUser,
		ActorID:        &aid,
		Action:         "machine.diagnostic.requested",
		ResourceType:   "command_ledger",
		ResourceID:     &rid,
		Metadata:       meta,
	})
}
