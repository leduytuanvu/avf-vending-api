package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/avf/avf-vending-api/internal/app/api"
	appfleet "github.com/avf/avf-vending-api/internal/app/fleet"
	"github.com/avf/avf-vending-api/internal/domain/compliance"
	domainfleet "github.com/avf/avf-vending-api/internal/domain/fleet"
	"github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func mountAdminFleetWriteRoutes(r chi.Router, app *api.HTTPApplication, writeRL func(http.Handler) http.Handler) {
	if app == nil || app.Fleet == nil {
		return
	}
	if writeRL == nil {
		writeRL = func(h http.Handler) http.Handler { return h }
	}
	f := app.Fleet

	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAnyPermission(auth.PermSiteRead, auth.PermTechnicianRead, auth.PermFleetRead))
		r.Get("/sites", serveAdminSitesList(app, f))
		r.Get("/sites/{siteId}", serveAdminSiteGet(app, f))
		r.Get("/technicians/{technicianId}", serveAdminTechnicianGet(app, f))
		r.Get("/technician-assignments", serveAdminTechnicianAssignmentsList(app))
		r.Get("/technician-assignments/{assignmentId}", serveAdminAssignmentGet(app, f))
		r.Get("/assignments/{assignmentId}", serveAdminAssignmentGet(app, f))
	})

	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAnyPermission(auth.PermSiteWrite, auth.PermFleetWrite))
		r.With(writeRL).Post("/sites", serveAdminSiteCreate(app, f))
		r.With(writeRL).Patch("/sites/{siteId}", serveAdminSitePatch(app, f))
		r.With(writeRL).Post("/sites/{siteId}/disable", serveAdminSiteDisable(app, f))
		r.With(writeRL).Post("/sites/{siteId}/archive", serveAdminSiteDisable(app, f))
		r.With(writeRL).Delete("/sites/{siteId}", serveAdminSiteDeactivate(app, f))

		r.With(writeRL).Post("/machines", serveAdminMachineCreate(app, f))
		r.With(writeRL).Patch("/machines/{machineId}", serveAdminMachinePatch(app, f))

		r.Group(func(r chi.Router) {
			r.Use(auth.RequireFleetMachineLifecycle)
			r.With(writeRL).Post("/machines/{machineId}/disable", serveAdminMachineDisable(app, f))
			r.With(writeRL).Post("/machines/{machineId}/suspend", serveAdminMachineDisable(app, f))
			r.With(writeRL).Post("/machines/{machineId}/enable", serveAdminMachineEnable(app, f))
			r.With(writeRL).Post("/machines/{machineId}/resume", serveAdminMachineEnable(app, f))
			r.With(writeRL).Post("/machines/{machineId}/activate", serveAdminMachineEnable(app, f))
			r.With(writeRL).Post("/machines/{machineId}/deactivate", serveAdminMachineDisable(app, f))
			r.With(writeRL).Post("/machines/{machineId}/retire", serveAdminMachineRetire(app, f))
			r.With(writeRL).Post("/machines/{machineId}/archive", serveAdminMachineRetire(app, f))
			r.With(writeRL).Post("/machines/{machineId}/mark-compromised", serveAdminMachineCompromised(app, f))
			r.With(writeRL).Post("/machines/{machineId}/rotate-credential", serveAdminMachineRotateCredential(app, f))
			r.With(writeRL).Post("/machines/{machineId}/rotate-credentials", serveAdminMachineRotateCredential(app, f))
			r.With(writeRL).Post("/machines/{machineId}/rotate-token-version", serveAdminMachineRotateCredential(app, f))
			r.With(writeRL).Post("/machines/{machineId}/revoke-credentials", serveAdminMachineRevokeCredential(app, f))
			r.With(writeRL).Post("/machines/{machineId}/revoke-token", serveAdminMachineRevokeCredential(app, f))
			r.With(writeRL).Post("/machines/{machineId}/transfer-site", serveAdminMachineTransferSite(app, f))
			r.With(writeRL).Post("/machines/{machineId}/revoke-sessions", serveAdminMachineRevokeSessions(app, f))
		})
	})

	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAnyPermission(auth.PermFleetRead, auth.PermTechnicianRead))
		r.Get("/machines/{machineId}/technicians", serveAdminMachineTechniciansList(app))
	})
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAnyPermission(auth.PermTechnicianWrite, auth.PermFleetWrite))
		r.With(writeRL).Post("/machines/{machineId}/technicians", serveAdminMachineTechnicianAssign(app, f))
		r.With(writeRL).Delete("/machines/{machineId}/technicians/{userId}", serveAdminMachineTechnicianRemove(app, f))
	})

	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAnyPermission(auth.PermTechnicianWrite, auth.PermFleetWrite))
		r.With(writeRL).Post("/technicians", serveAdminTechnicianCreate(app, f))
		r.With(writeRL).Patch("/technicians/{technicianId}", serveAdminTechnicianPatch(app, f))
		r.With(writeRL).Post("/technicians/{technicianId}/disable", serveAdminTechnicianDisable(app, f))
		r.With(writeRL).Post("/technicians/{technicianId}/enable", serveAdminTechnicianEnable(app, f))

		r.With(writeRL).Post("/technician-assignments", serveAdminAssignmentCreate(app, f))
		r.With(writeRL).Patch("/technician-assignments/{assignmentId}", serveAdminAssignmentPatch(app, f))
		r.With(writeRL).Post("/technician-assignments/{assignmentId}/cancel", serveAdminAssignmentCancel(app, f))
		r.With(writeRL).Delete("/technician-assignments/{assignmentId}", serveAdminAssignmentRelease(app, f))
		r.With(writeRL).Post("/assignments", serveAdminAssignmentCreate(app, f))
		r.With(writeRL).Delete("/assignments/{assignmentId}", serveAdminAssignmentRelease(app, f))
	})

}

