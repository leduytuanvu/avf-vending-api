package httpserver

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/avf/avf-vending-api/internal/app/api"
	appdevice "github.com/avf/avf-vending-api/internal/app/device"
	domaindevice "github.com/avf/avf-vending-api/internal/domain/device"
	"github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func mountDeviceCommandRoutes(r chi.Router, app *api.HTTPApplication) {
	// Register static paths before /commands/{sequence}/… so "receipts" is not parsed as a sequence.
	r.With(auth.RequireMachineURLAccess("machineId"), auth.RequireAnyRole(auth.RolePlatformAdmin, auth.RoleOrgAdmin)).
		Get("/machines/{machineId}/commands/receipts", listMachineCommandReceipts(app))
	r.With(auth.RequireMachineURLAccess("machineId"), auth.RequireAnyRole(auth.RolePlatformAdmin, auth.RoleOrgAdmin)).
		Get("/machines/{machineId}/commands/{sequence}/status", getMachineCommandDispatchStatus(app))
	r.With(auth.RequireMachineURLAccess("machineId"), auth.RequireAnyRole(auth.RolePlatformAdmin, auth.RoleOrgAdmin)).
		Post("/machines/{machineId}/commands/dispatch", postMachineCommandDispatch(app))
}

type postCommandDispatchBody struct {
	CommandType       string          `json:"command_type"`
	Payload           json.RawMessage `json:"payload"`
	CorrelationID     *uuid.UUID      `json:"correlation_id"`
	DesiredState      json.RawMessage `json:"desired_state"`
	OperatorSessionID *uuid.UUID      `json:"operator_session_id"`
}

func postMachineCommandDispatch(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if app == nil || app.RemoteCommands == nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", "application not configured")
			return
		}
		machineID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "machineId")))
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_machine_id", "invalid machineId")
			return
		}
		idem, err := requireWriteIdempotencyKey(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "missing_idempotency_key", err.Error())
			return
		}
		var body postCommandDispatchBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_json", "request body must be JSON")
			return
		}
		if strings.TrimSpace(body.CommandType) == "" {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_command", "command_type is required")
			return
		}
		payload := []byte(body.Payload)
		if len(payload) == 0 {
			payload = []byte("{}")
		}
		if !json.Valid(payload) {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_payload", "payload must be JSON")
			return
		}
		desired := []byte(body.DesiredState)
		if len(desired) == 0 {
			desired = []byte("{}")
		}
		if !json.Valid(desired) {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_desired_state", "desired_state must be JSON when set")
			return
		}

		corr := body.CorrelationID
		if corr != nil && *corr == uuid.Nil {
			corr = nil
		}

		out, err := app.RemoteCommands.DispatchRemoteMQTTCommand(r.Context(), appdevice.RemoteCommandDispatchInput{
			Append: domaindevice.AppendCommandInput{
				MachineID:         machineID,
				CommandType:       strings.TrimSpace(body.CommandType),
				Payload:           payload,
				CorrelationID:     corr,
				IdempotencyKey:    idem,
				DesiredState:      desired,
				OperatorSessionID: body.OperatorSessionID,
			},
		})
		if err != nil {
			if errors.Is(err, appdevice.ErrMQTTCommandPublisherMissing) {
				writeCapabilityNotConfigured(w, r.Context(), "mqtt_command_dispatch", "MQTT broker client is not configured for this API process (set MQTT_BROKER_URL and MQTT_CLIENT_ID)")
				return
			}
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "dispatch_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func getMachineCommandDispatchStatus(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if app == nil || app.RemoteCommands == nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", "application not configured")
			return
		}
		machineID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "machineId")))
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_machine_id", "invalid machineId")
			return
		}
		seqStr := strings.TrimSpace(chi.URLParam(r, "sequence"))
		seq, err := strconv.ParseInt(seqStr, 10, 64)
		if err != nil || seq < 0 {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_sequence", "sequence must be a non-negative integer")
			return
		}
		out, err := app.RemoteCommands.GetRemoteCommandStatus(r.Context(), machineID, seq)
		if err != nil {
			if errors.Is(err, appdevice.ErrNotFound) {
				writeAPIError(w, r.Context(), http.StatusNotFound, "command_not_found", "no command ledger row for this machine and sequence")
				return
			}
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func listMachineCommandReceipts(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if app == nil || app.RemoteCommands == nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", "application not configured")
			return
		}
		machineID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "machineId")))
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_machine_id", "invalid machineId")
			return
		}
		limit, lerr := parseOperatorListLimit(r)
		if lerr != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_limit", lerr.Error())
			return
		}
		items, err := app.RemoteCommands.ListRecentCommandReceipts(r.Context(), machineID, limit)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"items": items,
			"meta": map[string]any{
				"limit":    limit,
				"returned": len(items),
			},
		})
	}
}
