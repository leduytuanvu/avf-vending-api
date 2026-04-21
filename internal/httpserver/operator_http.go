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
	"github.com/avf/avf-vending-api/internal/app/operator"
	"github.com/avf/avf-vending-api/internal/app/setupapp"
	"github.com/avf/avf-vending-api/internal/domain/fleet"
	domainoperator "github.com/avf/avf-vending-api/internal/domain/operator"
	appmw "github.com/avf/avf-vending-api/internal/middleware"
	"github.com/avf/avf-vending-api/internal/modules/postgres"
	"github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// operatorLoginRequest carries non-identity fields only. Actor (technician vs user principal) is
// taken from the JWT principal, never from JSON.
//
// auth_method: optional; when empty the server defaults to "oidc". Other values must satisfy domain auth semantics.
type operatorLoginRequest struct {
	AuthMethod         string          `json:"auth_method"`
	ExpiresAt          *time.Time      `json:"expires_at"`
	ClientMetadata     json.RawMessage `json:"client_metadata"`
	ForceAdminTakeover bool            `json:"force_admin_takeover"`
}

type operatorLogoutRequest struct {
	SessionID   string `json:"session_id"`
	EndedReason string `json:"ended_reason"`
	AuthMethod  string `json:"auth_method"`
	// FinalStatus is optional: ENDED (default) or REVOKED. REVOKED is restricted to org admins or platform admins.
	FinalStatus string `json:"final_status,omitempty"`
}

// errInsightOrgIDRequired is returned when a platform admin calls operator-insights without organization_id.
var errInsightOrgIDRequired = errors.New("organization_id query is required for platform-wide access")

// operatorFetchMachine loads the machine row for operator routes and writes HTTP errors on failure.
func operatorFetchMachine(w http.ResponseWriter, svc *operator.Service, ctx context.Context, machineID uuid.UUID) (fleet.Machine, bool) {
	machine, err := svc.MachineByID(ctx, machineID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeAPIError(w, ctx, http.StatusNotFound, "machine_not_found", "machine not found")
			return fleet.Machine{}, false
		}
		writeAPIError(w, ctx, http.StatusInternalServerError, "internal", err.Error())
		return fleet.Machine{}, false
	}
	return machine, true
}

func decodeOperatorLoginBody(w http.ResponseWriter, r *http.Request) (operatorLoginRequest, bool) {
	var body operatorLoginRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	switch err := dec.Decode(&body); err {
	case nil, io.EOF:
		return body, true
	default:
		writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_json", "invalid request body")
		return body, false
	}
}

func decodeOperatorLogoutBody(w http.ResponseWriter, r *http.Request) (operatorLogoutRequest, bool) {
	var body operatorLogoutRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	switch err := dec.Decode(&body); err {
	case nil, io.EOF:
		return body, true
	default:
		writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_json", "invalid request body")
		return body, false
	}
}

func parseOperatorLogoutFinalStatus(raw string) (string, error) {
	s := strings.TrimSpace(strings.ToUpper(raw))
	switch s {
	case "", "ENDED":
		return domainoperator.SessionStatusEnded, nil
	case "REVOKED":
		return domainoperator.SessionStatusRevoked, nil
	default:
		return "", domainoperator.ErrInvalidSessionEndStatus
	}
}

func sessionRevokeAllowed(p auth.Principal, machine fleet.Machine) bool {
	if p.HasRole(auth.RolePlatformAdmin) {
		return true
	}
	return p.HasOrganization() && p.OrganizationID == machine.OrganizationID && p.HasRole(auth.RoleOrgAdmin)
}

// mountOperatorSessionRoutes wires machine-scoped operator session APIs under:
//
//	/v1/machines/{machineId}/operator-sessions/...
//
// Actor identity for login comes only from the authenticated principal (JWT); request bodies must
// not carry technician_id/user_principal (rejected by strict JSON on login/logout).
//
// Ops / incidents: ops/RUNBOOK.md (operator session issues); correlate X-Request-ID / X-Correlation-ID.
func mountOperatorSessionRoutes(r chi.Router, app *api.HTTPApplication) {
	if app == nil || app.MachineOperator == nil {
		return
	}
	svc := app.MachineOperator
	r.Route("/machines/{machineId}/operator-sessions", func(r chi.Router) {
		r.Use(auth.RequireMachineURLAccess("machineId"))
		r.Get("/current", operatorCurrentHandler(svc))
		r.Get("/history", operatorHistoryHandler(svc))
		r.Get("/auth-events", operatorAuthEventsHandler(svc))
		r.Get("/action-attributions", operatorActionAttributionsMachineHandler(svc))
		r.Get("/timeline", operatorTimelineHandler(svc))
		r.Post("/login", operatorLoginHandler(svc))
		r.Post("/logout", operatorLogoutHandler(svc))
		r.Post("/{sessionId}/heartbeat", operatorHeartbeatHandler(svc))
	})
}