func mountAdminCompanyFleetRoutes(r chi.Router, app *api.HTTPApplication, f *appfleet.Service, writeRL func(http.Handler) http.Handler) {
	r.Route("/", func(r chi.Router) {
		r.Group(func(r chi.Router) {
			r.Use(auth.RequireAnyPermission(auth.PermSiteRead, auth.PermFleetRead, auth.PermTechnicianRead))
			r.Get("/sites", serveAdminSitesList(app, f))
			r.Get("/sites/{siteId}", serveAdminSiteGet(app, f))
			r.Get("/machines", serveAdminMachinesList(app))
			r.Get("/machines/{machineId}", serveAdminMachineGet(app))
			r.Get("/machines/{machineId}/technicians", serveAdminMachineTechniciansList(app))
		})
		r.Group(func(r chi.Router) {
			r.Use(auth.RequireAnyPermission(auth.PermSiteWrite, auth.PermFleetWrite))
			r.With(writeRL).Post("/sites", serveAdminSiteCreate(app, f))
			r.With(writeRL).Patch("/sites/{siteId}", serveAdminSitePatch(app, f))
			r.With(writeRL).Post("/sites/{siteId}/archive", serveAdminSiteDisable(app, f))
			r.With(writeRL).Delete("/sites/{siteId}", serveAdminSiteDeactivate(app, f))
			r.With(writeRL).Post("/machines", serveAdminMachineCreate(app, f))
			r.With(writeRL).Patch("/machines/{machineId}", serveAdminMachinePatch(app, f))
			r.Group(func(r chi.Router) {
				r.Use(auth.RequireFleetMachineLifecycle)
				r.With(writeRL).Post("/machines/{machineId}/archive", serveAdminMachineRetire(app, f))
				r.With(writeRL).Post("/machines/{machineId}/suspend", serveAdminMachineDisable(app, f))
				r.With(writeRL).Post("/machines/{machineId}/resume", serveAdminMachineEnable(app, f))
				r.With(writeRL).Post("/machines/{machineId}/mark-compromised", serveAdminMachineCompromised(app, f))
				r.With(writeRL).Post("/machines/{machineId}/rotate-credentials", serveAdminMachineRotateCredential(app, f))
				r.With(writeRL).Post("/machines/{machineId}/rotate-token-version", serveAdminMachineRotateCredential(app, f))
				r.With(writeRL).Post("/machines/{machineId}/revoke-credentials", serveAdminMachineRevokeCredential(app, f))
				r.With(writeRL).Post("/machines/{machineId}/revoke-token", serveAdminMachineRevokeCredential(app, f))
				r.With(writeRL).Post("/machines/{machineId}/transfer-site", serveAdminMachineTransferSite(app, f))
				r.With(writeRL).Post("/machines/{machineId}/revoke-sessions", serveAdminMachineRevokeSessions(app, f))
			})
		})
		r.Group(func(r chi.Router) {
			r.Use(auth.RequireAnyPermission(auth.PermTechnicianWrite, auth.PermFleetWrite))
			r.With(writeRL).Post("/machines/{machineId}/technicians", serveAdminMachineTechnicianAssign(app, f))
			r.With(writeRL).Delete("/machines/{machineId}/technicians/{userId}", serveAdminMachineTechnicianRemove(app, f))
		})
		r.Group(func(r chi.Router) {
			r.Use(auth.RequireAnyPermission(auth.PermTechnicianRead, auth.PermFleetRead))
			r.Get("/technicians", serveAdminTechnicianDirectoryList(app))
			r.Get("/technicians/{technicianId}", serveAdminTechnicianGet(app, f))
			r.Get("/assignments", serveAdminTechnicianAssignmentsList(app))
			r.Get("/assignments/{assignmentId}", serveAdminAssignmentGet(app, f))
		})
		r.Group(func(r chi.Router) {
			r.Use(auth.RequireAnyPermission(auth.PermTechnicianWrite, auth.PermFleetWrite))
			r.With(writeRL).Post("/technicians", serveAdminTechnicianCreate(app, f))
			r.With(writeRL).Patch("/technicians/{technicianId}", serveAdminTechnicianPatch(app, f))
			r.With(writeRL).Post("/technicians/{technicianId}/disable", serveAdminTechnicianDisable(app, f))
			r.With(writeRL).Post("/technicians/{technicianId}/enable", serveAdminTechnicianEnable(app, f))
			r.With(writeRL).Post("/assignments", serveAdminAssignmentCreate(app, f))
			r.With(writeRL).Delete("/assignments/{assignmentId}", serveAdminAssignmentRelease(app, f))
		})
		mountAdminCompanyScopedActivationRoutes(r, app, writeRL)
		mountAdminPlanogramRoutes(r, app, writeRL)
		mountAdminOperationsRoutes(r, app, writeRL)
		mountAdminAnomalyRoutes(r, app, writeRL)
		mountAdminProvisioningRoutes(r, app, writeRL)
		mountAdminRolloutRoutes(r, app, writeRL)
	})
}

func fleetAudit(ctx context.Context, app *api.HTTPApplication, org uuid.UUID, action, resourceType string, resourceID *string, meta map[string]any) {
	if app == nil || app.EnterpriseAudit == nil {
		return
	}
	md, _ := json.Marshal(meta)
	at, aid := compliance.ActorUser, ""
	if p, ok := auth.PrincipalFromContext(ctx); ok {
		at, aid = p.Actor()
	}
	_ = app.EnterpriseAudit.Record(ctx, compliance.EnterpriseAuditRecord{
		ActorType:    at,
		ActorID:      stringPtrOrNil(aid),
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Metadata:     md,
	})
}

func stringPtrOrNil(s string) *string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return &s
}

func serveAdminTechnicianDirectoryList(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if app == nil || app.AdminTechnicians == nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", "application not configured")
			return
		}
		scope, err := parseAdminFleetListScope(r)
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		out, err := app.AdminTechnicians.ListTechnicians(r.Context(), scope)
		writeV1Collection(w, r.Context(), out, err)
	}
}

func serveAdminMachinesList(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if app == nil || app.AdminMachines == nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", "application not configured")
			return
		}
		scope, err := parseAdminFleetListScope(r)
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		out, err := app.AdminMachines.ListMachines(r.Context(), scope)
		writeV1Collection(w, r.Context(), out, err)
	}
}

func writeFleetAppError(w http.ResponseWriter, ctx context.Context, err error) {
	switch {
	case err == nil:
		return
	case errors.Is(err, appfleet.ErrNotFound):
		writeAPIError(w, ctx, http.StatusNotFound, "not_found", err.Error())
	case errors.Is(err, appfleet.ErrOrgMismatch):
		writeAPIError(w, ctx, http.StatusForbidden, "forbidden", err.Error())
	case errors.Is(err, appfleet.ErrInvalidArgument):
		writeAPIError(w, ctx, http.StatusBadRequest, "invalid_argument", err.Error())
	case errors.Is(err, appfleet.ErrConflict):
		writeAPIError(w, ctx, http.StatusConflict, "conflict", err.Error())
	case errors.Is(err, appfleet.ErrForbiddenTechnicianSelfAssignment):
		writeAPIError(w, ctx, http.StatusForbidden, "forbidden", err.Error())
	default:
		writeAPIError(w, ctx, http.StatusInternalServerError, "internal", err.Error())
	}
}

