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
	appfleet "github.com/avf/avf-vending-api/internal/app/fleet"
	domainoperator "github.com/avf/avf-vending-api/internal/domain/operator"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/avf/avf-vending-api/internal/platform/pgjson"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type v1AdminMachineLifecycleRequest struct {
	Reason            string          `json:"reason"`
	Notes             string          `json:"notes"`
	OperatorSessionID string          `json:"operator_session_id"`
	OperatorSession   string          `json:"operatorSessionId"`
	CorrelationID     string          `json:"correlation_id"`
	CorrelationCamel  string          `json:"correlationId"`
	Metadata          json.RawMessage `json:"metadata"`
}

func parseLifecycleMutation(r *http.Request) (appfleet.LifecycleMutationInput, bool, error) {
	var body v1AdminMachineLifecycleRequest
	if r.Body != nil {
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		if err := dec.Decode(&body); err != nil && !errors.Is(err, io.EOF) {
			return appfleet.LifecycleMutationInput{}, false, err
		}
	}
	in := appfleet.LifecycleMutationInput{
		Reason: strings.TrimSpace(body.Reason),
		Notes:  strings.TrimSpace(body.Notes),
	}
	if p, ok := auth.PrincipalFromContext(r.Context()); ok {
		if aid, err := uuid.Parse(strings.TrimSpace(p.Subject)); err == nil {
			in.ActorAccountID = aid
		}
	}
	if rid := strings.TrimSpace(r.Header.Get("X-Request-Id")); rid != "" {
		in.RequestID = rid
	}
	osRaw := strings.TrimSpace(body.OperatorSessionID)
	if osRaw == "" {
		osRaw = strings.TrimSpace(body.OperatorSession)
	}
	if osRaw != "" {
		if sid, err := uuid.Parse(osRaw); err == nil && sid != uuid.Nil {
			in.OperatorSessionID = &sid
		}
	}
	corrRaw := strings.TrimSpace(body.CorrelationID)
	if corrRaw == "" {
		corrRaw = strings.TrimSpace(body.CorrelationCamel)
	}
	if corrRaw != "" {
		if cid, err := uuid.Parse(corrRaw); err == nil && cid != uuid.Nil {
			in.CorrelationID = &cid
		}
	}
	if len(body.Metadata) > 0 {
		_ = json.Unmarshal(body.Metadata, &in.Metadata)
	}
	technicianOrigin := false
	if p, ok := auth.PrincipalFromContext(r.Context()); ok {
		technicianOrigin = p.HasRole(auth.RoleTechnician) && !p.HasRole(auth.RolePlatformAdmin) && !p.HasRole(auth.RoleOrgAdmin)
	}
	return in, technicianOrigin, nil
}

func writeLifecycleMutationError(w http.ResponseWriter, ctx context.Context, err error) {
	switch {
	case errors.Is(err, appfleet.ErrLifecycleReasonRequired):
		writeAPIError(w, ctx, http.StatusBadRequest, "reason_required", "reason is required")
	case errors.Is(err, appfleet.ErrOperatorSessionRequired):
		writeAPIError(w, ctx, http.StatusBadRequest, "operator_session_required", "operator_session_id is required for technician actions")
	default:
		writeFleetAppError(w, ctx, err)
	}
}

func lifecycleMutationJSON(machineID uuid.UUID, out appfleet.LifecycleMutationOutcome) map[string]any {
	resp := map[string]any{
		"machine_id":                machineID.String(),
		"previous_status":           out.Result.PreviousStatus,
		"new_status":                out.Result.NewStatus,
		"credential_version":        out.Result.CredentialVersion,
		"sessions_revoked_count":    out.Result.SessionsRevokedCount,
		"credentials_revoked_count": out.Result.CredentialsRevokedCount,
		"reason":                    out.Result.Reason,
		"occurred_at":               out.Result.OccurredAt.UTC().Format(time.RFC3339Nano),
	}
	if out.Result.ActorAccountID != uuid.Nil {
		resp["actor_account_id"] = out.Result.ActorAccountID.String()
	}
	if out.Result.OperatorSessionID != nil {
		resp["operator_session_id"] = out.Result.OperatorSessionID.String()
	}
	if out.Result.CorrelationID != nil {
		resp["correlation_id"] = out.Result.CorrelationID.String()
	}
	return resp
}

func recordLifecycleAuditAndAttribution(ctx context.Context, app *api.HTTPApplication, org uuid.UUID, action string, machineID uuid.UUID, in appfleet.LifecycleMutationInput, out appfleet.LifecycleMutationOutcome) {
	if app == nil {
		return
	}
	meta := map[string]any{
		"reason":          out.Result.Reason,
		"previous_status": out.Result.PreviousStatus,
		"new_status":      out.Result.NewStatus,
		"request_id":      in.RequestID,
	}
	if in.Notes != "" {
		meta["notes"] = in.Notes
	}
	if in.Metadata != nil {
		meta["metadata"] = in.Metadata
	}
	if out.Result.CorrelationID != nil {
		meta["correlation_id"] = out.Result.CorrelationID.String()
	}
	midStr := machineID.String()
	fleetAudit(ctx, app, org, action, "fleet.machine", &midStr, meta)
	if app.TelemetryStore == nil || in.OperatorSessionID == nil {
		return
	}
	pool := app.TelemetryStore.Pool()
	if pool == nil {
		return
	}
	q := db.New(pool)
	metaBytes, _ := json.Marshal(map[string]any{
		"action":          action,
		"previous_status": out.Result.PreviousStatus,
		"new_status":      out.Result.NewStatus,
		"reason":          out.Result.Reason,
	})
	_, _ = q.InsertMachineActionAttribution(ctx, db.InsertMachineActionAttributionParams{
		OccurredAt:        out.Result.OccurredAt,
		OperatorSessionID: pgtype.UUID{Bytes: *in.OperatorSessionID, Valid: true},
		MachineID:         machineID,
		ActionOriginType:  domainoperator.ActionOriginOperatorSession,
		ResourceType:      "fleet.machine",
		ResourceID:        machineID.String(),
		Metadata:          pgjson.RequiredString(metaBytes),
		CorrelationID:     optionalUUIDPg(out.Result.CorrelationID),
	})
}

func optionalUUIDPg(id *uuid.UUID) pgtype.UUID {
	if id == nil || *id == uuid.Nil {
		return pgtype.UUID{}
	}
	return pgtype.UUID{Bytes: *id, Valid: true}
}
