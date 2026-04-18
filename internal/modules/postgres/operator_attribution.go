package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	domainoperator "github.com/avf/avf-vending-api/internal/domain/operator"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/google/uuid"
)

// operatorAttributionSpec records one machine_action_attributions row tied to an operator session.
// Extended fields (organization_id, action_domain, actor, resource_table) live in metadata JSON
// alongside DB columns resource_type/resource_id.
type operatorAttributionSpec struct {
	MachineID         uuid.UUID
	OrganizationID    uuid.UUID // optional: when non-nil, must match the session's organization
	OperatorSessionID *uuid.UUID
	ActionDomain      string
	ActionType        string
	ResourceTable     string // stored as machine_action_attributions.resource_type
	ResourceID        string // text polymorphic id (uuid string)
	CorrelationID     *uuid.UUID
	OccurredAt        *time.Time
}

// insertOperatorSessionAttribution is a no-op when OperatorSessionID is nil.
// It validates the session belongs to the same machine (and organization when OrganizationID is set),
// then inserts machine_action_attributions in the same transaction as the caller's Querier.
//
// P0 safety: spec.MachineID must match the session row's machine_id. Do not bypass this helper to
// insert operator_session_id on a different machine's domain row — that would corrupt audit trails
// and break operator-insight queries that assume session and machine_action_attributions.machine_id align.
func insertOperatorSessionAttribution(ctx context.Context, q *db.Queries, spec operatorAttributionSpec) error {
	if spec.OperatorSessionID == nil || *spec.OperatorSessionID == uuid.Nil {
		return nil
	}
	sess, err := q.GetOperatorSessionByID(ctx, *spec.OperatorSessionID)
	if err != nil {
		if isNoRows(err) {
			return fmt.Errorf("operator attribution: %w", domainoperator.ErrSessionNotFound)
		}
		return fmt.Errorf("operator attribution: load session: %w", err)
	}
	if sess.MachineID != spec.MachineID {
		return errors.New("postgres: operator session does not match machine")
	}
	if spec.OrganizationID != uuid.Nil && sess.OrganizationID != spec.OrganizationID {
		return errors.New("postgres: operator session does not match organization")
	}

	meta := map[string]any{
		"organization_id":     sess.OrganizationID.String(),
		"machine_id":          sess.MachineID.String(),
		"operator_session_id": sess.ID.String(),
		"actor_type":          sess.ActorType,
		"action_origin":       domainoperator.ActionOriginOperatorSession,
		"action_domain":       spec.ActionDomain,
		"action_type":         spec.ActionType,
		"resource_table":      spec.ResourceTable,
		"resource_id":         spec.ResourceID,
	}
	if sess.TechnicianID != nil {
		meta["technician_id"] = sess.TechnicianID.String()
	}
	if sess.UserPrincipal != nil && *sess.UserPrincipal != "" {
		meta["user_principal"] = *sess.UserPrincipal
		meta["user_id"] = *sess.UserPrincipal
	}

	metaBytes, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("operator attribution: metadata: %w", err)
	}

	occ := spec.OccurredAt
	if occ == nil {
		now := time.Now().UTC()
		occ = &now
	}

	_, err = q.InsertMachineActionAttribution(ctx, db.InsertMachineActionAttributionParams{
		OperatorSessionID: spec.OperatorSessionID,
		MachineID:         spec.MachineID,
		ActionOriginType:  domainoperator.ActionOriginOperatorSession,
		ResourceType:      spec.ResourceTable,
		ResourceID:        spec.ResourceID,
		OccurredAt:        occ,
		Metadata:          metaBytes,
		CorrelationID:     spec.CorrelationID,
	})
	if err != nil {
		return fmt.Errorf("operator attribution: insert: %w", err)
	}
	return nil
}
