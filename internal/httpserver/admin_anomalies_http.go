package httpserver

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/avf/avf-vending-api/internal/app/anomalies"
	"github.com/avf/avf-vending-api/internal/app/api"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func mountAdminAnomalyRoutes(r chi.Router, app *api.HTTPApplication, writeRL func(http.Handler) http.Handler) {
	if app == nil || app.Anomalies == nil {
		return
	}
	if writeRL == nil {
		writeRL = func(h http.Handler) http.Handler { return h }
	}

	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAnyPermission(auth.PermInventoryRead, auth.PermFleetRead, auth.PermTelemetryRead))
		r.Get("/anomalies", serveAdminOperationalAnomaliesList(app))
		r.Get("/anomalies/{anomalyId}", serveAdminOperationalAnomalyGet(app))
		r.Get("/restock/suggestions", serveAdminOrgRestockSuggestions(app))
	})

	r.Group(func(r chi.Router) {
		r.Use(auth.RequirePermission(auth.PermInventoryAdjust))
		r.With(writeRL).Post("/anomalies/{anomalyId}/resolve", serveAdminOperationalAnomalyResolve(app))
		r.With(writeRL).Post("/anomalies/{anomalyId}/ignore", serveAdminOperationalAnomalyIgnore(app))
	})
}

func serveAdminOperationalAnomaliesList(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, err := parseAdminFleetOrganizationScope(r)
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		limit, offset, err := parseAdminLimitOffset(r)
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		refresh := strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("refresh")), "true")
		rows, err := app.Anomalies.List(r.Context(), orgID, nil, int32(limit), int32(offset), refresh)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		items := make([]map[string]any, 0, len(rows))
		for _, row := range rows {
			items = append(items, inventoryAnomalyJSON(row))
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items})
	}
}

func serveAdminOperationalAnomalyGet(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, err := parseAdminFleetOrganizationScope(r)
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		anomalyID, ok := parseChiUUID(w, r, "anomalyId")
		if !ok {
			return
		}
		row, err := app.Anomalies.Get(r.Context(), orgID, anomalyID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				writeAPIError(w, r.Context(), http.StatusNotFound, "not_found", "anomaly not found")
				return
			}
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, operationalAnomalyDetailJSON(row))
	}
}

func operationalAnomalyDetailJSON(row db.AnomaliesGetByOrgAndIDRow) map[string]any {
	out := map[string]any{
		"id":                  row.ID.String(),
		"organizationId":      row.OrganizationID.String(),
		"machineId":           row.MachineID.String(),
		"machineName":         row.MachineName,
		"machineSerialNumber": row.MachineSerialNumber,
		"anomalyType":         row.AnomalyType,
		"status":              row.Status,
		"fingerprint":         row.Fingerprint,
		"detectedAt":          row.DetectedAt.UTC().Format(time.RFC3339Nano),
		"createdAt":           row.CreatedAt.UTC().Format(time.RFC3339Nano),
		"updatedAt":           row.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
	if row.SlotCode.Valid {
		out["slotCode"] = row.SlotCode.String
	}
	if row.ProductID.Valid {
		out["productId"] = uuid.UUID(row.ProductID.Bytes).String()
	}
	if len(row.Payload) > 0 {
		out["payload"] = json.RawMessage(row.Payload)
	}
	if row.ResolvedAt.Valid {
		out["resolvedAt"] = row.ResolvedAt.Time.UTC().Format(time.RFC3339Nano)
	}
	if row.ResolvedBy.Valid {
		out["resolvedBy"] = uuid.UUID(row.ResolvedBy.Bytes).String()
	}
	if row.ResolutionNote.Valid {
		out["resolutionNote"] = row.ResolutionNote.String
	}
	return out
}

func serveAdminOperationalAnomalyResolve(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, err := parseAdminFleetOrganizationScope(r)
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		anomalyID, ok := parseChiUUID(w, r, "anomalyId")
		if !ok {
			return
		}
		var body resolveAnomalyBody
		if r.Body != nil {
			_ = json.NewDecoder(r.Body).Decode(&body)
		}
		actor := uuid.Nil
		if p, ok := auth.PrincipalFromContext(r.Context()); ok {
			if aid, err := uuid.Parse(strings.TrimSpace(p.Subject)); err == nil {
				actor = aid
			}
		}
		err = app.Anomalies.Resolve(r.Context(), orgID, anomalyID, actor, body.Note)
		if err != nil {
			if errors.Is(err, anomalies.ErrNotFoundOpen) || errors.Is(err, pgx.ErrNoRows) {
				writeAPIError(w, r.Context(), http.StatusNotFound, "not_found", "anomaly not found or already resolved")
				return
			}
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		fleetAudit(r.Context(), app, orgID, "admin.operational_anomaly.resolve", "inventory_anomalies", stringPtrOrNil(anomalyID.String()), map[string]any{"note": body.Note})
		writeJSON(w, http.StatusOK, map[string]any{"anomalyId": anomalyID.String(), "status": "resolved"})
	}
}

func serveAdminOperationalAnomalyIgnore(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, err := parseAdminFleetOrganizationScope(r)
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		anomalyID, ok := parseChiUUID(w, r, "anomalyId")
		if !ok {
			return
		}
		var body resolveAnomalyBody
		if r.Body != nil {
			_ = json.NewDecoder(r.Body).Decode(&body)
		}
		actor := uuid.Nil
		if p, ok := auth.PrincipalFromContext(r.Context()); ok {
			if aid, err := uuid.Parse(strings.TrimSpace(p.Subject)); err == nil {
				actor = aid
			}
		}
		err = app.Anomalies.Ignore(r.Context(), orgID, anomalyID, actor, body.Note)
		if err != nil {
			if errors.Is(err, anomalies.ErrNotFoundOpen) || errors.Is(err, pgx.ErrNoRows) {
				writeAPIError(w, r.Context(), http.StatusNotFound, "not_found", "anomaly not found or already ignored")
				return
			}
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		fleetAudit(r.Context(), app, orgID, "admin.operational_anomaly.ignore", "inventory_anomalies", stringPtrOrNil(anomalyID.String()), map[string]any{"note": body.Note})
		writeJSON(w, http.StatusOK, map[string]any{"anomalyId": anomalyID.String(), "status": "ignored"})
	}
}

func serveAdminOrgRestockSuggestions(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, err := parseAdminFleetOrganizationScope(r)
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		p, err := parseInventoryRefillForecastQuery(r, orgID, nil, false)
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		out, err := app.Anomalies.RestockSuggestions(r.Context(), p)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, mapRefillForecastResponse(out))
	}
}
