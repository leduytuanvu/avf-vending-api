package db

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type operatorSessionScanner interface {
	Scan(dest ...any) error
}

func scanMachineOperatorSession(row operatorSessionScanner) (MachineOperatorSession, error) {
	var s MachineOperatorSession
	err := row.Scan(
		&s.ID,
		&s.OrganizationID,
		&s.MachineID,
		&s.ActorType,
		&s.TechnicianID,
		&s.UserPrincipal,
		&s.Status,
		&s.StartedAt,
		&s.EndedAt,
		&s.ExpiresAt,
		&s.ClientMetadata,
		&s.LastActivityAt,
		&s.EndedReason,
		&s.CreatedAt,
		&s.UpdatedAt,
	)
	return s, err
}

const getOperatorSessionByID = `-- name: GetOperatorSessionByID :one
SELECT
    id,
    organization_id,
    machine_id,
    actor_type,
    technician_id,
    user_principal,
    status,
    started_at,
    ended_at,
    expires_at,
    client_metadata,
    last_activity_at,
    ended_reason,
    created_at,
    updated_at
FROM machine_operator_sessions
WHERE
    id = $1
`

func (q *Queries) GetOperatorSessionByID(ctx context.Context, id uuid.UUID) (MachineOperatorSession, error) {
	return scanMachineOperatorSession(q.db.QueryRow(ctx, getOperatorSessionByID, id))
}

const getOperatorSessionByIDForUpdate = `-- name: GetOperatorSessionByIDForUpdate :one
SELECT
    id,
    organization_id,
    machine_id,
    actor_type,
    technician_id,
    user_principal,
    status,
    started_at,
    ended_at,
    expires_at,
    client_metadata,
    last_activity_at,
    ended_reason,
    created_at,
    updated_at
FROM machine_operator_sessions
WHERE
    id = $1
FOR UPDATE
`

func (q *Queries) GetOperatorSessionByIDForUpdate(ctx context.Context, id uuid.UUID) (MachineOperatorSession, error) {
	return scanMachineOperatorSession(q.db.QueryRow(ctx, getOperatorSessionByIDForUpdate, id))
}

const getActiveOperatorSessionByMachineID = `-- name: GetActiveOperatorSessionByMachineID :one
SELECT
    id,
    organization_id,
    machine_id,
    actor_type,
    technician_id,
    user_principal,
    status,
    started_at,
    ended_at,
    expires_at,
    client_metadata,
    last_activity_at,
    ended_reason,
    created_at,
    updated_at
FROM machine_operator_sessions
WHERE
    machine_id = $1
    AND status = 'ACTIVE'
LIMIT
    1
`

func (q *Queries) GetActiveOperatorSessionByMachineID(ctx context.Context, machineID uuid.UUID) (MachineOperatorSession, error) {
	return scanMachineOperatorSession(q.db.QueryRow(ctx, getActiveOperatorSessionByMachineID, machineID))
}

const getActiveOperatorSessionByMachineIDForUpdate = `-- name: GetActiveOperatorSessionByMachineIDForUpdate :one
SELECT
    id,
    organization_id,
    machine_id,
    actor_type,
    technician_id,
    user_principal,
    status,
    started_at,
    ended_at,
    expires_at,
    client_metadata,
    last_activity_at,
    ended_reason,
    created_at,
    updated_at
FROM machine_operator_sessions
WHERE
    machine_id = $1
    AND status = 'ACTIVE'
LIMIT
    1
FOR UPDATE
`

func (q *Queries) GetActiveOperatorSessionByMachineIDForUpdate(ctx context.Context, machineID uuid.UUID) (MachineOperatorSession, error) {
	return scanMachineOperatorSession(q.db.QueryRow(ctx, getActiveOperatorSessionByMachineIDForUpdate, machineID))
}

type ResumeActiveOperatorSessionForActorParams struct {
	MachineID      uuid.UUID
	OrganizationID uuid.UUID
	ActorType      string
	TechnicianID   *uuid.UUID
	UserPrincipal  *string
	ExpiresAt      *time.Time
	ClientMetadata []byte
}

