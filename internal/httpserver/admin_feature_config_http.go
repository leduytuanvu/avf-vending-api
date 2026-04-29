package httpserver

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/avf/avf-vending-api/internal/app/api"
	appfeatureflags "github.com/avf/avf-vending-api/internal/app/featureflags"
	"github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func mountAdminFeatureConfigRoutes(r chi.Router, app *api.HTTPApplication, writeRL func(http.Handler) http.Handler) {
	if app == nil || app.FeatureFlags == nil {
		return
	}
	if writeRL == nil {
		writeRL = func(h http.Handler) http.Handler { return h }
	}
	svc := app.FeatureFlags
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAnyPermission(auth.PermFleetRead))
		r.Get("/feature-flags", listAdminFeatureFlags(svc))
		r.Get("/feature-flags/{flagId}", getAdminFeatureFlag(svc))
		r.Get("/machine-config/rollouts", listAdminMachineConfigRollouts(svc))
		r.Get("/machine-config/rollouts/{rolloutId}", getAdminMachineConfigRollout(svc))
	})
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAnyPermission(auth.PermFleetWrite))
		r.With(writeRL).Post("/feature-flags", postAdminFeatureFlag(svc))
		r.With(writeRL).Patch("/feature-flags/{flagId}", patchAdminFeatureFlag(svc))
		r.With(writeRL).Post("/feature-flags/{flagId}/enable", postAdminFeatureFlagEnable(svc))
		r.With(writeRL).Post("/feature-flags/{flagId}/disable", postAdminFeatureFlagDisable(svc))
		r.With(writeRL).Put("/feature-flags/{flagId}/targets", putAdminFeatureFlagTargets(svc))
		r.With(writeRL).Post("/machine-config/rollouts", postAdminMachineConfigRollouts(svc))
	})
}

func parseOrgFleet(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	orgID, err := parseAdminFleetOrganizationScope(r)
	if err != nil {
		writeV1ListError(w, r.Context(), err)
		return uuid.Nil, false
	}
	return orgID, true
}

func listAdminFeatureFlags(svc *appfeatureflags.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, ok := parseOrgFleet(w, r)
		if !ok {
			return
		}
		limit, offset, err := parseAdminLimitOffset(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_pagination", err.Error())
			return
		}
		out, err := svc.ListFlags(r.Context(), appfeatureflags.ListFlagsParams{
			OrganizationID: orgID,
			Limit:          limit,
			Offset:         offset,
		})
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"items": out.Items,
			"meta": map[string]any{
				"limit":    limit,
				"offset":   offset,
				"returned": len(out.Items),
				"total":    out.Total,
			},
		})
	}
}

func getAdminFeatureFlag(svc *appfeatureflags.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, ok := parseOrgFleet(w, r)
		if !ok {
			return
		}
		fid, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "flagId")))
		if err != nil || fid == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_flag_id", "invalid flagId")
			return
		}
		out, err := svc.GetFlag(r.Context(), orgID, fid)
		if err != nil {
			if errors.Is(err, appfeatureflags.ErrNotFound) {
				writeAPIError(w, r.Context(), http.StatusNotFound, "not_found", "feature flag not found")
				return
			}
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"flag":    out.Flag,
			"targets": out.Targets,
		})
	}
}

type postFeatureFlagBody struct {
	FlagKey     string          `json:"flagKey"`
	DisplayName string          `json:"displayName"`
	Description string          `json:"description"`
	Enabled     bool            `json:"enabled"`
	Metadata    json.RawMessage `json:"metadata,omitempty"`
}

func postAdminFeatureFlag(svc *appfeatureflags.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, ok := parseOrgFleet(w, r)
		if !ok {
			return
		}
		var body postFeatureFlagBody
		if !decodeStrictJSON(w, r, &body) {
			return
		}
		meta := body.Metadata
		if len(meta) == 0 {
			meta = json.RawMessage("{}")
		}
		out, err := svc.CreateFlag(r.Context(), appfeatureflags.CreateFlagParams{
			OrganizationID: orgID,
			FlagKey:        body.FlagKey,
			DisplayName:    body.DisplayName,
			Description:    body.Description,
			Enabled:        body.Enabled,
			Metadata:       meta,
		})
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, out)
	}
}

