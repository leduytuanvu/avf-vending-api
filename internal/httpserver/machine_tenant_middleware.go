package httpserver

import (
	"net/http"
	"strings"

	"github.com/avf/avf-vending-api/internal/app/api"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// RequireMachineTenantAccess enforces DB-backed tenant binding for a machine URL parameter.
// platform_admin bypasses org checks; others must match machines.organization_id or hold an explicit machine allow-list
// entry that matches the resolved row.
func RequireMachineTenantAccess(app *api.HTTPApplication, machineParam string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if app == nil || app.TelemetryStore == nil || app.TelemetryStore.Pool() == nil {
				writeAPIError(w, r.Context(), http.StatusServiceUnavailable, "misconfigured", "database not configured")
				return
			}
			p, ok := auth.PrincipalFromContext(r.Context())
			if !ok {
				writeAPIError(w, r.Context(), http.StatusUnauthorized, "unauthenticated", auth.ErrUnauthenticated.Error())
				return
			}
			raw := chi.URLParam(r, machineParam)
			machineID, err := uuid.Parse(strings.TrimSpace(raw))
			if err != nil || machineID == uuid.Nil {
				writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_machine_id", "invalid machine id")
				return
			}
			if p.HasRole(auth.RolePlatformAdmin) {
				next.ServeHTTP(w, r)
				return
			}
			q := db.New(app.TelemetryStore.Pool())
			machineOrg, err := q.GetMachineOrganizationID(r.Context(), machineID)
			if err != nil {
				if err == pgx.ErrNoRows {
					writeAPIError(w, r.Context(), http.StatusForbidden, "forbidden", auth.ErrForbidden.Error())
					return
				}
				writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
				return
			}
			if p.HasOrganization() && p.OrganizationID != machineOrg {
				writeAPIError(w, r.Context(), http.StatusForbidden, "forbidden", auth.ErrForbidden.Error())
				return
			}
			if p.AllowsMachine(machineID) {
				next.ServeHTTP(w, r)
				return
			}
			if p.HasOrganization() && p.OrganizationID == machineOrg &&
				p.HasAnyRole(auth.RoleOrgAdmin, auth.RoleOrgMember, auth.RoleTechnician) {
				next.ServeHTTP(w, r)
				return
			}
			writeAPIError(w, r.Context(), http.StatusForbidden, "forbidden", auth.ErrForbidden.Error())
		})
	}
}
