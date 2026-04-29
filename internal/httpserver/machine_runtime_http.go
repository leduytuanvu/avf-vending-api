package httpserver

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/avf/avf-vending-api/internal/app/api"
	appfeatureflags "github.com/avf/avf-vending-api/internal/app/featureflags"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/avf/avf-vending-api/internal/platform/observability/productionmetrics"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func mountMachineRuntimeRoutes(r chi.Router, app *api.HTTPApplication, abuse *AbuseProtection) {
	// Legacy HTTP check-ins and config-applies. Prefer gRPC MachineBootstrapService / MachineTelemetryService paths.
	if app == nil || app.TelemetryStore == nil {
		return
	}
	if abuse == nil {
		abuse = &AbuseProtection{}
	}
	r.With(abuse.MachineScoped(), RequireMachineTenantAccess(app, "machineId")).Post("/machines/{machineId}/check-ins", postMachineCheckIn(app))
	r.With(abuse.MachineScoped(), RequireMachineTenantAccess(app, "machineId")).Post("/machines/{machineId}/config-applies", postMachineConfigApply(app))
}

type machineCheckInRequest struct {
	AndroidID      *string         `json:"android_id"`
	SimSerial      *string         `json:"sim_serial"`
	PackageName    string          `json:"package_name"`
	VersionName    string          `json:"version_name"`
	VersionCode    int64           `json:"version_code"`
	AndroidRelease string          `json:"android_release"`
	SdkInt         int32           `json:"sdk_int"`
	Manufacturer   string          `json:"manufacturer"`
	Model          string          `json:"model"`
	Timezone       string          `json:"timezone"`
	NetworkState   string          `json:"network_state"`
	BootID         string          `json:"boot_id"`
	OccurredAt     string          `json:"occurred_at"`
	Metadata       json.RawMessage `json:"metadata,omitempty"`
}

func postMachineCheckIn(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		machineID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "machineId")))
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_machine_id", "invalid machineId")
			return
		}
		var body machineCheckInRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_json", "request body must be JSON")
			return
		}
		if strings.TrimSpace(body.OccurredAt) == "" {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_time", "occurred_at is required (RFC3339 with timezone offset)")
			return
		}
		occurredAt, err := parseAPITimeRFC3339(body.OccurredAt)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_time", "occurred_at must be RFC3339 with timezone offset")
			return
		}
		meta := []byte(body.Metadata)
		if len(meta) == 0 {
			meta = []byte("{}")
		}
		if !json.Valid(meta) {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_metadata", "metadata must be JSON when set")
			return
		}

		q := db.New(app.TelemetryStore.Pool())
		row, err := q.InsertMachineCheckIn(r.Context(), db.InsertMachineCheckInParams{
			ID:             machineID,
			AndroidID:      optionalStringPtrToPgText(body.AndroidID),
			SimSerial:      optionalStringPtrToPgText(body.SimSerial),
			PackageName:    strings.TrimSpace(body.PackageName),
			VersionName:    strings.TrimSpace(body.VersionName),
			VersionCode:    body.VersionCode,
			AndroidRelease: strings.TrimSpace(body.AndroidRelease),
			SdkInt:         body.SdkInt,
			Manufacturer:   strings.TrimSpace(body.Manufacturer),
			Model:          strings.TrimSpace(body.Model),
			Timezone:       strings.TrimSpace(body.Timezone),
			NetworkState:   strings.TrimSpace(body.NetworkState),
			BootID:         strings.TrimSpace(body.BootID),
			OccurredAt:     occurredAt.UTC(),
			Metadata:       meta,
		})
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "insert_failed", err.Error())
			return
		}
		_ = q.UpdateMachineCurrentSnapshotLastCheckIn(r.Context(), db.UpdateMachineCurrentSnapshotLastCheckInParams{
			MachineID:     machineID,
			LastCheckInAt: pgtype.Timestamptz{Time: occurredAt.UTC(), Valid: true},
		})

		productionmetrics.RecordMachineCheckIn("http")
		resp := map[string]any{
			"id":          strconv.FormatInt(row.ID, 10),
			"machine_id":  row.MachineID.String(),
			"occurred_at": formatAPITimeRFC3339Nano(row.OccurredAt),
		}
		if app.FeatureFlags != nil {
			if rh, err := app.FeatureFlags.RuntimeHintsForMachine(r.Context(), machineID); err == nil && rh != nil {
				resp["runtime_hints"] = runtimeHintsMap(rh)
			}
		}
		writeJSON(w, http.StatusCreated, resp)
	}
}

