package db

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type InsertCashCollectionParams struct {
	OrganizationID    uuid.UUID
	MachineID         uuid.UUID
	CollectedAt       time.Time
	AmountMinor       int64
	Currency          string
	Metadata          []byte
	OperatorSessionID *uuid.UUID
}

const insertCashCollection = `-- name: InsertCashCollection :one
INSERT INTO cash_collections (
    organization_id,
    machine_id,
    collected_at,
    amount_minor,
    currency,
    metadata,
    operator_session_id
)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING
    id,
    organization_id,
    machine_id,
    collected_at,
    amount_minor,
    currency,
    metadata,
    reconciliation_status,
    reconciled_by,
    reconciled_at,
    created_at,
    operator_session_id
`

func (q *Queries) InsertCashCollection(ctx context.Context, arg InsertCashCollectionParams) (CashCollection, error) {
	row := q.db.QueryRow(ctx, insertCashCollection,
		arg.OrganizationID,
		arg.MachineID,
		arg.CollectedAt,
		arg.AmountMinor,
		arg.Currency,
		arg.Metadata,
		arg.OperatorSessionID,
	)
	var c CashCollection
	err := row.Scan(
		&c.ID,
		&c.OrganizationID,
		&c.MachineID,
		&c.CollectedAt,
		&c.AmountMinor,
		&c.Currency,
		&c.Metadata,
		&c.ReconciliationStatus,
		&c.ReconciledBy,
		&c.ReconciledAt,
		&c.CreatedAt,
		&c.OperatorSessionID,
	)
	return c, err
}
