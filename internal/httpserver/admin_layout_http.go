package httpserver

import (
	"errors"
	"net/http"
	"strings"

	"github.com/avf/avf-vending-api/internal/app/api"
	"github.com/avf/avf-vending-api/internal/app/layoutassignment"
	"github.com/avf/avf-vending-api/internal/app/setupapp"
	domainoperator "github.com/avf/avf-vending-api/internal/domain/operator"
	appmw "github.com/avf/avf-vending-api/internal/middleware"
	"github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func mountAdminLayoutRoutes(r chi.Router, app *api.HTTPApplication, writeRL func(http.Handler) http.Handler) {
	if app == nil || app.InventoryAdmin == nil || app.TelemetryStore == nil {
		return
	}
	if writeRL == nil {
		writeRL = func(h http.Handler) http.Handler { return h }
	}
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAnyPermission(auth.PermInventoryRead))
		r.Get("/machines/{machineId}/layout-state", getAdminMachineLayoutState(app))
		r.Get("/layout-dimension-migration-audit", getAdminLayoutDimensionMigrationAudit(app))
	})
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAnyPermission(auth.PermInventoryWrite))
		r.With(writeRL).Put("/machines/{machineId}/layout-assignments/server", putAdminMachineServerLayoutAssignment(app))
		r.With(writeRL).Put("/machines/{machineId}/layout-desired-source", putAdminMachineLayoutDesiredSource(app))
		r.With(writeRL).Post("/machines/{machineId}/layout-assignments/server:bulk", postAdminMachineServerLayoutBulk(app))
	})
}

func layoutService(app *api.HTTPApplication) (*layoutassignment.Service, bool) {
	repo, ok := setupPool(app)
	if !ok {
		return nil, false
	}
	return &layoutassignment.Service{
		Pool:  app.TelemetryStore.Pool(),
		Setup: repo,
	}, true
}

type assignServerLayoutBody struct {
	LayoutVersionID         string `json:"layoutVersionId"`
	OrgLayoutVersionID      string `json:"orgLayoutVersionId,omitempty"`
	ExpectedCurrentRevision *int32 `json:"expectedCurrentRevision,omitempty"`
	OperatorSessionID       string `json:"operatorSessionId,omitempty"`
}

type assignServerLayoutResponse struct {
	AssignmentID  string  `json:"assignmentId"`
	Source        string  `json:"source"`
	Revision      int32   `json:"revision"`
	Rows          int32   `json:"rows"`
	Columns       int32   `json:"columns"`
	Fingerprint   string  `json:"fingerprint"`
	DesiredSource *string `json:"desiredSource,omitempty"`
	SyncStatus    string  `json:"syncStatus"`
	RequestID     string  `json:"requestId"`
}