// mountOperatorAdminInsightRoutes lists operator action attributions across machines for support workflows.
// organization_id query is required when the principal is platform_admin without tenant scope.
// Routes are mounted under /v1/operator-insights/...
func mountOperatorAdminInsightRoutes(r chi.Router, app *api.HTTPApplication) {
	if app == nil || app.MachineOperator == nil {
		return
	}
	svc := app.MachineOperator
	r.Get("/technicians/{technicianId}/action-attributions", operatorActionAttributionsTechnicianHandler(svc))
	r.Get("/users/action-attributions", operatorActionAttributionsUserHandler(svc))
}

// operatorLoginHandler creates or resumes one ACTIVE operator session per machine.
//
// Same principal (JWT-derived actor) POSTs login again while their session is still ACTIVE: the
// server resumes that row (last_activity_at, optional expires_at/client_metadata) and records
// session_refresh when auth_method is present.
//
// HTTP 409 active_session_exists: another principal holds an ACTIVE session that is not stale and
// this request did not request an authorized admin takeover. Org/platform admins may set
// force_admin_takeover to revoke the current session (ended_reason admin_forced_takeover) and open
// a new one. After idle beyond the server reclaim window, a different operator may log in and the
// prior session is ended with ended_reason stale_session_reclaimed.
func operatorLoginHandler(svc *operator.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		machineID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "machineId")))
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_machine_id", "invalid machineId")
			return
		}
		p, ok := auth.PrincipalFromContext(ctx)
		if !ok {
			writeAPIError(w, r.Context(), http.StatusUnauthorized, "unauthenticated", auth.ErrUnauthenticated.Error())
			return
		}
		machine, ok := operatorFetchMachine(w, svc, ctx, machineID)
		if !ok {
			return
		}
		if err := authorizeMachineOperatorAccess(p, machine); err != nil {
			writeAPIError(w, r.Context(), http.StatusForbidden, "forbidden", auth.ErrForbidden.Error())
			return
		}
		body, ok := decodeOperatorLoginBody(w, r)
		if !ok {
			return
		}
		if body.ForceAdminTakeover && !sessionRevokeAllowed(p, machine) {
			writeAPIError(w, r.Context(), http.StatusForbidden, "admin_takeover_forbidden", "force_admin_takeover requires org admin or platform admin")
			return
		}
		authMethod := strings.TrimSpace(body.AuthMethod)
		if authMethod == "" {
			authMethod = domainoperator.AuthMethodOIDC
		}
		actorType, techID, userPrincipal, err := deriveOperatorActor(p)
		if err != nil {
			_ = recordLoginFailure(ctx, svc, machine.OrganizationID, machineID, authMethod, correlationUUIDFromRequest(ctx), loginFailureMetadata(err, appmw.CorrelationIDFromContext(ctx)))
			writeOperatorError(w, r.Context(), err)
			return
		}
		meta := []byte("{}")
		if len(body.ClientMetadata) > 0 {
			meta = body.ClientMetadata
		}
		corr := correlationUUIDFromRequest(ctx)
		meta = mergeCorrelationMetadata(meta, appmw.CorrelationIDFromContext(ctx))
		adminTakeoverOK := body.ForceAdminTakeover && sessionRevokeAllowed(p, machine)
		sess, err := svc.StartOperatorSession(ctx, operator.StartOperatorSessionInput{
			OrganizationID:          machine.OrganizationID,
			MachineID:               machineID,
			ActorType:               actorType,
			TechnicianID:            techID,
			UserPrincipal:           userPrincipal,
			ExpiresAt:               body.ExpiresAt,
			ClientMetadata:          meta,
			InitialAuthMethod:       authMethod,
			CorrelationID:           corr,
			InitialAuthMetadata:     meta,
			ForceAdminTakeover:      body.ForceAdminTakeover,
			AdminTakeoverAuthorized: adminTakeoverOK,
		})
		if err != nil {
			_ = recordLoginFailure(ctx, svc, machine.OrganizationID, machineID, authMethod, corr, loginFailureMetadata(err, appmw.CorrelationIDFromContext(ctx)))
			writeOperatorError(w, r.Context(), err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"session": sessionView(sess)})
	}
}

