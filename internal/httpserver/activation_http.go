package httpserver

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/avf/avf-vending-api/internal/app/activation"
	"github.com/avf/avf-vending-api/internal/app/api"
	"github.com/avf/avf-vending-api/internal/config"
	"github.com/avf/avf-vending-api/internal/domain/compliance"
	"github.com/avf/avf-vending-api/internal/platform/auth"
	platformmqtt "github.com/avf/avf-vending-api/internal/platform/mqtt"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func mountPublicActivationClaim(r chi.Router, app *api.HTTPApplication, cfg *config.Config, abuse *AbuseProtection, writeRL func(http.Handler) http.Handler) {
	if app == nil || app.Activation == nil || cfg == nil {
		return
	}
	if abuse == nil {
		abuse = &AbuseProtection{}
	}
	if writeRL == nil {
		writeRL = func(next http.Handler) http.Handler { return next }
	}
	r.With(writeRL, abuse.ActivationClaimPOST()).Post("/setup/activation-codes/claim", postActivationClaim(app, cfg))
}

func mountAdminActivationRoutes(r chi.Router, app *api.HTTPApplication, writeRL func(http.Handler) http.Handler) {
	if app == nil || app.Activation == nil {
		return
	}
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAnyPermission(auth.PermFleetRead))
		r.Get("/machines/{machineId}/activation-codes", getAdminListActivationCodes(app, "machineId"))
		r.Get("/machine-codes/{machineCode}/activation-codes", getAdminListActivationCodes(app, "machineCode"))
	})
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAnyPermission(auth.PermSetupWrite))
		r.With(writeRL).Post("/machines/{machineId}/activation-codes", postAdminCreateActivationCode(app, "machineId"))
		r.With(writeRL).Post("/machine-codes/{machineCode}/activation-codes", postAdminCreateActivationCode(app, "machineCode"))
		r.Delete("/machines/{machineId}/activation-codes/{activationCodeId}", deleteAdminActivationCode(app, "machineId"))
		r.Delete("/machine-codes/{machineCode}/activation-codes/{activationCodeId}", deleteAdminActivationCode(app, "machineCode"))
	})
}

type adminCreateActivationBody struct {
	ExpiresInMinutes int32  `json:"expiresInMinutes"`
	MaxUses          int32  `json:"maxUses"`
	Notes            string `json:"notes"`
}

func resolveAdminMachineRef(w http.ResponseWriter, r *http.Request, app *api.HTTPApplication, paramName string) (activation.MachineIdentityRef, bool) {
	ref := strings.TrimSpace(chi.URLParam(r, paramName))
	identity, err := app.Activation.ResolveMachineRef(r.Context(), ref)
	if err != nil {
		switch {
		case errors.Is(err, activation.ErrMachineIdentifierRequired):
			writeAPIError(w, r.Context(), http.StatusBadRequest, "machine_identifier_required", "machine identifier required")
		case errors.Is(err, activation.ErrInvalidMachineIdentifier):
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_machine_identifier", "invalid machine identifier")
		case errors.Is(err, activation.ErrMachineNotFound):
			writeAPIError(w, r.Context(), http.StatusNotFound, "machine_not_found", "machine not found")
		default:
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
		}
		return activation.MachineIdentityRef{}, false
	}
	return identity, true
}

func writeAdminActivationCreateResponse(w http.ResponseWriter, out activation.CreateResult) {
	writeJSON(w, http.StatusCreated, map[string]any{
		"activationCode":   out.PlaintextCode,
		"activationCodeId": out.ID.String(),
		"machineId":        out.MachineID.String(),
		"machineCode":      out.MachineCode,
		"expiresAt":        out.ExpiresAt.Format("2006-01-02T15:04:05Z07:00"),
		"maxUses":          out.MaxUses,
		"remainingUses":    out.RemainingUses,
		"status":           out.Status,
	})
}

