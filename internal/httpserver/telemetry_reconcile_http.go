package httpserver

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/avf/avf-vending-api/internal/app/api"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func mountDeviceTelemetryReconcileRoutes(r chi.Router, app *api.HTTPApplication) {
	if app == nil || app.TelemetryStore == nil {
		return
	}
	r.Route("/device/machines/{machineId}", func(r chi.Router) {
		r.Use(RequireMachineTenantAccess(app, "machineId"))
		r.Use(auth.RequireAnyRole(auth.RolePlatformAdmin, auth.RoleOrgAdmin, auth.RoleMachine))
		r.Post("/events/reconcile", postTelemetryReconcileBatch(app))
		r.Get("/events/{idempotencyKey}/status", getTelemetryReconcileStatus(app))
	})
}

type reconcileBatchBody struct {
	IdempotencyKeys []string `json:"idempotencyKeys"`
}

func postTelemetryReconcileBatch(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		machineID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "machineId")))
		if err != nil || machineID == uuid.Nil {
			writeAPIError(w, ctx, http.StatusBadRequest, "invalid_machine_id", "invalid machineId")
			return
		}
		var body reconcileBatchBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeAPIError(w, ctx, http.StatusBadRequest, "invalid_json", "request body must be JSON")
			return
		}
		n := len(body.IdempotencyKeys)
		if n < 1 || n > 500 {
			writeAPIError(w, ctx, http.StatusBadRequest, "invalid_batch_size", "idempotencyKeys must contain 1 to 500 entries")
			return
		}
		q := db.New(app.TelemetryStore.Pool())
		items := make([]map[string]any, 0, n)
		for _, raw := range body.IdempotencyKeys {
			key := strings.TrimSpace(raw)
			if key == "" {
				writeAPIError(w, ctx, http.StatusBadRequest, "invalid_idempotency_key", "empty idempotency key")
				return
			}
			row, qerr := q.GetCriticalTelemetryEventStatus(ctx, db.GetCriticalTelemetryEventStatusParams{
				MachineID:      machineID,
				IdempotencyKey: key,
			})
			if qerr != nil {
				if qerr == pgx.ErrNoRows {
					items = append(items, map[string]any{
						"idempotencyKey": key,
						"status":         "not_found",
						"eventType":      nil,
						"acceptedAt":     nil,
						"processedAt":    nil,
						"retryable":      true,
					})
					continue
				}
				writeAPIError(w, ctx, http.StatusInternalServerError, "internal", qerr.Error())
				return
			}
			items = append(items, mapTelemetryStatusItem(key, row))
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"machineId": machineID.String(),
			"items":     items,
		})
	}
}

func getTelemetryReconcileStatus(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		machineID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "machineId")))
		if err != nil || machineID == uuid.Nil {
			writeAPIError(w, ctx, http.StatusBadRequest, "invalid_machine_id", "invalid machineId")
			return
		}
		key := strings.TrimSpace(chi.URLParam(r, "idempotencyKey"))
		if key == "" {
			writeAPIError(w, ctx, http.StatusBadRequest, "invalid_idempotency_key", "missing idempotency key")
			return
		}
		q := db.New(app.TelemetryStore.Pool())
		row, qerr := q.GetCriticalTelemetryEventStatus(ctx, db.GetCriticalTelemetryEventStatusParams{
			MachineID:      machineID,
			IdempotencyKey: key,
		})
		if qerr != nil {
			if qerr == pgx.ErrNoRows {
				writeJSON(w, http.StatusOK, map[string]any{
					"machineId":      machineID.String(),
					"idempotencyKey": key,
					"status":         "not_found",
					"eventType":      nil,
					"acceptedAt":     nil,
					"processedAt":    nil,
					"retryable":      true,
				})
				return
			}
			writeAPIError(w, ctx, http.StatusInternalServerError, "internal", qerr.Error())
			return
		}
		writeJSON(w, http.StatusOK, mapTelemetryStatusItem(key, row))
	}
}

func mapTelemetryStatusItem(key string, row db.CriticalTelemetryEventStatus) map[string]any {
	st := row.Status
	retry := st != "processed" && st != "failed_terminal"
	var ev any
	if row.EventType.Valid {
		ev = row.EventType.String
	} else {
		ev = nil
	}
	var acc any
	if row.AcceptedAt.Valid {
		acc = row.AcceptedAt.Time.UTC().Format("2006-01-02T15:04:05Z07:00")
	} else {
		acc = nil
	}
	var proc any
	if row.ProcessedAt.Valid {
		proc = row.ProcessedAt.Time.UTC().Format("2006-01-02T15:04:05Z07:00")
	} else {
		proc = nil
	}
	return map[string]any{
		"idempotencyKey": key,
		"status":         st,
		"eventType":      ev,
		"acceptedAt":     acc,
		"processedAt":    proc,
		"retryable":      retry,
	}
}