func siteJSON(s domainfleet.Site) map[string]any {
	return siteJSONWithMachineCount(s, nil)
}

func siteJSONWithMachineCount(s domainfleet.Site, machineCount *int64) map[string]any {
	out := map[string]any{
		"id":         s.ID.String(),
		"name":       s.Name,
		"timezone":   s.Timezone,
		"code":       s.Code,
		"status":     s.Status,
		"created_at": s.CreatedAt.UTC().Format(time.RFC3339Nano),
		"updated_at": s.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
	if s.RegionID != nil {
		out["region_id"] = s.RegionID.String()
	}
	if machineCount != nil {
		out["machine_count"] = *machineCount
	}
	if len(s.Address) > 0 {
		var raw map[string]any
		if json.Unmarshal(s.Address, &raw) == nil {
			out["address"] = raw
			if city, ok := raw["city"].(string); ok && strings.TrimSpace(city) != "" {
				out["city"] = city
			}
			if region, ok := raw["region"].(string); ok && strings.TrimSpace(region) != "" {
				out["region"] = region
			}
		} else {
			out["address"] = json.RawMessage(s.Address)
		}
	} else {
		out["address"] = map[string]any{}
	}
	return out
}

func technicianJSON(t domainfleet.Technician) map[string]any {
	out := map[string]any{
		"id":           t.ID.String(),
		"display_name": t.DisplayName,
		"status":       t.Status,
		"created_at":   t.CreatedAt.UTC().Format(time.RFC3339Nano),
		"updated_at":   t.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
	if t.Email != nil {
		out["email"] = *t.Email
	}
	if t.Phone != nil {
		out["phone"] = *t.Phone
	}
	if t.ExternalSubject != nil {
		out["external_subject"] = *t.ExternalSubject
	}
	return out
}

func assignmentJSON(a domainfleet.TechnicianMachineAssignment) map[string]any {
	out := map[string]any{
		"id":                a.ID.String(),
		"technician_id":     a.TechnicianID.String(),
		"machine_id":        a.MachineID.String(),
		"role":              a.Role,
		"assignment_domain": a.AssignmentDomain,
		"status":            a.Status,
		"valid_from":        a.ValidFrom.UTC().Format(time.RFC3339Nano),
		"created_at":        a.CreatedAt.UTC().Format(time.RFC3339Nano),
		"updated_at":        a.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
	if a.CreatedBy != nil {
		out["created_by"] = a.CreatedBy.String()
	}
	if a.ValidTo != nil {
		out["valid_to"] = a.ValidTo.UTC().Format(time.RFC3339Nano)
	}
	return out
}

func machineJSON(m domainfleet.Machine) map[string]any {
	out := map[string]any{
		"id":                 m.ID.String(),
		"site_id":            m.SiteID.String(),
		"serial_number":      m.SerialNumber,
		"code":               m.Code,
		"model":              m.Model,
		"cabinet_type":       m.CabinetType,
		"timezone":           m.Timezone,
		"name":               m.Name,
		"status":             m.Status,
		"credential_version": m.CredentialVersion,
		"command_sequence":   m.CommandSequence,
		"created_at":         m.CreatedAt.UTC().Format(time.RFC3339Nano),
		"updated_at":         m.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
	if m.LastSeenAt != nil {
		out["last_seen_at"] = m.LastSeenAt.UTC().Format(time.RFC3339Nano)
	}
	if m.ActivatedAt != nil {
		out["activated_at"] = m.ActivatedAt.UTC().Format(time.RFC3339Nano)
	}
	if m.RevokedAt != nil {
		out["revoked_at"] = m.RevokedAt.UTC().Format(time.RFC3339Nano)
	}
	if m.RotatedAt != nil {
		out["rotated_at"] = m.RotatedAt.UTC().Format(time.RFC3339Nano)
	}
	if m.HardwareProfileID != nil {
		out["hardware_profile_id"] = m.HardwareProfileID.String()
	}
	return out
}

func serveAdminSitesList(app *api.HTTPApplication, f *appfleet.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		scopeID, err := parseAdminFleetCompanyScope(r)
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		limit, offset, err := parseAdminLimitOffset(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_pagination", err.Error())
			return
		}
		var st *string
		if raw := strings.TrimSpace(r.URL.Query().Get("status")); raw != "" {
			st = &raw
		}
		search := strings.TrimSpace(r.URL.Query().Get("search"))
		city := strings.TrimSpace(r.URL.Query().Get("city"))
		region := strings.TrimSpace(r.URL.Query().Get("region"))
		items, total, err := f.ListSites(r.Context(), appfleet.ListSitesInput{
			Status: st,
			Search: search,
			City:   city,
			Region: region,
			Limit:  int32(limit),
			Offset: int32(offset),
		})
		if err != nil {
			writeFleetAppError(w, r.Context(), err)
			return
		}
		arr := make([]map[string]any, 0, len(items))
		for _, s := range items {
			var mc *int64
			if n, err := f.CountNonRetiredMachinesForSite(r.Context(), scopeID, s.ID); err == nil {
				mc = &n
			}
			arr = append(arr, siteJSONWithMachineCount(s, mc))
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"items": arr,
			"meta": V1AdminPageMeta{
				Limit:      limit,
				Offset:     offset,
				Returned:   len(arr),
				TotalCount: total,
			},
		})
	}
}

func serveAdminSiteGet(app *api.HTTPApplication, f *appfleet.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		scopeID, err := parseAdminFleetCompanyScope(r)
		_ = scopeID
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		sid, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "siteId")))
		if err != nil || sid == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_site_id", "invalid siteId")
			return
		}
		s, err := f.GetSite(r.Context(), scopeID, sid)
		if err != nil {
			writeFleetAppError(w, r.Context(), err)
			return
		}
		writeJSON(w, http.StatusOK, siteJSON(s))
	}
}