const resumeActiveOperatorSessionForActor = `-- name: ResumeActiveOperatorSessionForActor :one
UPDATE machine_operator_sessions
SET
    updated_at = now(),
    last_activity_at = now(),
    expires_at = COALESCE($6, expires_at),
    client_metadata = $7
WHERE
    machine_id = $1
    AND organization_id = $2
    AND status = 'ACTIVE'
    AND actor_type = $3
    AND technician_id IS NOT DISTINCT FROM $4
    AND user_principal IS NOT DISTINCT FROM $5
RETURNING
    id,
    organization_id,
    machine_id,
    actor_type,
    technician_id,
    user_principal,
    status,
    started_at,
    ended_at,
    expires_at,
    client_metadata,
    last_activity_at,
    ended_reason,
    created_at,
    updated_at
`

func (q *Queries) ResumeActiveOperatorSessionForActor(ctx context.Context, arg ResumeActiveOperatorSessionForActorParams) (MachineOperatorSession, error) {
	return scanMachineOperatorSession(q.db.QueryRow(ctx, resumeActiveOperatorSessionForActor,
		arg.MachineID,
		arg.OrganizationID,
		arg.ActorType,
		arg.TechnicianID,
		arg.UserPrincipal,
		arg.ExpiresAt,
		arg.ClientMetadata,
	))
}

type InsertMachineOperatorSessionParams struct {
	OrganizationID uuid.UUID
	MachineID      uuid.UUID
	ActorType      string
	TechnicianID   *uuid.UUID
	UserPrincipal  *string
	Status         string
	ExpiresAt      *time.Time
	ClientMetadata []byte
}

const insertMachineOperatorSession = `-- name: InsertMachineOperatorSession :one
INSERT INTO machine_operator_sessions (
    organization_id,
    machine_id,
    actor_type,
    technician_id,
    user_principal,
    status,
    expires_at,
    client_metadata
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6,
    $7,
    $8
)
RETURNING
    id,
    organization_id,
    machine_id,
    actor_type,
    technician_id,
    user_principal,
    status,
    started_at,
    ended_at,
    expires_at,
    client_metadata,
    last_activity_at,
    ended_reason,
    created_at,
    updated_at
`

func (q *Queries) InsertMachineOperatorSession(ctx context.Context, arg InsertMachineOperatorSessionParams) (MachineOperatorSession, error) {
	return scanMachineOperatorSession(q.db.QueryRow(ctx, insertMachineOperatorSession,
		arg.OrganizationID,
		arg.MachineID,
		arg.ActorType,
		arg.TechnicianID,
		arg.UserPrincipal,
		arg.Status,
		arg.ExpiresAt,
		arg.ClientMetadata,
	))
}

type EndMachineOperatorSessionParams struct {
	ID          uuid.UUID
	Status      string
	EndedAt     time.Time
	EndedReason *string
}

const endMachineOperatorSession = `-- name: EndMachineOperatorSession :one
UPDATE machine_operator_sessions
SET
    status = $2,
    ended_at = $3,
    updated_at = $3,
    ended_reason = $4
WHERE
    id = $1
    AND status = 'ACTIVE'
RETURNING
    id,
    organization_id,
    machine_id,
    actor_type,
    technician_id,
    user_principal,
    status,
    started_at,
    ended_at,
    expires_at,
    client_metadata,
    last_activity_at,
    ended_reason,
    created_at,
    updated_at
`

func (q *Queries) EndMachineOperatorSession(ctx context.Context, arg EndMachineOperatorSessionParams) (MachineOperatorSession, error) {
	return scanMachineOperatorSession(q.db.QueryRow(ctx, endMachineOperatorSession, arg.ID, arg.Status, arg.EndedAt, arg.EndedReason))
}

