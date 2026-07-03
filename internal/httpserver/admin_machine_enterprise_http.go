package httpserver

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	appactivation "github.com/avf/avf-vending-api/internal/app/activation"
	"github.com/avf/avf-vending-api/internal/app/adminops"
	"github.com/avf/avf-vending-api/internal/app/api"
	appfleet "github.com/avf/avf-vending-api/internal/app/fleet"
	"github.com/avf/avf-vending-api/internal/app/machineruntime"
	"github.com/avf/avf-vending-api/internal/config"
	"github.com/avf/avf-vending-api/internal/domain/compliance"
	domainoperator "github.com/avf/avf-vending-api/internal/domain/operator"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/avf/avf-vending-api/internal/platform/auth"
	platformmqtt "github.com/avf/avf-vending-api/internal/platform/mqtt"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func mountAdminMachineEnterpriseRoutes(r chi.Router, app *api.HTTPApplication, cfg *config.Config, writeRL func(http.Handler) http.Handler) {
	if app == nil {
		return
	}
	if writeRL == nil {
		writeRL = func(h http.Handler) http.Handler { return h }
	}
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAnyPermission(auth.PermFleetRead, auth.PermTechnicianRead))
		r.Get("/machines/ops-overview", serveAdminFleetOpsOverview(app))
		r.Get("/machines/{machineId}/runtime-sessions/current", serveAdminMachineRuntimeSessionCurrent(app))
		r.Get("/machines/{machineId}/runtime-sessions/history", serveAdminMachineRuntimeSessionHistory(app))
		r.Get("/machines/{machineId}/app-sessions/current", serveAdminMachineAppSessionCurrent(app))
		r.Get("/machines/{machineId}/app-sessions/history", serveAdminMachineAppSessionHistory(app))
		r.Get("/machines/{machineId}/device-attachments/current", serveAdminMachineDeviceAttachmentCurrent(app))
		r.Get("/machines/{machineId}/device-attachments", serveAdminMachineDeviceAttachmentsList(app))
		r.Get("/machines/{machineId}/ops-overview", serveAdminMachineOpsOverview(app))
		r.Get("/machines/{machineId}/timeline/unified", serveAdminMachineUnifiedTimeline(app))
	})
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireFleetMachineLifecycle)
		r.With(writeRL).Post("/machines/{machineId}/reattach-device", serveAdminMachineReattachDevice(app, cfg))
		r.With(writeRL).Post("/machines/{machineId}/runtime-sessions/revoke", serveAdminMachineRuntimeSessionRevoke(app))
		r.With(writeRL).Post("/machines/{machineId}/app-sessions/{sessionId}/force-end", serveAdminMachineAppSessionForceEnd(app))
		r.With(writeRL).Post("/machines/{machineId}/app-sessions/{sessionId}/mark-stale", serveAdminMachineAppSessionMarkStale(app))
	})
}

func serveAdminMachineRuntimeSessionCurrent(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if app.TelemetryStore == nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", "store not configured")
			return
		}
		machineID, ok := parseChiUUID(w, r, "machineId")
		if !ok {
			return
		}
		sess, err := db.New(app.TelemetryStore.Pool()).GetCurrentMachineRuntimeSession(r.Context(), machineID)
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusOK, map[string]any{"session": nil})
			return
		}
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"session": runtimeSessionJSON(sess)})
	}
}

func serveAdminMachineRuntimeSessionHistory(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if app.TelemetryStore == nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", "store not configured")
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
		rows, err := db.New(app.TelemetryStore.Pool()).ListMachineRuntimeSessionHistory(r.Context(), db.ListMachineRuntimeSessionHistoryParams{
			MachineID: machineID,
			Limit:     int32(limit),
			Offset:    int32(offset),
		})
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		items := make([]map[string]any, 0, len(rows))
		for _, row := range rows {
			items = append(items, runtimeSessionHistoryJSON(row))
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items})
	}
}