type v1AdminSiteWriteRequest struct {
	Name     string          `json:"name"`
	RegionID *string         `json:"region_id"`
	Address  json.RawMessage `json:"address"`
	Timezone string          `json:"timezone"`
	Code     string          `json:"code"`
}

func serveAdminSiteCreate(app *api.HTTPApplication, f *appfleet.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		scopeID, err := parseAdminFleetCompanyScope(r)
		_ = scopeID
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		var body v1AdminSiteWriteRequest
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_json", "invalid JSON body")
			return
		}
		var region *uuid.UUID
		if body.RegionID != nil && strings.TrimSpace(*body.RegionID) != "" {
			rid, perr := uuid.Parse(strings.TrimSpace(*body.RegionID))
			if perr != nil || rid == uuid.Nil {
				writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_region_id", "invalid region_id")
				return
			}
			region = &rid
		}
		addr := body.Address
		if len(addr) == 0 {
			addr = []byte("{}")
		}
		s, err := f.CreateSite(r.Context(), appfleet.CreateSiteInput{
			RegionID: region,
			Name:     body.Name,
			Address:  addr,
			Timezone: body.Timezone,
			Code:     body.Code,
		})
		if err != nil {
			writeFleetAppError(w, r.Context(), err)
			return
		}
		rid := s.ID.String()
		fleetAudit(r.Context(), app, scopeID, compliance.ActionSiteCreated, "fleet.site", &rid, map[string]any{"code": s.Code})
		writeJSON(w, http.StatusCreated, siteJSON(s))
	}
}

type v1AdminSitePatchRequest struct {
	Name     *string         `json:"name"`
	RegionID *string         `json:"region_id"`
	Address  json.RawMessage `json:"address"`
	Timezone *string         `json:"timezone"`
	Code     *string         `json:"code"`
	Status   *string         `json:"status"`
}

func serveAdminSitePatch(app *api.HTTPApplication, f *appfleet.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		scopeID, err := parseAdminFleetCompanyScope(r)
		_ = scopeID
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		sid, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "siteId")))
		if err != nil || sid == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_site_id", "invalid siteId")
			return
		}
		var body v1AdminSitePatchRequest
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_json", "invalid JSON body")
			return
		}
		var region *uuid.UUID
		if body.RegionID != nil {
			if strings.TrimSpace(*body.RegionID) == "" {
				region = nil
			} else {
				rid, perr := uuid.Parse(strings.TrimSpace(*body.RegionID))
				if perr != nil || rid == uuid.Nil {
					writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_region_id", "invalid region_id")
					return
				}
				region = &rid
			}
		}
		in := appfleet.UpdateSiteInput{
			SiteID:   sid,
			RegionID: region,
			Name:     body.Name,
			Timezone: body.Timezone,
			Code:     body.Code,
			Status:   body.Status,
		}
		if body.Address != nil {
			in.Address = body.Address
		}
		s, err := f.UpdateSite(r.Context(), in)
		if err != nil {
			writeFleetAppError(w, r.Context(), err)
			return
		}
		rid := s.ID.String()
		fleetAudit(r.Context(), app, scopeID, compliance.ActionSiteUpdated, "fleet.site", &rid, nil)
		writeJSON(w, http.StatusOK, siteJSON(s))
	}
}

func serveAdminSiteDeactivate(app *api.HTTPApplication, f *appfleet.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		scopeID, err := parseAdminFleetCompanyScope(r)
		_ = scopeID
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		sid, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "siteId")))
		if err != nil || sid == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_site_id", "invalid siteId")
			return
		}
		s, err := f.DeactivateSite(r.Context(), scopeID, sid)
		if err != nil {
			writeFleetAppError(w, r.Context(), err)
			return
		}
		rid := s.ID.String()
		fleetAudit(r.Context(), app, scopeID, compliance.ActionSiteDeactivated, "fleet.site", &rid, nil)
		writeJSON(w, http.StatusOK, siteJSON(s))
	}
}

func serveAdminSiteDisable(app *api.HTTPApplication, f *appfleet.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		scopeID, err := parseAdminFleetCompanyScope(r)
		_ = scopeID
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		sid, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "siteId")))
		if err != nil || sid == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_site_id", "invalid siteId")
			return
		}
		s, err := f.DeactivateSite(r.Context(), scopeID, sid)
		if err != nil {
			writeFleetAppError(w, r.Context(), err)
			return
		}
		rid := s.ID.String()
		fleetAudit(r.Context(), app, scopeID, compliance.ActionSiteDisabled, "fleet.site", &rid, nil)
		writeJSON(w, http.StatusOK, siteJSON(s))
	}
}

type v1AdminMachineCreateRequest struct {
	SiteID            string  `json:"site_id"`
	SiteIDCamel       string  `json:"siteId"`
	HardwareProfileID *string `json:"hardware_profile_id"`
	SerialNumber      string  `json:"serial_number"`
	SerialNumberCamel string  `json:"serialNumber"`
	Code              string  `json:"code"`
	Model             string  `json:"model"`
	CabinetType       string  `json:"cabinet_type"`
	CabinetTypeCamel  string  `json:"cabinetType"`
	Timezone          string  `json:"timezone"`
	Name              string  `json:"name"`
	Status            string  `json:"status"`
}

func serveAdminMachineCreate(app *api.HTTPApplication, f *appfleet.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		scopeID, err := parseAdminFleetCompanyScope(r)
		_ = scopeID
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		var body v1AdminMachineCreateRequest
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_json", "invalid JSON body")
			return
		}
		rawSiteID := strings.TrimSpace(body.SiteID)
		if rawSiteID == "" {
			rawSiteID = strings.TrimSpace(body.SiteIDCamel)
		}
		siteID, err := uuid.Parse(rawSiteID)
		if err != nil || siteID == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_site_id", "invalid site_id")
			return
		}
		var hw *uuid.UUID
		if body.HardwareProfileID != nil && strings.TrimSpace(*body.HardwareProfileID) != "" {
			hid, perr := uuid.Parse(strings.TrimSpace(*body.HardwareProfileID))
			if perr != nil || hid == uuid.Nil {
				writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_hardware_profile_id", "invalid hardware_profile_id")
				return
			}
			hw = &hid
		}
		st := strings.TrimSpace(body.Status)
		if st == "" {
			st = "draft"
		}
		serial := body.SerialNumber
		if strings.TrimSpace(serial) == "" {
			serial = body.SerialNumberCamel
		}
		cabinetType := body.CabinetType
		if strings.TrimSpace(cabinetType) == "" {
			cabinetType = body.CabinetTypeCamel
		}
		m, err := f.CreateMachine(r.Context(), appfleet.CreateMachineInput{
			SiteID:            siteID,
			HardwareProfileID: hw,
			SerialNumber:      serial,
			Code:              body.Code,
			Model:             body.Model,
			CabinetType:       cabinetType,
			Timezone:          body.Timezone,
			Name:              body.Name,
			Status:            st,
		})
		if err != nil {
			writeFleetAppError(w, r.Context(), err)
			return
		}
		mid := m.ID.String()
		fleetAudit(r.Context(), app, scopeID, compliance.ActionMachineCreated, "fleet.machine", &mid, map[string]any{"serial": m.SerialNumber})
		writeJSON(w, http.StatusCreated, machineJSON(m))
	}
}

