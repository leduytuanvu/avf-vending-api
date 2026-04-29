package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/avf/avf-vending-api/internal/app/api"
	approllout "github.com/avf/avf-vending-api/internal/app/rollout"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func mountAdminRolloutRoutes(r chi.Router, app *api.HTTPApplication, writeRL func(http.Handler) http.Handler) {
	if app == nil || app.Rollout == nil {
		return
	}
	if writeRL == nil {
		writeRL = func(h http.Handler) http.Handler { return h }
	}

	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAnyPermission(auth.PermFleetRead))
		r.Get("/rollouts", serveAdminRolloutList(app))
		r.Get("/rollouts/{rolloutId}", serveAdminRolloutGet(app))
	})

	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAnyPermission(auth.PermFleetWrite))
		r.With(writeRL).Post("/rollouts", serveAdminRolloutCreate(app))
		r.With(writeRL).Post("/rollouts/{rolloutId}/start", serveAdminRolloutStart(app))
		r.With(writeRL).Post("/rollouts/{rolloutId}/pause", serveAdminRolloutPause(app))
		r.With(writeRL).Post("/rollouts/{rolloutId}/resume", serveAdminRolloutResume(app))
		r.With(writeRL).Post("/rollouts/{rolloutId}/cancel", serveAdminRolloutCancel(app))
		r.With(writeRL).Post("/rollouts/{rolloutId}/rollback", serveAdminRolloutRollback(app))
	})
}

type v1AdminRolloutCreateRequest struct {
	RolloutType   string          `json:"rolloutType"`
	TargetVersion string          `json:"targetVersion"`
	Strategy      json.RawMessage `json:"strategy"`
}

func serveAdminRolloutCreate(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, err := parseAdminFleetOrganizationScope(r)
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		var body v1AdminRolloutCreateRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_json", "request body must be JSON")
			return
		}
		st := []byte("{}")
		if len(body.Strategy) > 0 {
			st = body.Strategy
		}
		var cb pgtype.UUID
		if u, ok := parseInteractiveActorUUID(r); ok {
			cb = pgtype.UUID{Bytes: *u, Valid: true}
		}
		row, err := app.Rollout.CreateCampaign(r.Context(), orgID, body.RolloutType, body.TargetVersion, st, cb)
		if err != nil {
			writeRolloutError(w, r.Context(), err)
			return
		}
		writeJSON(w, http.StatusCreated, encodeRolloutCampaign(row))
	}
}

func serveAdminRolloutList(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, err := parseAdminFleetOrganizationScope(r)
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		limit, offset, err := parseAdminLimitOffset(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_query", "limit and offset must be valid integers")
			return
		}
		rows, err := app.Rollout.ListCampaigns(r.Context(), orgID, limit, offset)
		if err != nil {
			writeRolloutError(w, r.Context(), err)
			return
		}
		items := make([]map[string]any, 0, len(rows))
		for _, row := range rows {
			items = append(items, encodeRolloutCampaign(row))
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"items": items,
			"meta": map[string]any{
				"limit":    limit,
				"offset":   offset,
				"returned": len(items),
			},
		})
	}
}

func serveAdminRolloutGet(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, err := parseAdminFleetOrganizationScope(r)
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		rid, ok := parseChiUUID(w, r, "rolloutId")
		if !ok {
			return
		}
		camp, targets, err := app.Rollout.GetCampaign(r.Context(), orgID, rid)
		if err != nil {
			writeRolloutError(w, r.Context(), err)
			return
		}
		tgtEnc := make([]map[string]any, 0, len(targets))
		for _, t := range targets {
			tgtEnc = append(tgtEnc, encodeRolloutTarget(t))
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"campaign": encodeRolloutCampaign(camp),
			"targets":  tgtEnc,
		})
	}
}

func serveAdminRolloutStart(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, err := parseAdminFleetOrganizationScope(r)
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		rid, ok := parseChiUUID(w, r, "rolloutId")
		if !ok {
			return
		}
		if err := app.Rollout.Start(r.Context(), orgID, rid); err != nil {
			writeRolloutError(w, r.Context(), err)
			return
		}
		camp, targets, err := app.Rollout.GetCampaign(r.Context(), orgID, rid)
		if err != nil {
			writeRolloutError(w, r.Context(), err)
			return
		}
		tgtEnc := make([]map[string]any, 0, len(targets))
		for _, t := range targets {
			tgtEnc = append(tgtEnc, encodeRolloutTarget(t))
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"campaign": encodeRolloutCampaign(camp),
			"targets":  tgtEnc,
		})
	}
}

