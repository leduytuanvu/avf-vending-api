package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/avf/avf-vending-api/internal/app/api"
	appretention "github.com/avf/avf-vending-api/internal/app/retention"
	"github.com/avf/avf-vending-api/internal/config"
	"github.com/avf/avf-vending-api/internal/domain/compliance"
	"github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// MountAdminRetentionSystemRoutes registers /v1/admin/system/retention/* (platform_admin; mutating routes rate-limited).
func MountAdminRetentionSystemRoutes(r chi.Router, app *api.HTTPApplication, cfg *config.Config, writeRL func(http.Handler) http.Handler) {
	if r == nil || app == nil || cfg == nil || app.TelemetryStore == nil {
		return
	}
	if writeRL == nil {
		writeRL = func(h http.Handler) http.Handler { return h }
	}
	svc := appretention.New(app.TelemetryStore.Pool())
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAnyRole(auth.RolePlatformAdmin))
		r.Get("/system/retention/stats", getAdminRetentionStats(svc, cfg))
		r.With(writeRL).Post("/system/retention/dry-run", postAdminRetentionDryRun(app, svc, cfg))
		r.With(writeRL).Post("/system/retention/run", postAdminRetentionRun(app, svc, cfg))
	})
}

func getAdminRetentionStats(svc *appretention.Service, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		out, err := svc.Stats(r.Context(), cfg)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "retention_stats_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func postAdminRetentionDryRun(app *api.HTTPApplication, svc *appretention.Service, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		out, err := svc.DryRun(r.Context(), cfg)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "retention_dry_run_failed", err.Error())
			return
		}
		orgID, aerr := resolveRetentionAuditOrg(r.Context(), app.TelemetryStore.Pool(), r)
		if aerr == nil {
			recordRetentionAuditEvent(r.Context(), app.EnterpriseAudit, orgID, compliance.ActionRetentionDryRun, out)
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func postAdminRetentionRun(app *api.HTTPApplication, svc *appretention.Service, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		out, err := svc.Run(r.Context(), cfg)
		if err != nil {
			if errors.Is(err, appretention.ErrDestructiveRetentionForbidden) {
				writeAPIError(w, r.Context(), http.StatusForbidden, "retention_run_forbidden",
					"destructive Postgres retention is disabled for this deployment; set APP_ENV=staging|production or RETENTION_ALLOW_DESTRUCTIVE_LOCAL=true for local deletes")
				return
			}
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "retention_run_failed", err.Error())
			return
		}
		orgID, aerr := resolveRetentionAuditOrg(r.Context(), app.TelemetryStore.Pool(), r)
		if aerr == nil {
			recordRetentionAuditEvent(r.Context(), app.EnterpriseAudit, orgID, compliance.ActionRetentionRun, out)
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func resolveRetentionAuditOrg(ctx context.Context, pool *pgxpool.Pool, r *http.Request) (uuid.UUID, error) {
	raw := strings.TrimSpace(r.URL.Query().Get("organization_id"))
	if raw != "" {
		id, err := uuid.Parse(raw)
		if err == nil {
			return id, nil
		}
	}
	if p, ok := auth.PrincipalFromContext(ctx); ok && p.OrganizationID != uuid.Nil {
		return p.OrganizationID, nil
	}
	var id uuid.UUID
	err := pool.QueryRow(ctx, `SELECT id FROM organizations ORDER BY created_at ASC LIMIT 1`).Scan(&id)
	if err != nil {
		return uuid.Nil, err
	}
	return id, nil
}

func recordRetentionAuditEvent(ctx context.Context, rec compliance.EnterpriseRecorder, orgID uuid.UUID, action string, outcome appretention.RunOutcome) {
	if rec == nil {
		return
	}
	at, aid := compliance.ActorUser, ""
	if p, ok := auth.PrincipalFromContext(ctx); ok {
		at, aid = p.Actor()
	}
	md, _ := json.Marshal(map[string]any{
		"overallDryRun":       outcome.OverallDryRun,
		"wouldModifyDatabase": outcome.WouldModifyDatabase,
		"telemetry":           outcome.Telemetry,
		"enterprise":          outcome.Enterprise,
	})
	rid := "system_retention"
	_ = rec.Record(ctx, compliance.EnterpriseAuditRecord{
		OrganizationID: orgID,
		ActorType:      at,
		ActorID:        stringPtrOrNil(aid),
		Action:         action,
		ResourceType:   "system_retention",
		ResourceID:     &rid,
		Metadata:       md,
	})
}