func operatorLogoutHandler(svc *operator.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		machineID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "machineId")))
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_machine_id", "invalid machineId")
			return
		}
		p, ok := auth.PrincipalFromContext(ctx)
		if !ok {
			writeAPIError(w, r.Context(), http.StatusUnauthorized, "unauthenticated", auth.ErrUnauthenticated.Error())
			return
		}
		machine, ok := operatorFetchMachine(w, svc, ctx, machineID)
		if !ok {
			return
		}
		if err := authorizeMachineOperatorAccess(p, machine); err != nil {
			writeAPIError(w, r.Context(), http.StatusForbidden, "forbidden", auth.ErrForbidden.Error())
			return
		}
		body, ok := decodeOperatorLogoutBody(w, r)
		if !ok {
			return
		}
		authMethod := strings.TrimSpace(body.AuthMethod)
		if authMethod == "" {
			authMethod = domainoperator.AuthMethodOIDC
		}
		rawSid := strings.TrimSpace(body.SessionID)
		var sid uuid.UUID
		if rawSid != "" {
			parsed, perr := uuid.Parse(rawSid)
			if perr != nil {
				writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_session_id", "session_id must be a valid UUID")
				return
			}
			sid = parsed
		} else {
			active, aerr := svc.ActiveSessionForMachine(ctx, machineID)
			if aerr != nil {
				writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", aerr.Error())
				return
			}
			if active == nil {
				writeAPIError(w, r.Context(), http.StatusBadRequest, "session_id_required", "session_id is required when no active session exists")
				return
			}
			sid = active.ID
		}
		sess, err := svc.GetSessionIfMatchesMachine(ctx, sid, machineID)
		if err != nil {
			writeOperatorError(w, r.Context(), err)
			return
		}
		if err := assertSessionMutableByPrincipal(p, machine, sess); err != nil {
			writeAPIError(w, r.Context(), http.StatusForbidden, "forbidden", auth.ErrForbidden.Error())
			return
		}
		finalStatus, err := parseOperatorLogoutFinalStatus(body.FinalStatus)
		if err != nil {
			writeOperatorError(w, r.Context(), err)
			return
		}
		if finalStatus == domainoperator.SessionStatusRevoked && !sessionRevokeAllowed(p, machine) {
			writeAPIError(w, r.Context(), http.StatusForbidden, "forbidden", "revoke requires org admin or platform admin")
			return
		}
		corr := correlationUUIDFromRequest(ctx)
		ended, err := svc.EndOperatorSession(ctx, operator.EndOperatorSessionInput{
			OrganizationID:      machine.OrganizationID,
			MachineID:           machineID,
			SessionID:           sid,
			FinalStatus:         finalStatus,
			EndedReason:         strings.TrimSpace(body.EndedReason),
			LogoutAuthMethod:    authMethod,
			LogoutCorrelationID: corr,
			LogoutMetadata:      mergeCorrelationMetadata([]byte("{}"), appmw.CorrelationIDFromContext(ctx)),
		})
		if err != nil {
			writeOperatorError(w, r.Context(), err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"session": sessionView(ended)})
	}
}

func operatorHeartbeatHandler(svc *operator.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		machineID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "machineId")))
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_machine_id", "invalid machineId")
			return
		}
		sessionID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "sessionId")))
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_session_id", "invalid sessionId")
			return
		}
		p, ok := auth.PrincipalFromContext(ctx)
		if !ok {
			writeAPIError(w, r.Context(), http.StatusUnauthorized, "unauthenticated", auth.ErrUnauthenticated.Error())
			return
		}
		machine, ok := operatorFetchMachine(w, svc, ctx, machineID)
		if !ok {
			return
		}
		if err := authorizeMachineOperatorAccess(p, machine); err != nil {
			writeAPIError(w, r.Context(), http.StatusForbidden, "forbidden", auth.ErrForbidden.Error())
			return
		}
		sess, err := svc.GetSessionIfMatchesMachine(ctx, sessionID, machineID)
		if err != nil {
			writeOperatorError(w, r.Context(), err)
			return
		}
		if err := assertSessionMutableByPrincipal(p, machine, sess); err != nil {
			writeAPIError(w, r.Context(), http.StatusForbidden, "forbidden", auth.ErrForbidden.Error())
			return
		}
		out, err := svc.HeartbeatOperatorSession(ctx, machine.OrganizationID, machineID, sessionID)
		if err != nil {
			writeOperatorError(w, r.Context(), err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"session": sessionView(out)})
	}
}