func writeAdminActivationListItem(row activation.ListRow) map[string]any {
	return map[string]any{
		"activationCodeId": row.ID.String(),
		"machineId":        row.MachineID.String(),
		"machineCode":      row.MachineCode,
		"expiresAt":        row.ExpiresAt.Format("2006-01-02T15:04:05Z07:00"),
		"maxUses":          row.MaxUses,
		"uses":             row.Uses,
		"remainingUses":    row.RemainingUses,
		"status":           row.Status,
		"notes":            row.Notes,
		"createdAt":        row.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

func postAdminCreateActivationCode(app *api.HTTPApplication, paramName string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		scopeID, err := parseAdminFleetCompanyScope(r)
		_ = scopeID
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		identity, ok := resolveAdminMachineRef(w, r, app, paramName)
		if !ok {
			return
		}
		var body adminCreateActivationBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_json", "request body must be JSON")
			return
		}
		out, err := app.Activation.CreateCode(r.Context(), activation.CreateInput{
			MachineID:        identity.MachineID,
			ExpiresInMinutes: body.ExpiresInMinutes,
			MaxUses:          body.MaxUses,
			Notes:            body.Notes,
		})
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		writeAdminActivationCreateResponse(w, out)
	}
}

func getAdminListActivationCodes(app *api.HTTPApplication, paramName string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		scopeID, err := parseAdminFleetCompanyScope(r)
		_ = scopeID
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		identity, ok := resolveAdminMachineRef(w, r, app, paramName)
		if !ok {
			return
		}
		rows, err := app.Activation.ListCodes(r.Context(), identity.MachineID)
		if err != nil {
			if errors.Is(err, activation.ErrUnauthorized) {
				writeAPIError(w, r.Context(), http.StatusForbidden, "forbidden", "forbidden")
				return
			}
			if errors.Is(err, activation.ErrNotFound) {
				writeAPIError(w, r.Context(), http.StatusNotFound, "not_found", "machine not found")
				return
			}
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		items := make([]map[string]any, 0, len(rows))
		for _, row := range rows {
			items = append(items, writeAdminActivationListItem(row))
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items})
	}
}

