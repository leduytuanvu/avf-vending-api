package httpserver

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/avf/avf-vending-api/internal/app/api"
	"github.com/avf/avf-vending-api/internal/app/setupapp"
	"github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type v1PlanogramMergePairsResponse struct {
	MachineID string                 `json:"machineId"`
	Revision  int32                  `json:"revision,omitempty"`
	Pairs     []V1PlanogramMergePair `json:"pairs"`
}

type v1PlanogramMergePairApplyItem struct {
	LeftSlotCode   string `json:"leftSlotCode"`
	RightSlotCode  string `json:"rightSlotCode"`
	CabinetCode    string `json:"cabinetCode,omitempty"`
	LayoutKey      string `json:"layoutKey,omitempty"`
	LayoutRevision int32  `json:"layoutRevision,omitempty"`
	Merge          bool   `json:"merge"`
}

type v1PlanogramMergePairsApplyRequest struct {
	OperatorSessionID string                          `json:"operator_session_id"`
	Items             []v1PlanogramMergePairApplyItem `json:"items"`
}

func mapMergePairsToV1(pairs []setupapp.LaneMergePair) []V1PlanogramMergePair {
	out := make([]V1PlanogramMergePair, 0, len(pairs))
	for _, p := range pairs {
		out = append(out, V1PlanogramMergePair{
			LeftSlotCode:   p.LeftSlotCode,
			RightSlotCode:  p.RightSlotCode,
			CabinetCode:    p.CabinetCode,
			LayoutKey:      p.LayoutKey,
			LayoutRevision: p.LayoutRevision,
			Revision:       p.Revision,
		})
	}
	return out
}

func getAdminMachinePlanogramMergePairs(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pool, ok := mergePairsPool(app)
		if !ok {
			writeCapabilityNotConfigured(w, r.Context(), "database", "database pool is not configured for this API process")
			return
		}
		machineID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "machineId")))
		if err != nil || machineID == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_machine_id", "invalid machineId")
			return
		}
		if _, err = resolveInventoryMachine(r, app.InventoryAdmin, machineID); err != nil {
			writeInventoryAccessOrResolveError(w, r, err)
			return
		}
		pairs, err := setupapp.ListActiveMergePairs(r.Context(), pool, machineID)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "merge_pairs_read_failed", err.Error())
			return
		}
		var revision int32
		if len(pairs) > 0 {
			revision = pairs[0].Revision
		}
		writeJSON(w, http.StatusOK, v1PlanogramMergePairsResponse{
			MachineID: machineID.String(),
			Revision:  revision,
			Pairs:     mapMergePairsToV1(pairs),
		})
	}
}

func putAdminMachinePlanogramMergePairs(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pool, ok := mergePairsPool(app)
		if !ok {
			writeCapabilityNotConfigured(w, r.Context(), "database", "database pool is not configured for this API process")
			return
		}
		machineID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "machineId")))
		if err != nil || machineID == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_machine_id", "invalid machineId")
			return
		}
		if _, err = resolveInventoryMachine(r, app.InventoryAdmin, machineID); err != nil {
			writeInventoryAccessOrResolveError(w, r, err)
			return
		}
		bodyRaw, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_body", "could not read body")
			return
		}
		var body v1PlanogramMergePairsApplyRequest
		if err := json.Unmarshal(bodyRaw, &body); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_json", "invalid JSON body")
			return
		}
		sid, ok := parseOperatorSessionIDField(w, r, body.OperatorSessionID)
		if !ok {
			return
		}
		if !requireActiveOperatorSession(w, r, app, machineID, sid) {
			return
		}
		items := make([]setupapp.MergePairApplyItem, 0, len(body.Items))
		for _, item := range body.Items {
			items = append(items, setupapp.MergePairApplyItem{
				LeftSlotCode:   item.LeftSlotCode,
				RightSlotCode:  item.RightSlotCode,
				CabinetCode:    item.CabinetCode,
				LayoutKey:      item.LayoutKey,
				LayoutRevision: item.LayoutRevision,
				Merge:          item.Merge,
			})
		}
		res, err := setupapp.ApplyMergePairBatch(r.Context(), pool, setupapp.MergePairBatchInput{
			MachineID:         machineID,
			OperatorSessionID: sid,
			Items:             items,
		})
		if err != nil {
			switch {
			case errors.Is(err, setupapp.ErrMergePairOverlap):
				writeAPIError(w, r.Context(), http.StatusConflict, "merge_pair_overlap", err.Error())
			case errors.Is(err, setupapp.ErrMergePairInvalidSlots):
				writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_merge_slots", err.Error())
			default:
				writeSetupMutationError(w, r.Context(), err)
			}
			return
		}
		writeJSON(w, http.StatusOK, v1PlanogramMergePairsResponse{
			MachineID: machineID.String(),
			Revision:  res.Revision,
			Pairs:     mapMergePairsToV1(res.Pairs),
		})
	}
}

func getMachinePlanogramMergePairs(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pool, ok := mergePairsPool(app)
		if !ok {
			writeCapabilityNotConfigured(w, r.Context(), "database", "database pool is not configured for this API process")
			return
		}
		machineID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "machineId")))
		if err != nil || machineID == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_machine_id", "invalid machineId")
			return
		}
		pairs, err := setupapp.ListActiveMergePairs(r.Context(), pool, machineID)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "merge_pairs_read_failed", err.Error())
			return
		}
		var revision int32
		if len(pairs) > 0 {
			revision = pairs[0].Revision
		}
		writeJSON(w, http.StatusOK, v1PlanogramMergePairsResponse{
			MachineID: machineID.String(),
			Revision:  revision,
			Pairs:     mapMergePairsToV1(pairs),
		})
	}
}

func mountPlanogramMergePairRoutes(r chi.Router, app *api.HTTPApplication, writeRL func(http.Handler) http.Handler) {
	if writeRL == nil {
		writeRL = func(h http.Handler) http.Handler { return h }
	}
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAnyPermission(auth.PermInventoryRead))
		r.Get("/machines/{machineId}/planogram/merge-pairs", getAdminMachinePlanogramMergePairs(app))
	})
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAnyPermission(auth.PermInventoryWrite))
		r.With(writeRL).Put("/machines/{machineId}/planogram/merge-pairs", putAdminMachinePlanogramMergePairs(app))
	})
}

func mountMachinePlanogramMergePairRoute(r chi.Router, app *api.HTTPApplication) {
	if app == nil {
		return
	}
	r.With(
		RequireMachineCompanyAccess(app, "machineId"),
		auth.RequireInteractivePermissionOrMachinePrincipal(auth.PermCatalogRead),
	).Get("/machines/{machineId}/planogram/merge-pairs", getMachinePlanogramMergePairs(app))
}

func mergePairsPool(app *api.HTTPApplication) (*pgxpool.Pool, bool) {
	if app == nil || app.TelemetryStore == nil || app.TelemetryStore.Pool() == nil {
		return nil, false
	}
	return app.TelemetryStore.Pool(), true
}