func operatorCurrentHandler(svc *operator.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		machineID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "machineId")))
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_machine_id", "invalid machineId")
			return
		}
		p, ok := auth.PrincipalFromContext(ctx)
		if !ok {
			writeAPIError(w, r.Context(), http.StatusUnauthorized, "unauthenticated", auth.ErrUnauthenticated.Error())
			return
		}
		machine, ok := operatorFetchMachine(w, svc, ctx, machineID)
		if !ok {
			return
		}
		if err := authorizeMachineOperatorAccess(p, machine); err != nil {
			writeAPIError(w, r.Context(), http.StatusForbidden, "forbidden", auth.ErrForbidden.Error())
			return
		}
		res, err := svc.ResolveCurrentOperatorForMachine(ctx, machine.OrganizationID, machineID)
		if err != nil {
			writeOperatorError(w, r.Context(), err)
			return
		}
		if res.ActiveSession == nil {
			writeJSON(w, http.StatusOK, map[string]any{"active_session": nil})
			return
		}
		v := sessionView(*res.ActiveSession)
		if res.TechnicianDisplayName != nil {
			v["technician_display_name"] = *res.TechnicianDisplayName
		}
		writeJSON(w, http.StatusOK, map[string]any{"active_session": v})
	}
}

func operatorHistoryHandler(svc *operator.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		machineID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "machineId")))
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_machine_id", "invalid machineId")
			return
		}
		p, ok := auth.PrincipalFromContext(ctx)
		if !ok {
			writeAPIError(w, r.Context(), http.StatusUnauthorized, "unauthenticated", auth.ErrUnauthenticated.Error())
			return
		}
		machine, ok := operatorFetchMachine(w, svc, ctx, machineID)
		if !ok {
			return
		}
		if err := authorizeMachineOperatorAccess(p, machine); err != nil {
			writeAPIError(w, r.Context(), http.StatusForbidden, "forbidden", auth.ErrForbidden.Error())
			return
		}
		limit, lerr := parseOperatorListLimit(r)
		if lerr != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_limit", lerr.Error())
			return
		}
		items, err := svc.ListSessionsByMachine(ctx, machine.OrganizationID, machineID, limit)
		if err != nil {
			writeOperatorError(w, r.Context(), err)
			return
		}
		out := make([]map[string]any, 0, len(items))
		for _, s := range items {
			out = append(out, sessionView(s))
		}
		writeOperatorListEnvelope(w, out, limit, len(out))
	}
}

func operatorAuthEventsHandler(svc *operator.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		machineID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "machineId")))
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_machine_id", "invalid machineId")
			return
		}
		p, ok := auth.PrincipalFromContext(ctx)
		if !ok {
			writeAPIError(w, r.Context(), http.StatusUnauthorized, "unauthenticated", auth.ErrUnauthenticated.Error())
			return
		}
		machine, ok := operatorFetchMachine(w, svc, ctx, machineID)
		if !ok {
			return
		}
		if err := authorizeMachineOperatorAccess(p, machine); err != nil {
			writeAPIError(w, r.Context(), http.StatusForbidden, "forbidden", auth.ErrForbidden.Error())
			return
		}
		limit, lerr := parseOperatorListLimit(r)
		if lerr != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_limit", lerr.Error())
			return
		}
		items, err := svc.ListAuthEventsForMachine(ctx, machine.OrganizationID, machineID, limit)
		if err != nil {
			writeOperatorError(w, r.Context(), err)
			return
		}
		out := make([]operator.AuthEventView, 0, len(items))
		for _, e := range items {
			out = append(out, operator.AuthEventViewFromDomain(e))
		}
		writeOperatorListEnvelope(w, out, limit, len(out))
	}
}

