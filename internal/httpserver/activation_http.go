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
		r.Get("/machines/{machineId}/activation-codes", getAdminListActivationCodes(app))
	})
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAnyPermission(auth.PermSetupWrite))
		r.With(writeRL).Post("/machines/{machineId}/activation-codes", postAdminCreateActivationCode(app))
		r.Delete("/machines/{machineId}/activation-codes/{activationCodeId}", deleteAdminActivationCode(app))
	})
}

type adminCreateActivationBody struct {
	ExpiresInMinutes int32  `json:"expiresInMinutes"`
	MaxUses          int32  `json:"maxUses"`
	Notes            string `json:"notes"`
}

func postAdminCreateActivationCode(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, err := parseAdminFleetOrganizationScope(r)
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		machineID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "machineId")))
		if err != nil || machineID == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_machine_id", "invalid machineId")
			return
		}
		var body adminCreateActivationBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_json", "request body must be JSON")
			return
		}
		out, err := app.Activation.CreateCode(r.Context(), activation.CreateInput{
			MachineID:        machineID,
			OrganizationID:   orgID,
			ExpiresInMinutes: body.ExpiresInMinutes,
			MaxUses:          body.MaxUses,
			Notes:            body.Notes,
		})
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{
			"activationCode":   out.PlaintextCode,
			"activationCodeId": out.ID.String(),
			"machineId":        out.MachineID.String(),
			"expiresAt":        out.ExpiresAt.Format("2006-01-02T15:04:05Z07:00"),
			"maxUses":          out.MaxUses,
			"remainingUses":    out.RemainingUses,
			"status":           out.Status,
		})
	}
}

func getAdminListActivationCodes(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, err := parseAdminFleetOrganizationScope(r)
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		machineID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "machineId")))
		if err != nil || machineID == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_machine_id", "invalid machineId")
			return
		}
		rows, err := app.Activation.ListCodes(r.Context(), machineID, orgID)
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
			items = append(items, map[string]any{
				"activationCodeId": row.ID.String(),
				"machineId":        row.MachineID.String(),
				"expiresAt":        row.ExpiresAt.Format("2006-01-02T15:04:05Z07:00"),
				"maxUses":          row.MaxUses,
				"uses":             row.Uses,
				"remainingUses":    row.RemainingUses,
				"status":           row.Status,
				"notes":            row.Notes,
				"createdAt":        row.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			})
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items})
	}
}

func deleteAdminActivationCode(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, err := parseAdminFleetOrganizationScope(r)
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		machineID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "machineId")))
		if err != nil || machineID == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_machine_id", "invalid machineId")
			return
		}
		codeID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "activationCodeId")))
		if err != nil || codeID == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_activation_code_id", "invalid activationCodeId")
			return
		}
		if err := app.Activation.RevokeCode(r.Context(), machineID, orgID, codeID); err != nil {
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
	ActivationCode    string                       `json:"activationCode"`
	DeviceFingerprint activation.DeviceFingerprint `json:"deviceFingerprint"`
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
		out, err := app.Activation.Claim(r.Context(), activation.ClaimInput{
			ActivationCode:    body.ActivationCode,
			DeviceFingerprint: body.DeviceFingerprint,
			ClientIP:          clientIP(r),
			UserAgent:         strings.TrimSpace(r.UserAgent()),
		}, broker, prefix)
		if err != nil {
			if errors.Is(err, activation.ErrInvalid) {
				writeAPIError(w, r.Context(), http.StatusBadRequest, "activation_invalid", "activation code is not valid")
				return
			}
			if errors.Is(err, activation.ErrMachineNotEligible) {
				writeAPIError(w, r.Context(), http.StatusForbidden, "machine_not_eligible", "machine cannot activate")
				return
			}
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		resp := map[string]any{
			"machineId":         out.MachineID.String(),
			"organizationId":    out.OrganizationID.String(),
			"siteId":            out.SiteID.String(),
			"machineName":       out.MachineName,
			"machineToken":      out.MachineToken,
			"tokenExpiresAt":    out.TokenExpiresAt.Format("2006-01-02T15:04:05Z07:00"),
			"bootstrapRequired": out.BootstrapRequired,
			"mqtt": map[string]any{
				"brokerUrl":   out.MQTTBrokerURL,
				"topicPrefix": out.MQTTTopicPrefix,
			},
			"bootstrapUrl": out.BootstrapPath,
		}
		if out.RefreshToken != "" {
			resp["refreshToken"] = out.RefreshToken
			resp["refreshTokenExpiresAt"] = out.RefreshExpiresAt.Format("2006-01-02T15:04:05Z07:00")
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

func mountAdminOrganizationScopedActivationRoutes(r chi.Router, app *api.HTTPApplication, writeRL func(http.Handler) http.Handler) {
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
	ExpiresInMinutes int32  `json:"expiresInMinutes"`
	MaxUses          int32  `json:"maxUses"`
	Notes            string `json:"notes"`
}

func getAdminOrgListActivationCodes(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, err := parseAdminFleetOrganizationScope(r)
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		limit, offset, err := parseAdminLimitOffset(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_pagination", err.Error())
			return
		}
		rows, total, err := app.Activation.ListCodesForOrganization(r.Context(), orgID, limit, offset)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		items := make([]map[string]any, 0, len(rows))
		for _, row := range rows {
			items = append(items, map[string]any{
				"activationCodeId": row.ID.String(),
				"machineId":        row.MachineID.String(),
				"expiresAt":        row.ExpiresAt.Format("2006-01-02T15:04:05Z07:00"),
				"maxUses":          row.MaxUses,
				"uses":             row.Uses,
				"remainingUses":    row.RemainingUses,
				"status":           row.Status,
				"notes":            row.Notes,
				"createdAt":        row.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			})
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
		orgID, err := parseAdminFleetOrganizationScope(r)
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		var body adminOrgCreateActivationBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_json", "request body must be JSON")
			return
		}
		rawMid := strings.TrimSpace(body.MachineID)
		if rawMid == "" {
			rawMid = strings.TrimSpace(body.MachineIDSnake)
		}
		machineID, err := uuid.Parse(rawMid)
		if err != nil || machineID == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_machine_id", "machineId is required")
			return
		}
		out, err := app.Activation.CreateCode(r.Context(), activation.CreateInput{
			MachineID:        machineID,
			OrganizationID:   orgID,
			ExpiresInMinutes: body.ExpiresInMinutes,
			MaxUses:          body.MaxUses,
			Notes:            body.Notes,
		})
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{
			"activationCode":   out.PlaintextCode,
			"activationCodeId": out.ID.String(),
			"machineId":        out.MachineID.String(),
			"expiresAt":        out.ExpiresAt.Format("2006-01-02T15:04:05Z07:00"),
			"maxUses":          out.MaxUses,
			"remainingUses":    out.RemainingUses,
			"status":           out.Status,
		})
	}
}

func postAdminOrgRevokeActivationCode(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, err := parseAdminFleetOrganizationScope(r)
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		codeID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "codeId")))
		if err != nil || codeID == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_activation_code_id", "invalid codeId")
			return
		}
		if err := app.Activation.RevokeCodeForOrganization(r.Context(), orgID, codeID); err != nil {
			if errors.Is(err, activation.ErrNotFound) {
				writeAPIError(w, r.Context(), http.StatusNotFound, "not_found", "not found")
				return
			}
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		cid := codeID.String()
		fleetAudit(r.Context(), app, orgID, compliance.ActionMachineActivationCodeRevoked, "fleet.activation_code", &cid, nil)
		w.WriteHeader(http.StatusNoContent)
	}
}
