package api

import (
	"context"
	"errors"

	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrMachineShadowNotFound indicates there is no machine_shadow row for the machine yet.
var ErrMachineShadowNotFound = errors.New("api: machine shadow not found")

// SQLMachineShadow reads machine_shadow rows via sqlc (no placeholder shadow documents).
type SQLMachineShadow struct {
	pool *pgxpool.Pool
}

// NewSQLMachineShadow returns a MachineShadowService backed by Postgres.
func NewSQLMachineShadow(pool *pgxpool.Pool) *SQLMachineShadow {
	if pool == nil {
		panic("api.NewSQLMachineShadow: nil pool")
	}
	return &SQLMachineShadow{pool: pool}
}

func (s *SQLMachineShadow) GetShadow(ctx context.Context, machineID uuid.UUID) (*ShadowView, error) {
	row, err := db.New(s.pool).GetMachineShadowByMachineID(ctx, machineID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrMachineShadowNotFound
		}
		return nil, err
	}
	desired, err := marshalJSONRawObject(row.DesiredState)
	if err != nil {
		return nil, err
	}
	reported, err := marshalJSONRawObject(row.ReportedState)
	if err != nil {
		return nil, err
	}
	return &ShadowView{
		MachineID:     row.MachineID,
		DesiredState:  desired,
		ReportedState: reported,
		Version:       row.Version,
	}, nil
}
