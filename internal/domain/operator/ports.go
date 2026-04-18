package operator

import (
	"context"

	"github.com/google/uuid"
)

// Repository persists operator sessions and related audit rows.
type Repository interface {
	StartOperatorSession(ctx context.Context, in StartOperatorSessionParams) (Session, error)
	GetOperatorSessionByID(ctx context.Context, id uuid.UUID) (Session, error)
	GetActiveSessionByMachineID(ctx context.Context, machineID uuid.UUID) (Session, error)
	EndOperatorSession(ctx context.Context, in EndOperatorSessionParams) (Session, error)
	TouchOperatorSessionActivity(ctx context.Context, sessionID uuid.UUID) (Session, error)
	TimeoutOperatorSessionIfExpired(ctx context.Context, sessionID uuid.UUID) (Session, error)
	InsertAuthEvent(ctx context.Context, in InsertAuthEventParams) (AuthEvent, error)
	InsertActionAttribution(ctx context.Context, in InsertActionAttributionParams) (ActionAttribution, error)
	ListSessionsByMachineID(ctx context.Context, machineID uuid.UUID, limit int32) ([]Session, error)
	ListSessionsByTechnicianID(ctx context.Context, technicianID uuid.UUID, limit int32) ([]Session, error)
	ListSessionsByUserPrincipal(ctx context.Context, in ListSessionsParams) ([]Session, error)
	ListAuthEventsByMachineID(ctx context.Context, machineID uuid.UUID, limit int32) ([]AuthEvent, error)
	ListActionAttributionsByMachineID(ctx context.Context, machineID uuid.UUID, limit int32) ([]ActionAttribution, error)
	ListActionAttributionsByMachineAndResource(ctx context.Context, machineID uuid.UUID, resourceType, resourceID string, limit int32) ([]ActionAttribution, error)
	ListActionAttributionsForTechnician(ctx context.Context, organizationID, technicianID uuid.UUID, limit int32) ([]ActionAttribution, error)
	ListActionAttributionsForUserPrincipal(ctx context.Context, organizationID uuid.UUID, userPrincipal string, limit int32) ([]ActionAttribution, error)
}