func serveAdminRolloutPause(app *api.HTTPApplication) http.HandlerFunc {
	return rolloutAction(app, func(ctx context.Context, org, rid uuid.UUID) error {
		return app.Rollout.Pause(ctx, org, rid)
	})
}

func serveAdminRolloutResume(app *api.HTTPApplication) http.HandlerFunc {
	return rolloutAction(app, func(ctx context.Context, org, rid uuid.UUID) error {
		return app.Rollout.Resume(ctx, org, rid)
	})
}

func serveAdminRolloutCancel(app *api.HTTPApplication) http.HandlerFunc {
	return rolloutAction(app, func(ctx context.Context, org, rid uuid.UUID) error {
		return app.Rollout.Cancel(ctx, org, rid)
	})
}

func serveAdminRolloutRollback(app *api.HTTPApplication) http.HandlerFunc {
	return rolloutAction(app, func(ctx context.Context, org, rid uuid.UUID) error {
		return app.Rollout.Rollback(ctx, org, rid)
	})
}

func rolloutAction(app *api.HTTPApplication, fn func(context.Context, uuid.UUID, uuid.UUID) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, err := parseAdminFleetOrganizationScope(r)
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		rid, ok := parseChiUUID(w, r, "rolloutId")
		if !ok {
			return
		}
		if err := fn(r.Context(), orgID, rid); err != nil {
			writeRolloutError(w, r.Context(), err)
			return
		}
		camp, targets, err := app.Rollout.GetCampaign(r.Context(), orgID, rid)
		if err != nil {
			writeRolloutError(w, r.Context(), err)
			return
		}
		tgtEnc := make([]map[string]any, 0, len(targets))
		for _, t := range targets {
			tgtEnc = append(tgtEnc, encodeRolloutTarget(t))
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"campaign": encodeRolloutCampaign(camp),
			"targets":  tgtEnc,
		})
	}
}

func encodeRolloutCampaign(b db.RolloutCampaign) map[string]any {
	out := map[string]any{
		"id":             b.ID.String(),
		"organizationId": b.OrganizationID.String(),
		"rolloutType":    b.RolloutType,
		"targetVersion":  b.TargetVersion,
		"status":         b.Status,
		"createdAt":      formatAPITimeRFC3339Nano(b.CreatedAt),
		"updatedAt":      formatAPITimeRFC3339Nano(b.UpdatedAt),
	}
	if len(b.Strategy) > 0 {
		out["strategy"] = json.RawMessage(b.Strategy)
	} else {
		out["strategy"] = json.RawMessage([]byte("{}"))
	}
	if b.CreatedBy.Valid {
		out["createdBy"] = uuid.UUID(b.CreatedBy.Bytes).String()
	}
	if b.StartedAt.Valid {
		out["startedAt"] = formatAPITimeRFC3339Nano(b.StartedAt.Time)
	}
	if b.CompletedAt.Valid {
		out["completedAt"] = formatAPITimeRFC3339Nano(b.CompletedAt.Time)
	}
	if b.CancelledAt.Valid {
		out["cancelledAt"] = formatAPITimeRFC3339Nano(b.CancelledAt.Time)
	}
	return out
}

func encodeRolloutTarget(t db.RolloutTarget) map[string]any {
	out := map[string]any{
		"id":             t.ID.String(),
		"organizationId": t.OrganizationID.String(),
		"campaignId":     t.CampaignID.String(),
		"machineId":      t.MachineID.String(),
		"status":         t.Status,
		"createdAt":      formatAPITimeRFC3339Nano(t.CreatedAt),
		"updatedAt":      formatAPITimeRFC3339Nano(t.UpdatedAt),
	}
	if t.ErrMessage.Valid {
		out["error"] = t.ErrMessage.String
	}
	if t.CommandID.Valid {
		out["commandId"] = uuid.UUID(t.CommandID.Bytes).String()
	}
	return out
}

func writeRolloutError(w http.ResponseWriter, ctx context.Context, err error) {
	switch {
	case errors.Is(err, approllout.ErrNotFound):
		writeAPIError(w, ctx, http.StatusNotFound, "not_found", err.Error())
	case errors.Is(err, approllout.ErrInvalidArgument):
		writeAPIError(w, ctx, http.StatusBadRequest, "invalid_argument", err.Error())
	case errors.Is(err, approllout.ErrForbiddenState):
		writeAPIError(w, ctx, http.StatusConflict, "invalid_state", err.Error())
	default:
		writeAPIError(w, ctx, http.StatusInternalServerError, "internal", err.Error())
	}
}
