package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/avf/avf-vending-api/internal/app/api"
	"github.com/avf/avf-vending-api/internal/app/planogram"
	"github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func mountAdminPlanogramRoutes(r chi.Router, app *api.HTTPApplication, writeRL func(http.Handler) http.Handler) {
	if app == nil || app.Planogram == nil {
		return
	}
	if writeRL == nil {
		writeRL = func(h http.Handler) http.Handler { return h }
	}
	r.Route("/machines/{machineId}", func(r chi.Router) {
		r.Group(func(r chi.Router) {
			r.Use(auth.RequireAnyPermission(auth.PermFleetRead))
			r.Get("/planogram", getOrgMachinePlanogram(app))
			r.Get("/planogram/versions", getOrgMachinePlanogramVersions(app))
		})
		r.Group(func(r chi.Router) {
			r.Use(auth.RequireAnyPermission(auth.PermFleetWrite))
			r.With(writeRL).Post("/planogram/drafts", postOrgMachinePlanogramDraft(app))
			r.With(writeRL).Patch("/planogram/drafts/{draftId}", patchOrgMachinePlanogramDraft(app))
			r.With(writeRL).Post("/planogram/drafts/{draftId}/validate", postOrgMachinePlanogramDraftValidate(app))
			r.With(writeRL).Post("/planogram/drafts/{draftId}/publish", postOrgMachinePlanogramDraftPublish(app))
			r.With(writeRL).Post("/planogram/versions/{versionId}/rollback", postOrgMachinePlanogramVersionRollback(app))
		})
	})
}

func parseInteractiveActorUUID(r *http.Request) (*uuid.UUID, bool) {
	p, ok := auth.PrincipalFromContext(r.Context())
	if !ok {
		return nil, false
	}
	s := strings.TrimSpace(p.Subject)
	if s == "" {
		return nil, false
	}
	u, err := uuid.Parse(s)
	if err != nil || u == uuid.Nil {
		return nil, false
	}
	return &u, true
}

func getOrgMachinePlanogram(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if app == nil || app.Planogram == nil {
			writeCapabilityNotConfigured(w, r.Context(), "planogram", "planogram service not configured")
			return
		}
		orgID, err := parseAdminFleetOrganizationScope(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_organization", err.Error())
			return
		}
		machineID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "machineId")))
		if err != nil || machineID == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_machine_id", "invalid machineId")
			return
		}
		out, err := app.Planogram.GetSummary(r.Context(), orgID, machineID)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

type planogramDraftCreateBody struct {
	Snapshot json.RawMessage `json:"snapshot"`
}

func postOrgMachinePlanogramDraft(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, err := parseAdminFleetOrganizationScope(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_organization", err.Error())
			return
		}
		machineID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "machineId")))
		if err != nil || machineID == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_machine_id", "invalid machineId")
			return
		}
		var body planogramDraftCreateBody
		if !decodeStrictJSON(w, r, &body) {
			return
		}
		id, err := app.Planogram.CreateDraft(r.Context(), orgID, machineID, body.Snapshot)
		if err != nil {
			writePlanogramError(w, r.Context(), err)
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"id": id.String()})
	}
}

type planogramDraftPatchBody struct {
	Snapshot json.RawMessage `json:"snapshot,omitempty"`
	Status   *string         `json:"status,omitempty"`
}

func patchOrgMachinePlanogramDraft(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, err := parseAdminFleetOrganizationScope(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_organization", err.Error())
			return
		}
		machineID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "machineId")))
		if err != nil || machineID == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_machine_id", "invalid machineId")
			return
		}
		draftID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "draftId")))
		if err != nil || draftID == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_draft_id", "invalid draftId")
			return
		}
		var body planogramDraftPatchBody
		if !decodeStrictJSON(w, r, &body) {
			return
		}
		err = app.Planogram.PatchDraft(r.Context(), orgID, machineID, draftID, body.Snapshot, body.Status)
		if err != nil {
			writePlanogramError(w, r.Context(), err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func postOrgMachinePlanogramDraftValidate(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, err := parseAdminFleetOrganizationScope(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_organization", err.Error())
			return
		}
		machineID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "machineId")))
		if err != nil || machineID == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_machine_id", "invalid machineId")
			return
		}
		draftID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "draftId")))
		if err != nil || draftID == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_draft_id", "invalid draftId")
			return
		}
		if err := app.Planogram.ValidateDraft(r.Context(), orgID, machineID, draftID); err != nil {
			writePlanogramError(w, r.Context(), err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func postOrgMachinePlanogramDraftPublish(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, err := parseAdminFleetOrganizationScope(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_organization", err.Error())
			return
		}
		machineID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "machineId")))
		if err != nil || machineID == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_machine_id", "invalid machineId")
			return
		}
		draftID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "draftId")))
		if err != nil || draftID == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_draft_id", "invalid draftId")
			return
		}
		idem := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
		var actor *uuid.UUID
		if u, ok := parseInteractiveActorUUID(r); ok {
			actor = u
		}
		out, err := app.Planogram.PublishDraft(r.Context(), orgID, machineID, draftID, idem, actor)
		if err != nil {
			writePlanogramError(w, r.Context(), err)
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func getOrgMachinePlanogramVersions(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if app == nil || app.Planogram == nil {
			writeCapabilityNotConfigured(w, r.Context(), "planogram", "planogram service not configured")
			return
		}
		orgID, err := parseAdminFleetOrganizationScope(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_organization", err.Error())
			return
		}
		machineID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "machineId")))
		if err != nil || machineID == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_machine_id", "invalid machineId")
			return
		}
		items, err := app.Planogram.ListVersions(r.Context(), orgID, machineID)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items})
	}
}

func postOrgMachinePlanogramVersionRollback(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, err := parseAdminFleetOrganizationScope(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_organization", err.Error())
			return
		}
		machineID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "machineId")))
		if err != nil || machineID == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_machine_id", "invalid machineId")
			return
		}
		versionID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "versionId")))
		if err != nil || versionID == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_version_id", "invalid versionId")
			return
		}
		idem := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
		out, err := app.Planogram.Rollback(r.Context(), orgID, machineID, versionID, idem)
		if err != nil {
			writePlanogramError(w, r.Context(), err)
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func writePlanogramError(w http.ResponseWriter, ctx context.Context, err error) {
	switch {
	case err == nil:
		return
	case errors.Is(err, planogram.ErrNotFound):
		writeAPIError(w, ctx, http.StatusNotFound, "not_found", err.Error())
	case errors.Is(err, planogram.ErrInvalidSnapshot):
		writeAPIError(w, ctx, http.StatusBadRequest, "invalid_snapshot", err.Error())
	case errors.Is(err, planogram.ErrValidation):
		writeAPIError(w, ctx, http.StatusBadRequest, "validation_failed", err.Error())
	default:
		writeAPIError(w, ctx, http.StatusInternalServerError, "internal", err.Error())
	}
}