type patchFeatureFlagBody struct {
	DisplayName *string          `json:"displayName"`
	Description *string          `json:"description"`
	Enabled     *bool            `json:"enabled"`
	Metadata    *json.RawMessage `json:"metadata"`
}

func patchAdminFeatureFlag(svc *appfeatureflags.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, ok := parseOrgFleet(w, r)
		if !ok {
			return
		}
		fid, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "flagId")))
		if err != nil || fid == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_flag_id", "invalid flagId")
			return
		}
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_json", "request body")
			return
		}
		var patch patchFeatureFlagBody
		if len(strings.TrimSpace(string(body))) > 0 {
			if err := json.Unmarshal(body, &patch); err != nil {
				writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_json", "invalid request body")
				return
			}
		}
		var metaBytes *[]byte
		if patch.Metadata != nil {
			b := []byte(*patch.Metadata)
			metaBytes = &b
		}
		out, err := svc.PatchFlag(r.Context(), appfeatureflags.PatchFlagParams{
			OrganizationID: orgID,
			FlagID:         fid,
			DisplayName:    patch.DisplayName,
			Description:    patch.Description,
			Enabled:        patch.Enabled,
			MetadataJSON:   metaBytes,
		})
		if err != nil {
			if errors.Is(err, appfeatureflags.ErrNotFound) {
				writeAPIError(w, r.Context(), http.StatusNotFound, "not_found", "feature flag not found")
				return
			}
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func postAdminFeatureFlagEnable(svc *appfeatureflags.Service) http.HandlerFunc {
	return enableDisableFeatureFlag(svc, true)
}

func postAdminFeatureFlagDisable(svc *appfeatureflags.Service) http.HandlerFunc {
	return enableDisableFeatureFlag(svc, false)
}

func enableDisableFeatureFlag(svc *appfeatureflags.Service, enabled bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, ok := parseOrgFleet(w, r)
		if !ok {
			return
		}
		fid, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "flagId")))
		if err != nil || fid == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_flag_id", "invalid flagId")
			return
		}
		out, err := svc.SetEnabled(r.Context(), orgID, fid, enabled)
		if err != nil {
			if errors.Is(err, appfeatureflags.ErrNotFound) {
				writeAPIError(w, r.Context(), http.StatusNotFound, "not_found", "feature flag not found")
				return
			}
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

type putTargetsBody struct {
	Targets []targetWire `json:"targets"`
}

type targetWire struct {
	TargetType        string          `json:"targetType"`
	SiteID            *string         `json:"siteId,omitempty"`
	MachineID         *string         `json:"machineId,omitempty"`
	HardwareProfileID *string         `json:"hardwareProfileId,omitempty"`
	CanaryPercent     *float64        `json:"canaryPercent,omitempty"`
	Priority          int32           `json:"priority"`
	Enabled           bool            `json:"enabled"`
	Metadata          json.RawMessage `json:"metadata,omitempty"`
}

func putAdminFeatureFlagTargets(svc *appfeatureflags.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, ok := parseOrgFleet(w, r)
		if !ok {
			return
		}
		fid, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "flagId")))
		if err != nil || fid == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_flag_id", "invalid flagId")
			return
		}
		var body putTargetsBody
		if !decodeStrictJSON(w, r, &body) {
			return
		}
		inputs := make([]appfeatureflags.TargetInput, 0, len(body.Targets))
		for _, tw := range body.Targets {
			in := appfeatureflags.TargetInput{
				TargetType: tw.TargetType,
				Priority:   tw.Priority,
				Enabled:    tw.Enabled,
				Metadata:   tw.Metadata,
			}
			if tw.SiteID != nil && strings.TrimSpace(*tw.SiteID) != "" {
				sid, perr := uuid.Parse(strings.TrimSpace(*tw.SiteID))
				if perr != nil {
					writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_site_id", "invalid siteId in targets")
					return
				}
				in.SiteID = &sid
			}
			if tw.MachineID != nil && strings.TrimSpace(*tw.MachineID) != "" {
				mid, perr := uuid.Parse(strings.TrimSpace(*tw.MachineID))
				if perr != nil {
					writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_machine_id", "invalid machineId in targets")
					return
				}
				in.MachineID = &mid
			}
			if tw.HardwareProfileID != nil && strings.TrimSpace(*tw.HardwareProfileID) != "" {
				hid, perr := uuid.Parse(strings.TrimSpace(*tw.HardwareProfileID))
				if perr != nil {
					writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_hardware_profile_id", "invalid hardwareProfileId in targets")
					return
				}
				in.HardwareProfileID = &hid
			}
			in.CanaryPercent = tw.CanaryPercent
			inputs = append(inputs, in)
		}
		out, err := svc.ReplaceTargets(r.Context(), appfeatureflags.PutTargetsParams{
			OrganizationID: orgID,
			FlagID:         fid,
			Targets:        inputs,
		})
		if err != nil {
			if errors.Is(err, appfeatureflags.ErrNotFound) {
				writeAPIError(w, r.Context(), http.StatusNotFound, "not_found", "feature flag not found")
				return
			}
			if errors.Is(err, appfeatureflags.ErrInvalidTarget) {
				writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_targets", err.Error())
				return
			}
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"targets": out})
	}
}