type v1AdminMachinePatchRequest struct {
	Name              *string `json:"name"`
	Status            *string `json:"status"`
	HardwareProfileID *string `json:"hardware_profile_id"`
	SiteID            *string `json:"site_id"`
	SiteIDCamel       *string `json:"siteId"`
	SerialNumber      *string `json:"serial_number"`
	SerialNumberCamel *string `json:"serialNumber"`
	Code              *string `json:"code"`
	Model             *string `json:"model"`
	CabinetType       *string `json:"cabinet_type"`
	CabinetTypeCamel  *string `json:"cabinetType"`
	Timezone          *string `json:"timezone"`
}

func serveAdminMachinePatch(app *api.HTTPApplication, f *appfleet.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		scopeID, err := parseAdminFleetCompanyScope(r)
		_ = scopeID
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		mid, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "machineId")))
		if err != nil || mid == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_machine_id", "invalid machineId")
			return
		}
		var body v1AdminMachinePatchRequest
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_json", "invalid JSON body")
			return
		}
		var hw *uuid.UUID
		if body.HardwareProfileID != nil && strings.TrimSpace(*body.HardwareProfileID) != "" {
			hid, perr := uuid.Parse(strings.TrimSpace(*body.HardwareProfileID))
			if perr != nil || hid == uuid.Nil {
				writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_hardware_profile_id", "invalid hardware_profile_id")
				return
			}
			hw = &hid
		}
		var sid *uuid.UUID
		rawSite := body.SiteID
		if rawSite == nil {
			rawSite = body.SiteIDCamel
		}
		if rawSite != nil {
			parsed, perr := uuid.Parse(strings.TrimSpace(*rawSite))
			if perr != nil || parsed == uuid.Nil {
				writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_site_id", "invalid site_id")
				return
			}
			sid = &parsed
		}
		serial := body.SerialNumber
		if serial == nil {
			serial = body.SerialNumberCamel
		}
		cabinetType := body.CabinetType
		if cabinetType == nil {
			cabinetType = body.CabinetTypeCamel
		}
		m, err := f.UpdateMachineMetadata(r.Context(), appfleet.UpdateMachineMetadataInput{
			MachineID:         mid,
			Name:              body.Name,
			Status:            body.Status,
			HardwareProfileID: hw,
			SiteID:            sid,
			SerialNumber:      serial,
			Code:              body.Code,
			Model:             body.Model,
			CabinetType:       cabinetType,
			Timezone:          body.Timezone,
		})
		if err != nil {
			writeFleetAppError(w, r.Context(), err)
			return
		}
		midStr := m.ID.String()
		fleetAudit(r.Context(), app, scopeID, compliance.ActionMachineUpdated, "fleet.machine", &midStr, nil)
		writeJSON(w, http.StatusOK, machineJSON(m))
	}
}

func serveAdminMachineDisable(app *api.HTTPApplication, f *appfleet.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		scopeID, err := parseAdminFleetCompanyScope(r)
		_ = scopeID
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		mid, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "machineId")))
		if err != nil || mid == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_machine_id", "invalid machineId")
			return
		}
		in, techOrigin, err := parseLifecycleMutation(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_body", err.Error())
			return
		}
		if err := appfleet.ValidateLifecycleMutation("suspend", in, techOrigin); err != nil {
			writeLifecycleMutationError(w, r.Context(), err)
			return
		}
		out, err := f.DisableMachine(r.Context(), scopeID, mid, in)
		if err != nil {
			writeLifecycleMutationError(w, r.Context(), err)
			return
		}
		recordLifecycleAuditAndAttribution(r.Context(), app, scopeID, compliance.ActionMachineDisabled, mid, in, out)
		writeJSON(w, http.StatusOK, lifecycleMutationJSON(mid, out))
	}
}

func serveAdminMachineEnable(app *api.HTTPApplication, f *appfleet.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		scopeID, err := parseAdminFleetCompanyScope(r)
		_ = scopeID
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		mid, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "machineId")))
		if err != nil || mid == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_machine_id", "invalid machineId")
			return
		}
		in, techOrigin, err := parseLifecycleMutation(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_body", err.Error())
			return
		}
		_ = techOrigin
		out, err := f.EnableMachine(r.Context(), scopeID, mid, in)
		if err != nil {
			writeLifecycleMutationError(w, r.Context(), err)
			return
		}
		recordLifecycleAuditAndAttribution(r.Context(), app, scopeID, compliance.ActionMachineEnabled, mid, in, out)
		writeJSON(w, http.StatusOK, lifecycleMutationJSON(mid, out))
	}
}

func serveAdminMachineRetire(app *api.HTTPApplication, f *appfleet.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		scopeID, err := parseAdminFleetCompanyScope(r)
		_ = scopeID
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		mid, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "machineId")))
		if err != nil || mid == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_machine_id", "invalid machineId")
			return
		}
		in, techOrigin, err := parseLifecycleMutation(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_body", err.Error())
			return
		}
		if err := appfleet.ValidateLifecycleMutation("archive", in, techOrigin); err != nil {
			writeLifecycleMutationError(w, r.Context(), err)
			return
		}
		out, err := f.RetireMachine(r.Context(), scopeID, mid, in)
		if err != nil {
			writeLifecycleMutationError(w, r.Context(), err)
			return
		}
		recordLifecycleAuditAndAttribution(r.Context(), app, scopeID, compliance.ActionMachineRetired, mid, in, out)
		writeJSON(w, http.StatusOK, lifecycleMutationJSON(mid, out))
	}
}

