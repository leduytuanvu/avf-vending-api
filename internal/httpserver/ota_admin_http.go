package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/avf/avf-vending-api/internal/app/api"
	appotaadmin "github.com/avf/avf-vending-api/internal/app/otaadmin"
	"github.com/avf/avf-vending-api/internal/domain/compliance"
	"github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// mountAdminOTACampaignRoutes registers /v1/admin/ota/campaigns (plus nested paths).
func mountAdminOTACampaignRoutes(r chi.Router, app *api.HTTPApplication, writeRL func(http.Handler) http.Handler) {
	if app == nil || app.AdminOTA == nil {
		return
	}
	if writeRL == nil {
		writeRL = func(h http.Handler) http.Handler { return h }
	}

	r.Route("/ota/campaigns", func(r chi.Router) {
		r.Group(func(r chi.Router) {
			r.Use(auth.RequireAnyPermission(auth.PermOTARead))
			r.Get("/", serveAdminOTACampaignsList(app))
			r.Get("/{campaignId}", serveAdminOTACampaignGet(app))
			r.Get("/{campaignId}/targets", serveAdminOTACampaignTargetsGet(app))
			r.Get("/{campaignId}/results", serveAdminOTACampaignResultsGet(app))
		})

		r.Group(func(r chi.Router) {
			r.Use(auth.RequireAnyPermission(auth.PermOTAWrite))
			r.With(writeRL).Post("/", serveAdminOTACampaignCreate(app))
			r.With(writeRL).Patch("/{campaignId}", serveAdminOTACampaignPatch(app))
			r.With(writeRL).Put("/{campaignId}/targets", serveAdminOTACampaignTargetsPut(app))
			r.With(writeRL).Post("/{campaignId}/pause", serveAdminOTACampaignPause(app))
			r.With(writeRL).Post("/{campaignId}/resume", serveAdminOTACampaignResume(app))
			r.With(writeRL).Post("/{campaignId}/cancel", serveAdminOTACampaignCancel(app))
		})

		r.Group(func(r chi.Router) {
			r.Use(auth.RequireAnyPermission(auth.PermOTAWrite))
			r.Use(requireOTAApproveStartRollback())
			r.With(writeRL).Post("/{campaignId}/approve", serveAdminOTACampaignApprove(app))
			r.With(writeRL).Post("/{campaignId}/start", serveAdminOTACampaignStart(app))
			r.With(writeRL).Post("/{campaignId}/publish", serveAdminOTACampaignPublish(app))
			r.With(writeRL).Post("/{campaignId}/rollback", serveAdminOTACampaignRollback(app))
		})
	})
}

func requireOTAApproveStartRollback() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p, ok := auth.PrincipalFromContext(r.Context())
			if !ok {
				writeAPIError(w, r.Context(), http.StatusUnauthorized, "unauthenticated", auth.ErrUnauthenticated.Error())
				return
			}
			if !(p.HasRole(auth.RoleOrgAdmin) || p.HasRole(auth.RolePlatformAdmin)) {
				writeAPIError(w, r.Context(), http.StatusForbidden, "forbidden", auth.ErrForbidden.Error())
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func writeOTAAdminError(w http.ResponseWriter, ctx context.Context, err error) {
	switch {
	case errors.Is(err, appotaadmin.ErrNotFound):
		writeAPIError(w, ctx, http.StatusNotFound, "not_found", err.Error())
	case errors.Is(err, appotaadmin.ErrInvalidArgument):
		writeAPIError(w, ctx, http.StatusBadRequest, "invalid_argument", err.Error())
	case errors.Is(err, appotaadmin.ErrInvalidTransition):
		writeAPIError(w, ctx, http.StatusConflict, "illegal_transition", err.Error())
	case errors.Is(err, appotaadmin.ErrTargetsLocked):
		writeAPIError(w, ctx, http.StatusConflict, "targets_locked", err.Error())
	case errors.Is(err, appotaadmin.ErrNeedsApproval):
		writeAPIError(w, ctx, http.StatusConflict, "needs_approval", err.Error())
	case errors.Is(err, appotaadmin.ErrRollbackArtifact):
		writeAPIError(w, ctx, http.StatusBadRequest, "rollback_artifact_required", err.Error())
	case errors.Is(err, appotaadmin.ErrNoTargets):
		writeAPIError(w, ctx, http.StatusBadRequest, "no_targets", err.Error())
	case errors.Is(err, appotaadmin.ErrMachinesNotInOrg):
		writeAPIError(w, ctx, http.StatusBadRequest, "machines_not_in_org", err.Error())
	case errors.Is(err, appotaadmin.ErrRolloutNotActive):
		writeAPIError(w, ctx, http.StatusConflict, "rollout_not_active", err.Error())
	case errors.Is(err, appotaadmin.ErrNothingLeftToRollout):
		writeAPIError(w, ctx, http.StatusConflict, "nothing_left_to_rollout", err.Error())
	default:
		writeAPIError(w, ctx, http.StatusInternalServerError, "internal", err.Error())
	}
}

func serveAdminOTACampaignsList(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		scope, err := parseAdminFleetListScope(r)
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		out, err := app.AdminOTA.ListCampaigns(r.Context(), appotaadmin.CampaignListParams{
			OrganizationID: scope.OrganizationID,
			Limit:          scope.Limit,
			Offset:         scope.Offset,
			Status:         scope.Status,
			From:           scope.From,
			To:             scope.To,
		})
		writeV1Collection(w, r.Context(), out, err)
	}
}