func serveAdminMachineRuntimeSessionRevoke(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if app.Fleet == nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", "fleet not configured")
			return
		}
		scopeID, err := parseAdminFleetCompanyScope(r)
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		machineID, ok := parseChiUUID(w, r, "machineId")
		if !ok {
			return
		}
		in, techOrigin, err := parseLifecycleMutation(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_body", err.Error())
			return
		}
		if err := appfleet.ValidateLifecycleMutation("revoke-sessions", in, techOrigin); err != nil {
			writeLifecycleMutationError(w, r.Context(), err)
			return
		}
		out, err := app.Fleet.RevokeMachineSessions(r.Context(), scopeID, machineID, in)
		if err != nil {
			writeLifecycleMutationError(w, r.Context(), err)
			return
		}
		recordLifecycleAuditAndAttribution(r.Context(), app, scopeID, compliance.ActionMachineSessionsRevoked, machineID, in, out)
		writeJSON(w, http.StatusOK, lifecycleMutationJSON(machineID, out))
	}
}

type reattachDeviceBody struct {
	DeviceFingerprint json.RawMessage                 `json:"device_fingerprint"`
	DeviceFpCamel     json.RawMessage                 `json:"deviceFingerprint"`
	OperatorSessionID string                          `json:"operator_session_id"`
	OperatorSession   string                          `json:"operatorSessionId"`
	Reason            string                          `json:"reason"`
	AppVersion        string                          `json:"app_version"`
	AppVersionCamel   string                          `json:"appVersion"`
	BootID            string                          `json:"boot_id"`
	BootIDCamel       string                          `json:"bootId"`
	CorrelationID     string                          `json:"correlation_id"`
	CorrelationCamel  string                          `json:"correlationId"`
	Metadata          json.RawMessage                 `json:"metadata"`
}