func operatorActionAttributionsMachineHandler(svc *operator.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		machineID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "machineId")))
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_machine_id", "invalid machineId")
			return
		}
		p, ok := auth.PrincipalFromContext(ctx)
		if !ok {
			writeAPIError(w, r.Context(), http.StatusUnauthorized, "unauthenticated", auth.ErrUnauthenticated.Error())
			return
		}
		machine, ok := operatorFetchMachine(w, svc, ctx, machineID)
		if !ok {
			return
		}
		if err := authorizeMachineOperatorAccess(p, machine); err != nil {
			writeAPIError(w, r.Context(), http.StatusForbidden, "forbidden", auth.ErrForbidden.Error())
			return
		}
		limit, lerr := parseOperatorListLimit(r)
		if lerr != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_limit", lerr.Error())
			return
		}
		items, err := svc.ListActionAttributionsForMachine(ctx, machine.OrganizationID, machineID, limit)
		if err != nil {
			writeOperatorError(w, r.Context(), err)
			return
		}
		out := make([]operator.ActionAttributionView, 0, len(items))
		for _, a := range items {
			out = append(out, operator.ActionAttributionViewFromDomain(a))
		}
		writeOperatorListEnvelope(w, out, limit, len(out))
	}
}

func operatorTimelineHandler(svc *operator.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		machineID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "machineId")))
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_machine_id", "invalid machineId")
			return
		}
		p, ok := auth.PrincipalFromContext(ctx)
		if !ok {
			writeAPIError(w, r.Context(), http.StatusUnauthorized, "unauthenticated", auth.ErrUnauthenticated.Error())
			return
		}
		machine, ok := operatorFetchMachine(w, svc, ctx, machineID)
		if !ok {
			return
		}
		if err := authorizeMachineOperatorAccess(p, machine); err != nil {
			writeAPIError(w, r.Context(), http.StatusForbidden, "forbidden", auth.ErrForbidden.Error())
			return
		}
		limit, lerr := parseOperatorListLimit(r)
		if lerr != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_limit", lerr.Error())
			return
		}
		items, err := svc.BuildMachineOperatorTimeline(ctx, machine.OrganizationID, machineID, limit)
		if err != nil {
			writeOperatorError(w, r.Context(), err)
			return
		}
		writeOperatorListEnvelope(w, items, limit, len(items))
	}
}

func operatorActionAttributionsTechnicianHandler(svc *operator.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		p, ok := auth.PrincipalFromContext(ctx)
		if !ok {
			writeAPIError(w, r.Context(), http.StatusUnauthorized, "unauthenticated", auth.ErrUnauthenticated.Error())
			return
		}
		orgID, err := resolveOrgScopeForInsight(p, r)
		if err != nil {
			if errors.Is(err, auth.ErrForbidden) {
				writeAPIError(w, r.Context(), http.StatusForbidden, "forbidden", auth.ErrForbidden.Error())
				return
			}
			if errors.Is(err, errInsightOrgIDRequired) {
				writeAPIError(w, r.Context(), http.StatusBadRequest, "missing_organization_id", err.Error())
				return
			}
			writeAPIError(w, r.Context(), http.StatusBadRequest, "bad_request", err.Error())
			return
		}
		tid, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "technicianId")))
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_technician_id", "invalid technicianId")
			return
		}
		limit, lerr := parseOperatorListLimit(r)
		if lerr != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_limit", lerr.Error())
			return
		}
		items, err := svc.ListActionAttributionsForTechnician(ctx, orgID, tid, limit)
		if err != nil {
			writeOperatorError(w, r.Context(), err)
			return
		}
		out := make([]operator.ActionAttributionView, 0, len(items))
		for _, a := range items {
			out = append(out, operator.ActionAttributionViewFromDomain(a))
		}
		writeOperatorListEnvelope(w, out, limit, len(out))
	}
}

func operatorActionAttributionsUserHandler(svc *operator.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		p, ok := auth.PrincipalFromContext(ctx)
		if !ok {
			writeAPIError(w, r.Context(), http.StatusUnauthorized, "unauthenticated", auth.ErrUnauthenticated.Error())
			return
		}
		orgID, err := resolveOrgScopeForInsight(p, r)
		if err != nil {
			if errors.Is(err, auth.ErrForbidden) {
				writeAPIError(w, r.Context(), http.StatusForbidden, "forbidden", auth.ErrForbidden.Error())
				return
			}
			if errors.Is(err, errInsightOrgIDRequired) {
				writeAPIError(w, r.Context(), http.StatusBadRequest, "missing_organization_id", err.Error())
				return
			}
			writeAPIError(w, r.Context(), http.StatusBadRequest, "bad_request", err.Error())
			return
		}
		principal := strings.TrimSpace(r.URL.Query().Get("user_principal"))
		if principal == "" {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "missing_user_principal", "user_principal query is required")
			return
		}
		limit, lerr := parseOperatorListLimit(r)
		if lerr != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_limit", lerr.Error())
			return
		}
		items, err := svc.ListActionAttributionsForUserPrincipal(ctx, orgID, principal, limit)
		if err != nil {
			writeOperatorError(w, r.Context(), err)
			return
		}
		out := make([]operator.ActionAttributionView, 0, len(items))
		for _, a := range items {
			out = append(out, operator.ActionAttributionViewFromDomain(a))
		}
		writeOperatorListEnvelope(w, out, limit, len(out))
	}
}

