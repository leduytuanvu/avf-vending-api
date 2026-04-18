package db

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type InsertRefillSessionParams struct {
	OrganizationID    uuid.UUID
	MachineID         uuid.UUID
	StartedAt         time.Time
	EndedAt           *time.Time
	OperatorSessionID *uuid.UUID
	Metadata          []byte
}

const insertRefillSession = `-- name: InsertRefillSession :one
INSERT INTO refill_sessions (
    organization_id,
    machine_id,
    started_at,
    ended_at,
    operator_session_id,
    metadata
)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING
    id,
    organization_id,
    machine_id,
    started_at,
    ended_at,
    operator_session_id,
    metadata,
    created_at
`

func (q *Queries) InsertRefillSession(ctx context.Context, arg InsertRefillSessionParams) (RefillSession, error) {
	row := q.db.QueryRow(ctx, insertRefillSession,
		arg.OrganizationID,
		arg.MachineID,
		arg.StartedAt,
		arg.EndedAt,
		arg.OperatorSessionID,
		arg.Metadata,
	)
	var r RefillSession
	err := row.Scan(
		&r.ID,
		&r.OrganizationID,
		&r.MachineID,
		&r.StartedAt,
		&r.EndedAt,
		&r.OperatorSessionID,
		&r.Metadata,
		&r.CreatedAt,
	)
	return r, err
}

type InsertMachineConfigApplicationParams struct {
	OrganizationID    uuid.UUID
	MachineID         uuid.UUID
	AppliedAt         time.Time
	ConfigRevision    int32
	ConfigPayload     []byte
	OperatorSessionID *uuid.UUID
	Metadata          []byte
}

const insertMachineConfigApplication = `-- name: InsertMachineConfigApplication :one
INSERT INTO machine_configs (
    organization_id,
    machine_id,
    applied_at,
    config_revision,
    config_payload,
    operator_session_id,
    metadata
)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING
    id,
    organization_id,
    machine_id,
    applied_at,
    config_revision,
    config_payload,
    operator_session_id,
    metadata,
    created_at
`

func (q *Queries) InsertMachineConfigApplication(ctx context.Context, arg InsertMachineConfigApplicationParams) (MachineConfig, error) {
	row := q.db.QueryRow(ctx, insertMachineConfigApplication,
		arg.OrganizationID,
		arg.MachineID,
		arg.AppliedAt,
		arg.ConfigRevision,
		arg.ConfigPayload,
		arg.OperatorSessionID,
		arg.Metadata,
	)
	var m MachineConfig
	err := row.Scan(
		&m.ID,
		&m.OrganizationID,
		&m.MachineID,
		&m.AppliedAt,
		&m.ConfigRevision,
		&m.ConfigPayload,
		&m.OperatorSessionID,
		&m.Metadata,
		&m.CreatedAt,
	)
	return m, err
}

type InsertIncidentParams struct {
	OrganizationID    uuid.UUID
	MachineID         uuid.UUID
	Status            string
	Title             string
	OpenedAt          time.Time
	UpdatedAt         time.Time
	OperatorSessionID *uuid.UUID
	Metadata          []byte
}

const insertIncident = `-- name: InsertIncident :one
INSERT INTO incidents (
    organization_id,
    machine_id,
    status,
    title,
    opened_at,
    updated_at,
    operator_session_id,
    metadata
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING
    id,
    organization_id,
    machine_id,
    status,
    title,
    opened_at,
    updated_at,
    operator_session_id,
    metadata
`

func (q *Queries) InsertIncident(ctx context.Context, arg InsertIncidentParams) (Incident, error) {
	row := q.db.QueryRow(ctx, insertIncident,
		arg.OrganizationID,
		arg.MachineID,
		arg.Status,
		arg.Title,
		arg.OpenedAt,
		arg.UpdatedAt,
		arg.OperatorSessionID,
		arg.Metadata,
	)
	var i Incident
	err := row.Scan(
		&i.ID,
		&i.OrganizationID,
		&i.MachineID,
		&i.Status,
		&i.Title,
		&i.OpenedAt,
		&i.UpdatedAt,
		&i.OperatorSessionID,
		&i.Metadata,
	)
	return i, err
}

type UpdateIncidentFromOperatorParams struct {
	ID                uuid.UUID
	Status            string
	Title             string
	Metadata          []byte
	OperatorSessionID *uuid.UUID
	OrganizationID    uuid.UUID
}

const updateIncidentFromOperator = `-- name: UpdateIncidentFromOperator :one
UPDATE incidents
SET
    status = $2,
    title = $3,
    metadata = $4,
    operator_session_id = $5,
    updated_at = now()
WHERE
    id = $1
    AND organization_id = $6
RETURNING
    id,
    organization_id,
    machine_id,
    status,
    title,
    opened_at,
    updated_at,
    operator_session_id,
    metadata
`

func (q *Queries) UpdateIncidentFromOperator(ctx context.Context, arg UpdateIncidentFromOperatorParams) (Incident, error) {
	row := q.db.QueryRow(ctx, updateIncidentFromOperator,
		arg.ID,
		arg.Status,
		arg.Title,
		arg.Metadata,
		arg.OperatorSessionID,
		arg.OrganizationID,
	)
	var i Incident
	err := row.Scan(
		&i.ID,
		&i.OrganizationID,
		&i.MachineID,
		&i.Status,
		&i.Title,
		&i.OpenedAt,
		&i.UpdatedAt,
		&i.OperatorSessionID,
		&i.Metadata,
	)
	return i, err
}