func serveAdminMachineReattachDevice(app *api.HTTPApplication, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if app.Activation == nil || cfg == nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", "activation not configured")
			return
		}
		machineID, ok := parseChiUUID(w, r, "machineId")
		if !ok {
			return
		}
		var body reattachDeviceBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_json", "invalid JSON body")
			return
		}
		fpRaw := body.DeviceFingerprint
		if len(fpRaw) == 0 {
			fpRaw = body.DeviceFpCamel
		}
		var fp appactivation.DeviceFingerprint
		if len(fpRaw) > 0 {
			_ = json.Unmarshal(fpRaw, &fp)
		}
		in := appactivation.ReattachInput{
			MachineID:            machineID,
			DeviceFingerprint:    fp,
			RawDeviceFingerprint: fpRaw,
			ClaimContext: appactivation.ClaimContext{
				Reason: strings.TrimSpace(body.Reason),
			},
			ClientIP:  clientIP(r),
			UserAgent: strings.TrimSpace(r.UserAgent()),
		}
		if av := strings.TrimSpace(body.AppVersion); av != "" {
			in.AppVersion = av
		} else {
			in.AppVersion = strings.TrimSpace(body.AppVersionCamel)
		}
		if bid := strings.TrimSpace(body.BootID); bid != "" {
			in.BootID = bid
		} else {
			in.BootID = strings.TrimSpace(body.BootIDCamel)
		}
		osRaw := strings.TrimSpace(body.OperatorSessionID)
		if osRaw == "" {
			osRaw = strings.TrimSpace(body.OperatorSession)
		}
		if osRaw != "" {
			if sid, err := uuid.Parse(osRaw); err == nil {
				in.OperatorSessionID = &sid
			}
		}
		corrRaw := strings.TrimSpace(body.CorrelationID)
		if corrRaw == "" {
			corrRaw = strings.TrimSpace(body.CorrelationCamel)
		}
		if corrRaw != "" {
			if cid, err := uuid.Parse(corrRaw); err == nil {
				in.CorrelationID = &cid
			}
		}
		if p, ok := auth.PrincipalFromContext(r.Context()); ok {
			if aid, err := uuid.Parse(strings.TrimSpace(p.Subject)); err == nil {
				in.ActivatedByAccountID = &aid
				in.AdminReattach = p.HasRole(auth.RolePlatformAdmin) || p.HasRole(auth.RoleOrgAdmin)
			}
		}
		if strings.TrimSpace(in.Reason) == "" {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "reason_required", "reason is required")
			return
		}
		if p, ok := auth.PrincipalFromContext(r.Context()); ok && p.HasRole(auth.RoleTechnician) {
			if in.OperatorSessionID == nil {
				writeAPIError(w, r.Context(), http.StatusBadRequest, "operator_session_required", "operator_session_id is required")
				return
			}
			if app.MachineOperator == nil {
				writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", "operator service not configured")
				return
			}
			osess, err := app.MachineOperator.GetSessionIfMatchesMachine(r.Context(), *in.OperatorSessionID, machineID)
			if errors.Is(err, domainoperator.ErrSessionNotFound) {
				writeAPIError(w, r.Context(), http.StatusNotFound, "operator_session_not_found", "operator session not found")
				return
			}
			if errors.Is(err, domainoperator.ErrSessionMachineMismatch) {
				writeAPIError(w, r.Context(), http.StatusForbidden, "operator_session_machine_mismatch", "operator session does not match machine")
				return
			}
			if err != nil {
				writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
				return
			}
			if !strings.EqualFold(strings.TrimSpace(osess.Status), domainoperator.SessionStatusActive) {
				writeAPIError(w, r.Context(), http.StatusConflict, "operator_session_not_active", "operator session is not active")
				return
			}
			techID, err := uuid.Parse(strings.TrimSpace(p.Subject))
			if err != nil || osess.TechnicianID == nil || *osess.TechnicianID != techID {
				writeAPIError(w, r.Context(), http.StatusForbidden, "operator_session_actor_mismatch", "operator session actor mismatch")
				return
			}
			in.TechnicianID = &techID
		}
		broker := strings.TrimSpace(cfg.MQTT.BrokerURL)
		prefix := strings.TrimSpace(cfg.MQTT.TopicPrefix)
		if prefix == "" {
			prefix = "avf/devices"
		}
		layout := platformmqtt.LayoutString(platformmqtt.NormalizeTopicLayout(cfg.MQTT.TopicLayout))
		out, err := app.Activation.ReattachDevice(r.Context(), in, broker, prefix, layout)
		if errors.Is(err, appactivation.ErrReattachDenied) {
			writeAPIError(w, r.Context(), http.StatusForbidden, "reattach_denied", err.Error())
			return
		}
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"machine_id":               out.MachineID.String(),
			"reattached":               out.Reattached,
			"access_token":             out.MachineToken,
			"access_token_expires_at":  out.TokenExpiresAt.Format(time.RFC3339Nano),
			"refresh_token":            out.RefreshToken,
			"refresh_token_expires_at": out.RefreshExpiresAt.Format(time.RFC3339Nano),
			"session_id":               out.SessionID.String(),
			"credential_version":       0,
			"operator_session_id":      optionalUUIDString(out.OperatorSessionID),
			"correlation_id":           optionalUUIDString(out.CorrelationID),
			"mqtt_broker_url":          out.MQTTBrokerURL,
			"mqtt_topic_prefix":        out.MQTTTopicPrefix,
			"mqtt_topic_layout":        out.MQTTTopicLayout,
			"mqtt_username":            out.MQTTUsername,
			"mqtt_password":            out.MQTTPassword,
		})
	}
}