const touchMachineOperatorSessionActivity = `-- name: TouchMachineOperatorSessionActivity :one
UPDATE machine_operator_sessions
SET
    updated_at = now(),
    last_activity_at = now()
WHERE
    id = $1
    AND status = 'ACTIVE'
RETURNING
    id,
    organization_id,
    machine_id,
    actor_type,
    technician_id,
    user_principal,
    status,
    started_at,
    ended_at,
    expires_at,
    client_metadata,
    last_activity_at,
    ended_reason,
    created_at,
    updated_at
`

func (q *Queries) TouchMachineOperatorSessionActivity(ctx context.Context, id uuid.UUID) (MachineOperatorSession, error) {
	return scanMachineOperatorSession(q.db.QueryRow(ctx, touchMachineOperatorSessionActivity, id))
}

const timeoutMachineOperatorSessionIfExpired = `-- name: TimeoutMachineOperatorSessionIfExpired :one
UPDATE machine_operator_sessions
SET
    status = 'EXPIRED',
    ended_at = now(),
    updated_at = now(),
    ended_reason = 'session_expired'
WHERE
    id = $1
    AND status = 'ACTIVE'
    AND expires_at IS NOT NULL
    AND expires_at <= now()
RETURNING
    id,
    organization_id,
    machine_id,
    actor_type,
    technician_id,
    user_principal,
    status,
    started_at,
    ended_at,
    expires_at,
    client_metadata,
    last_activity_at,
    ended_reason,
    created_at,
    updated_at
`

func (q *Queries) TimeoutMachineOperatorSessionIfExpired(ctx context.Context, id uuid.UUID) (MachineOperatorSession, error) {
	return scanMachineOperatorSession(q.db.QueryRow(ctx, timeoutMachineOperatorSessionIfExpired, id))
}

type InsertMachineOperatorAuthEventParams struct {
	OperatorSessionID *uuid.UUID
	MachineID         uuid.UUID
	EventType         string
	AuthMethod        string
	OccurredAt        *time.Time
	CorrelationID     *uuid.UUID
	Metadata          []byte
}

const insertMachineOperatorAuthEvent = `-- name: InsertMachineOperatorAuthEvent :one
INSERT INTO machine_operator_auth_events (
    operator_session_id,
    machine_id,
    event_type,
    auth_method,
    occurred_at,
    correlation_id,
    metadata
) VALUES (
    $1,
    $2,
    $3,
    $4,
    COALESCE($5::timestamptz, now()),
    $6,
    $7
)
RETURNING
    id,
    operator_session_id,
    machine_id,
    event_type,
    auth_method,
    occurred_at,
    correlation_id,
    metadata
`

func (q *Queries) InsertMachineOperatorAuthEvent(ctx context.Context, arg InsertMachineOperatorAuthEventParams) (MachineOperatorAuthEvent, error) {
	row := q.db.QueryRow(ctx, insertMachineOperatorAuthEvent,
		arg.OperatorSessionID,
		arg.MachineID,
		arg.EventType,
		arg.AuthMethod,
		arg.OccurredAt,
		arg.CorrelationID,
		arg.Metadata,
	)
	var e MachineOperatorAuthEvent
	err := row.Scan(
		&e.ID,
		&e.OperatorSessionID,
		&e.MachineID,
		&e.EventType,
		&e.AuthMethod,
		&e.OccurredAt,
		&e.CorrelationID,
		&e.Metadata,
	)
	return e, err
}

type InsertMachineActionAttributionParams struct {
	OperatorSessionID *uuid.UUID
	MachineID         uuid.UUID
	ActionOriginType  string
	ResourceType      string
	ResourceID        string
	OccurredAt        *time.Time
	Metadata          []byte
	CorrelationID     *uuid.UUID
}