func resolveOrgScopeForInsight(p auth.Principal, r *http.Request) (uuid.UUID, error) {
	if p.HasOrganization() {
		return p.OrganizationID, nil
	}
	if p.HasRole(auth.RolePlatformAdmin) {
		raw := strings.TrimSpace(r.URL.Query().Get("organization_id"))
		if raw == "" {
			return uuid.Nil, errInsightOrgIDRequired
		}
		return uuid.Parse(raw)
	}
	return uuid.Nil, auth.ErrForbidden
}

func authorizeMachineOperatorAccess(p auth.Principal, machine fleet.Machine) error {
	if p.HasRole(auth.RolePlatformAdmin) {
		return nil
	}
	if p.AllowsMachine(machine.ID) {
		return nil
	}
	if p.HasOrganization() && p.OrganizationID == machine.OrganizationID && p.HasAnyRole(auth.RoleOrgAdmin, auth.RoleOrgMember) {
		return nil
	}
	return auth.ErrForbidden
}

func assertSessionMutableByPrincipal(p auth.Principal, machine fleet.Machine, sess domainoperator.Session) error {
	if p.HasRole(auth.RolePlatformAdmin) {
		return nil
	}
	if p.HasOrganization() && p.OrganizationID == machine.OrganizationID && p.HasRole(auth.RoleOrgAdmin) {
		return nil
	}
	if sess.ActorType == domainoperator.ActorTypeTechnician && sess.TechnicianID != nil && p.TechnicianID != uuid.Nil &&
		*sess.TechnicianID == p.TechnicianID && p.AllowsMachine(machine.ID) {
		return nil
	}
	if sess.ActorType == domainoperator.ActorTypeUser && sess.UserPrincipal != nil && strings.TrimSpace(p.Subject) != "" &&
		*sess.UserPrincipal == strings.TrimSpace(p.Subject) && p.HasOrganization() && p.OrganizationID == machine.OrganizationID {
		return nil
	}
	return auth.ErrForbidden
}

func deriveOperatorActor(p auth.Principal) (actorType string, techID *uuid.UUID, userPrincipal *string, err error) {
	if p.TechnicianID != uuid.Nil {
		tid := p.TechnicianID
		return domainoperator.ActorTypeTechnician, &tid, nil, nil
	}
	sub := strings.TrimSpace(p.Subject)
	if sub == "" {
		return "", nil, nil, domainoperator.ErrInvalidActor
	}
	return domainoperator.ActorTypeUser, nil, &sub, nil
}

func correlationUUIDFromRequest(ctx context.Context) *uuid.UUID {
	s := appmw.CorrelationIDFromContext(ctx)
	if s == "" {
		return nil
	}
	if id, err := uuid.Parse(s); err == nil {
		return &id
	}
	return nil
}

func mergeCorrelationMetadata(base []byte, corr string) []byte {
	var m map[string]any
	if len(base) > 0 {
		_ = json.Unmarshal(base, &m)
	}
	if m == nil {
		m = map[string]any{}
	}
	if corr != "" {
		if _, ok := m["http_correlation"]; !ok {
			m["http_correlation"] = corr
		}
	}
	out, err := json.Marshal(m)
	if err != nil {
		return base
	}
	return out
}

func loginFailureMetadata(cause error, httpCorr string) []byte {
	m := map[string]any{"reason": cause.Error()}
	if httpCorr != "" {
		m["http_correlation"] = httpCorr
	}
	b, _ := json.Marshal(m)
	return b
}

func recordLoginFailure(ctx context.Context, svc *operator.Service, machineOrgID, machineID uuid.UUID, authMethod string, corr *uuid.UUID, meta []byte) error {
	am := strings.TrimSpace(authMethod)
	if am == "" {
		am = domainoperator.AuthMethodUnknown
	}
	if err := domainoperator.ValidateAuthEventSemantics(domainoperator.AuthEventLoginFailure, am); err != nil {
		am = domainoperator.AuthMethodUnknown
	}
	_, err := svc.RecordAuthEvent(ctx, operator.RecordAuthEventInput{
		OrganizationID:    machineOrgID,
		OperatorSessionID: nil,
		MachineID:         machineID,
		EventType:         domainoperator.AuthEventLoginFailure,
		AuthMethod:        am,
		CorrelationID:     corr,
		Metadata:          meta,
	})
	return err
}