type postRolloutBody struct {
	RollbackFromRolloutID *string         `json:"rollbackFromRolloutId,omitempty"`
	TargetVersionID       *string         `json:"targetVersionId,omitempty"`
	VersionLabel          *string         `json:"versionLabel,omitempty"`
	ConfigPayload         json.RawMessage `json:"configPayload,omitempty"`
	ParentVersionID       *string         `json:"parentVersionId,omitempty"`
	PreviousVersionID     *string         `json:"previousVersionId,omitempty"`
	Status                string          `json:"status,omitempty"`
	CanaryPercent         *float64        `json:"canaryPercent,omitempty"`
	ScopeType             string          `json:"scopeType"`
	SiteID                *string         `json:"siteId,omitempty"`
	MachineID             *string         `json:"machineId,omitempty"`
	HardwareProfileID     *string         `json:"hardwareProfileId,omitempty"`
	Metadata              json.RawMessage `json:"metadata,omitempty"`
}

func postAdminMachineConfigRollouts(svc *appfeatureflags.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, ok := parseOrgFleet(w, r)
		if !ok {
			return
		}
		var body postRolloutBody
		if !decodeStrictJSON(w, r, &body) {
			return
		}
		if body.RollbackFromRolloutID != nil && strings.TrimSpace(*body.RollbackFromRolloutID) != "" {
			rid, err := uuid.Parse(strings.TrimSpace(*body.RollbackFromRolloutID))
			if err != nil || rid == uuid.Nil {
				writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_rollout_id", "invalid rollbackFromRolloutId")
				return
			}
			out, err := svc.RollbackRollout(r.Context(), orgID, rid)
			if err != nil {
				if errors.Is(err, appfeatureflags.ErrNotFound) {
					writeAPIError(w, r.Context(), http.StatusNotFound, "not_found", "rollout not found")
					return
				}
				if errors.Is(err, appfeatureflags.ErrInvalidRollout) {
					writeAPIError(w, r.Context(), http.StatusBadRequest, "rollback_not_allowed", "rollout has no previous version to restore")
					return
				}
				writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
				return
			}
			writeJSON(w, http.StatusCreated, out)
			return
		}

		var targetVer uuid.UUID
		if body.TargetVersionID != nil && strings.TrimSpace(*body.TargetVersionID) != "" {
			var err error
			targetVer, err = uuid.Parse(strings.TrimSpace(*body.TargetVersionID))
			if err != nil || targetVer == uuid.Nil {
				writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_target_version_id", "invalid targetVersionId")
				return
			}
		} else {
			if body.VersionLabel == nil || strings.TrimSpace(*body.VersionLabel) == "" {
				writeAPIError(w, r.Context(), http.StatusBadRequest, "version_required", "targetVersionId or versionLabel+configPayload is required")
				return
			}
			payload := body.ConfigPayload
			if len(payload) == 0 {
				payload = json.RawMessage("{}")
			}
			var parent *uuid.UUID
			if body.ParentVersionID != nil && strings.TrimSpace(*body.ParentVersionID) != "" {
				p, err := uuid.Parse(strings.TrimSpace(*body.ParentVersionID))
				if err != nil || p == uuid.Nil {
					writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_parent_version_id", "invalid parentVersionId")
					return
				}
				parent = &p
			}
			ver, err := svc.CreateMachineConfigVersion(r.Context(), appfeatureflags.CreateMachineConfigVersionParams{
				OrganizationID:  orgID,
				VersionLabel:    strings.TrimSpace(*body.VersionLabel),
				ConfigPayload:   payload,
				ParentVersionID: parent,
			})
			if err != nil {
				writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
				return
			}
			targetVer = ver.ID
		}

		var prev *uuid.UUID
		if body.PreviousVersionID != nil && strings.TrimSpace(*body.PreviousVersionID) != "" {
			p, err := uuid.Parse(strings.TrimSpace(*body.PreviousVersionID))
			if err != nil || p == uuid.Nil {
				writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_previous_version_id", "invalid previousVersionId")
				return
			}
			prev = &p
		}

		st := strings.TrimSpace(body.Status)
		meta := body.Metadata
		if len(meta) == 0 {
			meta = json.RawMessage("{}")
		}

		var siteID *uuid.UUID
		if body.SiteID != nil && strings.TrimSpace(*body.SiteID) != "" {
			sid, err := uuid.Parse(strings.TrimSpace(*body.SiteID))
			if err != nil {
				writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_site_id", "invalid siteId")
				return
			}
			siteID = &sid
		}
		var mid *uuid.UUID
		if body.MachineID != nil && strings.TrimSpace(*body.MachineID) != "" {
			m, err := uuid.Parse(strings.TrimSpace(*body.MachineID))
			if err != nil {
				writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_machine_id", "invalid machineId")
				return
			}
			mid = &m
		}
		var hid *uuid.UUID
		if body.HardwareProfileID != nil && strings.TrimSpace(*body.HardwareProfileID) != "" {
			h, err := uuid.Parse(strings.TrimSpace(*body.HardwareProfileID))
			if err != nil {
				writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_hardware_profile_id", "invalid hardwareProfileId")
				return
			}
			hid = &h
		}

		out, err := svc.CreateRollout(r.Context(), appfeatureflags.CreateRolloutParams{
			OrganizationID:    orgID,
			TargetVersionID:   targetVer,
			PreviousVersionID: prev,
			Status:            st,
			CanaryPercent:     body.CanaryPercent,
			ScopeType:         body.ScopeType,
			SiteID:            siteID,
			MachineID:         mid,
			HardwareProfileID: hid,
			Metadata:          meta,
		})
		if err != nil {
			if errors.Is(err, appfeatureflags.ErrInvalidRollout) {
				writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_rollout", err.Error())
				return
			}
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, out)
	}
}

func listAdminMachineConfigRollouts(svc *appfeatureflags.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, ok := parseOrgFleet(w, r)
		if !ok {
			return
		}
		limit, offset, err := parseAdminLimitOffset(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_pagination", err.Error())
			return
		}
		out, err := svc.ListRollouts(r.Context(), appfeatureflags.ListRolloutsParams{
			OrganizationID: orgID,
			Limit:          limit,
			Offset:         offset,
		})
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"items": out.Items,
			"meta": map[string]any{
				"limit":    limit,
				"offset":   offset,
				"returned": len(out.Items),
				"total":    out.Total,
			},
		})
	}
}

func getAdminMachineConfigRollout(svc *appfeatureflags.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, ok := parseOrgFleet(w, r)
		if !ok {
			return
		}
		rid, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "rolloutId")))
		if err != nil || rid == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_rollout_id", "invalid rolloutId")
			return
		}
		out, err := svc.GetRollout(r.Context(), orgID, rid)
		if err != nil {
			if errors.Is(err, appfeatureflags.ErrNotFound) {
				writeAPIError(w, r.Context(), http.StatusNotFound, "not_found", "rollout not found")
				return
			}
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}
