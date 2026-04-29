package httpserver

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/avf/avf-vending-api/internal/app/api"
	appaudit "github.com/avf/avf-vending-api/internal/app/audit"
	"github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func mountAdminAuditRoutes(r chi.Router, app *api.HTTPApplication) {
	if app == nil || app.EnterpriseAudit == nil {
		return
	}
	svc := app.EnterpriseAudit
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAnyPermission(auth.PermAuditRead))
		r.Get("/audit/events", getAdminAuditEvents(svc))
		r.Get("/organizations/{organizationId}/audit-events", getAdminAuditEvents(svc))
		r.Get("/organizations/{organizationId}/audit-events/{auditEventId}", getAdminOrgAuditEventByID(svc))
	})
}

func getAdminAuditEvents(svc *appaudit.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, err := adminCatalogOrganizationID(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_scope", err.Error())
			return
		}
		q := r.URL.Query()
		action := strings.TrimSpace(q.Get("action"))
		actorID := strings.TrimSpace(q.Get("actorId"))
		if actorID == "" {
			actorID = strings.TrimSpace(q.Get("actor_id"))
		}
		resourceType := strings.TrimSpace(q.Get("resourceType"))
		if resourceType == "" {
			resourceType = strings.TrimSpace(q.Get("resource_type"))
		}
		resourceID := strings.TrimSpace(q.Get("resourceId"))
		if resourceID == "" {
			resourceID = strings.TrimSpace(q.Get("resource_id"))
		}
		actorType := strings.TrimSpace(q.Get("actorType"))
		if actorType == "" {
			actorType = strings.TrimSpace(q.Get("actor_type"))
		}
		outcome := strings.TrimSpace(q.Get("outcome"))
		limit, offset, err := parseAdminLimitOffset(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_pagination", err.Error())
			return
		}
		var from *time.Time
		if raw := strings.TrimSpace(q.Get("from")); raw != "" {
			t, perr := time.Parse(time.RFC3339Nano, raw)
			if perr != nil {
				t, perr = time.Parse(time.RFC3339, raw)
			}
			if perr != nil {
				writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_query", "from must be RFC3339 or RFC3339Nano")
				return
			}
			u := t.UTC()
			from = &u
		}
		var to *time.Time
		if raw := strings.TrimSpace(q.Get("to")); raw != "" {
			t, perr := time.Parse(time.RFC3339Nano, raw)
			if perr != nil {
				t, perr = time.Parse(time.RFC3339, raw)
			}
			if perr != nil {
				writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_query", "to must be RFC3339 or RFC3339Nano")
				return
			}
			u := t.UTC()
			to = &u
		}
		machineID := strings.TrimSpace(q.Get("machineId"))
		if machineID == "" {
			machineID = strings.TrimSpace(q.Get("machine_id"))
		}
		out, err := svc.ListEvents(r.Context(), appaudit.EventListParams{
			OrganizationID: orgID,
			Action:         action,
			ActorID:        actorID,
			ActorType:      actorType,
			Outcome:        outcome,
			ResourceType:   resourceType,
			ResourceID:     resourceID,
			MachineID:      machineID,
			From:           from,
			To:             to,
			Limit:          limit,
			Offset:         offset,
		})
		writeV1Collection(w, r.Context(), out, err)
	}
}

func getAdminOrgAuditEventByID(svc *appaudit.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, err := adminCatalogOrganizationID(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_scope", err.Error())
			return
		}
		rawID := strings.TrimSpace(chi.URLParam(r, "auditEventId"))
		if rawID == "" {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_path", "auditEventId is required")
			return
		}
		eventID, err := uuid.Parse(rawID)
		if err != nil || eventID == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_path", "auditEventId must be a UUID")
			return
		}
		out, err := svc.GetEventForOrg(r.Context(), orgID, eventID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				writeAPIError(w, r.Context(), http.StatusNotFound, "not_found", "audit event not found")
				return
			}
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}