func sessionView(s domainoperator.Session) map[string]any {
	out := map[string]any{
		"id":               s.ID.String(),
		"organization_id":  s.OrganizationID.String(),
		"machine_id":       s.MachineID.String(),
		"actor_type":       s.ActorType,
		"status":           s.Status,
		"started_at":       s.StartedAt.UTC().Format(time.RFC3339Nano),
		"last_activity_at": s.LastActivityAt.UTC().Format(time.RFC3339Nano),
		"created_at":       s.CreatedAt.UTC().Format(time.RFC3339Nano),
		"updated_at":       s.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
	if s.TechnicianID != nil {
		out["technician_id"] = s.TechnicianID.String()
	}
	if s.UserPrincipal != nil {
		out["user_principal"] = *s.UserPrincipal
	}
	if s.EndedAt != nil {
		out["ended_at"] = s.EndedAt.UTC().Format(time.RFC3339Nano)
	}
	if s.ExpiresAt != nil {
		out["expires_at"] = s.ExpiresAt.UTC().Format(time.RFC3339Nano)
	}
	if s.EndedReason != nil {
		out["ended_reason"] = *s.EndedReason
	}
	if len(s.ClientMetadata) > 0 {
		var raw any
		if json.Unmarshal(s.ClientMetadata, &raw) == nil {
			out["client_metadata"] = raw
		} else {
			out["client_metadata"] = json.RawMessage(s.ClientMetadata)
		}
	} else {
		out["client_metadata"] = map[string]any{}
	}
	return out
}

func writeOperatorError(w http.ResponseWriter, ctx context.Context, err error) {
	switch {
	case errors.Is(err, domainoperator.ErrInvalidActor),
		errors.Is(err, domainoperator.ErrInvalidAuthMethod),
		errors.Is(err, domainoperator.ErrInvalidAuthEventType),
		errors.Is(err, domainoperator.ErrInvalidActionOriginType),
		errors.Is(err, domainoperator.ErrInvalidSessionEndStatus),
		errors.Is(err, domainoperator.ErrInvalidSessionExpiry),
		errors.Is(err, domainoperator.ErrSessionMachineMismatch),
		errors.Is(err, domainoperator.ErrMachineContextRequired),
		errors.Is(err, domainoperator.ErrTimeoutNotApplicable):
		writeAPIError(w, ctx, http.StatusBadRequest, "bad_request", err.Error())
	case errors.Is(err, domainoperator.ErrOrganizationMismatch):
		writeAPIError(w, ctx, http.StatusForbidden, "forbidden", err.Error())
	case errors.Is(err, domainoperator.ErrSessionNotFound):
		writeAPIError(w, ctx, http.StatusNotFound, "not_found", err.Error())
	case errors.Is(err, domainoperator.ErrSessionNotActive):
		writeAPIError(w, ctx, http.StatusConflict, "session_not_active", err.Error())
	case errors.Is(err, domainoperator.ErrActiveSessionExists):
		writeAPIError(w, ctx, http.StatusConflict, "active_session_exists", err.Error())
	case errors.Is(err, domainoperator.ErrAdminTakeoverUnauthorized):
		writeAPIError(w, ctx, http.StatusForbidden, "admin_takeover_forbidden", err.Error())
	case errors.Is(err, domainoperator.ErrTechnicianNotAssignedToMachine):
		writeAPIError(w, ctx, http.StatusForbidden, "technician_not_assigned", err.Error())
	case errors.Is(err, domainoperator.ErrTechnicianAssignmentCheckerRequired):
		writeAPIError(w, ctx, http.StatusServiceUnavailable, "assignment_checker_misconfigured", err.Error())
	default:
		if errors.Is(err, pgx.ErrNoRows) {
			writeAPIError(w, ctx, http.StatusNotFound, "not_found", "not found")
			return
		}
		writeAPIError(w, ctx, http.StatusInternalServerError, "internal", err.Error())
	}
}

// mountSetupBootstrapRoutes registers GET /setup/machines/{machineId}/bootstrap under the /v1 router.
func mountSetupBootstrapRoutes(r chi.Router, app *api.HTTPApplication) {
	if app == nil {
		return
	}
	r.With(auth.RequireMachineURLAccess("machineId")).Get("/setup/machines/{machineId}/bootstrap", getMachineSetupBootstrap(app))
}

func getMachineSetupBootstrap(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		machineID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "machineId")))
		if err != nil || machineID == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_machine_id", "invalid machineId")
			return
		}
		if app.TelemetryStore == nil || app.TelemetryStore.Pool() == nil {
			writeCapabilityNotConfigured(w, r.Context(), "database", "database pool is not configured for this API process")
			return
		}
		repo := postgres.NewSetupRepository(app.TelemetryStore.Pool())
		b, err := repo.GetMachineBootstrap(r.Context(), machineID)
		if err != nil {
			if errors.Is(err, setupapp.ErrNotFound) {
				writeAPIError(w, r.Context(), http.StatusNotFound, "machine_not_found", "machine not found")
				return
			}
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, buildSetupBootstrapV1(b))
	}
}