func deleteAdminActivationCode(app *api.HTTPApplication, paramName string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		scopeID, err := parseAdminFleetCompanyScope(r)
		_ = scopeID
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		identity, ok := resolveAdminMachineRef(w, r, app, paramName)
		if !ok {
			return
		}
		codeID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "activationCodeId")))
		if err != nil || codeID == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_activation_code_id", "invalid activationCodeId")
			return
		}
		if err := app.Activation.RevokeCode(r.Context(), identity.MachineID, codeID); err != nil {
			if errors.Is(err, activation.ErrNotFound) {
				writeAPIError(w, r.Context(), http.StatusNotFound, "not_found", "not found")
				return
			}
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

type publicClaimBody struct {
	ActivationCode    string         `json:"activationCode"`
	DeviceFingerprint fingerprintDTO `json:"deviceFingerprint"`
	RequestID         string         `json:"requestId"`
	CorrelationID     string         `json:"correlationId"`
	AppVersion        string         `json:"appVersion"`
	BootID            string         `json:"bootId"`
	DeviceSerial      string         `json:"deviceSerial"`
	Reason            string         `json:"reason"`
	ActivationSource  string         `json:"activationSource"`
	OperatorSessionID string         `json:"operatorSessionId"`
}

func claimContextFromRequest(r *http.Request, body publicClaimBody) activation.ClaimContext {
	cc := activation.ClaimContext{
		RequestID:        strings.TrimSpace(r.Header.Get("X-Request-Id")),
		AppVersion:       strings.TrimSpace(body.AppVersion),
		BootID:           strings.TrimSpace(body.BootID),
		DeviceSerial:     strings.TrimSpace(body.DeviceSerial),
		Reason:           strings.TrimSpace(body.Reason),
		ActivationSource: strings.TrimSpace(body.ActivationSource),
	}
	if cc.RequestID == "" {
		cc.RequestID = strings.TrimSpace(body.RequestID)
	}
	corrRaw := strings.TrimSpace(r.Header.Get("X-Correlation-Id"))
	if corrRaw == "" {
		corrRaw = strings.TrimSpace(body.CorrelationID)
	}
	if corrRaw != "" {
		if id, err := uuid.Parse(corrRaw); err == nil {
			cc.CorrelationID = &id
		}
	}
	if sid := strings.TrimSpace(body.OperatorSessionID); sid != "" {
		if id, err := uuid.Parse(sid); err == nil {
			cc.OperatorSessionID = &id
		}
	}
	if p, ok := auth.PrincipalFromContext(r.Context()); ok && strings.TrimSpace(p.Subject) != "" {
		if id, err := uuid.Parse(strings.TrimSpace(p.Subject)); err == nil {
			cc.ActivatedByAccountID = &id
		}
	}
	return cc
}

func postActivationClaim(app *api.HTTPApplication, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body publicClaimBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_json", "request body must be JSON")
			return
		}
		broker := strings.TrimSpace(cfg.MQTT.BrokerURL)
		prefix := strings.TrimSpace(cfg.MQTT.TopicPrefix)
		if prefix == "" {
			prefix = "avf/devices"
		}
		layout := platformmqtt.LayoutString(platformmqtt.NormalizeTopicLayout(cfg.MQTT.TopicLayout))
		out, err := app.Activation.Claim(r.Context(), activation.ClaimInput{
			ActivationCode:    body.ActivationCode,
			DeviceFingerprint: body.DeviceFingerprint.DeviceFingerprint,
			ClientIP:          clientIP(r),
			UserAgent:         strings.TrimSpace(r.UserAgent()),
			ClaimContext:      claimContextFromRequest(r, body),
		}, broker, prefix, layout)
		if err != nil {
			if errors.Is(err, activation.ErrInvalid) {
				writeAPIError(w, r.Context(), http.StatusBadRequest, "activation_invalid", "activation code is not valid")
				return
			}
			if errors.Is(err, activation.ErrMachineNotEligible) {
				writeAPIError(w, r.Context(), http.StatusForbidden, "machine_not_eligible", "machine cannot activate")
				return
			}
			if errors.Is(err, activation.ErrMQTTProvisioning) {
				writeAPIError(w, r.Context(), http.StatusServiceUnavailable, "mqtt_provisioning_failed", "mqtt credential provisioning failed")
				return
			}
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		resp := map[string]any{
			"machineId":         out.MachineID.String(),
			"siteId":            out.SiteID.String(),
			"machineName":       out.MachineName,
			"machineToken":      out.MachineToken,
			"tokenExpiresAt":    out.TokenExpiresAt.Format("2006-01-02T15:04:05Z07:00"),
			"bootstrapRequired": out.BootstrapRequired,
			"mqtt": map[string]any{
				"brokerUrl":   out.MQTTBrokerURL,
				"topicPrefix": out.MQTTTopicPrefix,
				"topicLayout": out.MQTTTopicLayout,
			},
			"bootstrapUrl": out.BootstrapPath,
		}
		if out.MachineCode != "" {
			resp["machineCode"] = out.MachineCode
		}
		if out.RefreshToken != "" {
			resp["refreshToken"] = out.RefreshToken
			resp["refreshTokenExpiresAt"] = out.RefreshExpiresAt.Format("2006-01-02T15:04:05Z07:00")
		}
		if out.MQTTUsername != "" {
			resp["mqttUsername"] = out.MQTTUsername
		}
		if out.MQTTPassword != "" {
			resp["mqttPassword"] = out.MQTTPassword
		}
		if out.DeviceAttachmentID != nil {
			resp["deviceAttachmentId"] = out.DeviceAttachmentID.String()
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

func mountAdminCompanyScopedActivationRoutes(r chi.Router, app *api.HTTPApplication, writeRL func(http.Handler) http.Handler) {
	if app == nil || app.Activation == nil {
		return
	}
	if writeRL == nil {
		writeRL = func(h http.Handler) http.Handler { return h }
	}
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAnyPermission(auth.PermFleetRead))
		r.Get("/activation-codes", getAdminOrgListActivationCodes(app))
	})
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAnyPermission(auth.PermSetupWrite))
		r.With(writeRL).Post("/activation-codes", postAdminOrgCreateActivationCode(app))
		r.With(writeRL).Post("/activation-codes/{codeId}/revoke", postAdminOrgRevokeActivationCode(app))
	})
}