func putAdminMachineServerLayoutAssignment(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		svc, ok := layoutService(app)
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
		idem, err := requireWriteIdempotencyKey(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "missing_idempotency_key", err.Error())
			return
		}
		var body assignServerLayoutBody
		if !decodeStrictJSON(w, r, &body) {
			return
		}
		layoutVersionID, err := uuid.Parse(strings.TrimSpace(body.LayoutVersionID))
		if err != nil || layoutVersionID == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_layout_version_id", "layoutVersionId must be a UUID")
			return
		}
		var orgLayoutVersionID *uuid.UUID
		if strings.TrimSpace(body.OrgLayoutVersionID) != "" {
			u, perr := uuid.Parse(strings.TrimSpace(body.OrgLayoutVersionID))
			if perr != nil || u == uuid.Nil {
				writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_org_layout_version_id", "orgLayoutVersionId must be a UUID when set")
				return
			}
			orgLayoutVersionID = &u
		}
		var opSession *uuid.UUID
		if strings.TrimSpace(body.OperatorSessionID) != "" {
			u, perr := uuid.Parse(strings.TrimSpace(body.OperatorSessionID))
			if perr == nil && u != uuid.Nil {
				opSession = &u
			}
		}
		actor, ok := resolveAdminMachineAction(w, r, app, machineID, body.OperatorSessionID, domainoperator.SessionOptionalByOrigin)
		if !ok {
			return
		}
		var actorID *uuid.UUID
		if aid, ok := principalAccountID(r); ok {
			actorID = &aid
		}
		out, aerr := svc.AssignServerLayout(r.Context(), layoutassignment.AssignServerLayoutInput{
			MachineID:               machineID,
			LayoutVersionID:         layoutVersionID,
			OrgLayoutVersionID:      orgLayoutVersionID,
			ExpectedCurrentRevision: body.ExpectedCurrentRevision,
			IdempotencyKey:          idem,
			ActorAccountID:          actorID,
			OperatorSessionID:       opSession,
		})
		if aerr != nil {
			writeLayoutAssignmentError(w, r, aerr)
			return
		}
		if !recordResolvedMachineActionAttributionOrFail(w, r, app, actor, "machine_layout_assignments", out.AssignmentID.String(), "setup", "setup.layout_assign_server") {
			return
		}
		reqID := appmw.RequestIDFromContext(r.Context())
		writeJSON(w, http.StatusOK, assignServerLayoutResponse{
			AssignmentID:  out.AssignmentID.String(),
			Source:        out.Source,
			Revision:      out.Revision,
			Rows:          out.Rows,
			Columns:       out.Columns,
			Fingerprint:   out.Fingerprint,
			DesiredSource: out.DesiredSource,
			SyncStatus:    out.SyncStatus,
			RequestID:     reqID,
		})
	}
}

func getAdminMachineLayoutState(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		svc, ok := layoutService(app)
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
		state, err := svc.GetLayoutState(r.Context(), machineID)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, state)
	}
}

func getAdminLayoutDimensionMigrationAudit(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		svc, ok := layoutService(app)
		if !ok {
			writeCapabilityNotConfigured(w, r.Context(), "database", "database pool is not configured for this API process")
			return
		}
		report, err := svc.GetLayoutDimensionMigrationAuditReport(r.Context())
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, report)
	}
}

type desiredSourceBody struct {
	Source                  string `json:"source"`
	ExpectedCurrentRevision *int32 `json:"expectedCurrentRevision,omitempty"`
}

type desiredSourceResponse struct {
	DesiredSource      string `json:"desiredSource"`
	DesiredRevision    int32  `json:"desiredRevision"`
	DesiredFingerprint string `json:"desiredFingerprint"`
	SyncStatus         string `json:"syncStatus"`
}

func putAdminMachineLayoutDesiredSource(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		svc, ok := layoutService(app)
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
		var body desiredSourceBody
		if !decodeStrictJSON(w, r, &body) {
			return
		}
		src := strings.ToUpper(strings.TrimSpace(body.Source))
		if src != layoutassignment.SourceServer && src != layoutassignment.SourceLocal {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_source", "source must be SERVER or LOCAL")
			return
		}
		out, derr := svc.SetDesiredSource(r.Context(), layoutassignment.SetDesiredSourceInput{
			MachineID:               machineID,
			Source:                  src,
			ExpectedCurrentRevision: body.ExpectedCurrentRevision,
		})
		if derr != nil {
			writeLayoutAssignmentError(w, r, derr)
			return
		}
		writeJSON(w, http.StatusOK, desiredSourceResponse{
			DesiredSource:      out.DesiredSource,
			DesiredRevision:    out.DesiredRevision,
			DesiredFingerprint: out.DesiredFingerprint,
			SyncStatus:         out.SyncStatus,
		})
	}
}

type bulkAssignBody struct {
	MachineIDs      []string `json:"machineIds"`
	LayoutVersionID string   `json:"layoutVersionId"`
}