func serveAdminMachineCompromised(app *api.HTTPApplication, f *appfleet.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		scopeID, err := parseAdminFleetCompanyScope(r)
		_ = scopeID
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		mid, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "machineId")))
		if err != nil || mid == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_machine_id", "invalid machineId")
			return
		}
		in, techOrigin, err := parseLifecycleMutation(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_body", err.Error())
			return
		}
		if err := appfleet.ValidateLifecycleMutation("mark-compromised", in, techOrigin); err != nil {
			writeLifecycleMutationError(w, r.Context(), err)
			return
		}
		out, err := f.MarkMachineCompromised(r.Context(), scopeID, mid, in)
		if err != nil {
			writeLifecycleMutationError(w, r.Context(), err)
			return
		}
		recordLifecycleAuditAndAttribution(r.Context(), app, scopeID, compliance.ActionMachineCompromised, mid, in, out)
		writeJSON(w, http.StatusOK, lifecycleMutationJSON(mid, out))
	}
}

func serveAdminMachineRotateCredential(app *api.HTTPApplication, f *appfleet.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		scopeID, err := parseAdminFleetCompanyScope(r)
		_ = scopeID
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		mid, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "machineId")))
		if err != nil || mid == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_machine_id", "invalid machineId")
			return
		}
		in, techOrigin, err := parseLifecycleMutation(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_body", err.Error())
			return
		}
		if err := appfleet.ValidateLifecycleMutation("rotate-credentials", in, techOrigin); err != nil {
			writeLifecycleMutationError(w, r.Context(), err)
			return
		}
		out, err := f.RotateMachineCredential(r.Context(), scopeID, mid, in)
		if err != nil {
			writeLifecycleMutationError(w, r.Context(), err)
			return
		}
		recordLifecycleAuditAndAttribution(r.Context(), app, scopeID, compliance.ActionMachineCredRotated, mid, in, out)
		writeJSON(w, http.StatusOK, lifecycleMutationJSON(mid, out))
	}
}

type v1AdminMachineTransferSiteRequest struct {
	SiteID      string `json:"site_id"`
	SiteIDCamel string `json:"siteId"`
}

func serveAdminMachineTransferSite(app *api.HTTPApplication, f *appfleet.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if app == nil || app.AdminMachines == nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", "application not configured")
			return
		}
		scopeID, err := parseAdminFleetCompanyScope(r)
		_ = scopeID
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		mid, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "machineId")))
		if err != nil || mid == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_machine_id", "invalid machineId")
			return
		}
		beforeDTO, err := app.AdminMachines.GetMachine(r.Context(), scopeID, mid)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				writeAPIError(w, r.Context(), http.StatusNotFound, "not_found", "machine not found")
				return
			}
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		fromSite := strings.TrimSpace(beforeDTO.SiteID)
		var body v1AdminMachineTransferSiteRequest
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_json", "invalid JSON body")
			return
		}
		raw := strings.TrimSpace(body.SiteID)
		if raw == "" {
			raw = strings.TrimSpace(body.SiteIDCamel)
		}
		newSiteID, err := uuid.Parse(raw)
		if err != nil || newSiteID == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_site_id", "site_id is required")
			return
		}
		sid := newSiteID
		m, err := f.UpdateMachineMetadata(r.Context(), appfleet.UpdateMachineMetadataInput{
			MachineID: mid,
			SiteID:    &sid,
		})
		if err != nil {
			writeFleetAppError(w, r.Context(), err)
			return
		}
		midStr := m.ID.String()
		fleetAudit(r.Context(), app, scopeID, compliance.ActionMachineSiteTransferred, "fleet.machine", &midStr, map[string]any{
			"from_site_id": fromSite,
			"to_site_id":   m.SiteID.String(),
		})
		writeJSON(w, http.StatusOK, machineJSON(m))
	}
}

func serveAdminMachineRevokeSessions(app *api.HTTPApplication, f *appfleet.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		scopeID, err := parseAdminFleetCompanyScope(r)
		_ = scopeID
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		mid, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "machineId")))
		if err != nil || mid == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_machine_id", "invalid machineId")
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
		out, err := f.RevokeMachineSessions(r.Context(), scopeID, mid, in)
		if err != nil {
			writeLifecycleMutationError(w, r.Context(), err)
			return
		}
		recordLifecycleAuditAndAttribution(r.Context(), app, scopeID, compliance.ActionMachineSessionsRevoked, mid, in, out)
		writeJSON(w, http.StatusOK, lifecycleMutationJSON(mid, out))
	}
}

func serveAdminMachineRevokeCredential(app *api.HTTPApplication, f *appfleet.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		scopeID, err := parseAdminFleetCompanyScope(r)
		_ = scopeID
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		mid, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "machineId")))
		if err != nil || mid == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_machine_id", "invalid machineId")
			return
		}
		in, techOrigin, err := parseLifecycleMutation(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_body", err.Error())
			return
		}
		if err := appfleet.ValidateLifecycleMutation("revoke-credentials", in, techOrigin); err != nil {
			writeLifecycleMutationError(w, r.Context(), err)
			return
		}
		out, err := f.RevokeMachineCredential(r.Context(), scopeID, mid, in)
		if err != nil {
			writeLifecycleMutationError(w, r.Context(), err)
			return
		}
		recordLifecycleAuditAndAttribution(r.Context(), app, scopeID, compliance.ActionMachineCredentialRevoked, mid, in, out)
		writeJSON(w, http.StatusOK, lifecycleMutationJSON(mid, out))
	}
}

type v1AdminTechnicianCreateRequest struct {
	DisplayName     string `json:"display_name"`
	Email           string `json:"email"`
	Phone           string `json:"phone"`
	ExternalSubject string `json:"external_subject"`
}

func serveAdminTechnicianCreate(app *api.HTTPApplication, f *appfleet.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		scopeID, err := parseAdminFleetCompanyScope(r)
		_ = scopeID
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		var body v1AdminTechnicianCreateRequest
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_json", "invalid JSON body")
			return
		}
		t, err := f.CreateTechnician(r.Context(), appfleet.CreateTechnicianInput{
			DisplayName:     body.DisplayName,
			Email:           body.Email,
			Phone:           body.Phone,
			ExternalSubject: body.ExternalSubject,
		})
		if err != nil {
			writeFleetAppError(w, r.Context(), err)
			return
		}
		tid := t.ID.String()
		fleetAudit(r.Context(), app, scopeID, compliance.ActionTechnicianCreated, "fleet.technician", &tid, nil)
		writeJSON(w, http.StatusCreated, technicianJSON(t))
	}
}