func serveAdminFleetOpsOverview(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if app.MachineRuntime == nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", "machine runtime not configured")
			return
		}
		limit, offset, err := parseAdminLimitOffset(r)
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		f := machineruntime.OverviewFilter{
			OnlineStatus: strings.TrimSpace(r.URL.Query().Get("online_status")),
			MachineCode:  strings.TrimSpace(r.URL.Query().Get("machine_code")),
			Lifecycle:    strings.TrimSpace(r.URL.Query().Get("status")),
			MachineType:  strings.TrimSpace(r.URL.Query().Get("machine_type")),
			Limit:        int32(limit),
			Offset:       int32(offset),
		}
		if v := strings.TrimSpace(r.URL.Query().Get("sell_ready")); v != "" {
			b := strings.EqualFold(v, "true") || v == "1"
			f.SellReady = &b
		}
		if v := strings.TrimSpace(r.URL.Query().Get("has_active_operator_session")); v != "" {
			b := strings.EqualFold(v, "true") || v == "1"
			f.HasActiveOperatorSession = &b
		}
		if v := strings.TrimSpace(r.URL.Query().Get("site_id")); v != "" {
			if id, err := uuid.Parse(v); err == nil {
				f.SiteID = &id
			}
		}
		items, total, err := app.MachineRuntime.ListMachineAdminOperationalOverview(r.Context(), f)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		out := make([]map[string]any, 0, len(items))
		for _, it := range items {
			out = append(out, fleetOpsOverviewJSON(it))
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": out, "total": total})
	}
}

func serveAdminMachineAppSessionCurrent(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if app.MachineRuntime == nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", "machine runtime not configured")
			return
		}
		machineID, ok := parseChiUUID(w, r, "machineId")
		if !ok {
			return
		}
		sess, err := app.MachineRuntime.GetCurrentRuntimeAppSession(r.Context(), machineID)
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusOK, map[string]any{"session": nil})
			return
		}
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"session": appRuntimeSessionJSON(sess)})
	}
}

func serveAdminMachineAppSessionHistory(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if app.MachineRuntime == nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", "machine runtime not configured")
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
		rows, err := app.MachineRuntime.ListRuntimeAppSessionHistory(r.Context(), machineID, int32(limit), int32(offset))
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		items := make([]map[string]any, 0, len(rows))
		for _, row := range rows {
			items = append(items, appRuntimeSessionJSON(row))
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items})
	}
}

func serveAdminMachineDeviceAttachmentCurrent(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if app.MachineRuntime == nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", "machine runtime not configured")
			return
		}
		machineID, ok := parseChiUUID(w, r, "machineId")
		if !ok {
			return
		}
		att, err := app.MachineRuntime.GetActiveDeviceAttachment(r.Context(), machineID)
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusOK, map[string]any{"attachment": nil})
			return
		}
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"attachment": deviceAttachmentJSON(att)})
	}
}

func serveAdminMachineDeviceAttachmentsList(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if app.MachineRuntime == nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", "machine runtime not configured")
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
		rows, err := app.MachineRuntime.ListDeviceAttachments(r.Context(), machineID, int32(limit), int32(offset))
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		items := make([]map[string]any, 0, len(rows))
		for _, row := range rows {
			items = append(items, deviceAttachmentJSON(row))
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items})
	}
}