func postAdminMachineServerLayoutBulk(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		svc, ok := layoutService(app)
		if !ok {
			writeCapabilityNotConfigured(w, r.Context(), "database", "database pool is not configured for this API process")
			return
		}
		idem, err := requireWriteIdempotencyKey(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "missing_idempotency_key", err.Error())
			return
		}
		var body bulkAssignBody
		if !decodeStrictJSON(w, r, &body) {
			return
		}
		layoutVersionID, err := uuid.Parse(strings.TrimSpace(body.LayoutVersionID))
		if err != nil || layoutVersionID == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_layout_version_id", "layoutVersionId must be a UUID")
			return
		}
		reqID := appmw.RequestIDFromContext(r.Context())
		results := make([]layoutassignment.BulkAssignResult, 0, len(body.MachineIDs))
		for _, midStr := range body.MachineIDs {
			machineID, perr := uuid.Parse(strings.TrimSpace(midStr))
			if perr != nil || machineID == uuid.Nil {
				results = append(results, layoutassignment.BulkAssignResult{
					MachineID:    uuid.Nil,
					Status:       "failed",
					ErrorCode:    "invalid_machine_id",
					ErrorMessage: "invalid machineId",
					RequestID:    reqID,
				})
				continue
			}
			out, aerr := svc.AssignServerLayout(r.Context(), layoutassignment.AssignServerLayoutInput{
				MachineID:       machineID,
				LayoutVersionID: layoutVersionID,
				IdempotencyKey:  idem + ":" + machineID.String(),
			})
			if aerr != nil {
				code, msg := layoutErrorCode(aerr)
				results = append(results, layoutassignment.BulkAssignResult{
					MachineID:    machineID,
					Status:       "failed",
					ErrorCode:    code,
					ErrorMessage: msg,
					RequestID:    reqID,
				})
				continue
			}
			rev := out.Revision
			aid := out.AssignmentID
			results = append(results, layoutassignment.BulkAssignResult{
				MachineID:    machineID,
				Status:       "succeeded",
				AssignmentID: &aid,
				Revision:     &rev,
				RequestID:    reqID,
			})
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"results":   results,
			"requestId": reqID,
		})
	}
}

func writeLayoutAssignmentError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, setupapp.ErrNotFound), errors.Is(err, layoutassignment.ErrLayoutVersionNotFound):
		writeAPIError(w, r.Context(), http.StatusNotFound, "not_found", err.Error())
	case errors.Is(err, layoutassignment.ErrMachineMismatch):
		writeAPIError(w, r.Context(), http.StatusBadRequest, "machine_mismatch", err.Error())
	case errors.Is(err, layoutassignment.ErrRevisionConflict):
		writeAPIError(w, r.Context(), http.StatusConflict, "revision_conflict", err.Error())
	case errors.Is(err, layoutassignment.ErrLayoutRevisionConflict):
		writeAPIError(w, r.Context(), http.StatusConflict, "layout_revision_conflict", err.Error())
	case errors.Is(err, layoutassignment.ErrLayoutAssignmentNotFound):
		writeAPIError(w, r.Context(), http.StatusNotFound, "layout_assignment_not_found", err.Error())
	case errors.Is(err, layoutassignment.ErrUnknownDimensions):
		writeAPIError(w, r.Context(), http.StatusUnprocessableEntity, "layout_dimensions_unknown", err.Error())
	case errors.Is(err, layoutassignment.ErrExceedsHardwareLaneCapacity):
		writeAPIError(w, r.Context(), http.StatusUnprocessableEntity, "layout_exceeds_hardware_lane_capacity", err.Error())
	case errors.Is(err, layoutassignment.ErrInvalidDimensions):
		writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_dimensions", err.Error())
	case errors.Is(err, layoutassignment.ErrIdempotencyKeyConflict):
		writeAPIError(w, r.Context(), http.StatusConflict, "idempotency_key_conflict", err.Error())
	default:
		writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
	}
}

func layoutErrorCode(err error) (code, msg string) {
	switch {
	case errors.Is(err, layoutassignment.ErrExceedsHardwareLaneCapacity):
		return "layout_exceeds_hardware_lane_capacity", err.Error()
	case errors.Is(err, layoutassignment.ErrUnknownDimensions):
		return "layout_dimensions_unknown", err.Error()
	case errors.Is(err, layoutassignment.ErrRevisionConflict):
		return "revision_conflict", err.Error()
	default:
		return "internal", err.Error()
	}
}
