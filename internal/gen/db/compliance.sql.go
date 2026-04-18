package db

import (
	"context"

	"github.com/google/uuid"
)

type InsertAuditLogParams struct {
	OrganizationID uuid.UUID
	ActorType      string
	ActorID        string
	Action         string
	ResourceType   string
	ResourceID     *uuid.UUID
	Payload        []byte
	Ip             *string
}

const insertAuditLog = `-- name: InsertAuditLog :one
INSERT INTO audit_logs (
    organization_id,
    actor_type,
    actor_id,
    action,
    resource_type,
    resource_id,
    payload,
    ip
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id, organization_id, actor_type, actor_id, action, resource_type, resource_id, payload, ip, created_at
`

func (q *Queries) InsertAuditLog(ctx context.Context, arg InsertAuditLogParams) (AuditLog, error) {
	row := q.db.QueryRow(ctx, insertAuditLog,
		arg.OrganizationID,
		arg.ActorType,
		arg.ActorID,
		arg.Action,
		arg.ResourceType,
		arg.ResourceID,
		arg.Payload,
		arg.Ip,
	)
	var a AuditLog
	err := row.Scan(
		&a.ID,
		&a.OrganizationID,
		&a.ActorType,
		&a.ActorID,
		&a.Action,
		&a.ResourceType,
		&a.ResourceID,
		&a.Payload,
		&a.Ip,
		&a.CreatedAt,
	)
	return a, err
}

const listAuditLogsForOrganization = `-- name: ListAuditLogsForOrganization :many
SELECT
    id,
    organization_id,
    actor_type,
    actor_id,
    action,
    resource_type,
    resource_id,
    payload,
    ip,
    created_at
FROM audit_logs
WHERE
    organization_id = $1
ORDER BY
    created_at DESC
LIMIT $2
`

type ListAuditLogsForOrganizationParams struct {
	OrganizationID uuid.UUID `json:"organization_id"`
	Limit          int32     `json:"limit"`
}

func (q *Queries) ListAuditLogsForOrganization(ctx context.Context, arg ListAuditLogsForOrganizationParams) ([]AuditLog, error) {
	rows, err := q.db.Query(ctx, listAuditLogsForOrganization, arg.OrganizationID, arg.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AuditLog
	for rows.Next() {
		var a AuditLog
		if err := rows.Scan(
			&a.ID,
			&a.OrganizationID,
			&a.ActorType,
			&a.ActorID,
			&a.Action,
			&a.ResourceType,
			&a.ResourceID,
			&a.Payload,
			&a.Ip,
			&a.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

const listAuditLogsForActorInOrganization = `-- name: ListAuditLogsForActorInOrganization :many
SELECT
    id,
    organization_id,
    actor_type,
    actor_id,
    action,
    resource_type,
    resource_id,
    payload,
    ip,
    created_at
FROM audit_logs
WHERE
    organization_id = $1
    AND actor_type = $2
    AND actor_id = $3
ORDER BY
    created_at DESC
LIMIT $4
`

type ListAuditLogsForActorInOrganizationParams struct {
	OrganizationID uuid.UUID `json:"organization_id"`
	ActorType      string    `json:"actor_type"`
	ActorID        string    `json:"actor_id"`
	Limit          int32     `json:"limit"`
}

func (q *Queries) ListAuditLogsForActorInOrganization(ctx context.Context, arg ListAuditLogsForActorInOrganizationParams) ([]AuditLog, error) {
	rows, err := q.db.Query(ctx, listAuditLogsForActorInOrganization,
		arg.OrganizationID,
		arg.ActorType,
		arg.ActorID,
		arg.Limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AuditLog
	for rows.Next() {
		var a AuditLog
		if err := rows.Scan(
			&a.ID,
			&a.OrganizationID,
			&a.ActorType,
			&a.ActorID,
			&a.Action,
			&a.ResourceType,
			&a.ResourceID,
			&a.Payload,
			&a.Ip,
			&a.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
