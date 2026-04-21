package httpserver

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/avf/avf-vending-api/internal/app/api"
	"github.com/avf/avf-vending-api/internal/app/listscope"
	"github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// parseAdminFleetOrganizationScope resolves the tenant organization for org_admin vs platform_admin (organization_id query).
func parseAdminFleetOrganizationScope(r *http.Request) (uuid.UUID, error) {
	p, ok := auth.PrincipalFromContext(r.Context())
	if !ok {
		return uuid.Nil, listscope.ErrInvalidListQuery
	}
	q := r.URL.Query()
	var orgID uuid.UUID
	if p.HasRole(auth.RolePlatformAdmin) {
		raw := strings.TrimSpace(q.Get("organization_id"))
		id, perr := uuid.Parse(raw)
		if perr != nil || id == uuid.Nil {
			return uuid.Nil, api.ErrAdminTenantScopeRequired
		}
		orgID = id
	} else {
		if !p.HasOrganization() {
			return uuid.Nil, api.ErrAdminTenantScopeRequired
		}
		orgID = p.OrganizationID
		if raw := strings.TrimSpace(q.Get("organization_id")); raw != "" {
			qid, perr := uuid.Parse(raw)
			if perr != nil || qid != orgID {
				return uuid.Nil, listscope.ErrInvalidListQuery
			}
		}
	}
	return orgID, nil
}

func parseAdminFleetListScope(r *http.Request) (listscope.AdminFleet, error) {
	p, ok := auth.PrincipalFromContext(r.Context())
	if !ok {
		return listscope.AdminFleet{}, listscope.ErrInvalidListQuery
	}
	limit, offset, err := parseAdminLimitOffset(r)
	if err != nil {
		return listscope.AdminFleet{}, listscope.ErrInvalidListQuery
	}
	orgID, err := parseAdminFleetOrganizationScope(r)
	if err != nil {
		return listscope.AdminFleet{}, err
	}
	q := r.URL.Query()
	var siteID *uuid.UUID
	if raw := strings.TrimSpace(q.Get("site_id")); raw != "" {
		sid, perr := uuid.Parse(raw)
		if perr != nil || sid == uuid.Nil {
			return listscope.AdminFleet{}, listscope.ErrInvalidListQuery
		}
		siteID = &sid
	}
	var machineID *uuid.UUID
	if raw := strings.TrimSpace(q.Get("machine_id")); raw != "" {
		mid, perr := uuid.Parse(raw)
		if perr != nil || mid == uuid.Nil {
			return listscope.AdminFleet{}, listscope.ErrInvalidListQuery
		}
		machineID = &mid
	}
	var technicianID *uuid.UUID
	if raw := strings.TrimSpace(q.Get("technician_id")); raw != "" {
		tid, perr := uuid.Parse(raw)
		if perr != nil || tid == uuid.Nil {
			return listscope.AdminFleet{}, listscope.ErrInvalidListQuery
		}
		technicianID = &tid
	}
	var from *time.Time
	if raw := strings.TrimSpace(q.Get("from")); raw != "" {
		t, perr := time.Parse(time.RFC3339Nano, raw)
		if perr != nil {
			t, perr = time.Parse(time.RFC3339, raw)
		}
		if perr != nil {
			return listscope.AdminFleet{}, listscope.ErrInvalidListQuery
		}
		utc := t.UTC()
		from = &utc
	}
	var to *time.Time
	if raw := strings.TrimSpace(q.Get("to")); raw != "" {
		t, perr := time.Parse(time.RFC3339Nano, raw)
		if perr != nil {
			t, perr = time.Parse(time.RFC3339, raw)
		}
		if perr != nil {
			return listscope.AdminFleet{}, listscope.ErrInvalidListQuery
		}
		utc := t.UTC()
		to = &utc
	}
	return listscope.AdminFleet{
		IsPlatformAdmin: p.HasRole(auth.RolePlatformAdmin),
		OrganizationID:  orgID,
		SiteID:          siteID,
		MachineID:       machineID,
		TechnicianID:    technicianID,
		Status:          strings.TrimSpace(q.Get("status")),
		Search:          strings.TrimSpace(q.Get("search")),
		From:            from,
		To:              to,
		Limit:           limit,
		Offset:          offset,
	}, nil
}

func serveAdminMachineGet(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if app == nil || app.AdminMachines == nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", "application not configured")
			return
		}
		machineID, err := uuid.Parse(chi.URLParam(r, "machineId"))
		if err != nil || machineID == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_machine_id", "invalid machineId")
			return
		}
		orgID, err := parseAdminFleetOrganizationScope(r)
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		out, err := app.AdminMachines.GetMachine(r.Context(), orgID, machineID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				writeAPIError(w, r.Context(), http.StatusNotFound, "machine_not_found", "machine not found or not in organization")
				return
			}
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}