func serveAdminOTACampaignGet(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, err := parseAdminFleetOrganizationScope(r)
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		cid, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "campaignId")))
		if err != nil || cid == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_campaign_id", "invalid campaignId")
			return
		}
		out, err := app.AdminOTA.GetCampaignDetail(r.Context(), orgID, cid)
		if err != nil {
			writeOTAAdminError(w, r.Context(), err)
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

type adminCreateOTACampaignBody struct {
	Name               string     `json:"name"`
	ArtifactID         uuid.UUID  `json:"artifactId"`
	ArtifactVersion    *string    `json:"artifactVersion"`
	CampaignType       string     `json:"campaignType"`
	RolloutStrategy    string     `json:"rolloutStrategy"`
	CanaryPercent      int32      `json:"canaryPercent"`
	RollbackArtifactID *uuid.UUID `json:"rollbackArtifactId"`
}

func serveAdminOTACampaignCreate(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := requireWriteIdempotencyKey(r); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "missing_idempotency_key", err.Error())
			return
		}
		orgID, err := parseAdminFleetOrganizationScope(r)
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		actorID, ok := principalAccountID(r)
		if !ok {
			writeAPIError(w, r.Context(), http.StatusUnauthorized, "unauthenticated", auth.ErrUnauthenticated.Error())
			return
		}
		var body adminCreateOTACampaignBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_json", "request body must be JSON")
			return
		}
		out, err := app.AdminOTA.CreateCampaign(r.Context(), appotaadmin.CreateCampaignInput{
			OrganizationID:     orgID,
			Name:               body.Name,
			ArtifactID:         body.ArtifactID,
			ArtifactVersion:    body.ArtifactVersion,
			CampaignType:       body.CampaignType,
			RolloutStrategy:    body.RolloutStrategy,
			CanaryPercent:      body.CanaryPercent,
			RollbackArtifactID: body.RollbackArtifactID,
			CreatedBy:          actorID,
		})
		if err != nil {
			writeOTAAdminError(w, r.Context(), err)
			return
		}
		writeJSON(w, http.StatusCreated, out)
	}
}

type adminPatchOTACampaignBody struct {
	Name               *string    `json:"name"`
	ArtifactVersion    *string    `json:"artifactVersion"`
	CampaignType       *string    `json:"campaignType"`
	RolloutStrategy    *string    `json:"rolloutStrategy"`
	CanaryPercent      *int32     `json:"canaryPercent"`
	RollbackArtifactID *uuid.UUID `json:"rollbackArtifactId"`
}

func serveAdminOTACampaignPatch(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := requireWriteIdempotencyKey(r); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "missing_idempotency_key", err.Error())
			return
		}
		orgID, err := parseAdminFleetOrganizationScope(r)
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		cid, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "campaignId")))
		if err != nil || cid == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_campaign_id", "invalid campaignId")
			return
		}
		var body adminPatchOTACampaignBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_json", "request body must be JSON")
			return
		}
		out, err := app.AdminOTA.PatchCampaign(r.Context(), orgID, cid, appotaadmin.PatchCampaignInput{
			Name:               body.Name,
			ArtifactVersion:    body.ArtifactVersion,
			CampaignType:       body.CampaignType,
			RolloutStrategy:    body.RolloutStrategy,
			CanaryPercent:      body.CanaryPercent,
			RollbackArtifactID: body.RollbackArtifactID,
		})
		if err != nil {
			writeOTAAdminError(w, r.Context(), err)
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func serveAdminOTACampaignTargetsPut(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := requireWriteIdempotencyKey(r); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "missing_idempotency_key", err.Error())
			return
		}
		orgID, err := parseAdminFleetOrganizationScope(r)
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		cid, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "campaignId")))
		if err != nil || cid == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_campaign_id", "invalid campaignId")
			return
		}
		var body appotaadmin.CampaignTargetsPutBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_json", "request body must be JSON")
			return
		}
		err = app.AdminOTA.PutCampaignTargets(r.Context(), appotaadmin.PutTargetsInput{
			OrganizationID: orgID,
			CampaignID:     cid,
			MachineIDs:     body.MachineIDs,
		})
		if err != nil {
			writeOTAAdminError(w, r.Context(), err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func serveAdminOTACampaignTargetsGet(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, err := parseAdminFleetOrganizationScope(r)
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		cid, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "campaignId")))
		if err != nil || cid == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_campaign_id", "invalid campaignId")
			return
		}
		items, err := app.AdminOTA.ListCampaignTargets(r.Context(), orgID, cid)
		if err != nil {
			writeOTAAdminError(w, r.Context(), err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items})
	}
}