func serveAdminMachineAppSessionForceEnd(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if app.MachineRuntime == nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", "machine runtime not configured")
			return
		}
		machineID, ok := parseChiUUID(w, r, "machineId")
		if !ok {
			return
		}
		sessionID, ok := parseChiUUID(w, r, "sessionId")
		if !ok {
			return
		}
		var body struct {
			Reason string `json:"reason"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		row, err := app.MachineRuntime.ForceEndRuntimeAppSession(r.Context(), machineID, sessionID, body.Reason)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"session": appRuntimeSessionJSON(row)})
	}
}

func serveAdminMachineAppSessionMarkStale(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if app.MachineRuntime == nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", "machine runtime not configured")
			return
		}
		machineID, ok := parseChiUUID(w, r, "machineId")
		if !ok {
			return
		}
		sessionID, ok := parseChiUUID(w, r, "sessionId")
		if !ok {
			return
		}
		row, err := app.MachineRuntime.MarkRuntimeAppSessionStale(r.Context(), machineID, sessionID)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"session": appRuntimeSessionJSON(row)})
	}
}

func serveAdminMachineOpsOverview(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if app.AdminOps == nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", "admin ops not configured")
			return
		}
		scopeID, err := parseAdminFleetCompanyScope(r)
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		machineID, ok := parseChiUUID(w, r, "machineId")
		if !ok {
			return
		}
		overview, err := app.AdminOps.GetMachineOpsOverview(r.Context(), scopeID, machineID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				writeAPIError(w, r.Context(), http.StatusNotFound, "not_found", "machine not found")
				return
			}
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		out := opsOverviewJSON(overview)
		if app.MachineRuntime != nil {
			if enriched, err := app.MachineRuntime.BuildMachineAdminOperationalOverview(r.Context(), machineID); err == nil {
				mergeOpsOverviewJSON(out, enriched)
			}
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func serveAdminMachineUnifiedTimeline(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if app.AdminOps == nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", "admin ops not configured")
			return
		}
		scopeID, err := parseAdminFleetCompanyScope(r)
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		machineID, ok := parseChiUUID(w, r, "machineId")
		if !ok {
			return
		}
		limit, _, err := parseAdminLimitOffset(r)
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		var from, to *time.Time
		if v := strings.TrimSpace(r.URL.Query().Get("from")); v != "" {
			if t, err := time.Parse(time.RFC3339, v); err == nil {
				from = &t
			}
		}
		if v := strings.TrimSpace(r.URL.Query().Get("to")); v != "" {
			if t, err := time.Parse(time.RFC3339, v); err == nil {
				to = &t
			}
		}
		var opSess *uuid.UUID
		if v := strings.TrimSpace(r.URL.Query().Get("operator_session_id")); v != "" {
			if id, err := uuid.Parse(v); err == nil {
				opSess = &id
			}
		}
		items, err := app.AdminOps.UnifiedMachineTimeline(r.Context(), scopeID, machineID, adminops.TimelineFilter{
			From:              from,
			To:                to,
			OperatorSessionID: opSess,
			Limit:             int32(limit),
		})
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		out := make([]map[string]any, 0, len(items))
		for _, it := range items {
			out = append(out, unifiedTimelineJSON(it))
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": out})
	}
}

func runtimeSessionJSON(sess db.GetCurrentMachineRuntimeSessionRow) map[string]any {
	out := map[string]any{
		"session_id":         sess.ID.String(),
		"machine_id":         sess.MachineID.String(),
		"credential_version": sess.CredentialVersion,
		"status":             sess.Status,
		"issued_at":          sess.IssuedAt.UTC().Format(time.RFC3339Nano),
		"expires_at":         sess.ExpiresAt.UTC().Format(time.RFC3339Nano),
	}
	if sess.LastUsedAt.Valid {
		out["last_used_at"] = sess.LastUsedAt.Time.UTC().Format(time.RFC3339Nano)
	}
	if sess.RevokedAt.Valid {
		out["revoked_at"] = sess.RevokedAt.Time.UTC().Format(time.RFC3339Nano)
	}
	return out
}

func runtimeSessionHistoryJSON(sess db.ListMachineRuntimeSessionHistoryRow) map[string]any {
	out := map[string]any{
		"session_id":         sess.ID.String(),
		"machine_id":         sess.MachineID.String(),
		"credential_version": sess.CredentialVersion,
		"status":             sess.Status,
		"issued_at":          sess.IssuedAt.UTC().Format(time.RFC3339Nano),
		"expires_at":         sess.ExpiresAt.UTC().Format(time.RFC3339Nano),
	}
	if sess.LastUsedAt.Valid {
		out["last_used_at"] = sess.LastUsedAt.Time.UTC().Format(time.RFC3339Nano)
	}
	if sess.RevokedAt.Valid {
		out["revoked_at"] = sess.RevokedAt.Time.UTC().Format(time.RFC3339Nano)
	}
	return out
}

func opsOverviewJSON(o adminops.OpsOverview) map[string]any {
	out := map[string]any{
		"machine_id":         o.MachineID.String(),
		"status":             o.Status,
		"credential_version": o.CredentialVersion,
		"health":             machineHealthJSON(o.Health),
	}
	if o.RuntimeSession != nil {
		out["runtime_session"] = map[string]any{
			"session_id":         o.RuntimeSession.SessionID.String(),
			"credential_version": o.RuntimeSession.CredentialVersion,
			"status":             o.RuntimeSession.Status,
			"issued_at":          o.RuntimeSession.IssuedAt.Format(time.RFC3339Nano),
			"expires_at":         o.RuntimeSession.ExpiresAt.Format(time.RFC3339Nano),
		}
	}
	if len(o.LastActivationClaim) > 0 {
		out["last_activation_claim"] = json.RawMessage(o.LastActivationClaim)
	}
	if len(o.ActiveOperatorSession) > 0 {
		out["active_operator_session"] = json.RawMessage(o.ActiveOperatorSession)
	}
	return out
}

func unifiedTimelineJSON(it adminops.UnifiedTimelineItem) map[string]any {
	out := map[string]any{
		"id":            it.ID,
		"occurred_at":   it.OccurredAt.Format(time.RFC3339Nano),
		"event_type":    it.EventType,
		"severity":      it.Severity,
		"machine_id":    it.MachineID.String(),
		"actor_type":    it.ActorType,
		"resource_type": it.ResourceType,
		"resource_id":   it.ResourceID,
		"summary":       it.Summary,
		"metadata":      it.Metadata,
	}
	if it.ActorAccountID != nil {
		out["actor_account_id"] = it.ActorAccountID.String()
	}
	if it.OperatorSessionID != nil {
		out["operator_session_id"] = it.OperatorSessionID.String()
	}
	if it.MachineSessionID != nil {
		out["machine_session_id"] = it.MachineSessionID.String()
	}
	if it.CorrelationID != nil {
		out["correlation_id"] = it.CorrelationID.String()
	}
	if it.RequestID != "" {
		out["request_id"] = it.RequestID
	}
	if it.Reason != "" {
		out["reason"] = it.Reason
	}
	if it.ErrorCode != "" {
		out["error_code"] = it.ErrorCode
	}
	return out
}

func optionalUUIDString(id *uuid.UUID) any {
	if id == nil || *id == uuid.Nil {
		return nil
	}
	return id.String()
}

func appRuntimeSessionJSON(sess db.MachineRuntimeAppSession) map[string]any {
	out := map[string]any{
		"session_id":        sess.ID.String(),
		"machine_id":        sess.MachineID.String(),
		"status":            sess.Status,
		"start_reason":      sess.StartReason,
		"started_at":        sess.StartedAt.UTC().Format(time.RFC3339Nano),
		"storefront_state":  sess.StorefrontState,
		"sell_ready":        sess.SellReady,
		"last_network_state": sess.LastNetworkState,
		"last_mqtt_state":   sess.LastMqttState,
	}
	if sess.LastHeartbeatAt.Valid {
		out["last_heartbeat_at"] = sess.LastHeartbeatAt.Time.UTC().Format(time.RFC3339Nano)
	}
	if sess.LastMqttSeenAt.Valid {
		out["last_mqtt_seen_at"] = sess.LastMqttSeenAt.Time.UTC().Format(time.RFC3339Nano)
	}
	if sess.EndedAt.Valid {
		out["ended_at"] = sess.EndedAt.Time.UTC().Format(time.RFC3339Nano)
	}
	if sess.EndReason.Valid {
		out["end_reason"] = sess.EndReason.String
	}
	return out
}

func deviceAttachmentJSON(att db.MachineDeviceAttachment) map[string]any {
	out := map[string]any{
		"attachment_id": att.ID.String(),
		"machine_id":    att.MachineID.String(),
		"status":        att.Status,
		"reason":        att.Reason,
		"attached_at":   att.AttachedAt.UTC().Format(time.RFC3339Nano),
	}
	if att.DetachedAt.Valid {
		out["detached_at"] = att.DetachedAt.Time.UTC().Format(time.RFC3339Nano)
	}
	if att.AndroidID.Valid {
		out["android_id"] = att.AndroidID.String
	}
	if att.BoardSerial.Valid {
		out["board_serial"] = att.BoardSerial.String
	}
	if att.SimIccid.Valid {
		out["sim_iccid"] = att.SimIccid.String
	}
	return out
}

func fleetOpsOverviewJSON(o machineruntime.AdminOperationalOverview) map[string]any {
	out := map[string]any{
		"machine_id":         o.MachineID.String(),
		"machine_code":       o.MachineCode,
		"machine_name":       o.MachineName,
		"lifecycle_status":   o.LifecycleStatus,
		"online_status":      o.OnlineStatus,
		"sale_enabled":       o.SaleEnabled,
		"credential_version": o.CredentialVersion,
		"site_id":            o.SiteID.String(),
		"site_name":          o.SiteName,
		"final_sell_ready":   o.FinalSellReady,
	}
	mergeOpsOverviewJSON(out, o)
	return out
}

func mergeOpsOverviewJSON(base map[string]any, enriched machineruntime.AdminOperationalOverview) {
	base["machine_code"] = enriched.MachineCode
	base["online_status"] = enriched.OnlineStatus
	base["sale_enabled"] = enriched.SaleEnabled
	base["final_sell_ready"] = enriched.FinalSellReady
	if enriched.MachineType != "" {
		base["machine_type"] = enriched.MachineType
	}
	if enriched.LastSeenAt != nil {
		base["last_seen_at"] = enriched.LastSeenAt.Format(time.RFC3339Nano)
	}
	if len(enriched.Connectivity) > 0 {
		base["connectivity"] = json.RawMessage(enriched.Connectivity)
	}
	if enriched.AndroidBoard != nil {
		base["android_board"] = map[string]any{
			"attachment_id": enriched.AndroidBoard.AttachmentID.String(),
			"android_id":    enriched.AndroidBoard.AndroidID,
			"board_serial":  enriched.AndroidBoard.BoardSerial,
		}
	}
	if enriched.SIM != nil {
		base["sim"] = map[string]any{
			"iccid":    enriched.SIM.ICCID,
			"operator": enriched.SIM.Operator,
		}
	}
	if enriched.RuntimeAppSession != nil {
		sess := map[string]any{
			"session_id":       enriched.RuntimeAppSession.SessionID.String(),
			"status":           enriched.RuntimeAppSession.Status,
			"start_reason":     enriched.RuntimeAppSession.StartReason,
			"storefront_state": enriched.RuntimeAppSession.StorefrontState,
			"sell_ready":       enriched.RuntimeAppSession.SellReady,
			"started_at":       enriched.RuntimeAppSession.StartedAt.Format(time.RFC3339Nano),
		}
		if enriched.RuntimeAppSession.LastHeartbeatAt != nil {
			sess["last_heartbeat_at"] = enriched.RuntimeAppSession.LastHeartbeatAt.Format(time.RFC3339Nano)
		}
		if enriched.RuntimeAppSession.LastMQTTSSeenAt != nil {
			sess["last_mqtt_seen_at"] = enriched.RuntimeAppSession.LastMQTTSSeenAt.Format(time.RFC3339Nano)
		}
		if enriched.RuntimeAppSession.LastMQTTState != "" {
			sess["last_mqtt_state"] = enriched.RuntimeAppSession.LastMQTTState
		}
		if len(enriched.RuntimeAppSession.Blockers) > 0 {
			sess["blockers"] = json.RawMessage(enriched.RuntimeAppSession.Blockers)
		}
		base["runtime_app_session"] = sess
	}
	if len(enriched.Readiness) > 0 {
		base["readiness"] = json.RawMessage(enriched.Readiness)
	}
	if enriched.CredentialSession != nil {
		base["credential_session"] = map[string]any{
			"session_id":         enriched.CredentialSession.SessionID.String(),
			"status":             enriched.CredentialSession.Status,
			"credential_version": enriched.CredentialSession.CredentialVersion,
			"issued_at":          enriched.CredentialSession.IssuedAt.Format(time.RFC3339Nano),
		}
	}
	if enriched.OperatorSession != nil {
		base["operator_session"] = map[string]any{
			"session_id": enriched.OperatorSession.SessionID.String(),
			"status":     enriched.OperatorSession.Status,
			"started_at": enriched.OperatorSession.StartedAt.Format(time.RFC3339Nano),
		}
	}
}
