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
	"github.com/avf/avf-vending-api/internal/config"
	"github.com/avf/avf-vending-api/internal/domain/compliance"
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
		r.Get("/machines/{machineId}/runtime-sessions/current", serveAdminMachineRuntimeSessionCurrent(app))
		r.Get("/machines/{machineId}/runtime-sessions/history", serveAdminMachineRuntimeSessionHistory(app))
		r.Get("/machines/{machineId}/ops-overview", serveAdminMachineOpsOverview(app))
		r.Get("/machines/{machineId}/timeline/unified", serveAdminMachineUnifiedTimeline(app))
	})
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireFleetMachineLifecycle)
		r.With(writeRL).Post("/machines/{machineId}/reattach-device", serveAdminMachineReattachDevice(app, cfg))
		r.With(writeRL).Post("/machines/{machineId}/runtime-sessions/revoke", serveAdminMachineRuntimeSessionRevoke(app))
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
	DeviceFingerprint appactivation.DeviceFingerprint `json:"device_fingerprint"`
	DeviceFpCamel     appactivation.DeviceFingerprint `json:"deviceFingerprint"`
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
		fp := body.DeviceFingerprint
		if fp.AndroidID == "" && body.DeviceFpCamel.AndroidID != "" {
			fp = body.DeviceFpCamel
		}
		in := appactivation.ReattachInput{
			MachineID:         machineID,
			DeviceFingerprint: fp,
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
		writeJSON(w, http.StatusOK, opsOverviewJSON(overview))
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