const insertMachineActionAttribution = `-- name: InsertMachineActionAttribution :one
INSERT INTO machine_action_attributions (
    operator_session_id,
    machine_id,
    action_origin_type,
    resource_type,
    resource_id,
    occurred_at,
    metadata,
    correlation_id
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    COALESCE($6::timestamptz, now()),
    $7,
    $8
)
RETURNING
    id,
    operator_session_id,
    machine_id,
    action_origin_type,
    resource_type,
    resource_id,
    occurred_at,
    metadata,
    correlation_id
`

func (q *Queries) InsertMachineActionAttribution(ctx context.Context, arg InsertMachineActionAttributionParams) (MachineActionAttribution, error) {
	row := q.db.QueryRow(ctx, insertMachineActionAttribution,
		arg.OperatorSessionID,
		arg.MachineID,
		arg.ActionOriginType,
		arg.ResourceType,
		arg.ResourceID,
		arg.OccurredAt,
		arg.Metadata,
		arg.CorrelationID,
	)
	var a MachineActionAttribution
	err := row.Scan(
		&a.ID,
		&a.OperatorSessionID,
		&a.MachineID,
		&a.ActionOriginType,
		&a.ResourceType,
		&a.ResourceID,
		&a.OccurredAt,
		&a.Metadata,
		&a.CorrelationID,
	)
	return a, err
}

const listOperatorSessionsByMachineID = `-- name: ListOperatorSessionsByMachineID :many
SELECT
    id,
    organization_id,
    machine_id,
    actor_type,
    technician_id,
    user_principal,
    status,
    started_at,
    ended_at,
    expires_at,
    client_metadata,
    last_activity_at,
    ended_reason,
    created_at,
    updated_at
FROM machine_operator_sessions
WHERE
    machine_id = $1
ORDER BY started_at DESC
LIMIT $2
`