func buildSetupBootstrapV1(b setupapp.MachineBootstrap) V1SetupMachineBootstrapResponse {
	byCab := make(map[string][]setupapp.CabinetSlotConfigView)
	for _, s := range b.CurrentCabinetSlots {
		byCab[s.CabinetCode] = append(byCab[s.CabinetCode], s)
	}
	cabinets := make([]V1SetupTopologyCabinet, 0, len(b.Cabinets))
	for _, c := range b.Cabinets {
		slots := byCab[c.Code]
		if slots == nil {
			slots = []setupapp.CabinetSlotConfigView{}
		}
		sk := make([]V1SetupTopologySlot, 0, len(slots))
		for _, sl := range slots {
			var pid *string
			if sl.ProductID != nil {
				s := sl.ProductID.String()
				pid = &s
			}
			var idx *int32
			if sl.SlotIndex != nil {
				v := *sl.SlotIndex
				idx = &v
			}
			sk = append(sk, V1SetupTopologySlot{
				ConfigID:          sl.ConfigID.String(),
				SlotCode:          sl.SlotCode,
				SlotIndex:         idx,
				ProductID:         pid,
				ProductSku:        sl.ProductSKU,
				ProductName:       sl.ProductName,
				MaxQuantity:       sl.MaxQuantity,
				PriceMinor:        sl.PriceMinor,
				EffectiveFrom:     sl.EffectiveFrom.UTC().Format(time.RFC3339Nano),
				IsCurrent:         sl.IsCurrent,
				MachineSlotLayout: sl.MachineSlotLayout.String(),
				Metadata:          json.RawMessage("{}"),
			})
		}
		cabinets = append(cabinets, V1SetupTopologyCabinet{
			ID:        c.ID.String(),
			Code:      c.Code,
			Title:     c.Title,
			SortOrder: c.SortOrder,
			Metadata:  rawJSONMeta(c.Metadata),
			Slots:     sk,
		})
	}
	products := make([]V1SetupCatalogProduct, 0, len(b.AssortmentProducts))
	for _, p := range b.AssortmentProducts {
		products = append(products, V1SetupCatalogProduct{
			ProductID:      p.ProductID.String(),
			Sku:            p.SKU,
			Name:           p.Name,
			SortOrder:      p.SortOrder,
			AssortmentID:   p.AssortmentID.String(),
			AssortmentName: p.AssortmentName,
		})
	}
	m := b.Machine
	var hw *string
	if m.HardwareProfileID != nil {
		s := m.HardwareProfileID.String()
		hw = &s
	}
	return V1SetupMachineBootstrapResponse{
		Machine: V1SetupMachineSummary{
			MachineID:         m.ID.String(),
			OrganizationID:    m.OrganizationID.String(),
			SiteID:            m.SiteID.String(),
			HardwareProfileID: hw,
			SerialNumber:      m.SerialNumber,
			Name:              m.Name,
			Status:            m.Status,
			CommandSequence:   m.CommandSequence,
			CreatedAt:         m.CreatedAt.UTC().Format(time.RFC3339Nano),
			UpdatedAt:         m.UpdatedAt.UTC().Format(time.RFC3339Nano),
		},
		Topology: V1SetupTopology{Cabinets: cabinets},
		Catalog:  V1SetupCatalog{Products: products},
	}
}

func rawJSONMeta(b []byte) json.RawMessage {
	if len(b) == 0 || !json.Valid(b) {
		return json.RawMessage("{}")
	}
	return json.RawMessage(b)
}
