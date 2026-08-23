package httpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/avf/avf-vending-api/internal/app/api"
	appoperator "github.com/avf/avf-vending-api/internal/app/operator"
	domainoperator "github.com/avf/avf-vending-api/internal/domain/operator"
	appmw "github.com/avf/avf-vending-api/internal/middleware"
	"github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/google/uuid"
)

// parseOptionalOperatorSessionIDField accepts omitted/empty operator_session_id.
// A present value must be a non-zero UUID; the zero UUID is rejected.
func parseOptionalOperatorSessionIDField(w http.ResponseWriter, r *http.Request, raw string) (*uuid.UUID, bool) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return nil, true
	}
	id, err := uuid.Parse(s)
	if err != nil || id == uuid.Nil {
		writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_operator_session_id", "operator_session_id must be a UUID")
		return nil, false
	}
	return &id, true
}

func resolveAdminMachineAction(w http.ResponseWriter, r *http.Request, app *api.HTTPApplication, machineID uuid.UUID, rawSession string, policy domainoperator.SessionPolicy) (appoperator.MachineActionActor, bool) {
	supplied, ok := parseOptionalOperatorSessionIDField(w, r, rawSession)
	if !ok {
		return appoperator.MachineActionActor{}, false
	}
	if app == nil || app.MachineOperator == nil {
		writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", "operator service not configured")
		return appoperator.MachineActionActor{}, false
	}
	actor, err := app.MachineOperator.ResolveMachineActionActor(r.Context(), machineID, supplied, policy)
	if err != nil {
		writeOperatorError(w, r.Context(), err)
		return appoperator.MachineActionActor{}, false
	}
	return actor, true
}

func recordResolvedMachineActionAttribution(ctx context.Context, app *api.HTTPApplication, actor appoperator.MachineActionActor, resourceType, resourceID, actionDomain, actionType string) error {
	if app == nil || app.MachineOperator == nil {
		return domainoperator.ErrOperatorSessionRequired
	}
	p, _ := auth.PrincipalFromContext(ctx)
	actorType, actorID := p.Actor()
	meta := map[string]any{
		"machine_id":    actor.MachineID.String(),
		"action_origin": actor.Origin,
		"action_domain": actionDomain,
		"action_type":   actionType,
		"resource_type": resourceType,
		"resource_id":   resourceID,
		"actor_subject": actorID,
		"actor_type":    actorType,
		"roles":         p.Roles,
	}
	if actor.OperatorSessionID != nil {
		meta["operator_session_id"] = actor.OperatorSessionID.String()
	}
	if corr := strings.TrimSpace(appmw.CorrelationIDFromContext(ctx)); corr != "" {
		meta["http_correlation"] = corr
	}
	raw, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	_, err = app.MachineOperator.RecordActionAttribution(ctx, appoperator.RecordActionAttributionInput{
		OperatorSessionID: actor.OperatorSessionID,
		MachineID:         actor.MachineID,
		ActionOriginType:  actor.Origin,
		ResourceType:      resourceType,
		ResourceID:        resourceID,
		CorrelationID:     correlationUUIDFromRequest(ctx),
		Metadata:          raw,
	})
	return err
}

func recordResolvedMachineActionAttributionOrFail(w http.ResponseWriter, r *http.Request, app *api.HTTPApplication, actor appoperator.MachineActionActor, resourceType, resourceID, actionDomain, actionType string) bool {
	if err := recordResolvedMachineActionAttribution(r.Context(), app, actor, resourceType, resourceID, actionDomain, actionType); err != nil {
		writeAPIError(w, r.Context(), http.StatusInternalServerError, "action_attribution_failed", err.Error())
		return false
	}
	return true
}