func (q *Queries) ListOperatorSessionsByMachineID(ctx context.Context, arg ListOperatorSessionsByMachineIDParams) ([]MachineOperatorSession, error) {
	rows, err := q.db.Query(ctx, listOperatorSessionsByMachineID, arg.MachineID, arg.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanMachineOperatorSessionRows(rows)
}

type ListOperatorSessionsByMachineIDParams struct {
	MachineID uuid.UUID
	Limit     int32
}

const listOperatorSessionsByTechnicianID = `-- name: ListOperatorSessionsByTechnicianID :many
SELECT
    id,
    organization_id,
    machine_id,
    actor_type,
    technician_id,
    user_principal,
    status,
    started_at,
    ended_at,
    expires_at,
    client_metadata,
    last_activity_at,
    ended_reason,
    created_at,
    updated_at
FROM machine_operator_sessions
WHERE
    technician_id = $1
ORDER BY started_at DESC
LIMIT $2
`

func (q *Queries) ListOperatorSessionsByTechnicianID(ctx context.Context, arg ListOperatorSessionsByTechnicianIDParams) ([]MachineOperatorSession, error) {
	rows, err := q.db.Query(ctx, listOperatorSessionsByTechnicianID, arg.TechnicianID, arg.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanMachineOperatorSessionRows(rows)
}

type ListOperatorSessionsByTechnicianIDParams struct {
	TechnicianID uuid.UUID
	Limit        int32
}

const listOperatorSessionsByUserPrincipal = `-- name: ListOperatorSessionsByUserPrincipal :many
SELECT
    id,
    organization_id,
    machine_id,
    actor_type,
    technician_id,
    user_principal,
    status,
    started_at,
    ended_at,
    expires_at,
    client_metadata,
    last_activity_at,
    ended_reason,
    created_at,
    updated_at
FROM machine_operator_sessions
WHERE
    organization_id = $1
    AND actor_type = 'USER'
    AND user_principal = $2
ORDER BY started_at DESC
LIMIT $3
`

type ListOperatorSessionsByUserPrincipalParams struct {
	OrganizationID uuid.UUID
	UserPrincipal  string
	Limit          int32
}

func (q *Queries) ListOperatorSessionsByUserPrincipal(ctx context.Context, arg ListOperatorSessionsByUserPrincipalParams) ([]MachineOperatorSession, error) {
	rows, err := q.db.Query(ctx, listOperatorSessionsByUserPrincipal, arg.OrganizationID, arg.UserPrincipal, arg.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanMachineOperatorSessionRows(rows)
}

func scanMachineOperatorSessionRows(rows interface {
	Close()
	Err() error
	Next() bool
	Scan(dest ...any) error
}) ([]MachineOperatorSession, error) {
	var out []MachineOperatorSession
	for rows.Next() {
		var s MachineOperatorSession
		if err := rows.Scan(
			&s.ID,
			&s.OrganizationID,
			&s.MachineID,
			&s.ActorType,
			&s.TechnicianID,
			&s.UserPrincipal,
			&s.Status,
			&s.StartedAt,
			&s.EndedAt,
			&s.ExpiresAt,
			&s.ClientMetadata,
			&s.LastActivityAt,
			&s.EndedReason,
			&s.CreatedAt,
			&s.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

type ListMachineOperatorAuthEventsByMachineIDParams struct {
	MachineID uuid.UUID
	Limit     int32
}

const listMachineOperatorAuthEventsByMachineID = `-- name: ListMachineOperatorAuthEventsByMachineID :many
SELECT
    id,
    operator_session_id,
    machine_id,
    event_type,
    auth_method,
    occurred_at,
    correlation_id,
    metadata
FROM machine_operator_auth_events
WHERE
    machine_id = $1
ORDER BY occurred_at DESC
LIMIT $2
`

func (q *Queries) ListMachineOperatorAuthEventsByMachineID(ctx context.Context, arg ListMachineOperatorAuthEventsByMachineIDParams) ([]MachineOperatorAuthEvent, error) {
	rows, err := q.db.Query(ctx, listMachineOperatorAuthEventsByMachineID, arg.MachineID, arg.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []MachineOperatorAuthEvent
	for rows.Next() {
		var i MachineOperatorAuthEvent
		if err := rows.Scan(
			&i.ID,
			&i.OperatorSessionID,
			&i.MachineID,
			&i.EventType,
			&i.AuthMethod,
			&i.OccurredAt,
			&i.CorrelationID,
			&i.Metadata,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	return items, rows.Err()
}

type ListMachineActionAttributionsByMachineIDParams struct {
	MachineID uuid.UUID
	Limit     int32
}

const listMachineActionAttributionsByMachineID = `-- name: ListMachineActionAttributionsByMachineID :many
SELECT
    id,
    operator_session_id,
    machine_id,
    action_origin_type,
    resource_type,
    resource_id,
    occurred_at,
    metadata,
    correlation_id
FROM machine_action_attributions
WHERE
    machine_id = $1
ORDER BY occurred_at DESC
LIMIT $2
`

func (q *Queries) ListMachineActionAttributionsByMachineID(ctx context.Context, arg ListMachineActionAttributionsByMachineIDParams) ([]MachineActionAttribution, error) {
	rows, err := q.db.Query(ctx, listMachineActionAttributionsByMachineID, arg.MachineID, arg.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []MachineActionAttribution
	for rows.Next() {
		var i MachineActionAttribution
		if err := rows.Scan(
			&i.ID,
			&i.OperatorSessionID,
			&i.MachineID,
			&i.ActionOriginType,
			&i.ResourceType,
			&i.ResourceID,
			&i.OccurredAt,
			&i.Metadata,
			&i.CorrelationID,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	return items, rows.Err()
}

type ListMachineActionAttributionsByMachineAndResourceParams struct {
	MachineID    uuid.UUID
	ResourceType string
	ResourceID   string
	Limit        int32
}

const listMachineActionAttributionsByMachineAndResource = `-- name: ListMachineActionAttributionsByMachineAndResource :many
SELECT
    id,
    operator_session_id,
    machine_id,
    action_origin_type,
    resource_type,
    resource_id,
    occurred_at,
    metadata,
    correlation_id
FROM machine_action_attributions
WHERE
    machine_id = $1
    AND resource_type = $2
    AND resource_id = $3
ORDER BY occurred_at DESC
LIMIT $4
`

func (q *Queries) ListMachineActionAttributionsByMachineAndResource(ctx context.Context, arg ListMachineActionAttributionsByMachineAndResourceParams) ([]MachineActionAttribution, error) {
	rows, err := q.db.Query(ctx, listMachineActionAttributionsByMachineAndResource,
		arg.MachineID,
		arg.ResourceType,
		arg.ResourceID,
		arg.Limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []MachineActionAttribution
	for rows.Next() {
		var i MachineActionAttribution
		if err := rows.Scan(
			&i.ID,
			&i.OperatorSessionID,
			&i.MachineID,
			&i.ActionOriginType,
			&i.ResourceType,
			&i.ResourceID,
			&i.OccurredAt,
			&i.Metadata,
			&i.CorrelationID,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	return items, rows.Err()
}

type ListMachineActionAttributionsForTechnicianParams struct {
	TechnicianID   uuid.UUID
	OrganizationID uuid.UUID
	Limit          int32
}

const listMachineActionAttributionsForTechnician = `-- name: ListMachineActionAttributionsForTechnician :many
SELECT
    a.id,
    a.operator_session_id,
    a.machine_id,
    a.action_origin_type,
    a.resource_type,
    a.resource_id,
    a.occurred_at,
    a.metadata,
    a.correlation_id
FROM machine_action_attributions a
INNER JOIN machine_operator_sessions s ON s.id = a.operator_session_id
WHERE
    s.technician_id = $1
    AND s.organization_id = $2
ORDER BY
    a.occurred_at DESC
LIMIT $3
`

func (q *Queries) ListMachineActionAttributionsForTechnician(ctx context.Context, arg ListMachineActionAttributionsForTechnicianParams) ([]MachineActionAttribution, error) {
	rows, err := q.db.Query(ctx, listMachineActionAttributionsForTechnician, arg.TechnicianID, arg.OrganizationID, arg.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []MachineActionAttribution
	for rows.Next() {
		var i MachineActionAttribution
		if err := rows.Scan(
			&i.ID,
			&i.OperatorSessionID,
			&i.MachineID,
			&i.ActionOriginType,
			&i.ResourceType,
			&i.ResourceID,
			&i.OccurredAt,
			&i.Metadata,
			&i.CorrelationID,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	return items, rows.Err()
}

type ListMachineActionAttributionsForUserPrincipalParams struct {
	OrganizationID uuid.UUID
	UserPrincipal  string
	Limit          int32
}

const listMachineActionAttributionsForUserPrincipal = `-- name: ListMachineActionAttributionsForUserPrincipal :many
SELECT
    a.id,
    a.operator_session_id,
    a.machine_id,
    a.action_origin_type,
    a.resource_type,
    a.resource_id,
    a.occurred_at,
    a.metadata,
    a.correlation_id
FROM machine_action_attributions a
INNER JOIN machine_operator_sessions s ON s.id = a.operator_session_id
WHERE
    s.organization_id = $1
    AND s.actor_type = 'USER'
    AND s.user_principal = $2
ORDER BY
    a.occurred_at DESC
LIMIT $3
`

func (q *Queries) ListMachineActionAttributionsForUserPrincipal(ctx context.Context, arg ListMachineActionAttributionsForUserPrincipalParams) ([]MachineActionAttribution, error) {
	rows, err := q.db.Query(ctx, listMachineActionAttributionsForUserPrincipal, arg.OrganizationID, arg.UserPrincipal, arg.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []MachineActionAttribution
	for rows.Next() {
		var i MachineActionAttribution
		if err := rows.Scan(
			&i.ID,
			&i.OperatorSessionID,
			&i.MachineID,
			&i.ActionOriginType,
			&i.ResourceType,
			&i.ResourceID,
			&i.OccurredAt,
			&i.Metadata,
			&i.CorrelationID,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	return items, rows.Err()
}