type adminOrgCreateActivationBody struct {
	MachineID        string `json:"machineId"`
	MachineIDSnake   string `json:"machine_id"`
	MachineCode      string `json:"machineCode"`
	MachineCodeSnake string `json:"machine_code"`
	ExpiresInMinutes int32  `json:"expiresInMinutes"`
	MaxUses          int32  `json:"maxUses"`
	Notes            string `json:"notes"`
}

func getAdminOrgListActivationCodes(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		scopeID, err := parseAdminFleetCompanyScope(r)
		_ = scopeID
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		limit, offset, err := parseAdminLimitOffset(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_pagination", err.Error())
			return
		}
		rows, total, err := app.Activation.ListAllCodes(r.Context(), limit, offset)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		items := make([]map[string]any, 0, len(rows))
		for _, row := range rows {
			items = append(items, writeAdminActivationListItem(row))
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"items": items,
			"meta": V1AdminPageMeta{
				Limit:      limit,
				Offset:     offset,
				Returned:   len(items),
				TotalCount: total,
			},
		})
	}
}

func postAdminOrgCreateActivationCode(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		scopeID, err := parseAdminFleetCompanyScope(r)
		_ = scopeID
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		var body adminOrgCreateActivationBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_json", "request body must be JSON")
			return
		}
		identity, err := app.Activation.ResolveMachineBody(r.Context(), body.MachineID, body.MachineIDSnake, body.MachineCode, body.MachineCodeSnake)
		if err != nil {
			switch {
			case errors.Is(err, activation.ErrMachineIdentifierRequired):
				writeAPIError(w, r.Context(), http.StatusBadRequest, "machine_identifier_required", "machine identifier required")
			case errors.Is(err, activation.ErrInvalidMachineIdentifier):
				writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_machine_identifier", "invalid machine identifier")
			case errors.Is(err, activation.ErrMachineNotFound):
				writeAPIError(w, r.Context(), http.StatusNotFound, "machine_not_found", "machine not found")
			case errors.Is(err, activation.ErrMachineIdentifierConflict):
				writeAPIError(w, r.Context(), http.StatusBadRequest, "machine_identifier_conflict", "machine identifier conflict")
			default:
				writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			}
			return
		}
		out, err := app.Activation.CreateCode(r.Context(), activation.CreateInput{
			MachineID:        identity.MachineID,
			ExpiresInMinutes: body.ExpiresInMinutes,
			MaxUses:          body.MaxUses,
			Notes:            body.Notes,
		})
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		writeAdminActivationCreateResponse(w, out)
	}
}

func postAdminOrgRevokeActivationCode(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		scopeID, err := parseAdminFleetCompanyScope(r)
		_ = scopeID
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		codeID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "codeId")))
		if err != nil || codeID == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_activation_code_id", "invalid codeId")
			return
		}
		if err := app.Activation.RevokeCodeByID(r.Context(), codeID); err != nil {
			if errors.Is(err, activation.ErrNotFound) {
				writeAPIError(w, r.Context(), http.StatusNotFound, "not_found", "not found")
				return
			}
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		cid := codeID.String()
		fleetAudit(r.Context(), app, scopeID, compliance.ActionMachineActivationCodeRevoked, "fleet.activation_code", &cid, nil)
		w.WriteHeader(http.StatusNoContent)
	}
}