func serveAdminOTACampaignResultsGet(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, err := parseAdminFleetOrganizationScope(r)
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		cid, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "campaignId")))
		if err != nil || cid == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_campaign_id", "invalid campaignId")
			return
		}
		items, err := app.AdminOTA.ListCampaignResults(r.Context(), orgID, cid)
		if err != nil {
			writeOTAAdminError(w, r.Context(), err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items})
	}
}

func serveAdminOTACampaignPublish(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := requireWriteIdempotencyKey(r); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "missing_idempotency_key", err.Error())
			return
		}
		orgID, err := parseAdminFleetOrganizationScope(r)
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		actorID, ok := principalAccountID(r)
		if !ok {
			writeAPIError(w, r.Context(), http.StatusUnauthorized, "unauthenticated", auth.ErrUnauthenticated.Error())
			return
		}
		cid, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "campaignId")))
		if err != nil || cid == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_campaign_id", "invalid campaignId")
			return
		}
		out, err := app.AdminOTA.PublishCampaign(r.Context(), orgID, cid, actorID)
		if err != nil {
			writeOTAAdminError(w, r.Context(), err)
			return
		}
		if app.EnterpriseAudit != nil {
			cs := cid.String()
			md, _ := json.Marshal(map[string]any{"campaign_id": cs})
			at, aid := compliance.ActorUser, ""
			if p, ok := auth.PrincipalFromContext(r.Context()); ok {
				at, aid = p.Actor()
			}
			_ = app.EnterpriseAudit.Record(r.Context(), compliance.EnterpriseAuditRecord{
				OrganizationID: orgID,
				ActorType:      at,
				ActorID:        stringPtrOrNil(aid),
				Action:         compliance.ActionOTACampaignPublished,
				ResourceType:   "ota.campaign",
				ResourceID:     &cs,
				Metadata:       md,
			})
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func serveAdminOTACampaignApprove(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := requireWriteIdempotencyKey(r); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "missing_idempotency_key", err.Error())
			return
		}
		orgID, err := parseAdminFleetOrganizationScope(r)
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		actorID, ok := principalAccountID(r)
		if !ok {
			writeAPIError(w, r.Context(), http.StatusUnauthorized, "unauthenticated", auth.ErrUnauthenticated.Error())
			return
		}
		cid, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "campaignId")))
		if err != nil || cid == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_campaign_id", "invalid campaignId")
			return
		}
		out, err := app.AdminOTA.ApproveCampaign(r.Context(), orgID, cid, actorID)
		if err != nil {
			writeOTAAdminError(w, r.Context(), err)
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func serveAdminOTACampaignStart(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := requireWriteIdempotencyKey(r); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "missing_idempotency_key", err.Error())
			return
		}
		orgID, err := parseAdminFleetOrganizationScope(r)
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		cid, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "campaignId")))
		if err != nil || cid == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_campaign_id", "invalid campaignId")
			return
		}
		out, err := app.AdminOTA.StartCampaign(r.Context(), orgID, cid)
		if err != nil {
			writeOTAAdminError(w, r.Context(), err)
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func serveAdminOTACampaignPause(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := requireWriteIdempotencyKey(r); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "missing_idempotency_key", err.Error())
			return
		}
		orgID, err := parseAdminFleetOrganizationScope(r)
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		cid, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "campaignId")))
		if err != nil || cid == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_campaign_id", "invalid campaignId")
			return
		}
		out, err := app.AdminOTA.PauseCampaign(r.Context(), orgID, cid)
		if err != nil {
			writeOTAAdminError(w, r.Context(), err)
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func serveAdminOTACampaignResume(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := requireWriteIdempotencyKey(r); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "missing_idempotency_key", err.Error())
			return
		}
		orgID, err := parseAdminFleetOrganizationScope(r)
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		cid, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "campaignId")))
		if err != nil || cid == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_campaign_id", "invalid campaignId")
			return
		}
		out, err := app.AdminOTA.ResumeCampaign(r.Context(), orgID, cid)
		if err != nil {
			writeOTAAdminError(w, r.Context(), err)
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func serveAdminOTACampaignCancel(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := requireWriteIdempotencyKey(r); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "missing_idempotency_key", err.Error())
			return
		}
		orgID, err := parseAdminFleetOrganizationScope(r)
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		cid, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "campaignId")))
		if err != nil || cid == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_campaign_id", "invalid campaignId")
			return
		}
		out, err := app.AdminOTA.CancelCampaign(r.Context(), orgID, cid)
		if err != nil {
			writeOTAAdminError(w, r.Context(), err)
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func serveAdminOTACampaignRollback(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := requireWriteIdempotencyKey(r); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "missing_idempotency_key", err.Error())
			return
		}
		orgID, err := parseAdminFleetOrganizationScope(r)
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		cid, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "campaignId")))
		if err != nil || cid == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_campaign_id", "invalid campaignId")
			return
		}
		var body appotaadmin.RollbackCampaignBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil && !errors.Is(err, io.EOF) {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_json", "request body must be JSON")
			return
		}
		out, err := app.AdminOTA.RollbackCampaign(r.Context(), orgID, cid, body.RollbackArtifactID)
		if err != nil {
			writeOTAAdminError(w, r.Context(), err)
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}