func serveAdminTechnicianGet(app *api.HTTPApplication, f *appfleet.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		scopeID, err := parseAdminFleetCompanyScope(r)
		_ = scopeID
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		tid, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "technicianId")))
		if err != nil || tid == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_technician_id", "invalid technicianId")
			return
		}
		t, err := f.GetTechnician(r.Context(), scopeID, tid)
		if err != nil {
			writeFleetAppError(w, r.Context(), err)
			return
		}
		writeJSON(w, http.StatusOK, technicianJSON(t))
	}
}

type v1AdminTechnicianPatchRequest struct {
	DisplayName     *string `json:"display_name"`
	Email           *string `json:"email"`
	Phone           *string `json:"phone"`
	ExternalSubject *string `json:"external_subject"`
}

func serveAdminTechnicianPatch(app *api.HTTPApplication, f *appfleet.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		scopeID, err := parseAdminFleetCompanyScope(r)
		_ = scopeID
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		tid, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "technicianId")))
		if err != nil || tid == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_technician_id", "invalid technicianId")
			return
		}
		var body v1AdminTechnicianPatchRequest
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_json", "invalid JSON body")
			return
		}
		t, err := f.UpdateTechnician(r.Context(), appfleet.UpdateTechnicianInput{
			TechnicianID:    tid,
			DisplayName:     body.DisplayName,
			Email:           body.Email,
			Phone:           body.Phone,
			ExternalSubject: body.ExternalSubject,
		})
		if err != nil {
			writeFleetAppError(w, r.Context(), err)
			return
		}
		tidStr := t.ID.String()
		fleetAudit(r.Context(), app, scopeID, compliance.ActionTechnicianUpdated, "fleet.technician", &tidStr, nil)
		writeJSON(w, http.StatusOK, technicianJSON(t))
	}
}

func serveAdminTechnicianDisable(app *api.HTTPApplication, f *appfleet.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		scopeID, err := parseAdminFleetCompanyScope(r)
		_ = scopeID
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		tid, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "technicianId")))
		if err != nil || tid == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_technician_id", "invalid technicianId")
			return
		}
		t, err := f.DisableTechnician(r.Context(), scopeID, tid)
		if err != nil {
			writeFleetAppError(w, r.Context(), err)
			return
		}
		tidStr := t.ID.String()
		fleetAudit(r.Context(), app, scopeID, compliance.ActionTechnicianDisabled, "fleet.technician", &tidStr, nil)
		writeJSON(w, http.StatusOK, technicianJSON(t))
	}
}

func serveAdminTechnicianEnable(app *api.HTTPApplication, f *appfleet.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		scopeID, err := parseAdminFleetCompanyScope(r)
		_ = scopeID
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		tid, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "technicianId")))
		if err != nil || tid == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_technician_id", "invalid technicianId")
			return
		}
		t, err := f.EnableTechnician(r.Context(), scopeID, tid)
		if err != nil {
			writeFleetAppError(w, r.Context(), err)
			return
		}
		tidStr := t.ID.String()
		fleetAudit(r.Context(), app, scopeID, compliance.ActionTechnicianEnabled, "fleet.technician", &tidStr, nil)
		writeJSON(w, http.StatusOK, technicianJSON(t))
	}
}

type v1AdminAssignmentCreateRequest struct {
	TechnicianID string `json:"technician_id"`
	MachineID    string `json:"machine_id"`
	Role         string `json:"role"`
}

func serveAdminAssignmentCreate(app *api.HTTPApplication, f *appfleet.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		scopeID, err := parseAdminFleetCompanyScope(r)
		_ = scopeID
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		var body v1AdminAssignmentCreateRequest
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_json", "invalid JSON body")
			return
		}
		tid, err := uuid.Parse(strings.TrimSpace(body.TechnicianID))
		if err != nil || tid == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_technician_id", "invalid technician_id")
			return
		}
		mid, err := uuid.Parse(strings.TrimSpace(body.MachineID))
		if err != nil || mid == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_machine_id", "invalid machine_id")
			return
		}
		var actorTech uuid.UUID
		if p, ok := auth.PrincipalFromContext(r.Context()); ok && p.TechnicianID != uuid.Nil {
			actorTech = p.TechnicianID
		}
		a, err := f.AssignTechnicianToMachine(r.Context(), appfleet.AssignTechnicianInput{
			TechnicianID:      tid,
			MachineID:         mid,
			Role:              body.Role,
			ActorTechnicianID: actorTech,
		})
		if err != nil {
			writeFleetAppError(w, r.Context(), err)
			return
		}
		aid := a.ID.String()
		fleetAudit(r.Context(), app, scopeID, compliance.ActionTechnicianAssignmentCreated, "fleet.technician_assignment", &aid, map[string]any{
			"technician_id": tid.String(),
			"machine_id":    mid.String(),
		})
		writeJSON(w, http.StatusCreated, assignmentJSON(a))
	}
}

func serveAdminMachineTechniciansList(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if app == nil || app.AdminAssignments == nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", "application not configured")
			return
		}
		scope, err := parseAdminFleetListScope(r)
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		mid, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "machineId")))
		if err != nil || mid == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_machine_id", "invalid machineId")
			return
		}
		scope.MachineID = &mid
		out, err := app.AdminAssignments.ListAssignments(r.Context(), scope)
		writeV1Collection(w, r.Context(), out, err)
	}
}

type v1AdminMachineTechnicianAssignRequest struct {
	UserID           string `json:"userId"`
	TechnicianID     string `json:"technician_id"`
	Role             string `json:"role"`
	AssignmentDomain string `json:"assignment_domain"`
}