func runtimeHintsMap(h *appfeatureflags.RuntimeHints) map[string]any {
	if h == nil {
		return nil
	}
	pr := make([]map[string]any, len(h.PendingMachineConfigRollouts))
	for i, x := range h.PendingMachineConfigRollouts {
		pr[i] = map[string]any{
			"rolloutId":          x.RolloutID,
			"targetVersionId":    x.TargetVersionID,
			"targetVersionLabel": x.TargetVersionLabel,
			"status":             x.Status,
		}
	}
	return map[string]any{
		"featureFlags":                 h.FeatureFlags,
		"appliedMachineConfigRevision": h.AppliedMachineConfigRevision,
		"pendingMachineConfigRollouts": pr,
	}
}

func optionalStringPtrToPgText(s *string) pgtype.Text {
	if s == nil || strings.TrimSpace(*s) == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: strings.TrimSpace(*s), Valid: true}
}

type machineConfigApplyRequest struct {
	ConfigVersion     int32           `json:"config_version"`
	AppliedAt         string          `json:"applied_at"`
	AndroidID         string          `json:"android_id"`
	AppVersion        string          `json:"app_version"`
	OperatorSessionID *uuid.UUID      `json:"operator_session_id,omitempty"`
	ConfigPayload     json.RawMessage `json:"config_payload,omitempty"`
}

func postMachineConfigApply(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		machineID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "machineId")))
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_machine_id", "invalid machineId")
			return
		}
		var body machineConfigApplyRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_json", "request body must be JSON")
			return
		}
		if strings.TrimSpace(body.AppliedAt) == "" {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_time", "applied_at is required (RFC3339 with timezone offset)")
			return
		}
		appliedAt, err := parseAPITimeRFC3339(body.AppliedAt)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_time", "applied_at must be RFC3339 with timezone offset")
			return
		}
		if body.ConfigVersion <= 0 {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_config_version", "config_version must be positive")
			return
		}

		q := db.New(app.TelemetryStore.Pool())
		orgID, err := q.GetMachineOrganizationID(r.Context(), machineID)
		if err != nil {
			writeCommerceStoreError(w, r, err)
			return
		}

		payload := []byte(body.ConfigPayload)
		if len(payload) == 0 {
			payload = []byte("{}")
		}
		if !json.Valid(payload) {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_payload", "config_payload must be JSON when set")
			return
		}

		meta := map[string]any{
			"android_id":  strings.TrimSpace(body.AndroidID),
			"app_version": strings.TrimSpace(body.AppVersion),
		}
		metaBytes, err := json.Marshal(meta)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}

		op := pgtype.UUID{}
		if body.OperatorSessionID != nil && *body.OperatorSessionID != uuid.Nil {
			op = pgtype.UUID{Bytes: *body.OperatorSessionID, Valid: true}
		}

		row, err := q.InsertMachineConfigApplication(r.Context(), db.InsertMachineConfigApplicationParams{
			OrganizationID:    orgID,
			MachineID:         machineID,
			AppliedAt:         appliedAt.UTC(),
			ConfigRevision:    body.ConfigVersion,
			ConfigPayload:     payload,
			OperatorSessionID: op,
			Metadata:          metaBytes,
		})
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "insert_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{
			"id":              row.ID.String(),
			"machine_id":      row.MachineID.String(),
			"config_revision": row.ConfigRevision,
			"applied_at":      formatAPITimeRFC3339Nano(row.AppliedAt),
		})
	}
}