func serveAdminMachineTechnicianAssign(app *api.HTTPApplication, f *appfleet.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		scopeID, err := parseAdminFleetCompanyScope(r)
		_ = scopeID
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		mid, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "machineId")))
		if err != nil || mid == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_machine_id", "invalid machineId")
			return
		}
		var body v1AdminMachineTechnicianAssignRequest
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_json", "invalid JSON body")
			return
		}
		rawUserID := strings.TrimSpace(body.UserID)
		if rawUserID == "" {
			rawUserID = strings.TrimSpace(body.TechnicianID)
		}
		uid, err := uuid.Parse(rawUserID)
		if err != nil || uid == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_user_id", "invalid userId")
			return
		}
		actor, _ := principalAccountID(r)
		actorTech := uuid.Nil
		if p, ok := auth.PrincipalFromContext(r.Context()); ok && p.TechnicianID != uuid.Nil {
			actorTech = p.TechnicianID
		}
		a, err := f.AssignTechnicianToMachine(r.Context(), appfleet.AssignTechnicianInput{
			TechnicianID:      uid,
			MachineID:         mid,
			Role:              body.Role,
			AssignmentDomain:  body.AssignmentDomain,
			CreatedBy:         stringPtrUUID(actor),
			ActorTechnicianID: actorTech,
		})
		if err != nil {
			writeFleetAppError(w, r.Context(), err)
			return
		}
		aid := a.ID.String()
		fleetAudit(r.Context(), app, scopeID, compliance.ActionTechnicianAssignmentCreated, "fleet.technician_assignment", &aid, map[string]any{
			"user_id":    uid.String(),
			"machine_id": mid.String(),
		})
		writeJSON(w, http.StatusCreated, assignmentJSON(a))
	}
}

func stringPtrUUID(id uuid.UUID) *uuid.UUID {
	if id == uuid.Nil {
		return nil
	}
	return &id
}

func serveAdminMachineTechnicianRemove(app *api.HTTPApplication, f *appfleet.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		scopeID, err := parseAdminFleetCompanyScope(r)
		_ = scopeID
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		mid, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "machineId")))
		if err != nil || mid == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_machine_id", "invalid machineId")
			return
		}
		uid, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "userId")))
		if err != nil || uid == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_user_id", "invalid userId")
			return
		}
		a, err := f.ReleaseTechnicianAssignmentForMachineUser(r.Context(), scopeID, mid, uid)
		if err != nil {
			writeFleetAppError(w, r.Context(), err)
			return
		}
		aid := a.ID.String()
		fleetAudit(r.Context(), app, scopeID, compliance.ActionTechnicianAssignmentReleased, "fleet.technician_assignment", &aid, map[string]any{
			"user_id":    uid.String(),
			"machine_id": mid.String(),
		})
		w.WriteHeader(http.StatusNoContent)
	}
}

func serveAdminTechnicianAssignmentsList(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if app == nil || app.AdminAssignments == nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", "application not configured")
			return
		}
		scope, err := parseAdminFleetListScope(r)
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		out, err := app.AdminAssignments.ListAssignments(r.Context(), scope)
		writeV1Collection(w, r.Context(), out, err)
	}
}

func serveAdminAssignmentGet(app *api.HTTPApplication, f *appfleet.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		scopeID, err := parseAdminFleetCompanyScope(r)
		_ = scopeID
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		aid, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "assignmentId")))
		if err != nil || aid == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_assignment_id", "invalid assignmentId")
			return
		}
		a, err := f.GetTechnicianAssignment(r.Context(), scopeID, aid)
		if err != nil {
			writeFleetAppError(w, r.Context(), err)
			return
		}
		writeJSON(w, http.StatusOK, assignmentJSON(a))
	}
}

type v1AdminAssignmentPatchRequest struct {
	Role    *string `json:"role"`
	ValidTo *string `json:"valid_to"`
	Status  *string `json:"status"`
}

func serveAdminAssignmentPatch(app *api.HTTPApplication, f *appfleet.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		scopeID, err := parseAdminFleetCompanyScope(r)
		_ = scopeID
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		aid, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "assignmentId")))
		if err != nil || aid == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_assignment_id", "invalid assignmentId")
			return
		}
		var body v1AdminAssignmentPatchRequest
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_json", "invalid JSON body")
			return
		}
		var vto *time.Time
		if body.ValidTo != nil && strings.TrimSpace(*body.ValidTo) != "" {
			t, perr := time.Parse(time.RFC3339Nano, strings.TrimSpace(*body.ValidTo))
			if perr != nil {
				t, perr = time.Parse(time.RFC3339, strings.TrimSpace(*body.ValidTo))
			}
			if perr != nil {
				writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_valid_to", "valid_to must be RFC3339")
				return
			}
			utc := t.UTC()
			vto = &utc
		}
		a, err := f.UpdateTechnicianAssignment(r.Context(), appfleet.UpdateAssignmentHTTPInput{
			AssignmentID: aid,
			Role:         body.Role,
			ValidTo:      vto,
			Status:       body.Status,
		})
		if err != nil {
			writeFleetAppError(w, r.Context(), err)
			return
		}
		aidStr := a.ID.String()
		fleetAudit(r.Context(), app, scopeID, compliance.ActionTechnicianAssignmentUpdated, "fleet.technician_assignment", &aidStr, nil)
		writeJSON(w, http.StatusOK, assignmentJSON(a))
	}
}

func serveAdminAssignmentRelease(app *api.HTTPApplication, f *appfleet.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		scopeID, err := parseAdminFleetCompanyScope(r)
		_ = scopeID
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		aid, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "assignmentId")))
		if err != nil || aid == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_assignment_id", "invalid assignmentId")
			return
		}
		a, err := f.ReleaseTechnicianAssignment(r.Context(), scopeID, aid)
		if err != nil {
			writeFleetAppError(w, r.Context(), err)
			return
		}
		aidStr := a.ID.String()
		fleetAudit(r.Context(), app, scopeID, compliance.ActionTechnicianAssignmentReleased, "fleet.technician_assignment", &aidStr, nil)
		writeJSON(w, http.StatusOK, assignmentJSON(a))
	}
}

func serveAdminAssignmentCancel(app *api.HTTPApplication, f *appfleet.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		scopeID, err := parseAdminFleetCompanyScope(r)
		_ = scopeID
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		aid, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "assignmentId")))
		if err != nil || aid == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_assignment_id", "invalid assignmentId")
			return
		}
		a, err := f.ReleaseTechnicianAssignment(r.Context(), scopeID, aid)
		if err != nil {
			writeFleetAppError(w, r.Context(), err)
			return
		}
		aidStr := a.ID.String()
		fleetAudit(r.Context(), app, scopeID, compliance.ActionTechnicianAssignmentCanceled, "fleet.technician_assignment", &aidStr, nil)
		writeJSON(w, http.StatusOK, assignmentJSON(a))
	}
}
